package sso

import "net/http"

const CookieName = "__Host-sso_token"

func GenerateCookie(sessionID string) http.Cookie {
	return http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		MaxAge:   sessionMaxAge,
		SameSite: http.SameSiteStrictMode,
	}
}

func GenerateExpiredCookie() http.Cookie {
	return http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	}
}
