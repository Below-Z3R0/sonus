package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Keys are the supported dotted paths for config get/set/reset.
var Keys = []string{
	"variant",
	"paths.models",
	"paths.data",
	"paths.media",
	"paths.output",
	"defaults.asr",
	"defaults.tts",
	"defaults.stream",
	"services.phonemizer",
}

func IsKey(key string) bool {
	for _, k := range Keys {
		if k == key {
			return true
		}
	}
	return false
}

func raw(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func saveRaw(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
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

func Get(path, key string) (any, bool, error) {
	m, err := raw(path)
	if err != nil {
		return nil, false, err
	}
	parts := strings.Split(key, ".")
	var cur any = m
	for _, part := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false, nil
		}
	}
	return cur, true, nil
}

func Set(path, key, value string) error {
	m, err := raw(path)
	if err != nil {
		return err
	}
	parts := strings.Split(key, ".")
	cur := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
	return saveRaw(path, m)
}

func Reset(path, key string) error {
	m, err := raw(path)
	if err != nil {
		return err
	}
	parts := strings.Split(key, ".")
	cur := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	delete(cur, parts[len(parts)-1])
	return saveRaw(path, m)
}
