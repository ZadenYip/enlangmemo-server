package validation

import (
	"log/slog"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func HandleValidationError(w http.ResponseWriter, validator *Validator, errLog *slog.Logger) {
	if validator == nil {
		errLog.Error("unexpected validation error")
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())),
			errLog,
		)
		return
	}

	httpjson.ResponseError(w,
		aip.NewErrResponse().
			WithCodeAndStatus(aip.StatusInvalidArgument).
			WithMessage(validator.FailMsg).
			WithBadRequestDetail(validator.Detail()),
		errLog,
	)
}
