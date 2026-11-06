// Package catalog is the streaming ASR catalogue. It is separate from the
// offline Auris catalogue: these models are transducers consumed by
// sherpa.OnlineRecognizer, not Whisper encoder/decoder pairs.
package catalog

import (
	"fmt"

	"github.com/Below-Z3R0/sonus/internal/modeldl"
)

type Model struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Tier     string         `json:"tier"`
	Default  bool           `json:"default"`
	License  string         `json:"license"`
	Homepage string         `json:"homepage"`
	Notes    string         `json:"notes"`
	RAMMB    int            `json:"ram_mb"`
	DiskMB   int            `json:"disk_mb"`
	Files    []modeldl.File `json:"files"`
}

const baseURL = "https://huggingface.co/hudaiapa88/sherpa-stt-onnx/resolve/main/es/"

func krokoFiles(variant string) []modeldl.File {
	return []modeldl.File{
		{Name: "encoder.int8.onnx", URL: baseURL + variant + "/encoder.int8.onnx", Size: 153185997, SHA256: shaFor(variant, "encoder")},
		{Name: "decoder.int8.onnx", URL: baseURL + variant + "/decoder.int8.onnx", Size: 606495, SHA256: shaFor(variant, "decoder")},
		{Name: "joiner.int8.onnx", URL: baseURL + variant + "/joiner.int8.onnx", Size: 337105, SHA256: shaFor(variant, "joiner")},
		{Name: "tokens.txt", URL: baseURL + variant + "/tokens.txt", Size: 7037},
	}
}

func shaFor(variant, part string) string {
	if variant == "kroko_64l" {
		switch part {
		case "encoder":
			return "e6bb47f5425983960cf70c33f460e4a277ca1ec7f3c59e3747974ad3f431f056"
		case "decoder":
			return "624e8ae2886d5b1957e26ac77df54a00c3b55fdfe2153d7dbf2d687ca943e58b"
		case "joiner":
			return "f027a07c54ff3565419f95e9c43112196ae188450a796411784917740a9bf9d0"
		}
	}
	switch part {
	case "encoder":
		return "28c0ab872d407a5896ba53ff17d12f35d68b797144e28ca00be4b752b9590e71"
	case "decoder":
		return "c36e02e86a3c3bfff1483750eea22f16e5c43c57a5c34d22ecf4ca3eb341ddf9"
	case "joiner":
		return "d876cff6dc6d065a824a1e8e6b915832ec0a1c7b867f8c7e592f8f3877ce2924"
	}
	return ""
}

var models = []Model{
	{
		ID: "kroko-es-64l", Name: "Kroko Spanish 64L (streaming)", Tier: "fast", Default: true,
		License: "Apache-2.0", Homepage: "https://huggingface.co/hudaiapa88/sherpa-stt-onnx",
		Notes: "Zipformer2 transducer; faster inference, good accuracy.",
		RAMMB: 700, DiskMB: 160, Files: krokoFiles("kroko_64l"),
	},
	{
		ID: "kroko-es-128l", Name: "Kroko Spanish 128L (streaming)", Tier: "accurate",
		License: "Apache-2.0", Homepage: "https://huggingface.co/hudaiapa88/sherpa-stt-onnx",
		Notes: "Zipformer2 transducer; highest accuracy, more compute per chunk.",
		RAMMB: 900, DiskMB: 160, Files: krokoFiles("kroko_128l"),
	},
}

func List() []Model { return append([]Model(nil), models...) }

func Get(id string) (Model, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

func Default() Model {
	for _, m := range models {
		if m.Default {
			return m
		}
	}
	return models[0]
}

func Check(model Model) error {
	if len(model.Files) == 0 {
		return fmt.Errorf("%s no tiene archivos", model.ID)
	}
	return nil
}
