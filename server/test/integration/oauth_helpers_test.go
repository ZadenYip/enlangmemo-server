package integration

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

const (
	testOAuthRedirectURI  = "https://client.example/callback?existing=value"
	testOAuthState        = "state-value"
	testOAuthCodeVerifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
)

// codeChallengeFromVerifier 会返回 codeVerfier 对应的 codeChallenge
func codeChallengeFromVerifier(codeVerifier string) string {
	shaCode := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(shaCode[:])
}

// registerOAuthClient 注册一个 OAuth 客户端，并返回 client_id
func registerOAuthClient(t *testing.T, redirectURI string) string {
	t.Helper()

	var clientID string
	err := env.dbPool.QueryRow(
		t.Context(),
		`INSERT INTO oauth_clients (name, redirect_uri)
		 VALUES ($1, $2)
		 RETURNING id::text`,
		"integration test client",
		redirectURI,
	).Scan(&clientID)
	require.NoError(t, err)

	return clientID
}

// loginAndRegisterForAuthorizePKCE 会注册一个用户，并登录，返回登录后的 SSO Cookie
func loginAndRegisterForAuthorizePKCE(t *testing.T, loginID string) *http.Cookie {
	t.Helper()

	registerUserForLogin(t, loginID, "testpassword")
	resp := doLogin(t, marshalLoginRequest(t, auth.LoginRequest{
		LoginID:  loginID,
		Password: "testpassword",
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, resp.Cookies(), 1)
	require.Equal(t, sso.SSOCookieName, resp.Cookies()[0].Name)

	return resp.Cookies()[0]
}

// newAuthorizePKCERequest 会创建一个 OAuth 授权请求
func newAuthorizePKCERequest(t *testing.T, clientID, redirectURI string, ssoCookie *http.Cookie, change func(url.Values)) *http.Request {
	t.Helper()

	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {testOAuthState},
		"code_challenge":        {codeChallengeFromVerifier(testOAuthCodeVerifier)},
		"code_challenge_method": {"S256"},
	}
	if change != nil {
		change(query)
	}

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		testServer.URL+"/v1/oauth/authorize?"+query.Encode(),
		nil,
	)
	require.NoError(t, err)
	if ssoCookie != nil {
		req.AddCookie(ssoCookie)
	}

	return req
}

// doAuthorizePKCE 发送授权请求并返回第一跳响应。
// OAuth authorize 端点的主要结果是重定向，这里禁用自动跟随重定向，
// 方便测试直接断言 Location header。
//
// req - 写好的授权请求
func doAuthorizePKCE(t *testing.T, req *http.Request) *http.Response {
	t.Helper()

	client := *testClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

// authorizePKCE 会执行一个完整的 OAuth 授权请求（包含注册用户、OAuth 客户端本身的注册、登录、授权请求），并返回 client_id 和授权码
func authorizePKCE(t *testing.T, loginID, redirectURI string, change func(url.Values)) (clientID string, authCode string) {
	t.Helper()

	clientID = registerOAuthClient(t, redirectURI)
	ssoCookie := loginAndRegisterForAuthorizePKCE(t, loginID)
	resp := doAuthorizePKCE(t, newAuthorizePKCERequest(t, clientID, redirectURI, ssoCookie, change))

	require.Equal(t, http.StatusFound, resp.StatusCode)
	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, testOAuthState, location.Query().Get("state"))

	authCode = location.Query().Get("code")
	require.NotEmpty(t, authCode)

	return clientID, authCode
}
