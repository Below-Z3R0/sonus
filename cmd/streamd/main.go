// Command streamd serves low-latency Spanish ASR over WebSocket.
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/Below-Z3R0/sonus/internal/api"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/meminfo"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/obs"
	"github.com/Below-Z3R0/sonus/internal/runtime"
	"github.com/Below-Z3R0/sonus/internal/stream/catalog"
	streamengine "github.com/Below-Z3R0/sonus/internal/stream/engine"
)

const version = "0.1.0"

func main() {
	addr := flag.String("addr", "0.0.0.0:9103", "listen address")
	preload := flag.Bool("preload", false, "load the default model at startup")
	flag.Parse()

	cfg, err := config.Load(config.FilePath())
	if err != nil {
		log.Fatalf("streamd: %v", err)
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.Stream); ok {
		model = m
	}
	eng := streamengine.New(cfg.Paths.Models)
	mgr := runtime.New("stream", version, model.ID, func(id string) error {
		if st := modeldl.Status(cfg.Paths.Models, id); st.State != "ready" {
			return fmt.Errorf("%s no está instalado", id)
		}
		if m, ok := catalog.Get(id); ok {
			if err := meminfo.Check(m.RAMMB); err != nil {
				return err
			}
		}
		return eng.Load(id)
	}, eng.Unload)

	mux := api.Handler(func() any { return mgr.Status() }, catalog.List())
	api.RegisterLifecycle(mux, mgr)
	api.RegisterMetrics(mux, obs.Snapshot)
	api.RegisterDoctor(mux, func() any {
		st := mgr.Status()
		free, _ := modeldl.FreeBytes(cfg.Paths.Models)
		ram, _ := meminfo.AvailableMB()
		return map[string]any{
			"service": "stream", "status": st,
			"model_dir": cfg.Paths.Models, "free_mb": free >> 20, "available_ram_mb": ram,
		}
	})
	mux.HandleFunc("/v1/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		handleStream(w, r, eng)
	})

	log.Printf("streamd %s on %s (model %s)", version, *addr, model.ID)
	if *preload {
		if err := mgr.Load(model.ID); err != nil {
			log.Printf("preload: %v", err)
		}
	}
	if err := api.Serve(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

type message struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func handleStream(w http.ResponseWriter, r *http.Request, eng *streamengine.Engine) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(8 << 20)

	sess, err := eng.NewSession()
	if err != nil {
		_ = writeJSON(r.Context(), c, message{Type: "error", Text: err.Error()})
		return
	}
	defer sess.Close()

	var leftover []byte
	last := ""
	for {
		typ, data, err := c.Read(r.Context())
		if err != nil {
			break
		}
		if typ != websocket.MessageBinary {
			if typ == websocket.MessageText && string(data) == "done" {
				break
			}
			continue
		}
		if len(leftover) > 0 {
			data = append(leftover, data...)
			leftover = nil
		}
		if len(data)%2 != 0 {
			leftover = []byte{data[len(data)-1]}
			data = data[:len(data)-1]
		}
		if len(data) == 0 {
			continue
		}
		text, endpoint := sess.Feed(pcm16(data))
		if text != "" && text != last {
			last = text
			if err := writeJSON(r.Context(), c, message{Type: "partial", Text: text}); err != nil {
				return
			}
		}
		if endpoint {
			if text != "" {
				if err := writeJSON(r.Context(), c, message{Type: "final", Text: text}); err != nil {
					return
				}
			}
			last = ""
		}
	}
	final := sess.Finish()
	_ = writeJSON(context.Background(), c, message{Type: "final", Text: final})
	_ = c.Close(websocket.StatusNormalClosure, "done")
}

func writeJSON(ctx context.Context, c *websocket.Conn, m message) error {
	raw, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, raw)
}

func pcm16(data []byte) []float32 {
	out := make([]float32, len(data)/2)
	for i := range out {
		v := int16(binary.LittleEndian.Uint16(data[i*2:]))
		out[i] = float32(v) / math.MaxInt16
	}
	return out
}
