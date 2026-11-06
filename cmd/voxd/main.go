package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/meminfo"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/obs"
	"github.com/Below-Z3R0/sonus/internal/runtime"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
	voxengine "github.com/Below-Z3R0/sonus/internal/vox/engine"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9102", "listen address")
	preload := flag.Bool("preload", false, "load the default model at startup")
	flag.Parse()

	cfg, err := config.Load(config.FilePath())
	if err != nil {
		log.Fatalf("voxd: %v", err)
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.TTS); ok {
		model = m
	}

	eng := voxengine.New(cfg.Paths.Models, cfg.Services.Phonemizer)
	mgr := runtime.New("vox", version, model.ID, func(id string) error {
		if st := modeldl.Status(cfg.Paths.Models, id); st.State != "ready" {
			return fmt.Errorf("%s no está instalado", id)
		}
		if m, ok := catalog.Get(id); ok {
			if err := meminfo.Check(m.RAMMB); err != nil {
				return err
			}
		}
		return eng.Load(id)
	}, eng.Unload)
	mux := api.Handler(func() any { return mgr.Status() }, catalog.List())
	api.RegisterLifecycle(mux, mgr)
	api.RegisterMetrics(mux, obs.Snapshot)
	api.RegisterDoctor(mux, func() any {
		st := mgr.Status()
		free, _ := modeldl.FreeBytes(cfg.Paths.Models)
		ram, _ := meminfo.AvailableMB()
		ph := "ok"
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := eng.Health(ctx); err != nil {
			ph = "unreachable"
		}
		return map[string]any{
			"service": "vox", "status": st,
			"model_dir": cfg.Paths.Models, "free_mb": free >> 20, "available_ram_mb": ram,
			"phonemizer": ph,
		}
	})
	mux.HandleFunc("/v1/speech", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Text  string `json:"text"`
			Lang  string `json:"lang"`
			Voice string `json:"voice"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Lang == "" {
			req.Lang = "auto"
		}
		wav, err := eng.Speak(r.Context(), req.Text, req.Lang, req.Voice)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Length", fmt.Sprint(len(wav)))
		_, _ = w.Write(wav)
	})
	mux.HandleFunc("/v1/voices", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, model.Voices)
	})

	log.Printf("voxd %s on %s (model %s)", version, *addr, model.ID)
	if *preload {
		if err := mgr.Load(model.ID); err != nil {
			log.Printf("preload: %v", err)
		}
	}
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
