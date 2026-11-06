// Package runtime owns the one-model-at-a-time lifecycle of a voice service.
package runtime

import (
	"errors"
	"runtime/debug"
	"sync"

	"github.com/Below-Z3R0/sonus/internal/memtrim"
)

type Status struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Model   string `json:"model"`
	State   string `json:"state"` // not_loaded | loading | ready | error
	Error   string `json:"error,omitempty"`
	Loaded  bool   `json:"loaded"`
}

type Manager struct {
	service      string
	version      string
	defaultModel string
	mu           sync.Mutex
	loadFn       func(string) error
	unloadFn     func()
	model        string
	state        string
	err          string
}

func New(service, version, defaultModel string, load func(string) error, unload func()) *Manager {
	if load == nil {
		load = func(string) error { return errors.New("engine not wired") }
	}
	if unload == nil {
		unload = func() {}
	}
	return &Manager{
		service: service, version: version, defaultModel: defaultModel,
		loadFn: load, unloadFn: unload, state: "not_loaded",
	}
}

// Load swaps the resident model. Only one model stays loaded at a time.
func (m *Manager) Load(model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if model == "" {
		return errors.New("model is required")
	}
	if m.model == model && m.state == "ready" {
		return nil
	}
	if m.model != "" {
		m.unloadFn()
		m.model = ""
	}
	m.state = "loading"
	m.err = ""
	if err := m.loadFn(model); err != nil {
		m.state = "error"
		m.err = err.Error()
		return err
	}
	m.model = model
	m.state = "ready"
	return nil
}

// Unload releases the model but keeps the service (and MCP) alive.
func (m *Manager) Unload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.model != "" {
		m.unloadFn()
	}
	m.model = ""
	m.state = "not_loaded"
	m.err = ""
	debug.FreeOSMemory()
	memtrim.Trim()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	model := m.model
	if model == "" {
		model = m.defaultModel
	}
	return Status{
		Service: m.service,
		Version: m.version,
		Model:   model,
		State:   m.state,
		Error:   m.err,
		Loaded:  m.state == "ready",
	}
}
