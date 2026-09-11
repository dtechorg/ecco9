// Package eventmesh provides the CloudEvents-compatible envelope and the
// topic taxonomy for the NATS JetStream central nervous system.
package eventmesh

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Topic taxonomy mirroring cognitive pathways.
const (
	TopicThoughts   = "echo.thoughts"
	TopicEmotions   = "echo.emotions"
	TopicMemories   = "echo.memories"
	TopicDreams     = "echo.dreams"
	TopicResonance  = "echo.resonance"
	TopicTraining   = "echo.training"
	TopicMetrics    = "echo.metrics"
	TopicDirectives = "echo.directives"
)

// Envelope mirrors ecco9.events.v1.EventEnvelope (CloudEvents 1.0).
type Envelope struct {
	ID              string
	Source          string
	Type            string
	SpecVersion     string
	Subject         string
	Time            time.Time
	DataContentType string
	IdentityID      string
	RelevanceScore  float64
	EchoSignature   float64
	Data            []byte
}

// New constructs an envelope with a generated ID and current time.
func New(source, eventType, subject string, data []byte) *Envelope {
	return &Envelope{
		ID:              newID(),
		Source:          source,
		Type:            eventType,
		SpecVersion:     "1.0",
		Subject:         subject,
		Time:            time.Now().UTC(),
		DataContentType: "application/protobuf",
		Data:            data,
	}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	// Set version 4 and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}

// Publisher is the minimal abstraction over the event mesh, allowing the SDK
// to compile without a hard NATS dependency. A NATS JetStream implementation
// is provided in the eventmesh/nats subpackage of each deployment.
type Publisher interface {
	Publish(topic string, env *Envelope) error
}

// Subscriber consumes events from a topic with horizontal scaling support.
type Subscriber interface {
	Subscribe(topic, consumerGroup string, handler func(*Envelope)) error
}
