// Package deploy validates the Phase 7 production deployment topology
// (issue #8) without requiring a cluster: it parses topology.yaml and
// asserts the 7.1 production layout table is covered exactly, and that the
// 7.2 multi-region artifacts exist and reference their mechanisms.
//
// The topology entries are parsed with a minimal indentation scanner rather
// than a YAML library so this module stays dependency-free (matching the
// other org modules, which only take gRPC/protobuf dependencies).
package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chartEntry is one parsed service entry from topology.yaml.
type chartEntry struct {
	ReleaseName string
	Namespace   string
	Replicas    string
	MinReplicas string
	MaxReplicas string
	Requests    map[string]string
	Limits      map[string]string
	Placement   []string
}

// parseTopology scans topology.yaml line by line. It understands only the
// subset of YAML the generator file uses: top-level "- name:" list items,
// one-level "key: value" fields, and nested maps/lists under known keys
// (valuesInline, resources, requests, limits, hpa, placement).
func parseTopology(t *testing.T, path string) []chartEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read topology: %v", err)
	}

	var entries []chartEntry
	var cur *chartEntry
	section := "" // current nested section: "", "valuesInline", "requests", "limits", "hpa", "placement"

	trim := func(s string) string { return strings.TrimSpace(s) }
	kv := func(s string) (string, string, bool) {
		i := strings.Index(s, ":")
		if i < 0 {
			return "", "", false
		}
		return trim(s[:i]), trim(strings.Trim(s[i+1:], " \t\"'")), true
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t")
		s := trim(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))

		switch {
		case strings.HasPrefix(s, "- name:") && indent == 0:
			entries = append(entries, chartEntry{Requests: map[string]string{}, Limits: map[string]string{}})
			cur = &entries[len(entries)-1]
			section = ""
		case cur == nil:
			// Header comments before the first entry.
		case indent == 2 && strings.HasPrefix(s, "releaseName:"):
			_, v, _ := kv(s)
			cur.ReleaseName = v
		case indent == 2 && strings.HasPrefix(s, "namespace:"):
			_, v, _ := kv(s)
			cur.Namespace = v
		case strings.HasPrefix(s, "requests:"):
			section = "requests"
			if m := parseInlineMap(s); m != nil {
				cur.Requests = m
			}
		case strings.HasPrefix(s, "limits:"):
			section = "limits"
			if m := parseInlineMap(s); m != nil {
				cur.Limits = m
			}
		case strings.HasPrefix(s, "hpa:"):
			section = "hpa"
			parseHPA(s, cur)
		case strings.HasPrefix(s, "placement:"):
			section = "placement"
		case strings.HasPrefix(s, "resources:"):
			section = "resources"
		case strings.HasPrefix(s, "valuesInline:"):
			section = "valuesInline"
		case section == "requests" && indent >= 6:
			if k, v, ok := kv(s); ok {
				cur.Requests[k] = v
			}
		case section == "limits" && indent >= 6:
			if k, v, ok := kv(s); ok {
				cur.Limits[k] = v
			}
		case section == "hpa" && indent >= 6:
			if k, v, ok := kv(s); ok {
				switch k {
				case "minReplicas":
					cur.MinReplicas = v
				case "maxReplicas":
					cur.MaxReplicas = v
				}
			}
		case section == "placement" && indent >= 6:
			cur.Placement = append(cur.Placement, s)
		case indent == 4:
			// valuesInline-level scalar keys.
			k, v, ok := kv(s)
			if !ok {
				continue
			}
			switch k {
			case "replicas":
				cur.Replicas = v
			case "hpa":
				section = "hpa"
				parseHPA(s, cur)
			case "resources":
				section = "resources"
			case "placement":
				section = "placement"
			}
		}
	}
	return entries
}

// parseInlineMap extracts a flat {k: v, ...} inline map from a "key: {...}"
// line, returning nil if the line has no inline map (block-style nested maps
// are captured via section tracking instead).
func parseInlineMap(s string) map[string]string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < 0 || j <= i {
		return nil
	}
	m := map[string]string{}
	for _, pair := range strings.Split(s[i+1:j], ",") {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		m[strings.TrimSpace(kv[0])] = strings.Trim(strings.TrimSpace(kv[1]), "\"'")
	}
	return m
}

