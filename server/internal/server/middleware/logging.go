package middleware

import (
	"net/http"
	"time"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

func Logging(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.InfoCtx(r.Context(), "request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
		logger.InfoCtx(r.Context(), "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"duration", time.Since(start),
		)
	})
}
