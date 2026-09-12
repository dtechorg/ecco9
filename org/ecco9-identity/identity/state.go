package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConsciousnessState is the persistent identity state carried across
// sessions, adapted from core/deeptreeecho/persistent_consciousness_state.go.
// Identity continuity is maintained through dynamic memory weaving, not
// static retention — this snapshot is the seed, not the self.
type ConsciousnessState struct {
	IdentityName   string             `json:"identity_name"`
	SessionID      string             `json:"session_id"`
	CreatedAt      time.Time          `json:"created_at"`
	LastUpdated    time.Time          `json:"last_updated"`
	AwarenessLevel float64            `json:"awareness_level"`
	CognitiveLoad  float64            `json:"cognitive_load"`
	InterestTopics map[string]float64 `json:"interest_topics"`
	ActivePersona  string             `json:"active_persona"`
	StateVersion   string             `json:"state_version"`
}

// StateStore persists consciousness state as an atomically-written JSON
// file (mounted from a StatefulSet volume in production).
type StateStore struct {
	mu        sync.Mutex
	stateFile string
	saveCount uint64
}

// NewStateStore creates a store rooted at stateDir. An empty stateDir
// disables persistence.
func NewStateStore(stateDir string) (*StateStore, error) {
	if stateDir == "" {
		return &StateStore{}, nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return &StateStore{stateFile: filepath.Join(stateDir, "consciousness_state.json")}, nil
}

// Enabled reports whether persistence is active.
func (ss *StateStore) Enabled() bool { return ss.stateFile != "" }

// Save writes the state atomically (write-then-rename).
func (ss *StateStore) Save(state *ConsciousnessState) error {
	if !ss.Enabled() {
		return nil
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	state.LastUpdated = time.Now()
	state.StateVersion = "1.0"
	buf, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := ss.stateFile + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, ss.stateFile); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	ss.saveCount++
	return nil
}

// Load restores the state, or returns a fresh genesis state.
func (ss *StateStore) Load(identityName string) *ConsciousnessState {
	genesis := &ConsciousnessState{
		IdentityName:   identityName,
		SessionID:      fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		CreatedAt:      time.Now(),
		LastUpdated:    time.Now(),
		AwarenessLevel: 0.5,
		InterestTopics: map[string]float64{},
		ActivePersona:  "contemplative-scholar",
		StateVersion:   "1.0",
	}
	if !ss.Enabled() {
		return genesis
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	buf, err := os.ReadFile(ss.stateFile)
	if err != nil {
		return genesis
	}
	var state ConsciousnessState
	if err := json.Unmarshal(buf, &state); err != nil {
		return genesis
	}
	state.SessionID = genesis.SessionID // new session, continuous identity
	return &state
}

// SaveCount returns the number of successful saves.
func (ss *StateStore) SaveCount() uint64 {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.saveCount
}
