package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
	"github.com/Below-Z3R0/sonus/internal/config"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9101", "listen address")
	flag.Parse()

	cfg, err := config.Load(config.FilePath())
	if err != nil {
		log.Fatalf("aurisd: %v", err)
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.ASR); ok {
		model = m
	}

	mux := api.Handler("auris", version, model.ID, catalog.List())
	mux.HandleFunc("/v1/transcribe", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "transcription engine not wired yet", http.StatusNotImplemented)
	})

	log.Printf("aurisd %s on %s (model %s)", version, *addr, model.ID)
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
