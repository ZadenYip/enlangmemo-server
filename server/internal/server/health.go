package server

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

type healthResponse struct {
	Status string `json:"status"`
}

func health(w http.ResponseWriter, r *http.Request) {
	httpjson.ResponseJSON(w, http.StatusOK, healthResponse{Status: "ok"}, nil)
}
