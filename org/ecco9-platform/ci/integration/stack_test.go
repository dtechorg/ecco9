//go:build integration

// Package integration implements the cross-repo integration test tier of the
// ecco9 CI/CD architecture (issue #7, section 6.2). It deploys the full
// cognitive stack into an ephemeral, in-process namespace (one httptest server
// per service, mirroring the ephemeral Kubernetes namespace used in CI) and
// runs the Deep Tree Echo test suite against it:
//
//   - cognitive_integration_test.go: full-stack 12-step EchoBeats loop and
//     feedforward/feedback event flows
//   - echo_coherence_test.go: identity coherence stays >= threshold during
//     (simulated) deployments, else automatic rollback
//   - chaos_test.go: random service kills, network partitions, and latency
//     injection against cognitive resilience
//   - contract_test.go: consumer-driven contract tests between services
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	atomspaceserver "github.com/dtechorg/ecco9-atomspace/server"
	consciousnessserver "github.com/dtechorg/ecco9-consciousness/server"
	echobeatsserver "github.com/dtechorg/ecco9-echobeats/server"
	echodreamserver "github.com/dtechorg/ecco9-echodream/server"
	emotionserver "github.com/dtechorg/ecco9-emotion/server"
	"github.com/dtechorg/ecco9-identity/identity"
	identityserver "github.com/dtechorg/ecco9-identity/server"
	memoryserver "github.com/dtechorg/ecco9-memory/server"
	metacogserver "github.com/dtechorg/ecco9-metacog/server"
	relevanceserver "github.com/dtechorg/ecco9-relevance/server"
	"github.com/dtechorg/ecco9-reservoir/reservoir"
	reservoirserver "github.com/dtechorg/ecco9-reservoir/server"
	"github.com/dtechorg/ecco9-sdk-go/eventmesh"
	wisdomserver "github.com/dtechorg/ecco9-wisdom/server"
)

// coherenceThreshold mirrors the identity-coherence-gate AnalysisTemplate in
// org/ecco9-platform/servicemesh/rollout-analysis.yaml.
const coherenceThreshold = 0.7

// stack is the ephemeral full-stack deployment under test. Each service runs
// its real REST surface on an httptest server, wired together through the
// shared in-memory event mesh.
type stack struct {
	t *testing.T

	mesh *mesh

	services map[string]*httptest.Server

	Identity      *identityserver.Service
	Memory        *memoryserver.Service
	Reservoir     *reservoirserver.Service
	Emotion       *emotionserver.Service
	EchoBeats     *echobeatsserver.Service
	Wisdom        *wisdomserver.Service
	Consciousness *consciousnessserver.Service
	Relevance     *relevanceserver.Service
	EchoDream     *echodreamserver.Service
	AtomSpace     *atomspaceserver.Service
	Metacog       *metacogserver.Service

	// Chaos controls: per-service outage/partition switches and injected
	// latency, consulted by the stack's HTTP client transport.
	down    map[string]bool
	latency map[string]int64 // milliseconds
}

// newStack deploys the cognitive stack into an ephemeral namespace.
func newStack(t *testing.T) *stack {
	t.Helper()
	s := &stack{
		t:        t,
		mesh:     newMesh(),
		services: map[string]*httptest.Server{},
		down:     map[string]bool{},
		latency:  map[string]int64{},
	}

	s.Identity = identityserver.New("", "deep-tree-echo")
	s.Memory = memoryserver.New("")
	s.Reservoir = reservoirserver.New(64, reservoir.PersonaContemplativeScholar, 16)
	s.Emotion = emotionserver.New(8)
	s.EchoBeats = echobeatsserver.New()
	s.Wisdom = wisdomserver.New()
	s.Consciousness = consciousnessserver.New()
	s.Relevance = relevanceserver.New()
	s.EchoDream = echodreamserver.New()
	s.AtomSpace = atomspaceserver.New()
	s.Metacog = metacogserver.New()

	// Route every service's event publications through the shared mesh so
	// asynchronous cognitive processes (memory consolidation, dream cycles,
	// learning) flow as they do over NATS in production.
	s.Memory.SetPublisher(s.mesh)

	s.mount("identity", s.Identity.Routes())
	s.mount("memory", s.Memory.Routes())
	s.mount("reservoir", s.Reservoir.Routes())
	s.mount("emotion", s.Emotion.Routes())
	s.mount("echobeats", s.EchoBeats.Routes())
	s.mount("wisdom", s.Wisdom.Routes())
	s.mount("consciousness", s.Consciousness.Routes())
	s.mount("relevance", s.Relevance.Routes())
	s.mount("echodream", s.EchoDream.Routes())
	s.mount("atomspace", s.AtomSpace.Routes())
	s.mount("metacog", s.Metacog.Routes())

	t.Cleanup(func() {
		for _, srv := range s.services {
			srv.Close()
		}
	})
	return s
}

