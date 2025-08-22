package executor_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"sea-qa/internal/executor"
	"sea-qa/internal/ir"
)

func newTestServer() (*httptest.Server, *int32) {
	cleanupCount := new(int32)
	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			type req struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			}
			var in req
			_ = json.NewDecoder(r.Body).Decode(&in)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    "u-123",
				"email": in.Email,
				"name":  in.Name,
			})
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		// Always 200 to trigger a failing expectation (e.g., expect 418)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			atomic.AddInt32(cleanupCount, 1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	return srv, cleanupCount
}

func TestExecutor_StatusAndJSONPath_WithUUIDAndTeardown(t *testing.T) {
	srv, cleanupCount := newTestServer()
	defer srv.Close()

	suite := &ir.TestSuite{
		Name: "Users API",
		Scenarios: []ir.Scenario{
			{
				Name: "Create user 201, jsonPath email matches, teardown runs",
				Steps: []ir.Step{
					{
						Request: ir.Request{
							Method: http.MethodPost,
							URL:    srv.URL + "/users",
							Headers: map[string]string{
								"Content-Type": "application/json",
							},
							Body: map[string]any{
								"email": "qa+${uuid}@example.com",
								"name":  "Test User",
							},
							TimeoutMs: 2000,
						},
						Expect: []ir.Expectation{
							{Type: ir.ExpectStatus, Target: "code", Value: 201},
							{Type: ir.ExpectJSONPath, Target: "$.email", Value: "qa+${uuid}@example.com"},
						},
					},
				},
				Teardown: []ir.Action{
					{
						Name: "cleanup",
						Request: &ir.Request{
							Method:    http.MethodPost,
							URL:       srv.URL + "/cleanup",
							TimeoutMs: 1000,
						},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := executor.New().RunSuite(ctx, suite)
	if err != nil {
		t.Fatalf("RunSuite error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("suite should pass, got %+v", res)
	}
	if got := atomic.LoadInt32(cleanupCount); got != 1 {
		t.Fatalf("cleanup count = %d, want 1", got)
	}
}

func TestExecutor_TeardownRunsOnFailure_AndIsolation(t *testing.T) {
	srv, cleanupCount := newTestServer()
	defer srv.Close()

	suite := &ir.TestSuite{
		Name: "Failure path still tears down",
		Scenarios: []ir.Scenario{
			{
				Name: "This one fails expectations",
				Steps: []ir.Step{
					{
						Request: ir.Request{
							Method:    http.MethodPost,
							URL:       srv.URL + "/fail",
							TimeoutMs: 1000,
						},
						Expect: []ir.Expectation{
							{Type: ir.ExpectStatus, Target: "code", Value: 418}, // will fail
						},
					},
				},
				Teardown: []ir.Action{
					{Request: &ir.Request{Method: http.MethodPost, URL: srv.URL + "/cleanup", TimeoutMs: 1000}},
				},
			},
			{
				Name: "This one passes and also tears down",
				Steps: []ir.Step{
					{
						Request: ir.Request{
							Method: http.MethodPost,
							URL:    srv.URL + "/users",
							Headers: map[string]string{
								"Content-Type": "application/json",
							},
							Body: map[string]any{
								"email": "qa+${uuid}@example.com",
								"name":  "Other",
							},
							TimeoutMs: 1000,
						},
						Expect: []ir.Expectation{
							{Type: ir.ExpectStatus, Target: "code", Value: 201},
							{Type: ir.ExpectJSONPath, Target: "$.email", Value: "qa+${uuid}@example.com"},
						},
					},
				},
				Teardown: []ir.Action{
					{Request: &ir.Request{Method: http.MethodPost, URL: srv.URL + "/cleanup", TimeoutMs: 1000}},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := executor.New().RunSuite(ctx, suite)
	if err != nil {
		t.Fatalf("RunSuite error: %v", err)
	}

	if res.Passed {
		t.Fatalf("suite should fail because one scenario fails: %+v", res)
	}

	// Both teardowns should have run (even the failing scenario)
	if got := atomic.LoadInt32(cleanupCount); got != 2 {
		t.Fatalf("cleanup count = %d, want 2", got)
	}

	// Ensure per-scenario results reflect one pass / one fail
	var passed, failed int
	for _, sc := range res.Scenarios {
		if sc.Passed {
			passed++
		} else {
			failed++
		}
	}
	if passed != 1 || failed != 1 {
		t.Fatalf("want 1 passed and 1 failed scenario, got passed=%d failed=%d", passed, failed)
	}
}
