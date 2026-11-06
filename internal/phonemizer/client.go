// Package phonemizer talks to misakid, the text frontend used by Kokoro.
package phonemizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	URL  string
	HTTP *http.Client
}

type Result struct {
	Lang       string  `json:"lang"`
	Phonemes   string  `json:"phonemes"`
	Codepoints []int32 `json:"codepoints"`
}

func New(url string) *Client {
	return &Client{
		URL:  strings.TrimRight(url, "/"),
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

// Phonemize returns the phoneme string and its Unicode codepoints. The
// codepoints are what sherpa-onnx expects in GenerationConfig.
func (c *Client) Phonemize(ctx context.Context, text, lang string) (Result, error) {
	if c == nil || c.URL == "" {
		return Result{}, fmt.Errorf("phonemizer URL vacía")
	}
	body, err := json.Marshal(map[string]string{"text": text, "lang": lang})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/v1/phonemize", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("phonemizer: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("phonemizer: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out Result
	if err := json.Unmarshal(data, &out); err != nil {
		return Result{}, err
	}
	if out.Phonemes == "" || len(out.Codepoints) == 0 {
		return Result{}, fmt.Errorf("phonemizer: respuesta vacía")
	}
	return out, nil
}
