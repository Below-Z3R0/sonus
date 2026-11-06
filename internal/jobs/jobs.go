// Package jobs keeps a bounded registry of long-running voice operations.
package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type State string

const (
	Queued  State = "queued"
	Running State = "running"
	Done    State = "done"
	Failed  State = "failed"
)

type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	State      State     `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`
	Result     any       `json:"result,omitempty"`
}

type Manager struct {
	mu    sync.Mutex
	jobs  map[string]*Job
	order []string
	max   int
	seq   uint64
}

func New(max int) *Manager {
	if max <= 0 {
		max = 100
	}
	return &Manager{jobs: map[string]*Job{}, max: max}
}

// Start returns the job id immediately and runs fn without the request's
// cancellation, so a dropped HTTP client does not kill the work.
func (m *Manager) Start(kind string, ctx context.Context, fn func(context.Context) (any, error)) string {
	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano(), m.seq)
	job := &Job{ID: id, Kind: kind, State: Queued, CreatedAt: time.Now().UTC()}
	m.jobs[id] = job
	m.order = append(m.order, id)
	m.evictLocked()
	m.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.mu.Lock()
				job.State = Failed
				job.Error = fmt.Sprintf("panic: %v", r)
				job.FinishedAt = time.Now().UTC()
				m.evictLocked()
				m.mu.Unlock()
			}
		}()

		m.mu.Lock()
		job.State = Running
		job.StartedAt = time.Now().UTC()
		m.mu.Unlock()

		result, err := fn(context.WithoutCancel(ctx))

		m.mu.Lock()
		job.FinishedAt = time.Now().UTC()
		if err != nil {
			job.State = Failed
			job.Error = err.Error()
		} else {
			job.State = Done
			job.Result = result
		}
		m.evictLocked()
		m.mu.Unlock()
	}()
	return id
}

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, *m.jobs[id])
	}
	return out
}

// evictLocked keeps the newest terminal jobs; running jobs are never evicted.
func (m *Manager) evictLocked() {
	for len(m.order) > m.max {
		removed := false
		for i, id := range m.order {
			j := m.jobs[id]
			if j.State == Queued || j.State == Running {
				continue
			}
			delete(m.jobs, id)
			m.order = append(m.order[:i], m.order[i+1:]...)
			removed = true
			break
		}
		if !removed {
			return
		}
	}
}
