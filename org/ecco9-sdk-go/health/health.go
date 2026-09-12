// Package health provides the standard health/identity-coherence endpoint
// behavior shared by every ecco9 service.
package health

import (
	"encoding/json"
	"math"
	"net/http"
	"sync/atomic"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

// Reporter tracks serving status, identity coherence, and cognitive load for
// a single service instance.
type Reporter struct {
	status    atomic.Int32
	coherence atomic.Uint64 // float64 bits
	load      atomic.Uint64 // float64 bits
}

// NewReporter returns a Reporter initialized to SERVING with coherence 1.0.
func NewReporter() *Reporter {
	r := &Reporter{}
	r.SetStatus(contracts.ServingStatusServing)
	r.SetCoherence(1.0)
	r.SetLoad(0)
	return r
}

func (r *Reporter) SetStatus(s contracts.ServingStatus) { r.status.Store(int32(s)) }
func (r *Reporter) Status() contracts.ServingStatus     { return contracts.ServingStatus(r.status.Load()) }

func (r *Reporter) SetCoherence(v float64) { r.coherence.Store(math.Float64bits(v)) }
func (r *Reporter) Coherence() float64     { return math.Float64frombits(r.coherence.Load()) }

func (r *Reporter) SetLoad(v float64) { r.load.Store(math.Float64bits(v)) }
func (r *Reporter) Load() float64     { return math.Float64frombits(r.load.Load()) }

// Handler exposes /healthz with identity coherence and cognitive load.
func (r *Reporter) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             r.Status(),
			"identity_coherence": r.Coherence(),
			"cognitive_load":     r.Load(),
		})
	})
	return mux
}
