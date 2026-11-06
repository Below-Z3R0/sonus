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
	"path/filepath"
	"time"

	"github.com/Below-Z3R0/sonus/internal/auris/catalog"
	"github.com/Below-Z3R0/sonus/internal/cliutil"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/modeldl"
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
	case "service":
		err = cliutil.Service(base, os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "aurisctl version|status|service status|load <model>|unload|models list|show <id>|use <id>|transcribe <archivo> [--out DIR]")
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

func transcribeCmd(base string, args []string) error {
	fs := flag.NewFlagSet("transcribe", flag.ContinueOnError)
	out := fs.String("out", "", "output directory")
	flags, positional := cliutil.SplitFlags(args, map[string]bool{"--out": true, "-out": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if len(positional) != 1 {
		return fmt.Errorf("uso: aurisctl transcribe <archivo> [--out DIR]")
	}
	return transcribe(base, positional[0], *out)
}

func transcribe(base, path, out string) error {
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
