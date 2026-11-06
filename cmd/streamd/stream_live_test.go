package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

// TestStreamLive is gated because it needs a running streamd and a loaded model.
//
//	SONUS_STREAM_LIVE=http://127.0.0.1:9103 go test ./cmd/streamd -run TestStreamLive
func TestStreamLive(t *testing.T) {
	base := os.Getenv("SONUS_STREAM_LIVE")
	if base == "" {
		t.Skip("SONUS_STREAM_LIVE no configurado")
	}
	fixture := os.Getenv("SONUS_STREAM_FIXTURE")
	if fixture == "" {
		fixture = "../../testdata/stream-es.wav"
	}
	wave := sherpa.ReadWave(fixture)
	if wave == nil {
		t.Fatalf("fixture inválido: %s", fixture)
	}
	url := strings.Replace(base, "http://", "ws://", 1) + "/v1/stream"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	step := wave.SampleRate / 10
	for start := 0; start < len(wave.Samples); start += step {
		end := start + step
		if end > len(wave.Samples) {
			end = len(wave.Samples)
		}
		buf := make([]byte, 0, (end-start)*2)
		for _, sample := range wave.Samples[start:end] {
			var b [2]byte
			binary.LittleEndian.PutUint16(b[:], uint16(int16(sample*32767)))
			buf = append(buf, b[:]...)
		}
		if err := c.Write(ctx, websocket.MessageBinary, buf); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Write(ctx, websocket.MessageText, []byte("done")); err != nil {
		t.Fatal(err)
	}

	seen := false
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		var m struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(data, &m) == nil && m.Type == "final" {
			seen = true
			break
		}
	}
	if !seen {
		t.Fatal("no se recibió un mensaje final")
	}
}
