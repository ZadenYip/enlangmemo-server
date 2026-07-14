package oauth

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

// PKCE
type AuthorizationInfo struct {
	responseType string
	clientID     string
	redirectURI  string
	state        string

	codeChallenge       string
	codeChallengeMethod string

	userID string
}

type authorizeRequest struct {
	AuthorizationInfo
	validation.Validator
}

type authorizeResponse struct {
	authCode    string
	redirectURI string
	state       string
}

func (h *OAuthHandler) authorize(w http.ResponseWriter, r *http.Request) {
	var info = h.infoFromRequest(r)
	authorizeReq := authorizeRequest{
		AuthorizationInfo: info,
		Validator:         *validation.NewValidator(),
	}

	userID, loggedIn := h.checkUserLoggedIn(r)
	if loggedIn {
		authorizeReq.userID = userID
		return
	} else {
		redirectToLogin(w, r)
	}

	if h.isInValidRequest(w, r, &authorizeReq) {
		return
	}

	authCode, err := h.oaStore.GenCodeStoreSession(r.Context(), info)
	if err != nil {
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}

	authorizeResponse := authorizeResponse{
		authCode:    authCode,
		redirectURI: info.redirectURI,
		state:       info.state,
	}

	h.responseAuthCode(w, r, &authorizeResponse)
}

// infoFromRequest 从请求中提取授权请求信息
func (h *OAuthHandler) infoFromRequest(r *http.Request) AuthorizationInfo {
	return AuthorizationInfo{
		responseType:        r.URL.Query().Get("response_type"),
		clientID:            r.URL.Query().Get("client_id"),
		redirectURI:         r.URL.Query().Get("redirect_uri"),
		state:               r.URL.Query().Get("state"),
		codeChallenge:       r.URL.Query().Get("code_challenge"),
		codeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
	}
}

// checkUserLoggedIn 检查用户是否已登录，如果已登录则返回 token，否则返回空字符串
func (h *OAuthHandler) checkUserLoggedIn(r *http.Request) (string, bool) {
	ssoCookie, err := r.Cookie(sso.SSOCookieName)
	switch {
	case errors.Is(err, http.ErrNoCookie):
		return "", false
	case err != nil:
		return "", false
	}
	checked, err := h.ssoStore.GetUserID(r.Context(), ssoCookie.Value)
	switch {
	case errors.Is(err, redis.Nil):
		return "", false
	case err != nil:
		h.log.ErrorCtx(r.Context(), "failed to get userID from SSO store", "err", err)
		return "", false
	}

	return checked, true
}

// redirectToLogin 重定向到登录页面，并携带原始请求的授权信息
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse("/login")
	if err != nil {
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", nil)
		return
	}

	returnTo := r.URL.RequestURI()
	loginURL, _ := url.Parse("/login")
	query := loginURL.Query()

	// 登录成功后重定向回原始请求的授权页面
	query.Set("return_to", r.URL.RequestURI())
	setParams(query, "return_to", returnTo)

	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// isInValidRequest 验证请求参数是否有效，如果无效则直接响应错误并返回 false
