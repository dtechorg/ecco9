package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConcurrentPersonaSwitchesPersistSafely(t *testing.T) {
	svc := New(t.TempDir(), "deep-tree-echo")
	ts := httptest.NewServer(svc.Routes())
	defer ts.Close()

	personas := []string{"contemplative-scholar", "dynamic-explorer", "cautious-analyst"}
	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := 0; i < 60; i++ {
		persona := personas[i%len(personas)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := []byte(fmt.Sprintf(`{"identity_id":"deep-tree-echo","persona":%q}`, persona))
			resp, err := http.Post(ts.URL+"/v1/identity/persona", "application/json", bytes.NewReader(body))
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("unexpected status %s", resp.Status)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := svc.Store.SaveCount(); got != 60 {
		t.Fatalf("expected 60 persisted updates, got %d", got)
	}
}
