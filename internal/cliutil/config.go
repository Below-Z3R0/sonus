package cliutil

import (
	"fmt"

	"github.com/Below-Z3R0/sonus/internal/config"
)

// ConfigCmd implements `config get|set|reset` with a caller-owned validator.
func ConfigCmd(path string, args []string, validate func(string, string) error) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: config get <key>|set <key> <value>|reset <key>")
	}
	switch args[0] {
	case "get":
		if len(args) != 2 || !config.IsKey(args[1]) {
			return fmt.Errorf("clave inválida; usar: %v", config.Keys)
		}
		v, ok, err := config.Get(path, args[1])
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("<unset>")
			return nil
		}
		fmt.Println(v)
		return nil
	case "set":
		if len(args) != 3 || !config.IsKey(args[1]) {
			return fmt.Errorf("clave inválida; usar: %v", config.Keys)
		}
		if validate != nil {
			if err := validate(args[1], args[2]); err != nil {
				return err
			}
		}
		return config.Set(path, args[1], args[2])
	case "reset":
		if len(args) != 2 || !config.IsKey(args[1]) {
			return fmt.Errorf("clave inválida; usar: %v", config.Keys)
		}
		return config.Reset(path, args[1])
	default:
		return fmt.Errorf("uso: config get <key>|set <key> <value>|reset <key>")
	}
}
