package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

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
	case "listen":
		err = listenCmd(base, os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "streamctl version|status|service status|load <model>|unload|models list|show <id>|use <id>|config get|set|reset|stream <archivo.wav>|listen [--device D] [--threshold N] [--silence-ms N] [--seconds N]")
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
	s, err := dialStream(ctx, base)
	if err != nil {
		return err
	}
	defer s.close()
	s.start()

	step := wave.SampleRate / 10
	for start := 0; start < len(wave.Samples); start += step {
		end := start + step
		if end > len(wave.Samples) {
			end = len(wave.Samples)
		}
		if err := s.send(wave.Samples[start:end]); err != nil {
			return err
		}
	}
	return s.finish()
}

func listenCmd(base string, args []string) error {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	device := fs.String("device", "", "PipeWire/ALSA device")
	threshold := fs.Float64("threshold", 500, "RMS threshold for speech")
	silenceMS := fs.Int("silence-ms", 700, "silence before closing the utterance")
	maxSeconds := fs.Int("seconds", 60, "maximum capture time")
	flags, _ := cliutil.SplitFlags(args, map[string]bool{
		"--device": true, "-device": true,
		"--threshold": true, "-threshold": true,
		"--silence-ms": true, "-silence-ms": true,
		"--seconds": true, "-seconds": true,
	})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*maxSeconds)*time.Second)
	defer cancel()
	s, err := dialStream(ctx, base)
	if err != nil {
		return err
	}
	defer s.close()
	s.start()

	mic, cleanup, err := openMic(ctx, *device)
	if err != nil {
		return err
	}
	defer cleanup()

	frame := make([]byte, 640) // 20 ms at 16 kHz, s16le
	speech := false
	var silenceSince time.Time
	start := time.Now()
	for {
		n, err := mic.Read(frame)
		if n > 0 {
			chunk := frame[:n]
			if rms16(chunk) >= *threshold {
				speech = true
				silenceSince = time.Time{}
				if err := s.sendRaw(chunk); err != nil {
					return err
				}
			} else if speech {
				if silenceSince.IsZero() {
					silenceSince = time.Now()
				}
				if time.Since(silenceSince) >= time.Duration(*silenceMS)*time.Millisecond {
					break
				}
			} else if time.Since(start) > 10*time.Second {
				return fmt.Errorf("no se detectó voz")
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
		if time.Since(start) > time.Duration(*maxSeconds)*time.Second {
			break
		}
	}
	return s.finish()
}
