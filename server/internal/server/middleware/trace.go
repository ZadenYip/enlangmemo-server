package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

const TraceHeader = "traceparent"

func Trace(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := r.Header.Get(TraceHeader)
		if trace == "" {
			logger.Error().WarnContext(r.Context(), "traceparent header is missing in request", "url", r.URL.String())
			trace = uuid.New().String()
			r.Header.Set(TraceHeader, trace)
		}

		ctx := context.WithValue(r.Context(), logging.TraceKey{}, trace)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
