package validation

import (
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func HandleValidationError(w http.ResponseWriter, validator *Validator) {
	if validator == nil {
		log.Printf("Unexpected validation error")
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())),
		)
		return
	}

	httpjson.ResponseError(w,
		aip.NewErrResponse().
			WithCodeAndStatus(aip.StatusInvalidArgument).
			WithMessage(validator.FailMsg).
			WithBadRequestDetail(validator.Detail()),
	)
}
