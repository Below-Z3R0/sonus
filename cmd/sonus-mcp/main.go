// Command sonus-mcp exposes Auris and Vox over the Model Context Protocol.
// The tools talk to the resident daemons; they never load a model themselves.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type daemon struct {
	name string
	base string
	http *http.Client
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9201", "MCP listen address")
	voxURL := flag.String("vox", "http://127.0.0.1:9102", "voxd base URL")
	aurisURL := flag.String("auris", "http://127.0.0.1:9101", "aurisd base URL")
	readOnly := flag.Bool("read-only", false, "disable write tools")
	flag.Parse()

	vox := &daemon{name: "vox", base: *voxURL, http: &http.Client{Timeout: 2 * time.Minute}}
	auris := &daemon{name: "auris", base: *aurisURL, http: &http.Client{Timeout: 10 * time.Minute}}

	srv := newServer(vox, auris, *readOnly)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	fmt.Printf("sonus-mcp on %s (vox %s, auris %s)\n", *addr, *voxURL, *aurisURL)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServer(vox, auris *daemon, readOnly bool) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "sonus", Version: "0.1.0"}, nil)
	registerStatus(srv, vox)
	registerStatus(srv, auris)
	registerModels(srv, vox)
	registerModels(srv, auris)
	registerJobs(srv, vox)
	registerJobs(srv, auris)
	if !readOnly {
		mcp.AddTool(srv, &mcp.Tool{Name: "vox_speak", Description: "Queue text-to-speech with Kokoro; returns a job id"}, voxSpeak(vox))
		mcp.AddTool(srv, &mcp.Tool{Name: "auris_transcribe", Description: "Transcribe an audio file with Whisper; returns text or a job id"}, aurisTranscribe(auris))
		mcp.AddTool(srv, &mcp.Tool{Name: "vox_load", Description: "Load a Kokoro model into voxd"}, loadTool(vox))
		mcp.AddTool(srv, &mcp.Tool{Name: "auris_load", Description: "Load a Whisper model into aurisd"}, loadTool(auris))
		mcp.AddTool(srv, &mcp.Tool{Name: "vox_unload", Description: "Unload the resident Kokoro model"}, unloadTool(vox))
		mcp.AddTool(srv, &mcp.Tool{Name: "auris_unload", Description: "Unload the resident Whisper model"}, unloadTool(auris))
	}
	return srv
}

type statusInput struct{}

func registerStatus(srv *mcp.Server, d *daemon) {
	mcp.AddTool(srv, &mcp.Tool{Name: d.name + "_status", Description: "Resident model and service status"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.get(ctx, "/v1/status"))
	})
	mcp.AddTool(srv, &mcp.Tool{Name: d.name + "_doctor", Description: "Model directory, free disk, available RAM and engine health"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.get(ctx, "/v1/doctor"))
	})
}

type modelsInput struct{}

func registerModels(srv *mcp.Server, d *daemon) {
	mcp.AddTool(srv, &mcp.Tool{Name: d.name + "_models", Description: "List the model catalogue with install state"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ modelsInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.get(ctx, "/v1/models"))
	})
}

type jobInput struct {
	ID string `json:"id" jsonschema:"job id returned by a write tool"`
}

func registerJobs(srv *mcp.Server, d *daemon) {
	mcp.AddTool(srv, &mcp.Tool{Name: d.name + "_jobs", Description: "List recent jobs for this service"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.get(ctx, "/v1/jobs"))
	})
	mcp.AddTool(srv, &mcp.Tool{Name: d.name + "_job", Description: "Get one job by id"}, func(ctx context.Context, _ *mcp.CallToolRequest, in jobInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.get(ctx, "/v1/jobs/"+in.ID))
	})
}

type speakInput struct {
	Text  string `json:"text" jsonschema:"text to synthesize"`
	Lang  string `json:"lang,omitempty" jsonschema:"es, en or auto; default auto"`
	Voice string `json:"voice,omitempty" jsonschema:"optional Kokoro voice id"`
}

func voxSpeak(d *daemon) func(context.Context, *mcp.CallToolRequest, speakInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in speakInput) (*mcp.CallToolResult, any, error) {
		body, _ := json.Marshal(map[string]string{"text": in.Text, "lang": in.Lang, "voice": in.Voice})
		return textResult(d.post(ctx, "/v1/speech/jobs", "application/json", body))
	}
}

type transcribeInput struct {
	Path string `json:"path" jsonschema:"absolute path to a WAV file on this host"`
}

func aurisTranscribe(d *daemon) func(context.Context, *mcp.CallToolRequest, transcribeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in transcribeInput) (*mcp.CallToolResult, any, error) {
		f, err := os.Open(in.Path)
		if err != nil {
			return errResult(err), nil, nil
		}
		defer f.Close()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, err := mw.CreateFormFile("audio", filepath.Base(in.Path))
		if err != nil {
			return errResult(err), nil, nil
		}
		if _, err := io.Copy(part, f); err != nil {
			return errResult(err), nil, nil
		}
		if err := mw.Close(); err != nil {
			return errResult(err), nil, nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+"/v1/transcribe", &body)
		if err != nil {
			return errResult(err), nil, nil
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return textResult(d.do(req))
	}
}

type loadInput struct {
	Model string `json:"model" jsonschema:"catalogue model id"`
}

func loadTool(d *daemon) func(context.Context, *mcp.CallToolRequest, loadInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in loadInput) (*mcp.CallToolResult, any, error) {
		body, _ := json.Marshal(map[string]string{"model": in.Model})
		return textResult(d.post(ctx, "/v1/models/load", "application/json", body))
	}
}

func unloadTool(d *daemon) func(context.Context, *mcp.CallToolRequest, statusInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		return textResult(d.post(ctx, "/v1/models/unload", "application/json", []byte(`{"model":""}`)))
	}
}

func (d *daemon) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+path, nil)
	if err != nil {
		return nil, err
	}
	return d.do(req)
}

func (d *daemon) post(ctx context.Context, path, contentType string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return d.do(req)
}

func (d *daemon) do(req *http.Request) ([]byte, error) {
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s: %s", d.name, resp.Status, string(raw))
	}
	return raw, nil
}

func textResult(raw []byte, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return errResult(err), nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
}
