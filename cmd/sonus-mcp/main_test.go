package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerTools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/status":
			w.Write([]byte(`{"state":"ready"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	d := &daemon{name: "vox", base: ts.URL, http: http.DefaultClient}
	srv := newServer(d, d, false)
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "vox_status", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "ready") {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestReadOnly(t *testing.T) {
	d := &daemon{name: "vox", base: "http://127.0.0.1:1", http: http.DefaultClient}
	srv := newServer(d, d, true)
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "vox_speak" || tool.Name == "vox_load" {
			t.Fatalf("write tool exposed in read-only mode: %s", tool.Name)
		}
	}
}
