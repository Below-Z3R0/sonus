package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetGetReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Set(path, "defaults.tts", "kokoro-82m-onnx"); err != nil {
		t.Fatal(err)
	}
	v, ok, err := Get(path, "defaults.tts")
	if err != nil || !ok || v != "kokoro-82m-onnx" {
		t.Fatalf("got %#v %v %v", v, ok, err)
	}
	if err := Reset(path, "defaults.tts"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := Get(path, "defaults.tts"); ok {
		t.Fatal("key still present")
	}
}

func TestSetPreservesOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("variant: cpu\ndefaults:\n  asr: whisper-turbo-onnx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Set(path, "defaults.tts", "kokoro-82m-onnx"); err != nil {
		t.Fatal(err)
	}
	v, ok, _ := Get(path, "defaults.asr")
	if !ok || v != "whisper-turbo-onnx" {
		t.Fatalf("asr lost: %#v", v)
	}
}
