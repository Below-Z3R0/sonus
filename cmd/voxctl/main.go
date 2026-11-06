package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	auriscatalog "github.com/Below-Z3R0/sonus/internal/auris/catalog"
	"github.com/Below-Z3R0/sonus/internal/cliutil"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	streamcatalog "github.com/Below-Z3R0/sonus/internal/stream/catalog"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
)

const version = "0.1.0"

func main() {
	base := os.Getenv("SONUS_TTS_URL")
	if base == "" {
		base = "http://127.0.0.1:9102"
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
	case "voices":
		err = voicesCmd()
	case "models":
		err = modelsCmd(os.Args[2:])
	case "config":
		err = configCmd(os.Args[2:])
	case "service":
		err = cliutil.Service(base, os.Args[2:])
	case "jobs":
		err = cliutil.JobCmd(base, os.Args[2:])
	case "speak":
		err = speakCmd(base, os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "voxctl version|status|service status|load <model>|unload|voices|models list|show <id>|use <id>|config get|set|reset|jobs list|status|wait|retry|speak \"texto\" [--file F] [--lang auto] [--voice V] [--out F] [--async|--stream]")
}

func voicesCmd() error {
	cfg, err := config.Load(config.FilePath())
	if err != nil {
		return err
	}
	model := catalog.Default()
	if m, ok := catalog.Get(cfg.Defaults.TTS); ok {
		model = m
	}
	out, err := json.MarshalIndent(model.Voices, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func configCmd(args []string) error {
	validate := func(key, value string) error {
		switch key {
		case "variant":
			if value != "cpu" && value != "cuda" && value != "intel" && value != "amd" {
				return fmt.Errorf("variant inválida: %s", value)
			}
		case "defaults.tts":
			if _, ok := catalog.Get(value); !ok {
				return fmt.Errorf("modelo TTS desconocido: %s", value)
			}
		case "defaults.asr":
			if _, ok := auriscatalog.Get(value); !ok {
				return fmt.Errorf("modelo ASR desconocido: %s", value)
			}
		case "defaults.stream":
			if _, ok := streamcatalog.Get(value); !ok {
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
			Extra: fmt.Sprintf("%d voices", len(m.Voices)),
			State: modeldl.Describe(modeldl.Status(cfg.Paths.Models, m.ID)),
		})
	}
	show := func(id string) (any, bool) {
		m, ok := catalog.Get(id)
		return m, ok
	}
	set := func(c *config.Config, id string) { c.Defaults.TTS = id }
	install := func(id string, force bool) error {
		m, ok := catalog.Get(id)
		if !ok || m.Archive.URL == "" {
			return fmt.Errorf("%s no tiene descarga", id)
		}
		return modeldl.Install(cfg.Paths.Models, id, m.Archive.URL, m.Archive.SizeBytes, m.Archive.SHA256, int64(m.DiskMB)*3*1024*1024/2, force)
	}
	remove := func(id string) error { return modeldl.Remove(cfg.Paths.Models, id) }
	return cliutil.RunModels(args, cfgPath, cfg.Defaults.TTS, rows, show, set, install, remove)
}

func speakCmd(base string, args []string) error {
	fs := flag.NewFlagSet("speak", flag.ContinueOnError)
	file := fs.String("file", "", "read text from a file")
	out := fs.String("out", "", "output audio file")
	voice := fs.String("voice", "", "voice id (default: model voice for lang)")
	lang := fs.String("lang", "auto", "language code (auto, es, en, ...)")
	async := fs.Bool("async", false, "return a job id instead of waiting for audio")
	stream := fs.Bool("stream", false, "stream raw PCM16 audio")
	flags, positional := cliutil.SplitFlags(args, map[string]bool{
		"--file": true, "-file": true,
		"--out": true, "-out": true,
		"--voice": true, "-voice": true,
		"--lang": true, "-lang": true,
	})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	text := ""
	if len(positional) > 0 {
		text = positional[0]
	}
	if *file != "" {
		raw, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		text = string(raw)
	}
	if text == "" {
		return fmt.Errorf("uso: voxctl speak \"texto\" | --file archivo")
	}
	if *async {
		return speakAsync(base, text, *lang, *voice)
	}
	if *stream {
		return speakStream(base, text, *lang, *voice, *out)
	}
	return speak(base, text, *lang, *voice, *out)
}

func speakStream(base, text, lang, voice, out string) error {
	body, err := json.Marshal(map[string]string{"text": text, "lang": lang, "voice": voice})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/speech/stream", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("vox: %s: %s", resp.Status, string(raw))
	}
	var dst io.Writer = os.Stdout
	var f *os.File
	if out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		f, err = os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		dst = f
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

func speakAsync(base, text, lang, voice string) error {
	body, err := json.Marshal(map[string]string{"text": text, "lang": lang, "voice": voice})
	if err != nil {
		return err
	}
	resp, err := http.Post(base+"/v1/speech/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vox: %s: %s", resp.Status, string(raw))
	}
	var out struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	fmt.Println(out.JobID)
	return nil
}

func speak(base, text, lang, voice, out string) error {
	body, err := json.Marshal(map[string]string{"text": text, "lang": lang, "voice": voice})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/speech", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vox: %s: %s", resp.Status, string(raw))
	}
	if out == "" {
		return fmt.Errorf("sin --out: el motor todavía no reproduce audio")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, raw, 0o644)
}
