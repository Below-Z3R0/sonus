package cliutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/sonus/internal/config"
)

// PathsCmd prints the resolved state directories so the operator can confirm
// where models, data and output live before touching a daemon.
func PathsCmd() error {
	cfg, err := config.Load(config.FilePath())
	if err != nil {
		return err
	}
	rows := []struct {
		name string
		path string
	}{
		{"models", cfg.Paths.Models},
		{"data", cfg.Paths.Data},
		{"output", cfg.Paths.Output},
		{"media", cfg.Paths.Media},
	}
	root := filepath.Dir(cfg.Paths.Models)
	fmt.Printf("root    %s\n", root)
	for _, r := range rows {
		state := "missing"
		if _, err := os.Stat(r.path); err == nil {
			state = "ok"
		}
		fmt.Printf("%-8s %-12s %s\n", r.name, state, r.path)
	}
	return nil
}
