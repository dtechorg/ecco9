// Package server wires the gateway core into a REST service mirroring the
// LLMGatewayService proto contract (Generate, GenerateStream,
// GetProviderStatus).
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/dtechorg/ecco9-llm-gateway/gateway"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the gateway with health reporting.
type Service struct {
	Gateway *gateway.Gateway
	Health  *health.Reporter
}

// New constructs the service from environment-configured backends.
func New() (*Service, error) {
	g, err := gateway.New(gateway.DefaultBackends(os.Getenv), &gateway.StubTransport{})
	if err != nil {
		return nil, err
	}
	return &Service{Gateway: g, Health: health.NewReporter()}, nil
}

// Routes returns the REST adapter for the gRPC contracts defined in
// ecco9/llm/v1/llm.proto.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// LLMGatewayService.Generate
	mux.HandleFunc("POST /v1/llm/generate", func(w http.ResponseWriter, r *http.Request) {
		var req gateway.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Health.SetLoad(0.5)
		resp, err := s.Gateway.Generate(r.Context(), &req)
		s.Health.SetLoad(0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, resp)
	})

	// LLMGatewayService.GenerateStream: single buffered chunk for the stub
	// transport; a streaming transport would flush incrementally.
	mux.HandleFunc("POST /v1/llm/generate/stream", func(w http.ResponseWriter, r *http.Request) {
		var req gateway.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Stream = true
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/x-ndjson")
		resp, err := s.Gateway.Generate(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		resp.Done = false
		_ = json.NewEncoder(w).Encode(resp)
		if flusher != nil {
			flusher.Flush()
		}
		resp.Done = true
		resp.Content = ""
		_ = json.NewEncoder(w).Encode(resp)
	})

	// LLMGatewayService.GetProviderStatus
	mux.HandleFunc("GET /v1/llm/providers/status", func(w http.ResponseWriter, _ *http.Request) {
		healthMap, latency := s.Gateway.ProviderStatus()
		writeJSON(w, map[string]any{
			"provider_health":     healthMap,
			"provider_latency_ms": latency,
			"current_provider":    s.Gateway.CurrentProvider(),
			"stats":               s.Gateway.Stats(),
			"timestamp":           time.Now(),
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
