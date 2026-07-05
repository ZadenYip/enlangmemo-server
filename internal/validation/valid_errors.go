package validation

import (
	"errors"
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func HandleValidationError(w http.ResponseWriter, err error) {
	var validErr *ValidError
	if errors.As(err, &validErr) {
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInvalidArgument).
				WithMessage(validErr.Msg))
		return
	}

	log.Printf("Unexpected error: %v", err)
	httpjson.ResponseError(w,
		aip.NewErrResponse().
			WithCodeAndStatus(aip.StatusInternal).
			WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())))
}
