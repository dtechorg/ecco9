// Package atomspace implements the OpenCog AtomSpace extracted from
// core/opencog/atomspace.go: a weighted, labeled hypergraph with PLN truth
// values, ECAN attention allocation, and clause-based pattern matching.
package atomspace

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// AtomType mirrors ecco9.atomspace.v1.AtomType.
type AtomType int32

const (
	AtomTypeUnspecified AtomType = iota
	AtomTypeConcept
	AtomTypePredicate
	AtomTypeVariable
	AtomTypeSchema
)

func (t AtomType) String() string {
	switch t {
	case AtomTypeConcept:
		return "CONCEPT"
	case AtomTypePredicate:
		return "PREDICATE"
	case AtomTypeVariable:
		return "VARIABLE"
	case AtomTypeSchema:
		return "SCHEMA"
	}
	return "UNSPECIFIED"
}

// LinkType categorizes hyperedges (from atomspace.go).
type LinkType string

const (
	InheritanceLink LinkType = "InheritanceLink"
	SimilarityLink  LinkType = "SimilarityLink"
	EvaluationLink  LinkType = "EvaluationLink"
	MemberLink      LinkType = "MemberLink"
	ListLink        LinkType = "ListLink"
	ImplicationLink LinkType = "ImplicationLink"
)

// TruthValue is a PLN probabilistic truth value.
type TruthValue struct {
	Strength   float64 `json:"strength"`   // probability [0,1]
	Confidence float64 `json:"confidence"` // weight of evidence [0,1]
	Count      float64 `json:"count"`      // amount of evidence
}

// AttentionValue is an ECAN attention allocation.
type AttentionValue struct {
	STI int16   `json:"sti"` // short-term importance
	LTI int16   `json:"lti"` // long-term importance
	AF  float64 `json:"af"`  // attention focus
}

// Atom is a node in the hypergraph.
type Atom struct {
	ID        string          `json:"id"`
	Type      AtomType        `json:"type"`
	Name      string          `json:"name"`
	Truth     *TruthValue     `json:"truth"`
	Attention *AttentionValue `json:"attention"`
	Incoming  []string        `json:"incoming,omitempty"`
	Created   time.Time       `json:"created"`
	Modified  time.Time       `json:"modified"`
}

// Link is a hyperedge connecting atoms.
type Link struct {
	ID        string          `json:"id"`
	Type      LinkType        `json:"type"`
	Outgoing  []string        `json:"outgoing_atom_ids"`
	Truth     *TruthValue     `json:"truth"`
	Attention *AttentionValue `json:"attention"`
	Created   time.Time       `json:"created"`
	Modified  time.Time       `json:"modified"`
}

// AttentionBank manages ECAN funds and the forgetting boundary.
type AttentionBank struct {
	mu             sync.RWMutex
	STIFunds       int64
	LTIFunds       int64
	AFBoundary     float64
	ForgettingRate float64
	stiIndex       map[string]int16
	ltiIndex       map[string]int16
}

// Pattern is a query over the hypergraph: clauses with variable bindings.
type Pattern struct {
	Variables map[string]bool
	Clauses   []Clause
}

// Clause matches links of a type against atoms or variables.
type Clause struct {
	LinkType LinkType
	Atoms    []string // atom IDs or variable names ($var)
}

// QueryResult holds pattern matching bindings.
type QueryResult struct {
	Bindings []map[string]string `json:"bindings"`
	Count    int                 `json:"count"`
}

// AtomSpace is the weighted labeled hypergraph.
type AtomSpace struct {
	mu            sync.RWMutex
	Atoms         map[string]*Atom
	Links         map[string]*Link
	Incoming      map[string][]string
	AttentionBank *AttentionBank
	Created       time.Time
	Modified      time.Time
}

