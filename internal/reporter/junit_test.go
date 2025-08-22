package reporter_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"sea-qa/internal/executor"
	"sea-qa/internal/reporter"
)

func TestWriteJUnit_Basic(t *testing.T) {
	res := &executor.SuiteResult{
		Passed: false,
		Scenarios: []executor.ScenarioResult{
			{
				Name:   "Scenario A",
				Passed: true,
				Steps: []executor.StepResult{
					{Passed: true, StatusCode: 201},
				},
			},
			{
				Name:   "Scenario B",
				Passed: false,
				Steps: []executor.StepResult{
					{Passed: false, StatusCode: 418, Errors: []string{"status: got 200, want 418"}},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := reporter.WriteJUnit(&buf, "Users API", res); err != nil {
		t.Fatalf("WriteJUnit error: %v", err)
	}

	// sanity: XML starts with <testsuite ...>
	out := buf.String()
	if !strings.Contains(out, "<testsuite") {
		t.Fatalf("expected testsuite root, got: %s", out[:min(200, len(out))])
	}

	// well-formed XML
	var v struct{}
	if err := xml.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("invalid xml: %v", err)
	}

	// counts
	if !strings.Contains(out, `tests="2"`) {
		t.Fatalf("expected tests=2, got %s", out)
	}
	if !strings.Contains(out, `failures="1"`) {
		t.Fatalf("expected failures=1, got %s", out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
