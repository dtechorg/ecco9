// Package server wires the consciousness core into a REST service exposing
// the ConsciousnessService contract (PublishThought, StreamConsciousness).
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dtechorg/ecco9-consciousness/consciousness"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the stream of consciousness, layer hub, and health.
type Service struct {
	Stream *consciousness.Stream
	Hub    *consciousness.Hub
	Health *health.Reporter
}

// New constructs the consciousness service and starts its background loops.
func New() *Service {
	s := &Service{
		Stream: consciousness.NewStream(),
		Hub:    consciousness.NewHub(),
		Health: health.NewReporter(),
	}
	// Delivered inter-layer messages become layer_message stream events.
	s.Hub.SetMessageCallback(func(msg *consciousness.LayerMessage) {
		m := *msg
		s.Stream.BroadcastEvent(consciousness.StreamEvent{LayerMessage: &m})
	})
	s.Stream.Start()
	s.Hub.Start()
	return s
}

// Routes returns the REST adapter for the ConsciousnessService gRPC contract.
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// PublishThought
	mux.HandleFunc("POST /v1/consciousness/thoughts", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Thought consciousness.Thought `json:"thought"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		accepted := s.Stream.Publish(req.Thought)
		s.Health.SetLoad(float64(len(s.Stream.RecentThoughts(100))) / 100.0)
		writeJSON(w, map[string]any{"accepted": accepted})
	})

	// StreamConsciousness as Server-Sent Events (REST form of the server
	// stream). Events carry the oneof payload: thought or layer_message.
	mux.HandleFunc("GET /v1/consciousness/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		_ = r.URL.Query().Get("identity_id") // mirrors StreamConsciousnessRequest

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := s.Stream.Subscribe()
		defer s.Stream.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case ev := <-ch:
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	// Long-poll fallback: recent thoughts and inter-layer messages.
	mux.HandleFunc("GET /v1/consciousness/recent", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"thoughts":       s.Stream.RecentThoughts(20),
			"layer_messages": s.Hub.RecentMessages(20),
		})
	})

	// Send an inter-layer message (drives StreamConsciousness layer payloads).
	mux.HandleFunc("POST /v1/consciousness/layers/messages", func(w http.ResponseWriter, r *http.Request) {
		var msg consciousness.LayerMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.Hub.Send(&msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{"delivered": true})
	})

	// Metrics.
	mux.HandleFunc("GET /v1/consciousness/metrics", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"stream": s.Stream.Metrics(),
			"hub":    s.Hub.Metrics(),
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
