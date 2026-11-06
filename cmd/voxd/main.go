package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9102", "listen address")
	flag.Parse()
	cfg, _ := config.Load(config.FilePath())
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.TTS); ok {
		model = m
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"service": "vox", "version": version, "state": "scaffold", "model": model.ID})
	})
	mux.HandleFunc("/v1/speech", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "speech engine not wired yet", http.StatusNotImplemented)
	})
	mux.HandleFunc("/v1/voices", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, model.Voices)
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, catalog.List())
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(os.Stderr, "voxd %s on %s\n", version, *addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
