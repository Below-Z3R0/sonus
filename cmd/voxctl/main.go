package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Below-Z3R0/sonus/internal/config"
	"github.com/Below-Z3R0/sonus/internal/vox/catalog"
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
		cfg, _ := config.Load(config.FilePath())
		model := catalog.Default()
		if m, ok := catalog.Get(cfg.Defaults.TTS); ok {
			model = m
		}
		out, _ := json.MarshalIndent(model.Voices, "", "  ")
		fmt.Println(string(out))
	case "models":
		modelsCmd(os.Args[2:])
	case "speak":
		fs := flag.NewFlagSet("speak", flag.ExitOnError)
		file := fs.String("file", "", "read text from a file")
		out := fs.String("out", "", "output audio file")
		voice := fs.String("voice", "ef_dora", "voice id")
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
	fmt.Fprintln(os.Stderr, "voxctl version|status|voices|models list|show <id>|use <id>|speak \"texto\" [--file F] [--voice V] [--out F]")
}

func modelsCmd(args []string) {
	cfgPath := config.FilePath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(args) == 0 || args[0] == "list" {
		fmt.Println("ID	TIER	DEFAULT	VOICES")
		for _, m := range catalog.List() {
			def := ""
			if m.ID == cfg.Defaults.TTS {
				def = "*"
			}
			fmt.Printf("%s	%s	%s	%d\n", m.ID, m.Tier, def, len(m.Voices))
		}
		return
	}
	switch args[0] {
	case "show":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "uso: voxctl models show <id>")
			os.Exit(1)
		}
		m, ok := catalog.Get(args[1])
		if !ok {
			fmt.Fprintf(os.Stderr, "modelo %s no existe\n", args[1])
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(out))
	case "use":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "uso: voxctl models use <id>")
			os.Exit(1)
		}
		if _, ok := catalog.Get(args[1]); !ok {
			fmt.Fprintf(os.Stderr, "modelo %s no existe\n", args[1])
			os.Exit(1)
		}
		cfg.Defaults.TTS = args[1]
		if err := cfg.Save(cfgPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Vox default = %s (reiniciá el servicio para cargarlo)\n", args[1])
	default:
		usage()
		os.Exit(1)
	}
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
