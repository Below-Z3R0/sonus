// Package cliutil holds the CLI behavior shared by aurisctl and voxctl.
package cliutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Below-Z3R0/sonus/internal/config"
)

// SplitFlags separates flags from positionals. The standard flag package stops
// at the first positional, which breaks `tool transcribe file --out DIR`.
func SplitFlags(args []string, withValue map[string]bool) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		if withValue[args[i]] && i+1 < len(args) {
			flags = append(flags, args[i], args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			continue
		}
		positional = append(positional, args[i])
	}
	return flags, positional
}

// Get reads a small JSON/text response and rejects non-2xx status codes.
func Get(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func PrintGet(url string, timeout time.Duration) error {
	raw, err := Get(url, timeout)
	if err != nil {
		return err
	}
	os.Stdout.Write(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// ModelRow is the minimal data a CLI needs to list a model.
type ModelRow struct {
	ID    string
	Tier  string
	Extra string
}

// RunModels implements list/show/use for one service catalogue.
func RunModels(args []string, cfgPath, defaultID string, rows []ModelRow, show func(string) (any, bool), set func(*config.Config, string)) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		fmt.Println("ID\tTIER\tDEFAULT\tEXTRA")
		for _, row := range rows {
			def := ""
			if row.ID == defaultID {
				def = "*"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", row.ID, row.Tier, def, row.Extra)
		}
		return nil
	}
	switch args[0] {
	case "show":
		if len(args) != 2 {
			return errors.New("uso: models show <id>")
		}
		item, ok := show(args[1])
		if !ok {
			return fmt.Errorf("modelo %s no existe", args[1])
		}
		out, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	case "use":
		if len(args) != 2 {
			return errors.New("uso: models use <id>")
		}
		if _, ok := show(args[1]); !ok {
			return fmt.Errorf("modelo %s no existe", args[1])
		}
		set(&cfg, args[1])
		if err := cfg.Save(cfgPath); err != nil {
			return err
		}
		fmt.Printf("default = %s (reiniciá el servicio para cargarlo)\n", args[1])
	default:
		return errors.New("uso: models list|show <id>|use <id>")
	}
	return nil
}
