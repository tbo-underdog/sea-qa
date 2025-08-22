// internal/vars/vars_test.go
package vars_test

import (
	"os"
	"path/filepath"
	"testing"

	"sea-qa/internal/vars"
)

func TestLoadJSONFiles(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "env.json")
	if err := os.WriteFile(fp, []byte(`{"BASE_URL":"http://x","NUM":42,"BOOL":true}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, err := vars.LoadJSONFiles([]string{fp})
	if err != nil {
		t.Fatalf("LoadJSONFiles: %v", err)
	}
	if m["BASE_URL"] != "http://x" {
		t.Fatalf("BASE_URL = %q", m["BASE_URL"])
	}
	if m["NUM"] != "42" {
		t.Fatalf("NUM = %q, want 42", m["NUM"])
	}
	if m["BOOL"] != "true" {
		t.Fatalf("BOOL = %q, want true", m["BOOL"])
	}
}
