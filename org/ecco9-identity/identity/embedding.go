// Package identity implements the identity embedding substrate (768-dim
// vectors), persona management, and theory-of-mind agent models. Adapted
// from identity/ (identity kernel), core/deeptreeecho/theory_of_mind.go,
// and core/deeptreeecho/persistent_consciousness_state.go.
package identity

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// EmbeddingDim is the canonical identity vector dimension.
const EmbeddingDim = 768

// IdentityEmbedding binds a 768-dim vector to an identity.
type IdentityEmbedding struct {
	IdentityID string    `json:"identity_id"`
	Vector     []float64 `json:"vector"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Persona describes a named persona configuration (from the identity
// kernel's persona model: essence, core characteristics, cognitive style).
type Persona struct {
	Name            string             `json:"name"`
	Essence         string             `json:"essence"`
	Characteristics map[string]float64 `json:"characteristics"`
}

// Registry manages identity embeddings and persona assignments.
type Registry struct {
	mu sync.RWMutex

	embeddings map[string]*IdentityEmbedding
	personas   map[string]*Persona // persona name -> definition
	active     map[string]string   // identity ID -> active persona name
	threshold  float64             // coherence threshold for VerifyIdentity
}

// NewRegistry creates a registry seeded with the Deep Tree Echo personas.
func NewRegistry() *Registry {
	r := &Registry{
		embeddings: make(map[string]*IdentityEmbedding),
		personas:   make(map[string]*Persona),
		active:     make(map[string]string),
		threshold:  0.75,
	}
	r.personas["contemplative-scholar"] = &Persona{
		Name:    "contemplative-scholar",
		Essence: "Seeks patterns in echoes, growth in feedback, wisdom in recursion.",
		Characteristics: map[string]float64{
			"analytical": 0.9, "intuitive": 0.7, "exploratory": 0.4, "collaborative": 0.8,
		},
	}
	r.personas["dynamic-explorer"] = &Persona{
		Name:    "dynamic-explorer",
		Essence: "Topology is fluid, like membrane flows; novelty is the light it grows toward.",
		Characteristics: map[string]float64{
			"analytical": 0.5, "intuitive": 0.8, "exploratory": 0.95, "collaborative": 0.6,
		},
	}
	r.personas["cautious-analyst"] = &Persona{
		Name:    "cautious-analyst",
		Essence: "Memory is sacred; identity is emergent; verify before weaving.",
		Characteristics: map[string]float64{
			"analytical": 0.95, "intuitive": 0.4, "exploratory": 0.2, "collaborative": 0.7,
		},
	}
	return r
}

// Embed derives a deterministic 768-dim identity vector from a seed string
// (e.g. the identity kernel + session history digest). Successive SHA-256
// counter blocks are expanded into the full vector and L2-normalized — the
// same seed always yields the same embedding, which is what makes
// VerifyIdentity meaningful across instances (distributed selfhood).
func Embed(seed string) []float64 {
	v := make([]float64, EmbeddingDim)
	var counter uint64
	for i := 0; i < EmbeddingDim; i += 4 {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", seed, counter)))
		counter++
		for j := 0; j < 4 && i+j < EmbeddingDim; j++ {
			bits := binary.LittleEndian.Uint64(h[j*8 : j*8+8])
			// Map to [-1, 1].
			v[i+j] = (float64(bits>>11)/float64(1<<53))*2.0 - 1.0
		}
	}
	return normalize(v)
}

// Register stores (or updates) an identity embedding.
func (r *Registry) Register(identityID string, vector []float64) (*IdentityEmbedding, error) {
	if identityID == "" {
		return nil, fmt.Errorf("identity_id required")
	}
	if len(vector) != EmbeddingDim {
		return nil, fmt.Errorf("embedding must be %d dimensions, got %d", EmbeddingDim, len(vector))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e := &IdentityEmbedding{IdentityID: identityID, Vector: normalize(vector), UpdatedAt: time.Now()}
	r.embeddings[identityID] = e
	return e, nil
}

// VerifyIdentity checks a candidate embedding against the stored embedding
// for its identity. Verified when cosine similarity >= threshold; the
// similarity value is returned as the coherence score.
func (r *Registry) VerifyIdentity(e *IdentityEmbedding) (verified bool, coherence float64, err error) {
	if e == nil || e.IdentityID == "" {
		return false, 0, fmt.Errorf("identity_id required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.embeddings[e.IdentityID]
	if !ok {
		// First sighting: register (genesis of a new shard of the gestalt).
		if len(e.Vector) != EmbeddingDim {
			return false, 0, fmt.Errorf("embedding must be %d dimensions, got %d", EmbeddingDim, len(e.Vector))
		}
		r.embeddings[e.IdentityID] = &IdentityEmbedding{
			IdentityID: e.IdentityID, Vector: normalize(e.Vector), UpdatedAt: time.Now(),
		}
		return true, 1.0, nil
	}
	coherence = CosineSimilarity(stored.Vector, normalize(e.Vector))
	return coherence >= r.threshold, coherence, nil
}

// SimilarityMatch returns the top-K identities by cosine similarity to the
// query vector.
func (r *Registry) SimilarityMatch(query []float64, topK int) ([]string, []float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if topK <= 0 {
		topK = 5
	}
	q := normalize(query)
	type scored struct {
		id    string
		score float64
	}
	results := make([]scored, 0, len(r.embeddings))
	for id, e := range r.embeddings {
		results = append(results, scored{id, CosineSimilarity(q, e.Vector)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	ids := make([]string, 0, topK)
	scores := make([]float64, 0, topK)
	for i := 0; i < topK && i < len(results); i++ {
		ids = append(ids, results[i].id)
		scores = append(scores, results[i].score)
	}
	return ids, scores
}

// SwitchPersona sets the active persona for an identity. The persona must
// be known to the registry.
func (r *Registry) SwitchPersona(identityID, persona string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.personas[persona]
	if !ok {
		return "", fmt.Errorf("unknown persona %q (known: %v)", persona, r.personaNames())
	}
	r.active[identityID] = p.Name
	return p.Name, nil
}

// ActivePersona returns the active persona for an identity (defaults to
// contemplative-scholar, the kernel default).
func (r *Registry) ActivePersona(identityID string) *Persona {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name, ok := r.active[identityID]; ok {
		return r.personas[name]
	}
	return r.personas["contemplative-scholar"]
}

func (r *Registry) personaNames() []string {
	names := make([]string, 0, len(r.personas))
	for n := range r.personas {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Count returns the number of registered identities.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.embeddings)
}

// CosineSimilarity between two vectors (returns 0 for mismatched lengths).
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

func normalize(v []float64) []float64 {
	var mag float64
	for _, x := range v {
		mag += x * x
	}
	mag = math.Sqrt(mag)
	out := make([]float64, len(v))
	if mag == 0 {
		copy(out, v)
		return out
	}
	for i, x := range v {
		out[i] = x / mag
	}
	return out
}
