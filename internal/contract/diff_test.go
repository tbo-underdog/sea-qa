package contract_test

import (
	"encoding/json"
	"testing"

	"sea-qa/internal/contract"
)

const specA = `
openapi: 3.0.3
info: {title: A, version: "1"}
paths:
  /users:
    get:  { responses: {"200": {description: ok}} }
    post: { responses: {"201": {description: created}} }
  /health:
    get:  { responses: {"200": {description: ok}} }
`

const specB = `
openapi: 3.0.3
info: {title: B, version: "1"}
paths:
  /users:
    get:  { responses: {"200": {description: ok}} }
    post: { responses: {"200": {description: ok}} }   # status changed 201 -> 200
  /status:
    get:  { responses: {"200": {description: ok}} }   # new endpoint
`

func TestDiff_BasicAddRemoveAndStatus(t *testing.T) {
	a, err := contract.LoadFromBytes([]byte(specA))
	if err != nil {
		t.Fatalf("load A: %v", err)
	}
	b, err := contract.LoadFromBytes([]byte(specB))
	if err != nil {
		t.Fatalf("load B: %v", err)
	}

	rep := contract.DiffDocs(a.Doc(), b.Doc())

	// Added should include GET /status
	if !containsOp(rep.Added, "GET", "/status") {
		t.Fatalf("expected added GET /status, got: %+v", rep.Added)
	}
	// Removed should include GET /health
	if !containsOp(rep.Removed, "GET", "/health") {
		t.Fatalf("expected removed GET /health, got: %+v", rep.Removed)
	}

	// Changed status codes for POST /users (search the slice, not a map)
	var found *contract.StatusChange
	for i := range rep.ChangedStatus {
		ch := rep.ChangedStatus[i]
		if ch.Method == "POST" && ch.Path == "/users" {
			found = &ch
			break
		}
	}
	if found == nil {
		t.Fatalf("expected status change for POST /users")
	}
	if toCSV(found.A) != "201" || toCSV(found.B) != "200" {
		bs, _ := json.Marshal(found)
		t.Fatalf("status diff unexpected: %s", string(bs))
	}
}

func containsOp(ops []contract.OpSig, m, p string) bool {
	for _, o := range ops {
		if o.Method == m && o.Path == p {
			return true
		}
	}
	return false
}

func toCSV(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += "," + ss[i]
	}
	return out
}
