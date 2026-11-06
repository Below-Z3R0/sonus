package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
	aurisengine "github.com/Below-Z3R0/sonus/internal/auris/engine"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/meminfo"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/obs"
	"github.com/Below-Z3R0/sonus/internal/runtime"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9101", "listen address")
	preload := flag.Bool("preload", false, "load the default model at startup")
	flag.Parse()

	cfg, err := config.Load(config.FilePath())
	if err != nil {
		log.Fatalf("aurisd: %v", err)
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.ASR); ok {
		model = m
	}

	eng := aurisengine.New(cfg.Paths.Models)
	mgr := runtime.New("auris", version, model.ID, func(id string) error {
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
		return map[string]any{
			"service": "auris", "status": st,
			"model_dir": cfg.Paths.Models, "free_mb": free >> 20, "available_ram_mb": ram,
		}
	})
	mux.HandleFunc("/v1/transcribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
		if err := r.ParseMultipartForm(128 << 20); err != nil {
			http.Error(w, "multipart inválido", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("audio")
		if err != nil {
			http.Error(w, "falta el campo audio", http.StatusBadRequest)
			return
		}
		defer file.Close()
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".wav"
		}
		tmp, err := os.CreateTemp(cfg.Paths.Data, "auris-*"+ext)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, err := io.Copy(tmp, file); err != nil {
			tmp.Close()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := tmp.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out, err := eng.Transcribe(r.Context(), tmpName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.WriteJSON(w, out)
	})

	log.Printf("aurisd %s on %s (model %s)", version, *addr, model.ID)
	if *preload {
		if err := mgr.Load(model.ID); err != nil {
			log.Printf("preload: %v", err)
		}
	}
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
