package middleware

import (
	"context"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

const TraceHeader = "traceparent"

func Trace(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := r.Header.Get(TraceHeader)
		if trace == "" {
			logger.Error().ErrorContext(r.Context(), "traceparent header is missing in request", "url", r.URL.String())
			httpjson.ResponseError(
				w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusInternal).
					WithMessage("Nginx generated traceparent header is missing in request"),
				logger.Error(),
			)
			return
		}

		ctx := context.WithValue(r.Context(), logging.TraceKey{}, trace)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
