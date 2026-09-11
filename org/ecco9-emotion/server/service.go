// Package server wires the embodied emotion system into a REST service
// exposing the EmotionService contract (ecco9.emotion.v1).
package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/dtechorg/ecco9-emotion/emotion"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the per-identity emotion systems, the AAR core, and the
// health reporter.
type Service struct {
	mu      sync.RWMutex
	systems map[string]*emotion.System // identityID -> emotion system
	AAR     *emotion.AARCore
	Health  *health.Reporter
	stop    chan struct{}
}

// New constructs the service and starts the background decay/tick loop.
func New(dimensions int) *Service {
	s := &Service{
		systems: make(map[string]*emotion.System),
		AAR:     emotion.NewAARCore(dimensions),
		Health:  health.NewReporter(),
		stop:    make(chan struct{}),
	}
	go s.loop()
	return s
}

// systemFor returns (creating if needed) the emotion system for an identity.
func (s *Service) systemFor(identityID string) *emotion.System {
	if identityID == "" {
		identityID = "deep-tree-echo"
	}
	s.mu.RLock()
	sys, ok := s.systems[identityID]
	s.mu.RUnlock()
	if ok {
		return sys
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sys, ok = s.systems[identityID]; ok {
		return sys
	}
	sys = emotion.NewSystem()
	s.systems[identityID] = sys
	return sys
}

// loop decays emotions and advances the AAR dynamics at 10 Hz.
func (s *Service) loop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			dt := now.Sub(last)
			last = now
			s.mu.RLock()
			var arousal, valence float64
			for _, sys := range s.systems {
				sys.Update(dt)
				_, a, v, _, _ := sys.State()
				arousal, valence = a, v
			}
			s.mu.RUnlock()
			s.AAR.Tick(arousal, valence)
			coherence, _, _, _ := s.AAR.Metrics()
			s.Health.SetCoherence(coherence)
		}
	}
}

// Close stops the background loop.
func (s *Service) Close() { close(s.stop) }

// Routes exposes the REST adapter for ecco9.emotion.v1.EmotionService.
// The gRPC server is generated from ecco9-proto; this REST surface keeps
// the service runnable in the offline sandbox and for simple clients.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// rpc Feel
	mux.HandleFunc("POST /v1/emotion/feel", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IdentityID string `json:"identity_id"`
			Emotion    struct {
				Type      string  `json:"type"`
				Intensity float64 `json:"intensity"`
			} `json:"emotion"`
			Trigger string `json:"trigger"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sys := s.systemFor(req.IdentityID)
		current := sys.Feel(emotion.ParseEmotionType(req.Emotion.Type), req.Emotion.Intensity, req.Trigger)
		writeJSON(w, map[string]any{"current": current})
	})

	// rpc GetEmotionalState
	mux.HandleFunc("POST /v1/emotion/state", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IdentityID string `json:"identity_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sys := s.systemFor(req.IdentityID)
		dominant, arousal, valence, blend, fx := sys.State()
		coherence, stability, awareness, self := s.AAR.Metrics()
		writeJSON(w, map[string]any{
			"current":           dominant,
			"arousal":           arousal,
			"valence":           valence,
			"emotion_blend":     blend,
			"cognitive_effects": fx,
			"aar": map[string]any{
				"coherence":  coherence,
				"stability":  stability,
				"awareness":  awareness,
				"self_vector": self,
			},
		})
	})

	// rpc StreamEmotions (server-streaming in gRPC) — chunked JSON over HTTP:
	// emits one EmotionalState line per second until the client disconnects.
	mux.HandleFunc("POST /v1/emotion/stream", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IdentityID string `json:"identity_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sys := s.systemFor(req.IdentityID)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				dominant, arousal, valence, _, _ := sys.State()
				_ = json.NewEncoder(w).Encode(map[string]any{
					"state":   dominant,
					"arousal": arousal,
					"valence": valence,
				})
				flusher.Flush()
			}
		}
	})

	// AAR introspection: register a goal with the Agent component.
	mux.HandleFunc("POST /v1/emotion/aar/goal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Goal string `json:"goal"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Goal == "" {
			http.Error(w, "invalid goal", http.StatusBadRequest)
			return
		}
		s.AAR.AddGoal(req.Goal)
		coherence, stability, awareness, _ := s.AAR.Metrics()
		writeJSON(w, map[string]any{
			"added": req.Goal, "coherence": coherence,
			"stability": stability, "awareness": awareness,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
