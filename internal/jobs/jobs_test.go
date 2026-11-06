package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	m := New(10)
	id := m.Start("test", context.Background(), func(context.Context) (any, error) {
		return "ok", nil
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j, ok := m.Get(id)
		if ok && j.State == Done {
			if j.Result != "ok" {
				t.Fatalf("result = %#v", j.Result)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}

func TestFailure(t *testing.T) {
	m := New(10)
	id := m.Start("test", context.Background(), func(context.Context) (any, error) {
		return nil, errors.New("boom")
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j, ok := m.Get(id); ok && j.State == Failed {
			if j.Error != "boom" {
				t.Fatalf("error = %q", j.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not fail")
}

func TestBounded(t *testing.T) {
	m := New(2)
	for i := 0; i < 5; i++ {
		id := m.Start("test", context.Background(), func(context.Context) (any, error) {
			return i, nil
		})
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if j, ok := m.Get(id); ok && j.State == Done {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if got := len(m.List()); got > 2 {
		t.Fatalf("retained %d jobs", got)
	}
}
