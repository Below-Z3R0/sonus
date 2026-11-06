// Package streamengine wraps sherpa.OnlineRecognizer for low-latency ASR.
package streamengine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"

	"github.com/Below-Z3R0/sonus/internal/stream/catalog"
)

type Engine struct {
	root  string
	mu    sync.Mutex
	rec   *sherpa.OnlineRecognizer
	model catalog.Model
}

func New(modelsRoot string) *Engine {
	return &Engine{root: modelsRoot}
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
	cfg := sherpa.OnlineRecognizerConfig{}
	cfg.FeatConfig.SampleRate = 16000
	cfg.FeatConfig.FeatureDim = 80
	cfg.ModelConfig.Transducer.Encoder = filepath.Join(dir, "encoder.int8.onnx")
	cfg.ModelConfig.Transducer.Decoder = filepath.Join(dir, "decoder.int8.onnx")
	cfg.ModelConfig.Transducer.Joiner = filepath.Join(dir, "joiner.int8.onnx")
	cfg.ModelConfig.Tokens = filepath.Join(dir, "tokens.txt")
	cfg.ModelConfig.NumThreads = 4
	cfg.ModelConfig.Provider = "cpu"
	cfg.DecodingMethod = "greedy_search"
	cfg.EnableEndpoint = 1
	cfg.Rule1MinTrailingSilence = 2.4
	cfg.Rule2MinTrailingSilence = 2.0
	cfg.Rule3MinUtteranceLength = 20

	rec := sherpa.NewOnlineRecognizer(&cfg)
	if rec == nil {
		return fmt.Errorf("Kroko no pudo abrir %s", dir)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec != nil {
		sherpa.DeleteOnlineRecognizer(e.rec)
	}
	e.rec = rec
	e.model = m
	return nil
}

func (e *Engine) Unload() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec != nil {
		sherpa.DeleteOnlineRecognizer(e.rec)
		e.rec = nil
	}
}

func (e *Engine) NewSession() (*Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rec == nil {
		return nil, errors.New("modelo no cargado")
	}
	return &Session{engine: e, stream: sherpa.NewOnlineStream(e.rec)}, nil
}

type Session struct {
	engine *Engine
	stream *sherpa.OnlineStream
	closed bool
}

// Feed accepts PCM float samples at 16 kHz and returns the current text and
// whether an endpoint (end of utterance) was detected.
func (s *Session) Feed(samples []float32) (string, bool) {
	if s.closed || len(samples) == 0 {
		return "", false
	}
	s.engine.mu.Lock()
	defer s.engine.mu.Unlock()
	if s.engine.rec == nil {
		return "", false
	}
	s.stream.AcceptWaveform(16000, samples)
	for s.engine.rec.IsReady(s.stream) {
		s.engine.rec.Decode(s.stream)
	}
	out := s.engine.rec.GetResult(s.stream)
	text := ""
	if out != nil {
		text = out.Text
	}
	endpoint := s.engine.rec.IsEndpoint(s.stream)
	if endpoint {
		s.engine.rec.Reset(s.stream)
	}
	return text, endpoint
}

func (s *Session) Finish() string {
	if s.closed {
		return ""
	}
	s.engine.mu.Lock()
	defer s.engine.mu.Unlock()
	if s.engine.rec == nil {
		return ""
	}
	s.stream.InputFinished()
	for s.engine.rec.IsReady(s.stream) {
		s.engine.rec.Decode(s.stream)
	}
	out := s.engine.rec.GetResult(s.stream)
	if out == nil {
		return ""
	}
	return out.Text
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	s.closed = true
	sherpa.DeleteOnlineStream(s.stream)
}

func findModelDir(root, id string) (string, error) {
	base := filepath.Join(root, id)
	for _, name := range []string{"encoder.int8.onnx", "decoder.int8.onnx", "joiner.int8.onnx", "tokens.txt"} {
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			return "", fmt.Errorf("falta %s en %s", name, base)
		}
	}
	return base, nil
}
