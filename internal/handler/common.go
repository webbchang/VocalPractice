package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var ErrInvalidToken = errors.New("invalid token")
var ErrForbidden = errors.New("forbidden")

type errorResponse struct {
	Error string `json:"error"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, errorResponse{Error: msg})
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
