package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func PanicRecovery(next http.Handler, errLog *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				httpjson.ResponseError(w,
					aip.NewErrResponse().
						WithCodeAndStatus(aip.StatusInternal).
						WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())),
					errLog)
				errLog.Error("panic recovered",
					"panic", err,
					"stack", string(debug.Stack()),
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
