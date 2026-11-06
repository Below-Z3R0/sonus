package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultModels(t *testing.T) {
	cfg := Default()
	if cfg.Defaults.ASR != "whisper-turbo-onnx" || cfg.Defaults.TTS != "kokoro-82m-onnx" {
		t.Fatalf("defaults = %+v", cfg.Defaults)
	}
	if !filepath.IsAbs(cfg.Paths.Models) {
		t.Fatalf("models path is not absolute: %s", cfg.Paths.Models)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.Defaults.TTS = "piper-es-onnx"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Defaults.TTS != "piper-es-onnx" {
		t.Fatalf("TTS = %s", got.Defaults.TTS)
	}
}
