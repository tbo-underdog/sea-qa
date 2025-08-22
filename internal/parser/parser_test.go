package parser_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sea-qa/internal/ir"
	"sea-qa/internal/parser"
)

const validYAML = `
name: Users API
scenarios:
  - name: Create user returns 201
    env: staging
    tags: [users, smoke]
    setup: []
    steps:
      - request:
          method: POST
          url: http://localhost:8080/users
          timeout_ms: 10000
          headers:
            Content-Type: application/json
          body:
            email: "qa+${uuid}@example.com"
            name: "Test User"
        expect:
          - type: status
            target: code
            value: 201
          - type: jsonPath
            target: $.email
            value: "qa+${uuid}@example.com"
    teardown: []
`

const missingNameYAML = `
scenarios: []
`

const unknownFieldYAML = `
name: Foo
scenarios:
  - name: Bar
    steps:
      - request:
          method: POST
          url: http://localhost:8080
        expect: []
    notARealField: true
`

func TestParse_ValidSuite(t *testing.T) {
	p := parser.New()

	suite, err := p.ParseBytes([]byte(validYAML))
	if err != nil {
		t.Fatalf("ParseBytes error: %v", err)
	}
	if suite == nil {
		t.Fatal("suite is nil")
	}
	if diff := cmp.Diff("Users API", suite.Name); diff != "" {
		t.Fatalf("name mismatch (-want +got):\n%s", diff)
	}

	if len(suite.Scenarios) != 1 {
		t.Fatalf("scenarios len = %d, want 1", len(suite.Scenarios))
	}

	sc := suite.Scenarios[0]
	if sc.Env != "staging" {
		t.Fatalf("env = %s, want staging", sc.Env)
	}
	if got, want := len(sc.Steps), 1; got != want {
		t.Fatalf("steps len = %d, want %d", got, want)
	}
	step := sc.Steps[0]
	if step.Request.Method != "POST" {
		t.Fatalf("method = %s, want POST", step.Request.Method)
	}
	if step.Request.TimeoutMs != 10000 {
		t.Fatalf("timeoutMs = %d, want 10000", step.Request.TimeoutMs)
	}
	if got, want := len(step.Expect), 2; got != want {
		t.Fatalf("expect len = %d, want %d", got, want)
	}
	if step.Expect[0].Type != ir.ExpectStatus {
		t.Fatalf("expect[0].type = %s, want %s", step.Expect[0].Type, ir.ExpectStatus)
	}
}

func TestParse_Validation_MissingName(t *testing.T) {
	p := parser.New()

	_, err := p.ParseBytes([]byte(missingNameYAML))
	if err == nil {
		t.Fatal("expected error for missing suite name, got nil")
	}
	if !errors.Is(err, parser.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestParse_KnownFieldsEnforced(t *testing.T) {
	p := parser.New()

	_, err := p.ParseBytes([]byte(unknownFieldYAML))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}
