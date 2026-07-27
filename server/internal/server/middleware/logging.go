package middleware

import (
	"net/http"
	"time"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type wrapperWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriterHeader 记录响应状态码，以便在日志中记录
func (rr *wrapperWriter) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.InfoCtx(r.Context(), "request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		ww := &wrapperWriter{ResponseWriter: w, statusCode: 0}
		next.ServeHTTP(ww, r)

		logger.InfoCtx(r.Context(), "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"status", ww.statusCode,
			"duration", time.Since(start),
		)
	})
}
