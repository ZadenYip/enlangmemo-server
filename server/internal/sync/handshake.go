package sync

import (
	"context"
	"time"

	"connectrpc.com/connect"
	srvSession "github.com/zadenyip/enlangmemo-server/internal/server/session"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	"github.com/zadenyip/enlangmemo-server/internal/utils"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Handshake(
	ctx context.Context,
	r *connect.Request[syncv1.HandshakeRequest],
) (*connect.Response[syncv1.HandshakeResponse], error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		// 不应该出现这个状况，因为 AuthInterceptor 已经放入 userID 进 context 了
		h.logger.ErrorCtx(ctx, "invalid userID after AuthInterceptor in Handshake", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	h.logger.InfoCtx(ctx, "handshake request received",
		"userID", userID,
		"clientCollectionID", r.Msg.CollectionId,
		"clientSyncCursorUsn", r.Msg.ClientSyncCursorUsn,
		"clientLastSyncTime", r.Msg.ClientLastSyncTime,
		"clientNow", r.Msg.ClientNow,
		"hasLocalChanges", r.Msg.HasLocalChanges,
		"deviceID", r.Msg.DeviceId,
		"deviceName", r.Msg.DeviceName,
		"protocolVersion", r.Msg.ProtocolVersion,
	)

	colInfo, err := h.hskStore.GetColInfoForHandshake(ctx, userID)

	h.logger.InfoCtx(ctx, "handshake collection info retrieved",
		"userID", userID,
		"collectionID", colInfo.CollectionID,
		"syncCursorUSN", colInfo.SyncCursorUSN,
		"lastSyncTime", colInfo.LastSyncTime,
	)

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	if colInfo.CollectionID != "" && colInfo.CollectionID != r.Msg.CollectionId {
		h.logger.ErrorCtx(ctx, "handshake collection id mismatch", "userID", userID, "clientCollectionID", r.Msg.CollectionId, "serverCollectionID", colInfo.CollectionID)
		return nil, connect.NewError(connect.CodeFailedPrecondition, nil)
	}

	return h.determineHandshake(ctx, userID, colInfo, r.Msg)
}

// hskSession 将握手请求转换为 SyncSession
//
// 注意这里的服务器的 sync cursor usn 还没拿到
// 这里的 SessionID 是新生成的
func (h *SyncHandler) hskSession(ctx context.Context,
	req *syncv1.HandshakeRequest,
	userID int64,
) (ss.SyncSession, error) {
	sessionID, err := srvSession.NewID(16)
	if err != nil {
		h.logger.ErrorCtx(ctx, "failed to create session from request in SyncHandshake", "error", err)
		return ss.SyncSession{}, err
	}
	return ss.SyncSession{
		UserID:                      userID,
		State:                       ss.SyncSessionStateUnspecified,
		ExpectedBatchSeq:            0,
		SyncCursorUSN:               0,
		SessionID:                   sessionID,
		CliSyncCursorUSNAtHandshake: req.ClientSyncCursorUsn,
		SrvSyncCursorUSNAtHandshake: 0,
		DeviceID:                    req.DeviceId,
	}, nil
}

// 根据客户端集合状态和服务器集合状态，判断握手的状态
func (h *SyncHandler) determineHandshake(ctx context.Context, userID int64, colInfo CollectionInfoForHandshake, req *syncv1.HandshakeRequest) (*connect.Response[syncv1.HandshakeResponse], error) {

	var resp = &syncv1.HandshakeResponse{
		SessionId:           nil,
		ServerSyncCursorUsn: colInfo.SyncCursorUSN,
		ServerLastSyncTime:  colInfo.LastSyncTime,
		Status:              syncv1.HandshakeStatus_HANDSHAKE_STATUS_UNSPECIFIED,
	}

	switch {
	// case req.ProtocolVersion < 同步协议版本:
	//    hskStatus = syncv1.HandshakeStatus_HANDSHAKE_STATUS_CLIENT_TOO_OLD
	// case req.ProtocolVersion > 同步协议版本:
	//    hskStatus = syncv1.HandshakeStatus_HANDSHAKE_STATUS_SERVER_TOO_OLD
	// case req.ClientLastSyncTime <= 运维正式清理了 deleted 标记的同步数据:
	// 	hskStatus = syncv1.HandshakeStatus_HANDSHAKE_STATUS_CLIENT_DATA_TOO_OLD

	// 客户端和服务器时间差超过 5 分钟，两者之一时间不准确
	case utils.Abs(req.ClientNow-time.Now().UnixMilli()) >= 5*60*1000:
		resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_TIME_SKEW_TOO_LARGE
		return connect.NewResponse(resp), nil
	}

	// hskSession 根据情况设置 State、ExpectedBatchSeq、
	// SyncCursorUSN、
	// SrvSyncCursorUSNAtHandshake（下面这里已经接着设置了）
	hskSession, err := h.hskSession(ctx, req, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	// 将服务器的 sync cursor usn 放入握手 session
	hskSession.SrvSyncCursorUSNAtHandshake = colInfo.SyncCursorUSN
	resp.SessionId = &hskSession.SessionID

	switch {
	// 客户端已经有服务器所有数据，不需要拉取服务器的更新
	case req.ClientSyncCursorUsn == colInfo.SyncCursorUSN:
		resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES
		if req.HasLocalChanges {
			hskSession.State = ss.SyncSessionStatePushing
			hskSession.SyncCursorUSN = colInfo.SyncCursorUSN
			hskSession.ExpectedBatchSeq = 1
		} else {
			resp.SessionId = nil
			resp.ServerSyncCursorUsn = colInfo.SyncCursorUSN
			resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES
			return connect.NewResponse(resp), nil
		}

	// 需要拉取服务器的更新
	case req.ClientSyncCursorUsn < colInfo.SyncCursorUSN:
		resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_NEED_PULL
		hskSession.State = ss.SyncSessionStatePulling
		hskSession.ExpectedBatchSeq = 1
		hskSession.SyncCursorUSN = req.ClientSyncCursorUsn

	// 需要上传客户端所有数据
	case req.ClientSyncCursorUsn > colInfo.SyncCursorUSN:
		resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_UPLOAD_ALL
		hskSession.State = ss.SyncSessionStateAwaitingUploadAllConfirm
		hskSession.SyncCursorUSN = 0
		hskSession.ExpectedBatchSeq = 1

	// 前面 case 已经覆盖了所有情况，理论上不应该走到 default
	default:
		h.logger.ErrorCtx(ctx, "unexpected handshake case", "clientSyncCursorUsn", req.ClientSyncCursorUsn, "serverSyncCursorUsn", colInfo.SyncCursorUSN)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	result, err := h.sessionStore.CreateSession(ctx, hskSession)
	switch {
	case result == ss.CreateSessionAlreadyExists:
		resp.Status = syncv1.HandshakeStatus_HANDSHAKE_STATUS_LOCKED_BY_OTHER_CLIENT
		return connect.NewResponse(resp), nil
	case result == ss.CreateSessionCreated:
		h.logger.InfoCtx(ctx, "handshake session created", "userID", userID, "sessionID", hskSession.SessionID)
		return connect.NewResponse(resp), nil
	default:
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
}