// parseHPA reads minReplicas/maxReplicas from an inline hpa map.
func parseHPA(s string, cur *chartEntry) {
	m := parseInlineMap(s)
	if m == nil {
		return
	}
	cur.MinReplicas = m["minReplicas"]
	cur.MaxReplicas = m["maxReplicas"]
}

func (e chartEntry) limit(k string) string {
	if e.Limits == nil {
		return ""
	}
	return e.Limits[k]
}

// expectedRow is one row of the 7.1 production layout table.
type expectedRow struct {
	namespace string
	services  []string
	minRep    string
	maxRep    string
	cpuLimit  string
	memLimit  string
	gpu       bool
}

var layout = []expectedRow{
	{"ecco9-core", []string{"ecco9-reservoir", "ecco9-echobeats", "ecco9-emotion", "ecco9-consciousness"}, "2", "4", "2", "4Gi", false},
	{"ecco9-memory", []string{"ecco9-memory", "ecco9-atomspace", "ecco9-identity"}, "2", "3", "4", "8Gi", false},
	{"ecco9-inference", []string{"ecco9-llm-gateway", "ecco9-runner"}, "2", "8", "", "", true},
	{"ecco9-higher", []string{"ecco9-wisdom", "ecco9-metacog", "ecco9-echodream", "ecco9-ontogenesis"}, "1", "2", "1", "2Gi", false},
	{"ecco9-platform", []string{"ecco9-gateway", "ecco9-orchestrator"}, "3", "", "2", "4Gi", false},
}

func deployDir(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return d
}

// TestProductionLayoutCoverage asserts every row of the 7.1 table is
// represented in topology.yaml with the right namespace, replica range, and
// resource envelope.
func TestProductionLayoutCoverage(t *testing.T) {
	entries := parseTopology(t, filepath.Join(deployDir(t), "topology.yaml"))

	byRelease := map[string]chartEntry{}
	for _, e := range entries {
		byRelease[e.ReleaseName] = e
	}

	for _, row := range layout {
		for _, svc := range row.services {
			e, ok := byRelease[svc]
			if !ok {
				t.Errorf("7.1: service %s missing from topology.yaml", svc)
				continue
			}
			if e.Namespace != row.namespace {
				t.Errorf("7.1: %s namespace = %q, want %q", svc, e.Namespace, row.namespace)
			}
			if e.MinReplicas != row.minRep {
				t.Errorf("7.1: %s hpa.minReplicas = %q, want %q", svc, e.MinReplicas, row.minRep)
			}
			if row.maxRep != "" && e.MaxReplicas != row.maxRep {
				t.Errorf("7.1: %s hpa.maxReplicas = %q, want %q", svc, e.MaxReplicas, row.maxRep)
			}
			if !row.gpu {
				if got := e.limit("cpu"); got != row.cpuLimit {
					t.Errorf("7.1: %s cpu limit = %q, want %q", svc, got, row.cpuLimit)
				}
				if got := e.limit("memory"); got != row.memLimit {
					t.Errorf("7.1: %s memory limit = %q, want %q", svc, got, row.memLimit)
				}
			}
		}
	}

	// Platform floor: "3+ each" — minReplicas must be >= 3.
	for _, svc := range []string{"ecco9-gateway", "ecco9-orchestrator"} {
		e := byRelease[svc]
		if e.MinReplicas < "3" {
			t.Errorf("7.1: %s minReplicas = %q, want >= 3", svc, e.MinReplicas)
		}
	}
}

// TestGPUNodePlacement asserts inference services are pinned to GPU nodes.
func TestGPUNodePlacement(t *testing.T) {
	entries := parseTopology(t, filepath.Join(deployDir(t), "topology.yaml"))
	var runner *chartEntry
	for i := range entries {
		if entries[i].ReleaseName == "ecco9-runner" {
			runner = &entries[i]
		}
	}
	if runner == nil {
		t.Fatal("ecco9-runner entry missing")
	}
	if runner.Limits["nvidia.com/gpu"] != "1" {
		t.Errorf("runner must request nvidia.com/gpu, limits = %v", runner.Limits)
	}
	joined := strings.Join(runner.Placement, " ")
	if !strings.Contains(joined, "tolerations") && !strings.Contains(joined, "ecco9.io/gpu") {
		t.Errorf("runner placement must tolerate the gpu taint, placement = %v", runner.Placement)
	}
}

