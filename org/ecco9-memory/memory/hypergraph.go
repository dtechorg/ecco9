// Package memory implements the hypergraph memory substrate,
// adapted from core/memory/hypergraph.go (adjacency indices, BFS/DFS
// traversal, cosine similarity search) with stdlib-only dependencies.
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// NodeType classifies memory nodes (mirrors ecco9.memory.v1.NodeType).
type NodeType string

const (
	NodeConcept       NodeType = "concept"
	NodeEpisode       NodeType = "episode"
	NodePattern       NodeType = "pattern"
	NodeAnomaly       NodeType = "anomaly"
	NodeEchoSignature NodeType = "echo_signature"
)

// MemoryNode is a vertex in the hypergraph.
type MemoryNode struct {
	ID        string            `json:"id"`
	Type      NodeType          `json:"type"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Embedding []float64         `json:"embedding,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// MemoryEdge is a directed, weighted relation between two nodes.
type MemoryEdge struct {
	ID       string    `json:"id"`
	SourceID string    `json:"source_id"`
	TargetID string    `json:"target_id"`
	Relation string    `json:"relation"`
	Weight   float64   `json:"weight"`
	Created  time.Time `json:"created_at"`
}

// HyperEdge is a multi-way relationship over a set of nodes.
type HyperEdge struct {
	ID       string    `json:"id"`
	NodeIDs  []string  `json:"node_ids"`
	Relation string    `json:"relation"`
	Weight   float64   `json:"weight"`
	Created  time.Time `json:"created_at"`
}

// NewID returns a random hex identifier (stdlib-only UUID replacement).
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// HypergraphMemory is a thread-safe in-memory hypergraph with adjacency
// indices, a time index, and embedding-based similarity search.
type HypergraphMemory struct {
	mu sync.RWMutex

	nodes      map[string]*MemoryNode
	edges      map[string]*MemoryEdge
	hyperedges map[string]*HyperEdge

	outgoing map[string][]string // node ID -> edge IDs
	incoming map[string][]string

	typeIndex map[NodeType][]string
	timeIndex []string
}

// NewHypergraphMemory creates an empty hypergraph.
func NewHypergraphMemory() *HypergraphMemory {
	return &HypergraphMemory{
		nodes:      make(map[string]*MemoryNode),
		edges:      make(map[string]*MemoryEdge),
		hyperedges: make(map[string]*HyperEdge),
		outgoing:   make(map[string][]string),
		incoming:   make(map[string][]string),
		typeIndex:  make(map[NodeType][]string),
		timeIndex:  make([]string, 0),
	}
}

// AddNode inserts a node, assigning an ID and timestamp if unset.
func (hg *HypergraphMemory) AddNode(node *MemoryNode) (string, error) {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	if node.ID == "" {
		node.ID = NewID()
	}
	if node.Type == "" {
		node.Type = NodeConcept
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	hg.nodes[node.ID] = node
	hg.typeIndex[node.Type] = append(hg.typeIndex[node.Type], node.ID)
	hg.timeIndex = append(hg.timeIndex, node.ID)
	if _, ok := hg.outgoing[node.ID]; !ok {
		hg.outgoing[node.ID] = []string{}
	}
	if _, ok := hg.incoming[node.ID]; !ok {
		hg.incoming[node.ID] = []string{}
	}
	return node.ID, nil
}

// AddEdge inserts a directed edge between existing nodes.
func (hg *HypergraphMemory) AddEdge(edge *MemoryEdge) (string, error) {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	if _, ok := hg.nodes[edge.SourceID]; !ok {
		return "", fmt.Errorf("source node not found: %s", edge.SourceID)
	}
	if _, ok := hg.nodes[edge.TargetID]; !ok {
		return "", fmt.Errorf("target node not found: %s", edge.TargetID)
	}
	if edge.ID == "" {
		edge.ID = NewID()
	}
	if edge.Relation == "" {
		edge.Relation = "related_to"
	}
	if edge.Created.IsZero() {
		edge.Created = time.Now()
	}

	hg.edges[edge.ID] = edge
	hg.outgoing[edge.SourceID] = append(hg.outgoing[edge.SourceID], edge.ID)
	hg.incoming[edge.TargetID] = append(hg.incoming[edge.TargetID], edge.ID)
	return edge.ID, nil
}

// AddHyperEdge inserts a multi-way relationship over existing nodes.
func (hg *HypergraphMemory) AddHyperEdge(he *HyperEdge) (string, error) {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	for _, id := range he.NodeIDs {
		if _, ok := hg.nodes[id]; !ok {
			return "", fmt.Errorf("node not found in hyperedge: %s", id)
		}
	}
	if he.ID == "" {
		he.ID = NewID()
	}
	if he.Created.IsZero() {
		he.Created = time.Now()
	}
	hg.hyperedges[he.ID] = he
	return he.ID, nil
}

// GetNode fetches a node by ID.
func (hg *HypergraphMemory) GetNode(id string) (*MemoryNode, error) {
	hg.mu.RLock()
	defer hg.mu.RUnlock()
	n, ok := hg.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return n, nil
}

// Traverse performs BFS from a start node up to maxDepth, optionally
// filtering by relation name. Returns visited nodes and traversed edges.
func (hg *HypergraphMemory) Traverse(startID string, maxDepth int, relationFilter string) ([]*MemoryNode, []*MemoryEdge, error) {
	hg.mu.RLock()
	defer hg.mu.RUnlock()

	if _, ok := hg.nodes[startID]; !ok {
		return nil, nil, fmt.Errorf("start node not found: %s", startID)
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}

	visited := map[string]bool{startID: true}
	nodes := []*MemoryNode{}
	edges := []*MemoryEdge{}

	type item struct {
		id    string
		depth int
	}
	queue := []item{{id: startID, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth > maxDepth {
			continue
		}
		if n, ok := hg.nodes[cur.id]; ok {
			nodes = append(nodes, n)
		}
		for _, edgeID := range hg.outgoing[cur.id] {
			e := hg.edges[edgeID]
			if relationFilter != "" && e.Relation != relationFilter {
				continue
			}
			edges = append(edges, e)
			if !visited[e.TargetID] {
				visited[e.TargetID] = true
				queue = append(queue, item{id: e.TargetID, depth: cur.depth + 1})
			}
		}
	}
	return nodes, edges, nil
}

// SimilaritySearch returns the top-K nodes whose embeddings are closest to
// the query vector by cosine similarity.
func (hg *HypergraphMemory) SimilaritySearch(query []float64, topK int) []*MemoryNode {
	hg.mu.RLock()
	defer hg.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}
	type scored struct {
		node  *MemoryNode
		score float64
	}
	results := make([]scored, 0, len(hg.nodes))
	for _, n := range hg.nodes {
		if len(n.Embedding) == 0 {
			continue
		}
		results = append(results, scored{n, CosineSimilarity(query, n.Embedding)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	out := make([]*MemoryNode, 0, topK)
	for i := 0; i < topK && i < len(results); i++ {
		out = append(out, results[i].node)
	}
	return out
}

// Stats returns node/edge/hyperedge counts for metrics and health.
func (hg *HypergraphMemory) Stats() (nodes, edges, hyperedges int) {
	hg.mu.RLock()
	defer hg.mu.RUnlock()
	return len(hg.nodes), len(hg.edges), len(hg.hyperedges)
}

// Snapshot exports the full graph (used by the persistence layer).
func (hg *HypergraphMemory) Snapshot() (map[string]*MemoryNode, map[string]*MemoryEdge, map[string]*HyperEdge) {
	hg.mu.RLock()
	defer hg.mu.RUnlock()
	nodes := make(map[string]*MemoryNode, len(hg.nodes))
	for k, v := range hg.nodes {
		nodes[k] = v
	}
	edges := make(map[string]*MemoryEdge, len(hg.edges))
	for k, v := range hg.edges {
		edges[k] = v
	}
	hyper := make(map[string]*HyperEdge, len(hg.hyperedges))
	for k, v := range hg.hyperedges {
		hyper[k] = v
	}
	return nodes, edges, hyper
}

// CosineSimilarity computes the cosine similarity between two vectors.
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
