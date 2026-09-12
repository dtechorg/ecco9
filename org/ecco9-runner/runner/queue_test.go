package runner

import (
	"context"
	"testing"
	"time"
)

func TestQueueMissingModelClosesChannels(t *testing.T) {
	q := NewQueue(1, 1, &StubBackend{}, NewRegistry())
	job := &Job{
		Req:     &GenerateRequest{Model: "missing"},
		Results: make(chan GenerateResponse, 1),
		Err:     make(chan error, 1),
	}
	if err := q.Submit(context.Background(), job); err != nil {
		t.Fatalf("submit: %v", err)
	}

	select {
	case err, ok := <-job.Err:
		if !ok || err == nil {
			t.Fatalf("expected a model resolution error, got ok=%v err=%v", ok, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queue error")
	}

	select {
	case _, ok := <-job.Results:
		if ok {
			t.Fatal("results channel remained open after terminal error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for results channel closure")
	}
}
