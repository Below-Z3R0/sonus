package catalog

import "testing"

func TestAurisCatalogIsSeparate(t *testing.T) {
	if len(List()) == 0 {
		t.Fatal("empty ASR catalogue")
	}
	if Default().ID != "whisper-turbo-onnx" {
		t.Fatalf("default ASR = %s", Default().ID)
	}
	if _, ok := Get("kokoro-82m-onnx"); ok {
		t.Fatal("a TTS model leaked into the Auris catalogue")
	}
}
