package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

const TraceHeader = "traceparent"

type TraceKey struct{}

func Trace(next http.Handler, errLog *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace := r.Header.Get(TraceHeader)
		if trace == "" {
			errLog.ErrorContext(r.Context(), "traceparent header is missing in request", "url", r.URL.String())
			httpjson.ResponseError(
				w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusInternal).
					WithMessage("Nginx generated traceparent header is missing in request"),
				errLog,
			)
			return
		}

		ctx := context.WithValue(r.Context(), TraceKey{}, trace)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func TraceIDFromCtx(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(TraceKey{}).(string)
	return traceID, ok
}
