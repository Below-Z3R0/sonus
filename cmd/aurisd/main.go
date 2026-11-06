package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/runtime"
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

	mgr := runtime.New("auris", version, model.ID, func(id string) error {
		if st := modeldl.Status(cfg.Paths.Models, id); st.State != "ready" {
			return fmt.Errorf("%s no está instalado", id)
		}
		return nil
	}, func() {})
	mux := api.Handler(func() any { return mgr.Status() }, catalog.List())
	api.RegisterLifecycle(mux, mgr)
	mux.HandleFunc("/v1/transcribe", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "transcription engine not wired yet", http.StatusNotImplemented)
	})

	log.Printf("aurisd %s on %s (model %s)", version, *addr, model.ID)
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
