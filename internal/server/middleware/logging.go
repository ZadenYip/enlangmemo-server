package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logging(next http.Handler, infoLog *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		infoLog.Info("request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"traceparent", r.Header.Get(TraceHeader),
		)
		next.ServeHTTP(w, r)
		infoLog.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"duration", time.Since(start),
			"traceparent", r.Header.Get(TraceHeader),
		)
	})
}
