package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Below-Z3R0/sonus/internal/models"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9101", "listen address")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"service": "auris", "version": version, "state": "scaffold"})
	})
	mux.HandleFunc("/v1/transcribe", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "transcription engine not wired yet", http.StatusNotImplemented)
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, models.List(models.TaskASR))
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(os.Stderr, "aurisd %s on %s\n", version, *addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
