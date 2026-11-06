package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/coder/websocket"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"

	"github.com/Below-Z3R0/sonus/internal/cliutil"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	"github.com/Below-Z3R0/sonus/internal/stream/catalog"
)

const version = "0.1.0"

func main() {
	base := os.Getenv("SONUS_STREAM_URL")
	if base == "" {
		base = "http://127.0.0.1:9103"
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "status":
		err = cliutil.PrintGet(base+"/v1/status", 10*time.Second)
	case "models":
		err = modelsCmd(os.Args[2:])
	case "config":
		err = configCmd(os.Args[2:])
	case "service":
		err = cliutil.Service(base, os.Args[2:])
	case "stream":
		err = streamCmd(base, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "streamctl version|status|service status|load <model>|unload|models list|show <id>|use <id>|config get|set|reset|stream <archivo.wav>")
}

func modelsCmd(args []string) error {
	cfgPath := config.FilePath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	rows := make([]cliutil.ModelRow, 0, len(catalog.List()))
	for _, m := range catalog.List() {
		rows = append(rows, cliutil.ModelRow{
			ID: m.ID, Tier: m.Tier,
			Extra: "streaming es",
			State: modeldl.Describe(modeldl.Status(cfg.Paths.Models, m.ID)),
		})
	}
	show := func(id string) (any, bool) {
		m, ok := catalog.Get(id)
		return m, ok
	}
	set := func(c *config.Config, id string) { c.Defaults.Stream = id }
	install := func(id string, force bool) error {
		m, ok := catalog.Get(id)
		if !ok || len(m.Files) == 0 {
			return fmt.Errorf("%s no tiene descarga", id)
		}
		return modeldl.InstallFiles(cfg.Paths.Models, id, m.Files, int64(m.DiskMB)*3*1024*1024/2, force)
	}
	remove := func(id string) error { return modeldl.Remove(cfg.Paths.Models, id) }
	return cliutil.RunModels(args, cfgPath, cfg.Defaults.Stream, rows, show, set, install, remove)
}

func configCmd(args []string) error {
	validate := func(key, value string) error {
		switch key {
		case "variant":
			if value != "cpu" && value != "cuda" && value != "intel" && value != "amd" {
				return fmt.Errorf("variant inválida: %s", value)
			}
		case "defaults.stream":
			if _, ok := catalog.Get(value); !ok {
				return fmt.Errorf("modelo streaming desconocido: %s", value)
			}
		case "paths.models", "paths.data", "paths.media", "paths.output", "services.phonemizer":
			if value == "" {
				return fmt.Errorf("%s no puede estar vacío", key)
			}
		}
		return nil
	}
	return cliutil.ConfigCmd(config.FilePath(), args, validate)
}

func streamCmd(base string, args []string) error {
	fs := flag.NewFlagSet("stream", flag.ContinueOnError)
	file := fs.String("file", "", "WAV file to stream")
	flags, positional := cliutil.SplitFlags(args, map[string]bool{"--file": true, "-file": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if *file == "" && len(positional) > 0 {
		*file = positional[0]
	}
	if *file == "" {
		return fmt.Errorf("uso: streamctl stream <archivo.wav>")
	}
	wave := sherpa.ReadWave(*file)
	if wave == nil {
		return fmt.Errorf("WAV inválido: %s", *file)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	url := base + "/v1/stream"
	if len(url) > 4 && url[:4] == "http" {
		url = "ws" + url[4:]
	}
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return err
	}
	defer c.CloseNow()

	done := make(chan error, 1)
	go func() {
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					done <- nil
					return
				}
				done <- err
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

	step := wave.SampleRate / 10
	for start := 0; start < len(wave.Samples); start += step {
		end := start + step
		if end > len(wave.Samples) {
			end = len(wave.Samples)
		}
		buf := make([]byte, 0, (end-start)*2)
		for _, sample := range wave.Samples[start:end] {
			v := int16(math.Round(float64(sample) * math.MaxInt16))
			var b [2]byte
			binary.LittleEndian.PutUint16(b[:], uint16(v))
			buf = append(buf, b[:]...)
		}
		if err := c.Write(ctx, websocket.MessageBinary, buf); err != nil {
			return err
		}
	}
	if err := c.Write(ctx, websocket.MessageText, []byte("done")); err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
