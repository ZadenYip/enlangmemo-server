package enlangmemo

import (
	"context"
	"errors"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/httpauth"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
)

type AccessTokenStore interface {
	GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error)
}

type UserProfileStore interface {
	GetUserProfile(ctx context.Context, userID string) (auth.UserProfile, error)
}

type Handler struct {
	tokenStore AccessTokenStore
	users      UserProfileStore
	log        logging.Logger
}

type meResponse struct {
	UserID   string `json:"user_id"`
	LoginID  string `json:"login_id"`
	Nickname string `json:"nickname"`
}

func NewHandler(tokenStore AccessTokenStore, users UserProfileStore, logger logging.Logger) *Handler {
	return &Handler{
		tokenStore: tokenStore,
		users:      users,
		log:        logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/apps/enlangmemo/me", h.me)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := httpauth.BearerToken(r)
	if !ok {
		h.responseUnauthorized(w)
		return
	}

	userID, err := h.tokenStore.GetUserIDByAccessToken(r.Context(), accessToken)
	switch {
	case errors.Is(err, oauth.ErrAccessTokenNotFound):
		h.responseUnauthorized(w)
		return
	case err != nil:
		h.log.ErrorCtx(r.Context(), "failed to get current enlangmemo user", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}

	profile, err := h.users.GetUserProfile(r.Context(), userID)
	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to get current enlangmemo user profile", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}

	response := meResponse{
		UserID:   profile.UserID,
		LoginID:  profile.LoginID,
		Nickname: profile.Nickname,
	}

	httpjson.ResponseJSON(w, http.StatusOK, response, h.log.Error())
}

func (h *Handler) responseUnauthorized(w http.ResponseWriter) {
	httpjson.ResponseStatusError(w, aip.StatusUnauthenticated, "Invalid or missing access token", h.log.Error())
}
