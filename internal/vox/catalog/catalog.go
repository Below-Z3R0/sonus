// Package catalog is the Vox-only TTS catalogue. Voices and models live here,
// separate from Auris.
package catalog

type Voice struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Gender   string `json:"gender"`
	SID      int    `json:"sid"`
}

type Profile struct {
	Speed    float64 `json:"speed"`
	MaxChars int     `json:"max_chars"`
}

// Frontend describes the text frontend a model needs before inference.
// PhonemeFormat is the token contract the engine expects.
type Frontend struct {
	Type          string `json:"type"`
	PhonemeFormat string `json:"phoneme_format"`
	Fallback      string `json:"fallback,omitempty"`
}

type Archive struct {
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256,omitempty"`
}

type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Tier          string   `json:"tier"`
	Default       bool     `json:"default"`
	License       string   `json:"license"`
	Homepage      string   `json:"homepage"`
	Notes         string   `json:"notes"`
	RAMMB         int      `json:"ram_mb"`
	DiskMB        int      `json:"disk_mb"`
	Languages     int      `json:"languages"`
	LanguageCodes []string `json:"language_codes"`
	Frontend      Frontend `json:"frontend"`
	Voices        []Voice  `json:"voices"`
	Profile       Profile  `json:"profile"`
	Archive       Archive  `json:"archive"`
}

var models = []Model{
	{
		ID: "kokoro-82m-onnx", Name: "Kokoro-82M (ONNX)",
		Tier: "balanced", Default: true, License: "Apache-2.0",
		Homepage: "https://huggingface.co/hexgrad/Kokoro-82M",
		Notes:    "82M parameters, 54 voices, 8 languages including Spanish; CPU-friendly.",
		RAMMB:    400, DiskMB: 350, Languages: 8,
		LanguageCodes: []string{"en", "es"},
		Frontend:      Frontend{Type: "misaki", PhonemeFormat: "kokoro", Fallback: "espeak"},
		Voices: []Voice{
			{ID: "ef_dora", Language: "es", Gender: "female", SID: 28},
			{ID: "em_alex", Language: "es", Gender: "male", SID: 29},
			{ID: "em_santa", Language: "es", Gender: "male", SID: 53},
			{ID: "af_heart", Language: "en-US", Gender: "female", SID: 3},
			{ID: "am_puck", Language: "en-US", Gender: "male", SID: 18},
		},
		Profile: Profile{Speed: 1.0, MaxChars: 400},
		Archive: Archive{URL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/kokoro-multi-lang-v1_0.tar.bz2", SizeBytes: 349886476, SHA256: "60703707bc05ccd48d60e1eb723f38b07d6e474795a0b6a588d7220400e0f7a4"},
	},
	{
		ID: "piper-es-onnx", Name: "Piper Spanish (ONNX)",
		Tier: "fast", License: "GPL-3.0 engine; voice license per file",
		Homepage: "https://github.com/OHF-Voice/piper1-gpl",
		Notes:    "Very light Spanish voices; fallback for edge and CPU-only hosts.",
		RAMMB:    300, DiskMB: 120, Languages: 1,
		LanguageCodes: []string{"es"},
		Frontend:      Frontend{Type: "piper_phonemize", PhonemeFormat: "piper"},
		Voices: []Voice{
			{ID: "es_ES-davefx-medium", Language: "es-ES", Gender: "male", SID: 0},
			{ID: "es_ES-sharvard-medium", Language: "es-ES", Gender: "male", SID: 1},
		},
		Profile: Profile{Speed: 1.0, MaxChars: 300},
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

func Voices(id string) []Voice {
	m, ok := Get(id)
	if !ok {
		return nil
	}
	return append([]Voice(nil), m.Voices...)
}
