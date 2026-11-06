package voxengine

import "testing"

func TestSplitText(t *testing.T) {
	got := SplitText("Hola. Esto es una prueba. ¿Y esto?", 20)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != "Hola." || got[1] != "Esto es una prueba." {
		t.Fatalf("got %#v", got)
	}
}

func TestSplitTextLongSentence(t *testing.T) {
	text := "uno dos tres cuatro cinco seis siete ocho nueve diez"
	got := SplitText(text, 15)
	if len(got) < 2 {
		t.Fatalf("got %#v", got)
	}
	for _, chunk := range got {
		if len([]rune(chunk)) > 15 {
			t.Fatalf("chunk too long: %q", chunk)
		}
	}
}

func TestSplitTextEmpty(t *testing.T) {
	if got := SplitText("   ", 20); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}
