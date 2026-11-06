// Package aurisengine wires Whisper ONNX to the Auris lifecycle.
package aurisengine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"

	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
)

type Engine struct {
	root  string
	mu    sync.Mutex
	rec   *sherpa.OfflineRecognizer
	model catalog.Model
}

func New(modelsRoot string) *Engine {
	return &Engine{root: modelsRoot}
}

type Result struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

func (e *Engine) Load(id string) error {
	m, ok := catalog.Get(id)
	if !ok {
		return fmt.Errorf("modelo desconocido: %s", id)
	}
	dir, err := findModelDir(e.root, id)
	if err != nil {
		return err
	}
	cfg := sherpa.OfflineRecognizerConfig{}
	cfg.FeatConfig.SampleRate = 16000
	cfg.FeatConfig.FeatureDim = 80
	cfg.ModelConfig.Whisper.Encoder = firstFile(dir, "*encoder*.onnx")
	cfg.ModelConfig.Whisper.Decoder = firstFile(dir, "*decoder*.onnx")
	cfg.ModelConfig.Whisper.Task = "transcribe"
	cfg.ModelConfig.Whisper.EnableTokenTimestamps = 1
	cfg.ModelConfig.Tokens = firstFile(dir, "*tokens.txt")
	if cfg.ModelConfig.Whisper.Encoder == "" || cfg.ModelConfig.Whisper.Decoder == "" || cfg.ModelConfig.Tokens == "" {
		return fmt.Errorf("faltan encoder/decoder/tokens en %s", dir)
	}
	cfg.ModelConfig.NumThreads = 4
	cfg.ModelConfig.Provider = "cpu"
	cfg.DecodingMethod = "greedy_search"

	rec := sherpa.NewOfflineRecognizer(&cfg)
	if rec == nil {
		return fmt.Errorf("Whisper no pudo abrir %s", dir)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec != nil {
		sherpa.DeleteOfflineRecognizer(e.rec)
	}
	e.rec = rec
	e.model = m
	return nil
}

func (e *Engine) Unload() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec != nil {
		sherpa.DeleteOfflineRecognizer(e.rec)
		e.rec = nil
	}
}

// Transcribe reads a WAV file and returns Whisper's text.
func (e *Engine) Transcribe(ctx context.Context, path string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec == nil {
		return Result{}, errors.New("modelo no cargado")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	wave := sherpa.ReadWave(path)
	if wave == nil || len(wave.Samples) == 0 {
		return Result{}, fmt.Errorf("audio inválido o formato no soportado: %s", filepath.Base(path))
	}
	stream := sherpa.NewOfflineStream(e.rec)
	defer sherpa.DeleteOfflineStream(stream)
	stream.AcceptWaveform(wave.SampleRate, wave.Samples)
	e.rec.Decode(stream)
	out := stream.GetResult()
	if out == nil {
		return Result{}, errors.New("Whisper no devolvió resultado")
	}
	return Result{Text: out.Text, Lang: out.Lang}, nil
}

func findModelDir(root, id string) (string, error) {
	base := filepath.Join(root, id)
	var found string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "turbo-tokens.txt" && d.Name() != "small-tokens.txt" {
			return nil
		}
		found = filepath.Dir(path)
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("tokens de Whisper no encontrados en %s", base)
	}
	return found, nil
}

func firstFile(dir, pattern string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}