func (h *OAuthHandler) isInValidRequest(w http.ResponseWriter, r *http.Request, req *authorizeRequest) bool {
	req.CheckField(req.clientID != "", "client_id", "client_id is required")
	req.CheckField(req.redirectURI != "", "redirect_uri", "redirect_uri is required")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid request parameters", "clientID", req.clientID, "redirectURI", req.redirectURI, "state", req.state)
		h.responseValidErrInJson(w, req)
		return true
	}

	clientConfig, err := h.oaStore.GetClientInfo(r.Context(), req.clientID)
	switch {
	case errors.Is(err, errOAClientNotFound):
		// 此 client_id 没注册
		h.log.InfoCtx(r.Context(), "invalid client_id", "clientID", req.clientID)
		req.AddFieldError("client_id", "Invalid client_id")
		h.responseValidErrInJson(w, req)
		return true
	case err != nil:
		// 其他错误
		h.log.ErrorCtx(r.Context(), "failed to get oauth client info", "clientID", req.clientID, "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return true
	}

	// 验证 redirect_uri 是否与注册的 client 的 redirect_uri 一致
	// 注意这里得先验证 URI 是不是相同，不相同返回 JSON 不是重定向，不然会重定向到不安全的 URI
	req.CheckField(req.redirectURI == clientConfig.RedirectURI, "redirect_uri", "Invalid redirect_uri")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid redirect_uri", "redirectURI", req.redirectURI)
		h.responseValidErrInJson(w, req)
		return true
	}

	// 下面得用重定向的 URI 查询组件重定向
	// 验证 response_type 是否符合 PKCE 要求的 "code"
	errorRedirect := OAErrorRedirect{
		errorCode:   authorInvalidRequest,
		state:       req.state,
		redirectURI: clientConfig.RedirectURI,
	}

	// 验证 response_type 是否为 "code"
	req.CheckField(req.responseType == "code", "response_type", "response_type must be 'code'")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid response_type", "responseType", req.responseType)
		errorRedirect.errorDescription = "response_type must be 'code'"
		h.redirectWithErr(w, r, errorRedirect)
		return true
	}

	// 强制要求 state 参数必须存在（协议安全要求：防止 CSRF 攻击）
	req.CheckField(req.state != "", "state", "state is required")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid state", "state", req.state)
		errorRedirect.errorDescription = "state is required"
		h.redirectWithErr(w, r, errorRedirect)
		return true
	}

	// 验证 code_challenge 是否存在
	req.CheckField(req.codeChallenge != "", "code_challenge", "code_challenge is required")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid code_challenge", "code_challenge", req.codeChallenge)
		errorRedirect.errorDescription = "code_challenge is required"
		h.redirectWithErr(w, r, errorRedirect)
		return true
	}

	// 验证 code_challenge_method 是否存在且为 S256（这里强制要求 S256，虽然协议允许 plain）
	req.CheckField(req.codeChallengeMethod == "S256", "code_challenge_method", "code_challenge_method must be 'S256'")
	if !req.Valid() {
		h.log.InfoCtx(r.Context(), "invalid code_challenge_method", "code_challenge_method", req.codeChallengeMethod)
		errorRedirect.errorDescription = "code_challenge_method must be 'S256'"
		h.redirectWithErr(w, r, errorRedirect)
		return true
	}

	return false
}

// responseAuthCode 将授权码通过重定向返回给客户端
func (h *OAuthHandler) responseAuthCode(w http.ResponseWriter, r *http.Request, resp *authorizeResponse) {
	u, err := url.Parse(resp.redirectURI)
	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to parse redirect_uri", "redirectURI", resp.redirectURI, "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}
	values := u.Query()
	setParams(values, "code", resp.authCode)
	setParams(values, "state", resp.state)

	u.RawQuery = values.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// responseValidErrInJson 将验证错误以 JSON 格式返回给客户端
func (h *OAuthHandler) responseValidErrInJson(w http.ResponseWriter, req *authorizeRequest) {
	httpjson.ResponseError(
		w,
		aip.NewErrResponse().
			WithCodeAndStatus(aip.StatusInvalidArgument).
			WithMessage("Invalid request parameters").
			WithBadRequestDetail(req.Detail()),
		h.log.Error(),
	)
}

type OAErrorRedirect struct {
	errorCode        OAAuthorErr
	state            string
	redirectURI      string
	errorDescription string
}

// redirectWithErr 将错误信息通过重定向返回给客户端
func (h *OAuthHandler) redirectWithErr(w http.ResponseWriter, r *http.Request, errorDirect OAErrorRedirect) {
	u, err := url.Parse(errorDirect.redirectURI)
	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to parse redirect_uri", "redirectURI", errorDirect.redirectURI, "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}
	values := u.Query()
	setParams(values, "error", string(errorDirect.errorCode))
	setParams(values, "state", errorDirect.state)
	setParams(values, "error_description", errorDirect.errorDescription)

	u.RawQuery = values.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// 设置 URL 查询参数的 helper 函数
// 如果 value 为空则不设置
func setParams(u url.Values, key string, value string) {
	if value == "" {
		return
	}
	u.Set(key, value)
}
