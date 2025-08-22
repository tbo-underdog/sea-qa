package executor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sea-qa/internal/executor"
	"sea-qa/internal/ir"
)

func TestRunSuite_ParallelScenarios(t *testing.T) {
	// Mock server that sleeps 250ms per request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Two scenarios, each one step hitting the slow endpoint.
	suite := &ir.TestSuite{
		Name: "parallel",
		Scenarios: []ir.Scenario{
			{Name: "A", Steps: []ir.Step{{Request: ir.Request{Method: "GET", URL: srv.URL, TimeoutMs: 2000},
				Expect: []ir.Expectation{{Type: ir.ExpectStatus, Target: "code", Value: 200}}}}},
			{Name: "B", Steps: []ir.Step{{Request: ir.Request{Method: "GET", URL: srv.URL, TimeoutMs: 2000},
				Expect: []ir.Expectation{{Type: ir.ExpectStatus, Target: "code", Value: 200}}}}},
		},
	}

	// Parallel(2) should finish in ~250-350ms instead of ~500ms (sequential).
	r := executor.New().WithParallel(2)
	start := time.Now()
	res, err := r.RunSuite(context.Background(), suite)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	elapsed := time.Since(start)

	if !res.Passed {
		t.Fatalf("suite failed: %+v", res)
	}
	if elapsed >= 450*time.Millisecond {
		t.Fatalf("expected parallel speedup (<450ms), got %v", elapsed)
	}
}