func (s *stack) mount(name string, h http.Handler) {
	srv := httptest.NewServer(h)
	s.t.Cleanup(srv.Close)
	s.services[name] = srv
}

// kill simulates a random pod kill: all subsequent calls to the service fail
// with connection-style errors until heal is called.
func (s *stack) kill(name string) { s.down[name] = true }

// heal restores a killed or partitioned service.
func (s *stack) heal(name string) {
	delete(s.down, name)
	delete(s.latency, name)
}

// partition simulates a network partition isolating a service.
func (s *stack) partition(name string) { s.down[name] = true }

// injectLatency adds fixed latency (ms) to every call to the service.
func (s *stack) injectLatency(name string, ms int64) { s.latency[name] = ms }

// do performs an HTTP call against a service in the ephemeral namespace,
// honoring chaos controls (kills, partitions, latency injection).
func (s *stack) do(service, method, path string, body any) (int, map[string]any, error) {
	if s.down[service] {
		return 0, nil, fmt.Errorf("%s unreachable (chaos: killed/partitioned)", service)
	}
	if ms := s.latency[service]; ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	return s.rawDo(service, method, path, body)
}

func (s *stack) rawDo(service, method, path string, body any) (int, map[string]any, error) {
	srv, ok := s.services[service]
	if !ok {
		return 0, nil, fmt.Errorf("unknown service %q", service)
	}
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, srv.URL+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out, nil
}

// mustPost posts and requires a 2xx status.
func (s *stack) mustPost(service, path string, body any) map[string]any {
	s.t.Helper()
	code, out, err := s.do(service, http.MethodPost, path, body)
	if err != nil {
		s.t.Fatalf("POST %s%s: %v", service, path, err)
	}
	if code < 200 || code >= 300 {
		s.t.Fatalf("POST %s%s = %d (%v)", service, path, code, out)
	}
	return out
}

// mustGet gets and requires a 2xx status.
func (s *stack) mustGet(service, path string) map[string]any {
	s.t.Helper()
	code, out, err := s.do(service, http.MethodGet, path, nil)
	if err != nil {
		s.t.Fatalf("GET %s%s: %v", service, path, err)
	}
	if code < 200 || code >= 300 {
		s.t.Fatalf("GET %s%s = %d (%v)", service, path, code, out)
	}
	return out
}

// coherence verifies the identity embedding for identityID against the
// identity service and returns the coherence score.
func (s *stack) coherence(identityID string) float64 {
	s.t.Helper()
	vec := identity.Embed(identityID + "/kernel")
	out := s.mustPost("identity", "/v1/identity/verify", map[string]any{
		"embedding": map[string]any{"identity_id": identityID, "vector": vec},
	})
	c, _ := out["coherence"].(float64)
	return c
}

// mesh is the in-process stand-in for the NATS JetStream event mesh: it
// records published envelopes per topic and lets tests assert on the
// asynchronous cognitive flows.
type mesh struct {
	topics []string
	envs   []*eventmesh.Envelope
}

func newMesh() *mesh { return &mesh{} }

func (m *mesh) Publish(topic string, env *eventmesh.Envelope) error {
	m.topics = append(m.topics, topic)
	m.envs = append(m.envs, env)
	return nil
}

func (m *mesh) count(topic string) int {
	n := 0
	for _, t := range m.topics {
		if t == topic {
			n++
		}
	}
	return n
}
