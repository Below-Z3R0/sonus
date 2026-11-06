package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
	"github.com/Below-Z3R0/sonus/internal/cliutil"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
	streamcatalog "github.com/Below-Z3R0/sonus/internal/stream/catalog"
	voxcatalog "github.com/Below-Z3R0/sonus/internal/vox/catalog"
)

const version = "0.1.0"

func main() {
	base := os.Getenv("SONUS_ASR_URL")
	if base == "" {
		base = "http://127.0.0.1:9101"
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
	case "paths":
		err = cliutil.PathsCmd()
	case "service":
		err = cliutil.Service(base, os.Args[2:])
	case "jobs":
		err = cliutil.JobCmd(base, os.Args[2:])
	case "transcribe":
		err = transcribeCmd(base, os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "aurisctl version|status|paths|service status|load <model>|unload|models list|show <id>|use <id>|config get|set|reset|jobs list|status|wait|retry|transcribe <archivo> [--out DIR] [--async]")
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
			State: modeldl.Describe(modeldl.Status(cfg.Paths.Models, m.ID)),
		})
	}
	show := func(id string) (any, bool) {
		m, ok := catalog.Get(id)
		return m, ok
	}
	set := func(c *config.Config, id string) { c.Defaults.ASR = id }
	install := func(id string, force bool) error {
		m, ok := catalog.Get(id)
		if !ok || m.Archive.URL == "" {
			return fmt.Errorf("%s no tiene descarga", id)
		}
		return modeldl.Install(cfg.Paths.Models, id, m.Archive.URL, m.Archive.SizeBytes, m.Archive.SHA256, int64(m.DiskMB)*3*1024*1024/2, force)
	}
	remove := func(id string) error { return modeldl.Remove(cfg.Paths.Models, id) }
	return cliutil.RunModels(args, cfgPath, cfg.Defaults.ASR, rows, show, set, install, remove)
}

func configCmd(args []string) error {
	validate := func(key, value string) error {
		switch key {
		case "variant":
			if value != "cpu" && value != "cuda" && value != "intel" && value != "amd" {
				return fmt.Errorf("variant inválida: %s", value)
			}
		case "defaults.asr":
			if _, ok := catalog.Get(value); !ok {
				return fmt.Errorf("modelo ASR desconocido: %s", value)
			}
		case "defaults.tts":
			if _, ok := voxcatalog.Get(value); !ok {
				return fmt.Errorf("modelo TTS desconocido: %s", value)
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

func prepareAudio(path string) (string, func(), error) {
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		return path, func() {}, nil
	}
	tmp, err := os.CreateTemp("", "auris-*.wav")
	if err != nil {
		return "", func() {}, err
	}
	name := tmp.Name()
	tmp.Close()
	cmd := exec.Command("ffmpeg", "-y", "-i", path, "-ar", "16000", "-ac", "1", "-f", "wav", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(name)
		return "", func() {}, fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return name, func() { os.Remove(name) }, nil
}

func transcribeCmd(base string, args []string) error {
	fs := flag.NewFlagSet("transcribe", flag.ContinueOnError)
	out := fs.String("out", "", "output directory")
	async := fs.Bool("async", false, "return a job id instead of waiting for text")
	flags, positional := cliutil.SplitFlags(args, map[string]bool{"--out": true, "-out": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if len(positional) != 1 {
		return fmt.Errorf("uso: aurisctl transcribe <archivo> [--out DIR]")
	}
	if *async {
		return transcribeAsync(base, positional[0])
	}
	return transcribe(base, positional[0], *out)
}

func transcribeAsync(base, path string) error {
	path, cleanup, err := prepareAudio(path)
	if err != nil {
		return err
	}
	defer cleanup()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("audio", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/transcribe/jobs", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auris: %s: %s", resp.Status, string(raw))
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

func transcribe(base, path, out string) error {
	path, cleanup, err := prepareAudio(path)
	if err != nil {
		return err
	}
	defer cleanup()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("audio", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/transcribe", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("auris: %s: %s", resp.Status, string(raw))
	}
	var result struct {
		Text string `json:"text"`
		Lang string `json:"lang"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("auris: respuesta inválida: %w", err)
	}
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
		name := filepath.Join(out, filepath.Base(path)+".txt")
		if err := os.WriteFile(name, []byte(result.Text+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Println(name)
		return nil
	}
	fmt.Println(result.Text)
	return nil
}
