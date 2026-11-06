// Package catalog is the Auris-only ASR catalogue. It is deliberately
// separate from Vox: the model names and tuning knobs are not interchangeable.
package catalog

type Profile struct {
	BeamSize     int  `json:"beam_size"`
	VAD          bool `json:"vad"`
	ChunkSeconds int  `json:"chunk_seconds"`
}

type Archive struct {
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256,omitempty"`
}

type Model struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Tier      string  `json:"tier"`
	Default   bool    `json:"default"`
	License   string  `json:"license"`
	Homepage  string  `json:"homepage"`
	Notes     string  `json:"notes"`
	RAMMB     int     `json:"ram_mb"`
	DiskMB    int     `json:"disk_mb"`
	Languages int     `json:"languages"`
	Profile   Profile `json:"profile"`
	Archive   Archive `json:"archive"`
}

var models = []Model{
	{
		ID: "whisper-turbo-onnx", Name: "Whisper large-v3-turbo (ONNX)",
		Tier: "balanced", Default: true, License: "MIT",
		Homepage: "https://huggingface.co/openai/whisper-large-v3-turbo",
		Notes:    "Multilingual ASR; encoder/decoder ONNX export; 4 decoder layers.",
		RAMMB:    1600, DiskMB: 1200, Languages: 99,
		Profile: Profile{BeamSize: 5, VAD: true, ChunkSeconds: 30},
		Archive: Archive{URL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-whisper-turbo.tar.bz2", SizeBytes: 563790207},
	},
	{
		ID: "whisper-small-onnx", Name: "Whisper small (ONNX)",
		Tier: "fast", License: "MIT",
		Homepage: "https://huggingface.co/openai/whisper-small",
		Notes:    "CPU fallback; lower memory and latency.",
		RAMMB:    600, DiskMB: 500, Languages: 99,
		Profile: Profile{BeamSize: 3, VAD: true, ChunkSeconds: 30},
		Archive: Archive{URL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-whisper-small.tar.bz2", SizeBytes: 639387718},
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