// TestReservoirTemporalCore asserts the reservoir is pinned to
// high-memory-bandwidth nodes and co-located with the LLM gateway
// (issue #8, design decision 2).
func TestReservoirTemporalCore(t *testing.T) {
	entries := parseTopology(t, filepath.Join(deployDir(t), "topology.yaml"))
	var res *chartEntry
	for i := range entries {
		if entries[i].ReleaseName == "ecco9-reservoir" {
			res = &entries[i]
		}
	}
	if res == nil {
		t.Fatal("ecco9-reservoir entry missing")
	}
	joined := strings.Join(res.Placement, " ")
	if !strings.Contains(joined, "memory-bandwidth") {
		t.Errorf("reservoir must target memory-bandwidth nodes, placement = %v", res.Placement)
	}
	if !strings.Contains(joined, "ecco9-llm-gateway") {
		t.Errorf("reservoir must co-locate with ecco9-llm-gateway, placement = %v", res.Placement)
	}
}

// TestNamespaces asserts the six 7.1 namespaces exist with tier labels.
func TestNamespaces(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(deployDir(t), "namespaces", "namespaces.yaml"))
	if err != nil {
		t.Fatalf("read namespaces: %v", err)
	}
	text := string(data)
	for _, ns := range []string{"ecco9-core", "ecco9-memory", "ecco9-inference", "ecco9-higher", "ecco9-platform", "ecco9-data"} {
		if !strings.Contains(text, "name: "+ns) {
			t.Errorf("namespace %s missing from namespaces.yaml", ns)
		}
	}
	if !strings.Contains(text, "ecco9.io/tier") {
		t.Error("namespaces must carry ecco9.io/tier labels")
	}
}

// TestMultiRegionArtifacts asserts the four 7.2 multi-region mechanisms are
// present and reference their replication technology.
func TestMultiRegionArtifacts(t *testing.T) {
	cases := []struct {
		file     string
		keywords []string
	}{
		{"nats-supercluster.yaml", []string{"gateway", "supercluster", "identity", "echo.resonance"}},
		{"reservoir-crdt.yaml", []string{"crdt", "eventual", "merge", "ecco9-reservoir"}},
		{"model-artifacts.yaml", []string{"cdn", "replicat", "cache", "model"}},
		{"orchestrator-federation.yaml", []string{"federation", "global", "regional", "orchestrator"}},
	}
	for _, c := range cases {
		path := filepath.Join(deployDir(t), "multiregion", c.file)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("7.2: read %s: %v", c.file, err)
			continue
		}
		lower := strings.ToLower(string(data))
		for _, kw := range c.keywords {
			if !strings.Contains(lower, kw) {
				t.Errorf("7.2: %s missing keyword %q", c.file, kw)
			}
		}
	}
}

// TestCognitiveLoadScaling asserts every HPA targets the cognitive-load
// custom metric rather than CPU (issue #8, design decision 5).
func TestCognitiveLoadScaling(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(deployDir(t), "topology.yaml"))
	if err != nil {
		t.Fatalf("read topology: %v", err)
	}
	if !strings.Contains(string(data), "cognitiveLoadTarget") {
		t.Error("topology HPAs must target cognitive load (cognitiveLoadTarget)")
	}
	// The library chart HPA must scale on ecco9_cognitive_load, not CPU.
	hpa, err := os.ReadFile(filepath.Join(deployDir(t), "..", "helm", "ecco9-service", "templates", "_hpa.yaml"))
	if err != nil {
		t.Fatalf("read library HPA template: %v", err)
	}
	if !strings.Contains(string(hpa), "ecco9_cognitive_load") {
		t.Error("library HPA must scale on ecco9_cognitive_load metric")
	}
}
