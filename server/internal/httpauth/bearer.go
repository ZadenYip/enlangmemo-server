package httpauth

import (
	"net/http"
	"strings"
)

// BearerToken 提取 HTTP 请求中的 Bearer Token
func BearerToken(h http.Header) (string, bool) {
	const scheme = "Bearer"

	fields := strings.Fields(h.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], scheme) {
		return "", false
	}

	return fields[1], true
}
