package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

func PanicRecovery(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				httpjson.ResponseStatusError(
					w,
					aip.StatusInternal,
					http.StatusText(aip.StatusInternal.HTTPCode()),
					logger.Error(),
				)
				logger.ErrorCtx(r.Context(), "panic recovered",
					"panic", err,
					"stack", string(debug.Stack()),
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
