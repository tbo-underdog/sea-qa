package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sea-qa/internal/contract"
	"sea-qa/internal/executor"
	"sea-qa/internal/ir"
)

const openapiYAML = `
openapi: 3.0.3
info: { title: Test API, version: "1.0.0" }
paths:
  /users:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                email: { type: string }
                name: { type: string }
              required: [email, name]
      responses:
        "201":
          description: created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id: { type: string }
                  email: { type: string }
                  name: { type: string }
                required: [id, email, name]
  /health:
    get:
      responses:
        "200": { description: ok }
`

func newServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "u-1",
			"email": "qa@example.com",
			"name":  "T",
		})
	})
	return httptest.NewServer(mux)
}

func TestContract_ValidatesResponse_OK(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	v, err := contract.LoadFromBytes([]byte(openapiYAML))
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}

	suite := &ir.TestSuite{
		Name: "Contract OK",
		Scenarios: []ir.Scenario{{
			Name: "POST /users",
			Steps: []ir.Step{{
				Request: ir.Request{
					Method:    http.MethodPost,
					URL:       srv.URL + "/users",
					Headers:   map[string]string{"Content-Type": "application/json"},
					Body:      map[string]any{"email": "qa@example.com", "name": "T"},
					TimeoutMs: 2000,
				},
				Expect: []ir.Expectation{
					{Type: ir.ExpectStatus, Target: "code", Value: 201},
					{Type: ir.ExpectContract, Value: true},
				},
			}},
		}},
	}

	res, err := executor.New().WithContract(v).RunSuite(context.Background(), suite)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if !res.Passed {
		t.Fatalf("suite should pass, got: %+v", res)
	}
}

func TestContract_StatusMismatch_Fails(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	v, err := contract.LoadFromBytes([]byte(openapiYAML))
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}

	// Force bad status expectation (expects 200 but server returns 201)
	suite := &ir.TestSuite{
		Name: "Contract bad",
		Scenarios: []ir.Scenario{{
			Name: "POST /users",
			Steps: []ir.Step{{
				Request: ir.Request{
					Method:  http.MethodPost,
					URL:     srv.URL + "/users",
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    map[string]any{"email": "qa@example.com", "name": "T"},
				},
				Expect: []ir.Expectation{
					{Type: ir.ExpectStatus, Target: "code", Value: 200}, // wrong
					{Type: ir.ExpectContract, Value: true},
				},
			}},
		}},
	}

	res, _ := executor.New().WithContract(v).RunSuite(context.Background(), suite)
	if res.Passed {
		t.Fatalf("suite should fail due to status mismatch")
	}
	// Optional: ensure failure text mentions status or contract
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(res)
	if !bytes.Contains(buf.Bytes(), []byte("status")) && !bytes.Contains(buf.Bytes(), []byte("contract")) {
		t.Fatalf("expected status/contract failure details in result")
	}
}

func TestContract_ResponseMissingContentType_Fails(t *testing.T) {
	// Spec that *requires* JSON body for 201 on POST /users
	v, err := contract.LoadFromBytes([]byte(openapiYAML))
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}

	// Server that returns 201 JSON body BUT NO Content-Type header
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusCreated) // <- no Content-Type
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "u-1", "email": "qa@example.com", "name": "T",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Run a suite that expects contract validation
	suite := &ir.TestSuite{
		Name: "Contract Missing CT",
		Scenarios: []ir.Scenario{{
			Name: "POST /users without Content-Type in response",
			Steps: []ir.Step{{
				Request: ir.Request{
					Method:  http.MethodPost,
					URL:     srv.URL + "/users",
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    map[string]any{"email": "qa@example.com", "name": "T"},
				},
				Expect: []ir.Expectation{
					{Type: ir.ExpectStatus, Target: "code", Value: 201},
					{Type: ir.ExpectContract, Value: true},
				},
			}},
		}},
	}

	res, _ := executor.New().WithContract(v).RunSuite(context.Background(), suite)
	if res.Passed {
		t.Fatalf("suite should fail: missing response Content-Type must break contract")
	}
}
