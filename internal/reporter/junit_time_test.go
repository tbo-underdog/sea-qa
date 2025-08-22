package reporter_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"sea-qa/internal/executor"
	"sea-qa/internal/reporter"
)

func TestWriteJUnit_IncludesTimings(t *testing.T) {
	res := &executor.SuiteResult{
		Passed: true,
		Scenarios: []executor.ScenarioResult{
			{
				Name:       "S1",
				Passed:     true,
				DurationMs: 123.0,
				Steps: []executor.StepResult{
					{Passed: true, DurationMs: 45.5},
					{Passed: true, DurationMs: 77.5},
				},
			},
		},
		DurationMs: 123.0,
	}
	var buf bytes.Buffer
	if err := reporter.WriteJUnit(&buf, "timed", res); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `time="0.123"`) { // suite time in seconds
		t.Fatalf("expected suite time=\"0.123\", got: %s", out)
	}
	// ensure XML is valid
	var v any
	if err := xml.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("invalid xml: %v", err)
	}
}
