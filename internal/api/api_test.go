package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Below-Z3R0/sonus/internal/runtime"
)

func TestHandlerHealthAndStatus(t *testing.T) {
	mgr := runtime.New("auris", "1.0", "whisper-turbo-onnx", nil, nil)
	h := Handler(func() any { return mgr.Status() }, []string{"m"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Fatalf("health: %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["service"] != "auris" || out["model"] != "whisper-turbo-onnx" {
		t.Fatalf("status = %v", out)
	}
}
