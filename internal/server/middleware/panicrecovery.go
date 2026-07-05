package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				httpjson.ResponseError(w,
					aip.NewErrResponse().
						WithCodeAndStatus(aip.StatusInternal).
						WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())))
				log.Println(string(debug.Stack()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
