// Package models owns the ONNX model catalogue for Auris and Vox.
package models

type Task string

const (
	TaskASR Task = "asr"
	TaskTTS Task = "tts"
	TaskVAD Task = "vad"
)

// Model is one catalogue entry. Only ONNX models are listed: the voice stack
// deliberately shares one runtime with Argos.
type Model struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Task      Task   `json:"task"`
	Tier      string `json:"tier"`
	Default   bool   `json:"default"`
	License   string `json:"license"`
	Homepage  string `json:"homepage"`
	Notes     string `json:"notes"`
	RAMMB     int    `json:"ram_mb"`
	DiskMB    int    `json:"disk_mb"`
	Languages int    `json:"languages,omitempty"`
}

var catalog = []Model{
	{
		ID: "whisper-turbo-onnx", Name: "Whisper large-v3-turbo (ONNX)", Task: TaskASR,
		Tier: "balanced", Default: true, License: "MIT",
		Homepage: "https://huggingface.co/openai/whisper-large-v3-turbo",
		Notes:    "Multilingual; encoder/decoder ONNX export; 4 decoder layers.",
		RAMMB:    1600, DiskMB: 1200, Languages: 99,
	},
	{
		ID: "whisper-small-onnx", Name: "Whisper small (ONNX)", Task: TaskASR,
		Tier: "fast", License: "MIT",
		Homepage: "https://huggingface.co/openai/whisper-small",
		Notes:    "CPU fallback and latency-sensitive jobs.",
		RAMMB:    600, DiskMB: 500, Languages: 99,
	},
	{
		ID: "kokoro-82m-onnx", Name: "Kokoro-82M (ONNX)", Task: TaskTTS,
		Tier: "balanced", Default: true, License: "Apache-2.0",
		Homepage: "https://huggingface.co/hexgrad/Kokoro-82M",
		Notes:    "54 voices, 8 languages including Spanish; fast on CPU.",
		RAMMB:    400, DiskMB: 350, Languages: 8,
	},
	{
		ID: "piper-es-onnx", Name: "Piper Spanish (ONNX)", Task: TaskTTS,
		Tier: "fast", License: "GPL-3.0 engine; voice license per file",
		Homepage: "https://github.com/OHF-Voice/piper1-gpl",
		Notes:    "Spanish voices, very light; fallback for edge/CPU.",
		RAMMB:    300, DiskMB: 120, Languages: 1,
	},
	{
		ID: "silero-vad-onnx", Name: "Silero VAD (ONNX)", Task: TaskVAD,
		Tier: "balanced", Default: true, License: "MIT",
		Homepage: "https://github.com/snakers4/silero-vad",
		Notes:    "Voice activity detection for streaming and chunking.",
		RAMMB:    50, DiskMB: 2,
	},
}

func List(task Task) []Model {
	out := make([]Model, 0, len(catalog))
	for _, m := range catalog {
		if task == "" || m.Task == task {
			out = append(out, m)
		}
	}
	return out
}

func Get(id string) (Model, bool) {
	for _, m := range catalog {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}
