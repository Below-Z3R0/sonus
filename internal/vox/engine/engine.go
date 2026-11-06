// Package voxengine wires Kokoro ONNX to the Vox lifecycle.
package voxengine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"

	"github.com/Below-Z3R0/sonus/internal/langdetect"
	"github.com/Below-Z3R0/sonus/internal/phonemizer"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
)

type Engine struct {
	root  string
	phon  *phonemizer.Client
	mu    sync.Mutex
	tts   *sherpa.OfflineTts
	model catalog.Model
}

func New(modelsRoot, phonemizerURL string) *Engine {
	return &Engine{root: modelsRoot, phon: phonemizer.New(phonemizerURL)}
}

// Load opens the Kokoro model and keeps exactly one resident instance.
func (e *Engine) Load(id string) error {
	m, ok := catalog.Get(id)
	if !ok {
		return fmt.Errorf("modelo desconocido: %s", id)
	}
	if m.Frontend.Type != "misaki" || m.Frontend.PhonemeFormat != "kokoro" {
		return fmt.Errorf("frontend no soportado: %s", m.Frontend.Type)
	}
	dir, err := findModelDir(e.root, id)
	if err != nil {
		return err
	}
	cfg := sherpa.OfflineTtsConfig{}
	cfg.Model.Kokoro.Model = filepath.Join(dir, "model.onnx")
	cfg.Model.Kokoro.Voices = filepath.Join(dir, "voices.bin")
	cfg.Model.Kokoro.Tokens = filepath.Join(dir, "tokens.txt")
	cfg.Model.NumThreads = 4
	cfg.Model.Provider = "cpu"

	tts := sherpa.NewOfflineTts(&cfg)
	if tts == nil {
		return fmt.Errorf("Kokoro no pudo abrir %s", dir)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tts != nil {
		sherpa.DeleteOfflineTts(e.tts)
	}
	e.tts = tts
	e.model = m
	return nil
}

func (e *Engine) Unload() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tts != nil {
		sherpa.DeleteOfflineTts(e.tts)
		e.tts = nil
	}
}

// VoiceFor resolves an explicit voice or the first voice for a language.
func VoiceFor(m catalog.Model, lang, voiceID string) (catalog.Voice, error) {
	if voiceID != "" {
		for _, v := range m.Voices {
			if v.ID == voiceID {
				return v, nil
			}
		}
		return catalog.Voice{}, fmt.Errorf("voz desconocida: %s", voiceID)
	}
	lang = strings.ToLower(lang)
	for _, v := range m.Voices {
		if strings.HasPrefix(strings.ToLower(v.Language), lang) {
			return v, nil
		}
	}
	return catalog.Voice{}, fmt.Errorf("el modelo no tiene voz para %q", lang)
}

// Health checks the external phonemizer.
func (e *Engine) Health(ctx context.Context) error {
	return e.phon.Health(ctx)
}

// Speak phonemizes with misakid and synthesizes a WAV. Long text is split
// into chunks, and each chunk detects its own language when lang is auto.
func (e *Engine) Speak(ctx context.Context, text, lang, voiceID string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tts == nil {
		return nil, errors.New("modelo no cargado")
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("texto vacío")
	}
	chunks := SplitText(text, e.model.Profile.MaxChars)
	if len(chunks) == 0 {
		return nil, errors.New("texto vacío")
	}
	var samples []float32
	var sampleRate int
	for i, chunk := range chunks {
		chunkLang := lang
		if chunkLang == "" || chunkLang == "auto" {
			chunkLang = langdetect.Detect(chunk)
		}
		voice, err := VoiceFor(e.model, chunkLang, voiceID)
		if err != nil {
			return nil, err
		}
		ph, err := e.phon.Phonemize(ctx, chunk, chunkLang)
		if err != nil {
			return nil, err
		}
		audio := e.tts.GenerateWithConfig("", &sherpa.GenerationConfig{
			Sid:               voice.SID,
			Speed:             float32(e.model.Profile.Speed),
			PhonemeCodepoints: [][]int32{ph.Codepoints},
		}, nil)
		if audio == nil || len(audio.Samples) == 0 {
			return nil, errors.New("Kokoro no generó audio")
		}
		if sampleRate == 0 {
			sampleRate = audio.SampleRate
		} else if audio.SampleRate != sampleRate {
			return nil, errors.New("Kokoro cambió el sample rate entre chunks")
		}
		if i > 0 {
			samples = append(samples, make([]float32, sampleRate/20)...) // 50 ms
		}
		samples = append(samples, audio.Samples...)
	}
	wav := (&sherpa.GeneratedAudio{Samples: samples, SampleRate: sampleRate}).ToBuffer()
	if len(wav) == 0 {
		return nil, errors.New("Kokoro no serializó WAV")
	}
	return wav, nil
}

func findModelDir(root, id string) (string, error) {
	base := filepath.Join(root, id)
	var found string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "model.onnx" {
			return nil
		}
		dir := filepath.Dir(path)
		for _, name := range []string{"voices.bin", "tokens.txt"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				return nil
			}
		}
		found = dir
		return filepath.SkipAll
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("model.onnx/voices.bin/tokens.txt no encontrados en %s", base)
	}
	return found, nil
}
