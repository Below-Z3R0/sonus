package main

import (
	"context"
	"encoding/json"
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
	"github.com/Below-Z3R0/sonus/internal/jobs"
	"github.com/Below-Z3R0/sonus/internal/meminfo"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/obs"
	"github.com/Below-Z3R0/sonus/internal/runtime"
)

func saveUpload(w http.ResponseWriter, r *http.Request, dir string) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		return "", fmt.Errorf("multipart inválido")
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		return "", fmt.Errorf("falta el campo audio")
	}
	defer file.Close()
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".wav"
	}
	tmp, err := os.CreateTemp(dir, "auris-*"+ext)
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

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
	jm := jobs.New(filepath.Join(cfg.Paths.Data, "jobs.json"), 50)
	api.RegisterJobs(mux, jm)
	api.RegisterDoctor(mux, func() any {
		st := mgr.Status()
		free, _ := modeldl.FreeBytes(cfg.Paths.Models)
		ram, _ := meminfo.AvailableMB()
		return map[string]any{
			"service": "auris", "status": st,
			"model_dir": cfg.Paths.Models, "free_mb": free >> 20, "available_ram_mb": ram,
		}
	})
	mux.HandleFunc("/v1/jobs/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil || req.ID == "" {
			http.Error(w, "job id requerido", http.StatusBadRequest)
			return
		}
		old, ok := jm.Get(req.ID)
		if !ok || old.Kind != "asr" {
			http.Error(w, "job asr no encontrado", http.StatusNotFound)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(old.Payload, &payload); err != nil || payload["path"] == "" {
			http.Error(w, "payload inválido", http.StatusBadRequest)
			return
		}
		path := payload["path"]
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "el audio original ya no existe", http.StatusGone)
			return
		}
		id := jm.Start("asr", payload, r.Context(), func(ctx context.Context) (any, error) {
			defer os.Remove(path)
			return eng.Transcribe(ctx, path)
		})
		w.WriteHeader(http.StatusAccepted)
		api.WriteJSON(w, map[string]string{"job_id": id})
	})
	mux.HandleFunc("/v1/transcribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		path, err := saveUpload(w, r, cfg.Paths.Data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer os.Remove(path)
		out, err := eng.Transcribe(r.Context(), path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.WriteJSON(w, out)
	})
	mux.HandleFunc("/v1/transcribe/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		path, err := saveUpload(w, r, cfg.Paths.Data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := jm.Start("asr", map[string]string{"path": path}, r.Context(), func(ctx context.Context) (any, error) {
			defer os.Remove(path)
			return eng.Transcribe(ctx, path)
		})
		w.WriteHeader(http.StatusAccepted)
		api.WriteJSON(w, map[string]string{"job_id": id})
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
