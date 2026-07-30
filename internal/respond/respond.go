package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

type Error struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type envelope struct {
	Data  any    `json:"data"`
	Error *Error `json:"error"`
	Meta  *Meta  `json:"meta,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any, err *Error, meta *Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := envelope{Data: data, Error: err, Meta: meta}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		slog.Error("respond json encode", "error", encErr)
	}
}
