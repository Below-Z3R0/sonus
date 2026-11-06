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
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "status":
		get(base + "/v1/status")
	case "voices":
		get(base + "/v1/voices")
	case "models":
		get(base + "/v1/models")
	case "speak":
		fs := flag.NewFlagSet("speak", flag.ExitOnError)
		file := fs.String("file", "", "read text from a file")
		out := fs.String("out", "", "output audio file")
		voice := fs.String("voice", "es_female", "voice id")
		_ = fs.Parse(os.Args[2:])
		text := fs.Arg(0)
		if *file != "" {
			raw, err := os.ReadFile(*file)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			text = string(raw)
		}
		if text == "" {
			fmt.Fprintln(os.Stderr, "uso: voxctl speak \"texto\" | --file archivo")
			os.Exit(1)
		}
		speak(base, text, *voice, *out)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "voxctl version|status|voices|models|speak \"texto\" [--file F] [--voice V] [--out F]")
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

func speak(base, text, voice, out string) {
	body, _ := json.Marshal(map[string]string{"text": text, "voice": voice})
	req, err := http.NewRequest(http.MethodPost, base+"/v1/speech", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, string(raw))
		os.Exit(1)
	}
	if out == "" {
		fmt.Fprintln(os.Stderr, "audio listo (sin --out no se guarda todavía)")
		return
	}
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
