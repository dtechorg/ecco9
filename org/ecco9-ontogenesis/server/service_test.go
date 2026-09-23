package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrainingMetricsEndpoint(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/ontogenesis/training_metrics",
		strings.NewReader(`{"checkpoint_version":5,"samples_trained":256,"last_error":0.08,"held_out_mse":0.12,"spectral_radius":0.99,"input_scaling":0.2,"leak_rate":0.3,"forgetting_factor":0.99}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("training_metrics = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"genome_id"`) {
		t.Fatalf("expected genome_id, got %s", rec.Body.String())
	}
}
