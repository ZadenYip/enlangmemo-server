package httpauth

import (
	"net/http"
	"strings"
)

// BearerToken 提取 HTTP 请求中的 Bearer Token
func BearerToken(r *http.Request) (string, bool) {
	const scheme = "Bearer"

	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], scheme) {
		return "", false
	}

	return fields[1], true
}
