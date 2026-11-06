package api

import (
	"encoding/json"
	"net/http"

	"github.com/Below-Z3R0/sonus/internal/runtime"
)

type modelRequest struct {
	Model string `json:"model"`
}

// RegisterLifecycle exposes load/unload without stopping the HTTP service.
func RegisterLifecycle(mux *http.ServeMux, mgr *runtime.Manager) {
	mux.HandleFunc("/v1/models/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var req modelRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if err := mgr.Load(req.Model); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		WriteJSON(w, mgr.Status())
	})
	mux.HandleFunc("/v1/models/unload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
			return
		}
		mgr.Unload()
		WriteJSON(w, mgr.Status())
	})
}
