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
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "status":
		get(base + "/v1/status")
	case "models":
		get(base + "/v1/models")
	case "transcribe":
		fs := flag.NewFlagSet("transcribe", flag.ExitOnError)
		out := fs.String("out", "", "output file (empty = stdout)")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "uso: aurisctl transcribe <archivo> [--out DIR]")
			os.Exit(1)
		}
		transcribe(base, fs.Arg(0), *out)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "aurisctl version|status|models|transcribe <archivo> [--out DIR]")
}

func get(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
}

func transcribe(base, path, out string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("audio", filepath.Base(path))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := io.Copy(part, f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mw.Close()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/transcribe", &body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != "" {
		_ = os.MkdirAll(out, 0o755)
		name := filepath.Join(out, filepath.Base(path)+".txt")
		_ = os.WriteFile(name, raw, 0o644)
		fmt.Println(name)
		return
	}
	os.Stdout.Write(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		fmt.Println()
	}
	_ = json.Valid
}
