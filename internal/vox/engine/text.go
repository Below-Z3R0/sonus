package voxengine

import "strings"

// SplitText breaks text into chunks that fit the model profile. It prefers
// sentence boundaries, then whitespace, and only cuts a word when a single
// token is longer than the limit.
func SplitText(text string, max int) []string {
	if max <= 0 {
		max = 400
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			if s := strings.TrimSpace(b.String()); s != "" {
				sentences = append(sentences, s)
			}
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		sentences = append(sentences, s)
	}

	var chunks []string
	var cur string
	for _, sentence := range sentences {
		for len([]rune(sentence)) > max {
			cut := lastSpace(sentence, max)
			if cut <= 0 {
				cut = max
			}
			part := strings.TrimSpace(string([]rune(sentence)[:cut]))
			if part != "" {
				chunks = append(chunks, part)
			}
			sentence = strings.TrimSpace(string([]rune(sentence)[cut:]))
		}
		if sentence == "" {
			continue
		}
		if cur == "" {
			cur = sentence
			continue
		}
		if len([]rune(cur))+1+len([]rune(sentence)) <= max {
			cur += " " + sentence
			continue
		}
		chunks = append(chunks, cur)
		cur = sentence
	}
	if cur != "" {
		chunks = append(chunks, cur)
	}
	return chunks
}

func lastSpace(s string, max int) int {
	runes := []rune(s)
	limit := max
	if limit > len(runes) {
		limit = len(runes)
	}
	for i := limit - 1; i > 0; i-- {
		if runes[i] == ' ' {
			return i
		}
	}
	return 0
}