// New creates an empty AtomSpace with a funded attention bank.
func New() *AtomSpace {
	return &AtomSpace{
		Atoms:    make(map[string]*Atom),
		Links:    make(map[string]*Link),
		Incoming: make(map[string][]string),
		AttentionBank: &AttentionBank{
			STIFunds:       100000,
			LTIFunds:       100000,
			AFBoundary:     0.5,
			ForgettingRate: 0.01,
			stiIndex:       make(map[string]int16),
			ltiIndex:       make(map[string]int16),
		},
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// AddAtom adds an atom node to the hypergraph.
func (as *AtomSpace) AddAtom(atomType AtomType, name string, tv *TruthValue) (*Atom, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if tv == nil {
		tv = &TruthValue{Strength: 1.0}
	}
	atom := &Atom{
		ID:        fmt.Sprintf("atom_%s_%d", name, time.Now().UnixNano()),
		Type:      atomType,
		Name:      name,
		Truth:     tv,
		Attention: &AttentionValue{},
		Created:   time.Now(),
		Modified:  time.Now(),
	}
	as.Atoms[atom.ID] = atom
	as.Modified = time.Now()
	as.AttentionBank.register(atom.ID, atom.Attention)
	return atom, nil
}

// AddLink adds a hyperedge, verifying every outgoing target exists.
func (as *AtomSpace) AddLink(linkType LinkType, outgoing []string, tv *TruthValue) (*Link, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, id := range outgoing {
		if _, ok := as.Atoms[id]; !ok {
			if _, lok := as.Links[id]; !lok {
				return nil, fmt.Errorf("atom or link %s not found", id)
			}
		}
	}
	if tv == nil {
		tv = &TruthValue{Strength: 1.0}
	}
	link := &Link{
		ID:        fmt.Sprintf("link_%s_%d", linkType, time.Now().UnixNano()),
		Type:      linkType,
		Outgoing:  append([]string(nil), outgoing...),
		Truth:     tv,
		Attention: &AttentionValue{},
		Created:   time.Now(),
		Modified:  time.Now(),
	}
	as.Links[link.ID] = link
	as.Modified = time.Now()
	for _, id := range outgoing {
		as.Incoming[id] = append(as.Incoming[id], link.ID)
		if atom, ok := as.Atoms[id]; ok {
			atom.Incoming = append(atom.Incoming, link.ID)
		}
	}
	as.AttentionBank.register(link.ID, link.Attention)
	return link, nil
}

// GetAtom retrieves an atom by ID.
func (as *AtomSpace) GetAtom(id string) (*Atom, bool) {
	as.mu.RLock()
	defer as.mu.RUnlock()
	a, ok := as.Atoms[id]
	return a, ok
}

// GetLink retrieves a link by ID.
func (as *AtomSpace) GetLink(id string) (*Link, bool) {
	as.mu.RLock()
	defer as.mu.RUnlock()
	l, ok := as.Links[id]
	return l, ok
}

// GetIncoming returns link IDs pointing at an atom.
func (as *AtomSpace) GetIncoming(atomID string) []string {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return append([]string(nil), as.Incoming[atomID]...)
}

// UpdateAttention sets an atom or link's attention value and registers it
// with the attention bank index.
func (as *AtomSpace) UpdateAttention(id string, av *AttentionValue) bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	switch target := as.resolve(id); t := target.(type) {
	case *Atom:
		t.Attention = av
		t.Modified = time.Now()
	case *Link:
		t.Attention = av
		t.Modified = time.Now()
	default:
		return false
	}
	as.AttentionBank.register(id, av)
	as.Modified = time.Now()
	return true
}

func (as *AtomSpace) resolve(id string) any {
	if a, ok := as.Atoms[id]; ok {
		return a
	}
	if l, ok := as.Links[id]; ok {
		return l
	}
	return nil
}

// SpreadAttention runs one ECAN diffusion round: STI flows from links to
// their outgoing atoms proportional to link truth-value strength.
func (as *AtomSpace) SpreadAttention() {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, link := range as.Links {
		strength := link.Truth.Strength
		sourceSTI := link.Attention.STI
		for _, targetID := range link.Outgoing {
			if atom, ok := as.Atoms[targetID]; ok {
				transfer := int16(float64(sourceSTI) * strength * 0.1)
				atom.Attention.STI += transfer
				link.Attention.STI -= transfer
			}
		}
	}
}

// Forget removes atoms below the attention-focus forgetting threshold and
// prunes orphaned links.
func (as *AtomSpace) Forget() {
	as.mu.Lock()
	defer as.mu.Unlock()
	threshold := as.AttentionBank.AFBoundary * as.AttentionBank.ForgettingRate
	for id, atom := range as.Atoms {
		if atom.Attention.AF < threshold && len(atom.Incoming) == 0 {
			delete(as.Atoms, id)
			delete(as.Incoming, id)
			as.AttentionBank.unregister(id)
		}
	}
	for id, link := range as.Links {
		for _, atomID := range link.Outgoing {
			if _, ok := as.Atoms[atomID]; !ok {
				delete(as.Links, id)
				as.AttentionBank.unregister(id)
				break
			}
		}
	}
	as.Modified = time.Now()
}

// Match runs clause-based pattern matching with variable binding.
func (as *AtomSpace) Match(p *Pattern) *QueryResult {
	as.mu.RLock()
	defer as.mu.RUnlock()
	result := &QueryResult{Bindings: []map[string]string{}}
	for _, clause := range p.Clauses {
		for _, link := range as.Links {
			if link.Type != clause.LinkType || len(link.Outgoing) != len(clause.Atoms) {
				continue
			}
			binding := make(map[string]string)
			matched := true
			for i, patternAtom := range clause.Atoms {
				if p.Variables[patternAtom] || strings.HasPrefix(patternAtom, "$") {
					binding[patternAtom] = link.Outgoing[i]
				} else if link.Outgoing[i] != patternAtom {
					matched = false
					break
				}
			}
			if matched {
				result.Bindings = append(result.Bindings, binding)
			}
		}
	}
	result.Count = len(result.Bindings)
	return result
}

// ParsePattern parses a simple text pattern:
//
//	InheritanceLink($x, atom_cat_1); SimilarityLink($x, $y)
//
// Variables are $-prefixed; other tokens must match atom/link IDs exactly.
func ParsePattern(query string) *Pattern {
	p := &Pattern{Variables: map[string]bool{}}
	for _, raw := range strings.Split(query, ";") {
		raw = strings.TrimSpace(raw)
		open := strings.Index(raw, "(")
		if open < 0 || !strings.HasSuffix(raw, ")") {
			continue
		}
		clause := Clause{LinkType: LinkType(strings.TrimSpace(raw[:open]))}
		for _, arg := range strings.Split(raw[open+1:len(raw)-1], ",") {
			arg = strings.TrimSpace(arg)
			if strings.HasPrefix(arg, "$") {
				p.Variables[arg] = true
			}
			clause.Atoms = append(clause.Atoms, arg)
		}
		p.Clauses = append(p.Clauses, clause)
	}
	return p
}

// FuseTruthValues combines truth values (PLN revision/fusion operations).
func FuseTruthValues(tv1, tv2 *TruthValue, op string) *TruthValue {
	switch op {
	case "and":
		return &TruthValue{
			Strength:   tv1.Strength * tv2.Strength,
			Confidence: math.Min(tv1.Confidence, tv2.Confidence),
			Count:      tv1.Count + tv2.Count,
		}
	case "or":
		return &TruthValue{
			Strength:   tv1.Strength + tv2.Strength - tv1.Strength*tv2.Strength,
			Confidence: math.Min(tv1.Confidence, tv2.Confidence),
			Count:      tv1.Count + tv2.Count,
		}
	case "not":
		return &TruthValue{Strength: 1 - tv1.Strength, Confidence: tv1.Confidence, Count: tv1.Count}
	default: // revision
		return &TruthValue{
			Strength:   (tv1.Strength + tv2.Strength) / 2,
			Confidence: (tv1.Confidence + tv2.Confidence) / 2,
			Count:      tv1.Count + tv2.Count,
		}
	}
}

// Status returns hypergraph statistics.
func (as *AtomSpace) Status() map[string]any {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return map[string]any{
		"atoms":       len(as.Atoms),
		"links":       len(as.Links),
		"created":     as.Created,
		"modified":    as.Modified,
		"sti_funds":   as.AttentionBank.STIFunds,
		"lti_funds":   as.AttentionBank.LTIFunds,
		"af_boundary": as.AttentionBank.AFBoundary,
	}
}

func (ab *AttentionBank) register(id string, av *AttentionValue) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	ab.stiIndex[id] = av.STI
	ab.ltiIndex[id] = av.LTI
}

func (ab *AttentionBank) unregister(id string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	delete(ab.stiIndex, id)
	delete(ab.ltiIndex, id)
}
