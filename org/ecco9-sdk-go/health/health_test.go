package health

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

func TestReporterDefaults(t *testing.T) {
	r := NewReporter()
	if r.Status() != contracts.ServingStatusServing {
		t.Fatalf("default status = %v, want SERVING", r.Status())
	}
	if r.Coherence() != 1.0 {
		t.Fatalf("default coherence = %v, want 1.0", r.Coherence())
	}
}

func TestHandlerReportsState(t *testing.T) {
	r := NewReporter()
	r.SetStatus(contracts.ServingStatusDegraded)
	r.SetCoherence(0.62)
	r.SetLoad(0.85)

	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("status code = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"].(float64) != float64(contracts.ServingStatusDegraded) {
		t.Fatalf("status = %v", body["status"])
	}
	if body["identity_coherence"].(float64) != 0.62 {
		t.Fatalf("identity_coherence = %v", body["identity_coherence"])
	}
	if body["cognitive_load"].(float64) != 0.85 {
		t.Fatalf("cognitive_load = %v", body["cognitive_load"])
	}
}
