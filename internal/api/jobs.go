package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Below-Z3R0/sonus/internal/jobs"
)

// RegisterJobs exposes the shared job registry and a polling SSE stream.
func RegisterJobs(mux *http.ServeMux, mgr *jobs.Manager) {
	mux.HandleFunc("/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		WriteJSON(w, mgr.List())
	})
	mux.HandleFunc("/v1/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
		id, suffix, _ := strings.Cut(rest, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		if suffix == "events" {
			streamJob(w, r, mgr, id)
			return
		}
		if suffix != "" {
			http.NotFound(w, r)
			return
		}
		job, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		WriteJSON(w, job)
	})
}

func streamJob(w http.ResponseWriter, r *http.Request, mgr *jobs.Manager, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming no soportado", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		job, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		raw, _ := json.Marshal(job)
		if string(raw) != last {
			last = string(raw)
			_, _ = w.Write([]byte("data: " + last + "\n\n"))
			flusher.Flush()
		}
		if job.State == jobs.Done || job.State == jobs.Failed {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
