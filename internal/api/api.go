// Package api centralizes the HTTP surface shared by Auris and Vox.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Below-Z3R0/sonus/internal/obs"
)

func Handler(status func() any, models any) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, status())
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, models)
	})
	return mux
}

// RegisterMetrics exposes the counters collected by obs.Middleware.
func RegisterMetrics(mux *http.ServeMux, snapshot func() map[string]any) {
	mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, snapshot())
	})
}

// RegisterDoctor exposes service-specific diagnostics without failing the
// request when an optional dependency is down.
func RegisterDoctor(mux *http.ServeMux, doctor func() any) {
	mux.HandleFunc("/v1/doctor", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, doctor())
	})
}

func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Serve runs an HTTP server with explicit timeouts and graceful shutdown.
func Serve(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           obs.Middleware(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("shutdown: %w", err)
		}
		log.Printf("%s stopped", addr)
		return nil
	}
}
