package sso

import "net/http"

const SSOCookieName = "__Host-sso_token"

func GenerateCookie(cookieName, sessionID string) http.Cookie {
	return http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		MaxAge:   sessionMaxAge,
		SameSite: http.SameSiteStrictMode,
	}
}

func GenerateExpiredCookie(cookieName string) http.Cookie {
	return http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	}
}
