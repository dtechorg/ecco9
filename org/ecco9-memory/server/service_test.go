package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtechorg/ecco9-sdk-go/eventmesh"
)

// fakePublisher captures published envelopes.
type fakePublisher struct {
	topics []string
	envs   []*eventmesh.Envelope
}

func (f *fakePublisher) Publish(topic string, env *eventmesh.Envelope) error {
	f.topics = append(f.topics, topic)
	f.envs = append(f.envs, env)
	return nil
}

func TestEpisodeNodePublishedToMemoriesTopic(t *testing.T) {
	s := New("") // in-memory store
	pub := &fakePublisher{}
	s.SetPublisher(pub)

	req := httptest.NewRequest(http.MethodPost, "/v1/memory/nodes",
		strings.NewReader(`{"node":{"type":"episode","content":"a memorable event"}}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("add node = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if len(pub.topics) != 1 {
		t.Fatalf("expected 1 published episode, got %d", len(pub.topics))
	}
	if pub.topics[0] != eventmesh.TopicMemories {
		t.Fatalf("published to %q, want %q", pub.topics[0], eventmesh.TopicMemories)
	}
	if pub.envs[0].Source != "ecco9-memory" {
		t.Fatalf("envelope source = %q, want ecco9-memory", pub.envs[0].Source)
	}
}

func TestNonEpisodeNodeNotPublished(t *testing.T) {
	s := New("")
	pub := &fakePublisher{}
	s.SetPublisher(pub)

	req := httptest.NewRequest(http.MethodPost, "/v1/memory/nodes",
		strings.NewReader(`{"node":{"type":"concept","content":"an abstract idea"}}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("add node = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if len(pub.topics) != 0 {
		t.Fatalf("non-episode nodes should not be published, got %d", len(pub.topics))
	}
}

func TestNilPublisherDoesNotBreakStorage(t *testing.T) {
	s := New("") // no publisher attached (offline sandbox)
	req := httptest.NewRequest(http.MethodPost, "/v1/memory/nodes",
		strings.NewReader(`{"node":{"type":"episode","content":"offline episode"}}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("add node without publisher = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
}
