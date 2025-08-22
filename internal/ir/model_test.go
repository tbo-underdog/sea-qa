package ir_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"sea-qa/internal/ir"
)

func TestIR_Basics(t *testing.T) {
	suite := ir.TestSuite{
		Name: "Users API",
		Scenarios: []ir.Scenario{
			{
				Name: "Create user returns 201",
				Steps: []ir.Step{
					{
						Request: ir.Request{
							Method:  "POST",
							URL:     "http://localhost:8080/users",
							Headers: map[string]string{"Content-Type": "application/json"},
							Body: map[string]any{
								"email": "qa+${uuid}@example.com",
								"name":  "Test User",
							},
							TimeoutMs: 10000,
						},
						Expect: []ir.Expectation{
							{Type: ir.ExpectStatus, Target: "code", Value: 201},
							{Type: ir.ExpectJSONPath, Target: "$.email", Value: "qa+${uuid}@example.com"},
						},
					},
				},
				Setup:    []ir.Action{},
				Teardown: []ir.Action{},
				Tags:     []string{"users", "smoke"},
				Env:      "staging",
			},
		},
	}

	if diff := cmp.Diff("Users API", suite.Name); diff != "" {
		t.Fatalf("suite name mismatch (-want +got):\n%s", diff)
	}
	if got, want := len(suite.Scenarios), 1; got != want {
		t.Fatalf("scenarios len = %d, want %d", got, want)
	}

	step := suite.Scenarios[0].Steps[0]
	if step.Request.Method != "POST" {
		t.Fatalf("method = %s, want POST", step.Request.Method)
	}
	if step.Request.URL == "" {
		t.Fatal("url must not be empty")
	}
	if step.Request.TimeoutMs == 0 {
		t.Fatal("timeout should propagate")
	}
	if len(step.Expect) != 2 {
		t.Fatalf("expect len = %d, want 2", len(step.Expect))
	}
}
