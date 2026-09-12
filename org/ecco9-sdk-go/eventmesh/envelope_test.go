package eventmesh

import (
	"strings"
	"testing"
	"time"
)

func TestNewEnvelope(t *testing.T) {
	before := time.Now().UTC()
	env := New("//ecco9/reservoir/instance-0", "ecco9.echo.memories.v1", "identity-1", []byte("payload"))
	if env.ID == "" {
		t.Fatal("envelope ID must be generated")
	}
	if env.SpecVersion != "1.0" {
		t.Fatalf("spec version = %q, want 1.0 (CloudEvents)", env.SpecVersion)
	}
	if env.DataContentType != "application/protobuf" {
		t.Fatalf("content type = %q", env.DataContentType)
	}
	if env.Time.Before(before) || env.Time.After(time.Now().UTC()) {
		t.Fatalf("envelope time %v out of expected range", env.Time)
	}
	if string(env.Data) != "payload" {
		t.Fatalf("data = %q", env.Data)
	}
}

func TestEnvelopeIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		env := New("s", "t", "sub", nil)
		if seen[env.ID] {
			t.Fatalf("duplicate envelope ID %q", env.ID)
		}
		seen[env.ID] = true
		if len(env.ID) != 32 {
			t.Fatalf("ID %q is not 16 bytes hex", env.ID)
		}
	}
}

func TestTopicTaxonomy(t *testing.T) {
	// The taxonomy mirrors cognitive pathways; all topics share the echo. prefix.
	topics := []string{
		TopicThoughts, TopicEmotions, TopicMemories, TopicDreams,
		TopicResonance, TopicTraining, TopicMetrics, TopicDirectives,
	}
	want := map[string]bool{
		"echo.thoughts": true, "echo.emotions": true, "echo.memories": true,
		"echo.dreams": true, "echo.resonance": true, "echo.training": true,
		"echo.metrics": true, "echo.directives": true,
	}
	for _, topic := range topics {
		if !want[topic] {
			t.Fatalf("unexpected topic %q", topic)
		}
		if !strings.HasPrefix(topic, "echo.") {
			t.Fatalf("topic %q missing echo. prefix", topic)
		}
		delete(want, topic)
	}
	if len(want) != 0 {
		t.Fatalf("missing topics: %v", want)
	}
}
