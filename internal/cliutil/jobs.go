package cliutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Job struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
}

// JobCmd implements `jobs list|status|wait` against the shared job registry.
func JobCmd(base string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: jobs list|status <id>|wait <id>")
	}
	switch args[0] {
	case "list":
		return PrintGet(base+"/v1/jobs", 10*time.Second)
	case "status":
		if len(args) != 2 {
			return fmt.Errorf("uso: jobs status <id>")
		}
		return printJob(base, args[1])
	case "retry":
		if len(args) != 2 {
			return fmt.Errorf("uso: jobs retry <id>")
		}
		return retryJob(base, args[1])
	case "wait":
		if len(args) != 2 {
			return fmt.Errorf("uso: jobs wait <id>")
		}
		return waitJob(base, args[1])
	default:
		return fmt.Errorf("uso: jobs list|status <id>|wait <id>")
	}
}

func printJob(base, id string) error {
	raw, err := Get(base+"/v1/jobs/"+id, 10*time.Second)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func retryJob(base, id string) error {
	body, _ := json.Marshal(map[string]string{"id": id})
	resp, err := http.Post(base+"/v1/jobs/retry", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("job: %s: %s", resp.Status, string(raw))
	}
	fmt.Println(string(raw))
	return nil
}

func waitJob(base, id string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/v1/jobs/" + id)
		if err != nil {
			return err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("job: %s: %s", resp.Status, string(raw))
		}
		var job Job
		if err := json.Unmarshal(raw, &job); err != nil {
			return err
		}
		if job.State == "done" || job.State == "failed" {
			fmt.Println(string(raw))
			if job.State == "failed" {
				return fmt.Errorf("job %s falló: %s", id, job.Error)
			}
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("job %s no terminó en 30 minutos", id)
}
