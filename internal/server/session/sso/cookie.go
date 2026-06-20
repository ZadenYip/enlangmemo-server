package sso

import "net/http"

func GenerateCookie(sessionID string) http.Cookie {
	return http.Cookie{
		Name:     "__Host-sso_token",
		Value:    sessionID,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}
