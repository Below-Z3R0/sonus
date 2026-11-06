package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"

	"github.com/coder/websocket"
)

type streamClient struct {
	c    *websocket.Conn
	ctx  context.Context
	done chan error
}

func dialStream(ctx context.Context, base string) (*streamClient, error) {
	url := base + "/v1/stream"
	if len(url) > 4 && url[:4] == "http" {
		url = "ws" + url[4:]
	}
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return &streamClient{c: c, ctx: ctx, done: make(chan error, 1)}, nil
}

func (s *streamClient) start() {
	go func() {
		for {
			typ, data, err := s.c.Read(s.ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					s.done <- nil
					return
				}
				s.done <- err
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			var m struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(data, &m) == nil && m.Text != "" {
				fmt.Printf("%s: %s\n", m.Type, m.Text)
			}
		}
	}()
}

func (s *streamClient) send(samples []float32) error {
	buf := pcmBytes(samples)
	if len(buf) == 0 {
		return nil
	}
	return s.c.Write(s.ctx, websocket.MessageBinary, buf)
}

func (s *streamClient) sendRaw(buf []byte) error {
	return s.c.Write(s.ctx, websocket.MessageBinary, buf)
}

func (s *streamClient) finish() error {
	if err := s.c.Write(s.ctx, websocket.MessageText, []byte("done")); err != nil {
		return err
	}
	select {
	case err := <-s.done:
		return err
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *streamClient) close() { s.c.CloseNow() }

func pcmBytes(samples []float32) []byte {
	buf := make([]byte, 0, len(samples)*2)
	for _, sample := range samples {
		v := int16(math.Round(float64(sample) * math.MaxInt16))
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(v))
		buf = append(buf, b[:]...)
	}
	return buf
}

func openMic(ctx context.Context, device, backend string) (io.ReadCloser, func(), error) {
	useParec := backend == "parec" || (backend == "" || backend == "auto") && !strings.HasPrefix(device, "hw:") && !strings.HasPrefix(device, "plughw:")
	if useParec {
		if _, err := exec.LookPath("parec"); err == nil {
			args := []string{"--format=s16le", "--rate=16000", "--channels=1", "--latency-msec=20"}
			if device != "" {
				args = append(args, "--device="+device)
			}
			return startCapture(ctx, "parec", args...)
		}
	}
	if backend == "parec" {
		return nil, func() {}, fmt.Errorf("parec no está en PATH")
	}
	if _, err := exec.LookPath("arecord"); err == nil {
		args := []string{"-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw"}
		if device != "" {
			args = append(args, "-D", device)
		}
		return startCapture(ctx, "arecord", args...)
	}
	return nil, func() {}, fmt.Errorf("no hay parec ni arecord en PATH")
}

func startCapture(ctx context.Context, name string, args ...string) (io.ReadCloser, func(), error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, func() {}, err
	}
	if err := cmd.Start(); err != nil {
		return nil, func() {}, err
	}
	return stdout, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}, nil
}

func rms16(buf []byte) float64 {
	if len(buf) < 2 {
		return 0
	}
	var sum float64
	n := len(buf) / 2
	for i := 0; i < n; i++ {
		v := int16(binary.LittleEndian.Uint16(buf[i*2:]))
		x := float64(v)
		sum += x * x
	}
	return math.Sqrt(sum / float64(n))
}
