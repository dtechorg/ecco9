package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrainingValidationEndpoint(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/metacog/training_validation",
		strings.NewReader(`{"checkpoint_version":3,"held_out_mse":0.12,"train_error":0.05,"samples_trained":128}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("training_validation = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"passed":true`) {
		t.Fatalf("expected passed:true, got %s", body)
	}
	if !strings.Contains(body, `"checkpoint_version":3`) {
		t.Fatalf("expected checkpoint_version:3, got %s", body)
	}
}

func TestTrainingValidationFailureFlowsToAssessment(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/metacog/training_validation",
		strings.NewReader(`{"checkpoint_version":4,"held_out_mse":0.9,"train_error":0.4,"samples_trained":64}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("training_validation = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"passed":false`) {
		t.Fatalf("expected passed:false, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/metacog/assessment", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("assessment = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Rollback reservoir") {
		t.Fatalf("expected rollback recommendation, got %s", rec.Body.String())
	}
}
