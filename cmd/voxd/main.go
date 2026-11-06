package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/jobs"
	"github.com/Below-Z3R0/sonus/internal/meminfo"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/obs"
	"github.com/Below-Z3R0/sonus/internal/runtime"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
	voxengine "github.com/Below-Z3R0/sonus/internal/vox/engine"
)

type speechRequest struct {
	Text  string `json:"text"`
	Lang  string `json:"lang"`
	Voice string `json:"voice"`
}

func writePCM(w io.Writer, samples []float32) error {
	var buf [2]byte
	for _, sample := range samples {
		v := int32(math.Round(float64(sample) * 32767))
		if v > math.MaxInt16 {
			v = math.MaxInt16
		}
		if v < math.MinInt16 {
			v = math.MinInt16
		}
		binary.LittleEndian.PutUint16(buf[:], uint16(int16(v)))
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	}
	return nil
}

func parseSpeech(w http.ResponseWriter, r *http.Request) (speechRequest, bool) {
	var req speechRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return req, false
	}
	if req.Lang == "" {
		req.Lang = "auto"
	}
	return req, true
}

func speakJob(ctx context.Context, eng *voxengine.Engine, cfg config.Config, req speechRequest) (any, error) {
	wav, err := eng.Speak(ctx, req.Text, req.Lang, req.Voice)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Paths.Output, 0o755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(cfg.Paths.Output, "tts-*.wav")
	if err != nil {
		return nil, err
	}
	name := f.Name()
	if _, err := f.Write(wav); err != nil {
		f.Close()
		os.Remove(name)
		return nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return nil, err
	}
	return map[string]any{"path": name, "bytes": len(wav)}, nil
}

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
	jm := jobs.New(filepath.Join(cfg.Paths.Data, "jobs.json"), 50)
	api.RegisterJobs(mux, jm)
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
		req, ok := parseSpeech(w, r)
		if !ok {
			return
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
	mux.HandleFunc("/v1/speech/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		req, ok := parseSpeech(w, r)
		if !ok {
			return
		}
		id := jm.Start("tts", req, r.Context(), func(ctx context.Context) (any, error) {
			return speakJob(ctx, eng, cfg, req)
		})
		w.WriteHeader(http.StatusAccepted)
		api.WriteJSON(w, map[string]string{"job_id": id})
	})
	mux.HandleFunc("/v1/speech/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		req, ok := parseSpeech(w, r)
		if !ok {
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming no soportado", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "audio/L16")
		w.Header().Set("X-Audio-Format", "pcm_s16le")
		w.Header().Set("X-Sample-Rate", "24000")
		w.Header().Set("Cache-Control", "no-cache")
		err := eng.Stream(r.Context(), req.Text, req.Lang, req.Voice, func(samples []float32, _ int) error {
			if err := writePCM(w, samples); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})
		if err != nil {
			log.Printf("speech stream: %v", err)
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
		if !ok || old.Kind != "tts" {
			http.Error(w, "job tts no encontrado", http.StatusNotFound)
			return
		}
		var sp speechRequest
		if err := json.Unmarshal(old.Payload, &sp); err != nil {
			http.Error(w, "payload inválido", http.StatusBadRequest)
			return
		}
		id := jm.Start("tts", sp, r.Context(), func(ctx context.Context) (any, error) {
			return speakJob(ctx, eng, cfg, sp)
		})
		w.WriteHeader(http.StatusAccepted)
		api.WriteJSON(w, map[string]string{"job_id": id})
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
