package langdetect

import "testing"

func TestDetect(t *testing.T) {
	cases := map[string]string{
		"Hola, esto es una prueba de voz.": "es",
		"Hello, this is a voice test.":     "en",
		"":                                 "es",
	}
	for text, want := range cases {
		if got := Detect(text); got != want {
			t.Errorf("Detect(%q) = %q, want %q", text, got, want)
		}
	}
}
