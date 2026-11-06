package modeldl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusTransitions(t *testing.T) {
	root := t.TempDir()
	if got := Status(root, "m"); got.State != "missing" {
		t.Fatalf("missing = %+v", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "m"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "m", ".state.json"), []byte(`{"state":"downloading","downloaded":50,"total":100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Describe(Status(root, "m")); got != "downloading 50%" {
		t.Fatalf("describe = %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "m", ".installed.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Status(root, "m"); got.State != "ready" {
		t.Fatalf("ready = %+v", got)
	}
}
