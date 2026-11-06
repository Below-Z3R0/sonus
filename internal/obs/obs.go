// Package obs keeps the small request counters exposed by both daemons.
package obs

import (
	"net/http"
	"sync/atomic"
	"time"
)

type Stats struct {
	Requests   atomic.Uint64
	Errors     atomic.Uint64
	DurationMS atomic.Int64
}

var Default Stats

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		Default.Requests.Add(1)
		next.ServeHTTP(rec, r)
		Default.DurationMS.Add(time.Since(start).Milliseconds())
		if rec.status >= http.StatusBadRequest {
			Default.Errors.Add(1)
		}
	})
}

func Snapshot() map[string]any {
	return map[string]any{
		"requests":    Default.Requests.Load(),
		"errors":      Default.Errors.Load(),
		"duration_ms": Default.DurationMS.Load(),
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
