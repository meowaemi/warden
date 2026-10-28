// Package api serves the problems of the local mode over HTTP.
package api

import (
	"encoding/json"
	"net/http"
	"warden/internal/logger"
	"warden/internal/messages"
	"warden/internal/store"
)

// messageDTO is a message with its key.
type messageDTO struct {
	messages.Message
	Key string `json:"key"`
}

// Handler serves GET /api/messages.
func Handler(st *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/messages", func(w http.ResponseWriter, r *http.Request) {
		list := st.List()
		dtos := make([]messageDTO, 0, len(list))
		for _, m := range list {
			dtos = append(dtos, messageDTO{Message: m, Key: m.Key()})
		}
		writeJSON(w, dtos)
	})
	return mux
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Log.Error("write response", "err", err)
	}
}
