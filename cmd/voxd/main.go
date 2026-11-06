package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9102", "listen address")
	flag.Parse()

	cfg, err := config.Load(config.FilePath())
	if err != nil {
		log.Fatalf("voxd: %v", err)
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.TTS); ok {
		model = m
	}

	mux := api.Handler("vox", version, model.ID, catalog.List())
	mux.HandleFunc("/v1/speech", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "speech engine not wired yet", http.StatusNotImplemented)
	})
	mux.HandleFunc("/v1/voices", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, model.Voices)
	})

	log.Printf("voxd %s on %s (model %s)", version, *addr, model.ID)
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
