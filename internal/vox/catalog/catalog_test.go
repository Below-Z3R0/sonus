package catalog

import "testing"

func TestVoxCatalogIsSeparate(t *testing.T) {
	if Default().ID != "kokoro-82m-onnx" {
		t.Fatalf("default TTS = %s", Default().ID)
	}
	if _, ok := Get("whisper-turbo-onnx"); ok {
		t.Fatal("an ASR model leaked into the Vox catalogue")
	}
	voices := Voices("kokoro-82m-onnx")
	foundES := false
	for _, v := range voices {
		if v.Language == "es" {
			foundES = true
		}
	}
	if !foundES {
		t.Fatal("Kokoro needs Spanish voices")
	}
}
