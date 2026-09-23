package orchestrator

import "testing"

func TestTopologyMirrorsNeuralArchitectureLayers(t *testing.T) {
	layers := make(map[Layer]int)
	services := make(map[string]bool)
	for _, n := range Topology {
		layers[n.Layer]++
		services[n.Service] = true
	}
	// The diagram has five tiers.
	for _, l := range []Layer{LayerSensory, LayerCore, LayerCognitive, LayerHigherOrder, LayerMeta} {
		if layers[l] == 0 {
			t.Fatalf("layer %s has no services", l)
		}
	}
	// The load-bearing services from the diagram are present.
	for _, svc := range []string{
		"ecco9-gateway", "ecco9-reservoir", "ecco9-echobeats", "ecco9-llm-gateway",
		"ecco9-memory", "ecco9-emotion", "ecco9-consciousness", "ecco9-relevance",
		"ecco9-atomspace", "ecco9-echodream", "ecco9-wisdom", "ecco9-identity", "ecco9-metacog",
	} {
		if !services[svc] {
			t.Fatalf("topology missing service %s", svc)
		}
	}
}

func TestFlowPatternsCoverFourCanonicalFlows(t *testing.T) {
	kinds := make(map[string]bool)
	for _, fp := range FlowPatterns {
		kinds[fp.Kind] = true
		if len(fp.Path) == 0 {
			t.Fatalf("flow pattern %s has empty path", fp.Kind)
		}
	}
	for _, k := range []string{"feedforward", "feedback", "recurrent", "modulatory"} {
		if !kinds[k] {
			t.Fatalf("missing flow pattern %s", k)
		}
	}

	// Feedforward must run perception to action through the core.
	var ff FlowPattern
	for _, fp := range FlowPatterns {
		if fp.Kind == "feedforward" {
			ff = fp
		}
	}
	want := []string{"ecco9-gateway", "ecco9-reservoir", "ecco9-echobeats", "ecco9-llm-gateway", "response"}
	if len(ff.Path) != len(want) {
		t.Fatalf("feedforward path = %v, want %v", ff.Path, want)
	}
	for i, s := range want {
		if ff.Path[i] != s {
			t.Fatalf("feedforward[%d] = %s, want %s", i, ff.Path[i], s)
		}
	}

	// Feedback must terminate at the reservoir for the weight update.
	for _, fp := range FlowPatterns {
		if fp.Kind == "feedback" && fp.Path[len(fp.Path)-1] != "ecco9-reservoir" {
			t.Fatalf("feedback should terminate at reservoir, got %v", fp.Path)
		}
	}
}
