package phonemizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhonemize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/phonemize" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["lang"] != "es" || req["text"] != "hola" {
			t.Fatalf("req = %#v", req)
		}
		json.NewEncoder(w).Encode(Result{Lang: "es", Phonemes: "ˈola", Codepoints: []int32{712, 111, 108, 97}})
	}))
	defer srv.Close()

	got, err := New(srv.URL).Phonemize(context.Background(), "hola", "es")
	if err != nil {
		t.Fatal(err)
	}
	if got.Phonemes != "ˈola" || len(got.Codepoints) != 4 {
		t.Fatalf("got = %#v", got)
	}
}

func TestPhonemizeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad language", http.StatusBadRequest)
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Phonemize(context.Background(), "hola", "xx"); err == nil {
		t.Fatal("expected error")
	}
}
