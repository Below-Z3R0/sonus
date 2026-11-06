package jobs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	m := New(path, 10)
	id := m.Start("test", map[string]string{"text": "hola"}, context.Background(), func(context.Context) (any, error) {
		return "ok", nil
	})
	waitDone(t, m, id)

	m2 := New(path, 10)
	got, ok := m2.Get(id)
	if !ok {
		t.Fatal("job not restored")
	}
	if got.State != Done || got.Result != "ok" {
		t.Fatalf("got %#v", got)
	}
	if len(got.Payload) == 0 {
		t.Fatal("payload not persisted")
	}
}

func TestInterruptedOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	raw := `[{"id":"1","kind":"tts","state":"running"}]`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(path, 10)
	got, ok := m.Get("1")
	if !ok || got.State != Failed {
		t.Fatalf("got %#v", got)
	}
}

func waitDone(t *testing.T, m *Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j, ok := m.Get(id); ok && (j.State == Done || j.State == Failed) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}
