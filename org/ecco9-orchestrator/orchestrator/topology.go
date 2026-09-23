package orchestrator

// This file encodes the network topology as a neural architecture (issue
// section 5.1/5.2): the service mesh mirrors the cognitive architecture and
// inter-service communication follows four canonical event-flow patterns.

// Layer identifies a tier of the neural architecture.
type Layer string

const (
	LayerSensory     Layer = "sensory"         // external API (sensory input)
	LayerCore        Layer = "core_processing" // reservoir, echobeats, llm-gateway
	LayerCognitive   Layer = "cognitive"       // memory, emotion, consciousness, relevance, atomspace
	LayerHigherOrder Layer = "higher_order"    // echodream, wisdom, identity
	LayerMeta        Layer = "meta_cognitive"  // metacog self-improvement
)

// TopologyNode is a service placed in the neural architecture.
type TopologyNode struct {
	Service string `json:"service"`
	Layer   Layer  `json:"layer"`
	Role    string `json:"role"`
}

// FlowPattern is a canonical event-flow pathway through the mesh (5.2).
type FlowPattern struct {
	Kind    string   `json:"kind"`    // feedforward | feedback | recurrent | modulatory
	Path    []string `json:"path"`    // ordered services on the pathway
	Purpose string   `json:"purpose"` // cognitive function
}

// Topology mirrors the service-mesh neural architecture diagram.
var Topology = []TopologyNode{
	{Service: "ecco9-gateway", Layer: LayerSensory, Role: "external API (sensory input)"},
	{Service: "ecco9-reservoir", Layer: LayerCore, Role: "echo state network (temporal core)"},
	{Service: "ecco9-echobeats", Layer: LayerCore, Role: "cognitive rhythm"},
	{Service: "ecco9-llm-gateway", Layer: LayerCore, Role: "inference"},
	{Service: "ecco9-memory", Layer: LayerCognitive, Role: "hypergraph memory"},
	{Service: "ecco9-emotion", Layer: LayerCognitive, Role: "affective state"},
	{Service: "ecco9-consciousness", Layer: LayerCognitive, Role: "stream of consciousness"},
	{Service: "ecco9-relevance", Layer: LayerCognitive, Role: "salience / relevance realization"},
	{Service: "ecco9-atomspace", Layer: LayerCognitive, Role: "knowledge atomspace"},
	{Service: "ecco9-echodream", Layer: LayerHigherOrder, Role: "memory consolidation"},
	{Service: "ecco9-wisdom", Layer: LayerHigherOrder, Role: "insight / wisdom cultivation"},
	{Service: "ecco9-identity", Layer: LayerHigherOrder, Role: "selfhood / identity coherence"},
	{Service: "ecco9-metacog", Layer: LayerMeta, Role: "meta-cognitive monitor (self-improvement)"},
}

// FlowPatterns are the four canonical event-flow patterns through the mesh.
var FlowPatterns = []FlowPattern{
	{
		Kind:    "feedforward",
		Path:    []string{"ecco9-gateway", "ecco9-reservoir", "ecco9-echobeats", "ecco9-llm-gateway", "response"},
		Purpose: "perception to action",
	},
	{
		Kind:    "feedback",
		Path:    []string{"response", "ecco9-memory", "ecco9-relevance", "ecco9-echodream", "ecco9-wisdom", "ecco9-reservoir"},
		Purpose: "experience to learning (reservoir weight update)",
	},
	{
		Kind:    "recurrent",
		Path:    []string{"ecco9-reservoir", "ecco9-consciousness", "ecco9-emotion"},
		Purpose: "continuous state maintenance (bidirectional synchronization)",
	},
	{
		Kind:    "modulatory",
		Path:    []string{"ecco9-metacog", "ecco9-orchestrator", "all-services"},
		Purpose: "adaptive parameter tuning (meta-control)",
	},
}
