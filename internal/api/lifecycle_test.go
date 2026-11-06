package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Below-Z3R0/sonus/internal/runtime"
)

func TestLifecycleLoadUnload(t *testing.T) {
	mgr := runtime.New("vox", "1.0", "kokoro-82m-onnx",
		func(string) error { return nil }, func() {})
	mux := Handler(func() any { return mgr.Status() }, []string{"m"})
	RegisterLifecycle(mux, mgr)

	req := httptest.NewRequest(http.MethodPost, "/v1/models/load", bytes.NewBufferString(`{"model":"kokoro-82m-onnx"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !mgr.Status().Loaded {
		t.Fatalf("load: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/models/unload", nil))
	if rec.Code != http.StatusOK || mgr.Status().Loaded {
		t.Fatalf("unload: %d %s", rec.Code, rec.Body.String())
	}
}
