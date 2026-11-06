// Package config stores the sonus defaults and paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Paths struct {
	Models string `yaml:"models"`
	Data   string `yaml:"data"`
	Media  string `yaml:"media"`
	Output string `yaml:"output"`
}

type Defaults struct {
	ASR    string `yaml:"asr"`
	TTS    string `yaml:"tts"`
	Stream string `yaml:"stream"`
}

type Services struct {
	Phonemizer string `yaml:"phonemizer"`
}

type Config struct {
	Variant  string   `yaml:"variant"`
	Paths    Paths    `yaml:"paths"`
	Defaults Defaults `yaml:"defaults"`
	Services Services `yaml:"services"`
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

// Default returns the built-in paths. All state lives under one root so a
// backup or wipe is a single directory: SONUS_HOME overrides it, and the
// per-path variables still win over it.
func Default() Config {
	base := envOr("SONUS_HOME", filepath.Join(home(), "Services", "sonus"))
	return Config{
		Variant: envOr("SONUS_VARIANT", "cpu"),
		Paths: Paths{
			Models: envOr("SONUS_MODELS_DIR", filepath.Join(base, "models")),
			Data:   envOr("SONUS_DATA_DIR", filepath.Join(base, "data")),
			Media:  envOr("SONUS_MEDIA", filepath.Join(home(), "Media")),
			Output: envOr("SONUS_OUTPUT", filepath.Join(base, "output")),
		},
		Defaults: Defaults{ASR: "whisper-turbo-onnx", TTS: "kokoro-82m-onnx", Stream: "kroko-es-64l"},
		Services: Services{Phonemizer: envOr("SONUS_PHONEMIZER_URL", "http://127.0.0.1:9200")},
	}
}

func FilePath() string {
	if p := os.Getenv("SONUS_CONFIG"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "sonus", "config.yaml")
	}
	return filepath.Join(home(), ".config", "sonus", "config.yaml")
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
