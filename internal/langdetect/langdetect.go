// Package langdetect picks the language used by the TTS frontend.
package langdetect

import (
	"strings"

	"github.com/pemistahl/lingua-go"
)

var detector = lingua.NewLanguageDetectorBuilder().
	FromLanguages(lingua.Spanish, lingua.English).
	Build()

// Detect returns es or en. Short or ambiguous text falls back to Spanish,
// which is the default voice of the project.
func Detect(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "es"
	}
	lang, ok := detector.DetectLanguageOf(text)
	if !ok {
		return "es"
	}
	switch lang {
	case lingua.English:
		return "en"
	case lingua.Spanish:
		return "es"
	default:
		return "es"
	}
}
