package reporter_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"sea-qa/internal/executor"
	"sea-qa/internal/reporter"
)

func TestWriteJSON_Basic(t *testing.T) {
	res := &executor.SuiteResult{
		Passed: true,
		Scenarios: []executor.ScenarioResult{
			{Name: "S1", Passed: true, Steps: []executor.StepResult{{Passed: true}}},
		},
	}

	var buf bytes.Buffer
	if err := reporter.WriteJSON(&buf, res); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var roundtrip executor.SuiteResult
	if err := json.Unmarshal(buf.Bytes(), &roundtrip); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if !roundtrip.Passed {
		t.Fatalf("roundtrip.Passed = false, want true")
	}
	if len(roundtrip.Scenarios) != 1 {
		t.Fatalf("roundtrip scenarios len = %d, want 1", len(roundtrip.Scenarios))
	}
}
