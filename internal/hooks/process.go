package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"sea-qa/internal/ir"
)

type Input struct {
	Vars     map[string]string `json:"vars,omitempty"`
	Request  *ir.Request       `json:"request,omitempty"`  // present for "before"
	Response *Resp             `json:"response,omitempty"` // present for "after"
}

type Resp struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    json.RawMessage     `json:"body,omitempty"` // raw bytes (may be JSON or not)
}

type Output struct {
	Vars    map[string]string `json:"vars,omitempty"`    // merged into runner vars
	Request *ReqPatch         `json:"request,omitempty"` // ONLY honored for "before"
	Errors  []string          `json:"errors,omitempty"`  // adds step errors
	Redact  []string          `json:"redact,omitempty"`  // reserved for future logging redaction
}

type ReqPatch struct {
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

func RunProcessHook(ctx context.Context, when string, h ir.Hook, in Input) (*Output, error) {
	if h.Type != "process" {
		return nil, fmt.Errorf("unsupported hook type %q", h.Type)
	}
	tmo := time.Duration(h.TimeoutMs) * time.Millisecond
	if tmo <= 0 {
		tmo = 10 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, tmo)
	defer cancel()

	cmd := exec.CommandContext(cctx, h.Cmd, h.Args...)

	// inherit env + add hook env
	cmd.Env = os.Environ()
	for k, v := range h.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	enc := json.NewEncoder(stdin)
	if err := enc.Encode(in); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("encode stdin: %w", err)
	}
	_ = stdin.Close()

	var out Output
	dec := json.NewDecoder(stdout)
	if err := dec.Decode(&out); err != nil {
		_ = cmd.Wait()
		return nil, fmt.Errorf("decode stdout: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("hook exit: %w", err)
	}

	// Guard: only "before" may change request
	if when != "before" {
		out.Request = nil
	}
	return &out, nil
}
