package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

func handleValidationError(w http.ResponseWriter, err error) {
	var validErr *validation.ValidError
	if errors.As(err, &validErr) {
		httpjson.ResponseError(w, http.StatusBadRequest, "INVALID_ARGUMENT", validErr.Msg)
		return
	}

	log.Printf("Unexpected error: %v", err)
	const hStatus = http.StatusInternalServerError
	httpjson.ResponseError(w, hStatus, "INTERNAL", http.StatusText(hStatus))
}
