package cliutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service runs the shared `service status|load|unload` commands.
func Service(base string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: service status|load [model]|unload")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	switch args[0] {
	case "status":
		raw, err := Get(base+"/v1/status", 10*time.Second)
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	case "load":
		if len(args) != 2 {
			return fmt.Errorf("uso: service load <model>")
		}
		return postModel(client, base+"/v1/models/load", args[1])
	case "unload":
		return postModel(client, base+"/v1/models/unload", "")
	default:
		return fmt.Errorf("uso: service status|load [model]|unload")
	}
}

func postModel(client *http.Client, url, model string) error {
	body, _ := json.Marshal(map[string]string{"model": model})
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s", resp.Status)
	}
	return nil
}
