package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

func handleValidationError(w http.ResponseWriter, err error) {
	var validErr *validation.ValidError
	if errors.As(err, &validErr) {
		httpjson.ResponseError(w, aip.StatusInvalidArgument, validErr.Msg)
		return
	}

	log.Printf("Unexpected error: %v", err)
	httpjson.ResponseError(w, aip.StatusInternal, http.StatusText(aip.StatusInternal.HTTPCode()))
}
