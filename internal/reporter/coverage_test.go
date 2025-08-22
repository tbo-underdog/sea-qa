package reporter_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"sea-qa/internal/reporter"
)

func TestWriteCoverage(t *testing.T) {
	spec := `
openapi: 3.0.3
info: {title: X, version: "1"}
paths:
  /users:
    post: { responses: { "201": { description: ok } } }
    get:  { responses: { "200": { description: ok } } }
  /health:
    get: { responses: { "200": { description: ok } } }
`
	loader := &openapi3.Loader{}
	doc, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	covered := map[string]map[string]bool{
		"POST": {"/users": true},
	}

	var buf bytes.Buffer
	if err := reporter.WriteCoverage(&buf, doc, covered); err != nil {
		t.Fatalf("WriteCoverage: %v", err)
	}

	var rep reporter.CoverageReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rep.Total != 3 {
		t.Fatalf("total=%d", rep.Total)
	}
	if rep.Covered != 1 {
		t.Fatalf("covered=%d", rep.Covered)
	}
	if rep.Percent <= 30 || rep.Percent >= 40 {
		t.Fatalf("percent=%v", rep.Percent)
	}
}
