package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"sea-qa/internal/contract"
	"sea-qa/internal/hooks"
	"sea-qa/internal/ir"
)

// ---- Results model ----

type SuiteResult struct {
	Passed     bool
	Scenarios  []ScenarioResult
	DurationMs float64
}

type ScenarioResult struct {
	Name        string
	Passed      bool
	TeardownRan bool
	Steps       []StepResult
	DurationMs  float64
}

type StepResult struct {
	Name       string
	Passed     bool
	StatusCode int
	Errors     []string
	DurationMs float64

	Method      string
	URL         string
	ReqHeaders  map[string]string
	ReqBody     string
	RespHeaders map[string][]string
	RespBody    string
}

// ---- Runner ----

type Runner struct {
	httpClient *http.Client
	baseVars   map[string]string

	contractV *contract.Validator
	covered   map[string]map[string]bool // method -> pathTemplate -> true

	parallel int
	failFast bool
}

func New() *Runner {
	tr := &http.Transport{
		MaxIdleConns:        128,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Runner{httpClient: &http.Client{Transport: tr}}
}

func NewWithVars(vars map[string]string) *Runner {
	tr := &http.Transport{
		MaxIdleConns:        128,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Runner{httpClient: &http.Client{Transport: tr}, baseVars: clone(vars)}
}

func (r *Runner) WithContract(v *contract.Validator) *Runner {
	if r.covered == nil {
		r.covered = map[string]map[string]bool{}
	}
	r.contractV = v
	return r
}
func (r *Runner) WithParallel(n int) *Runner {
	if n < 1 {
		n = 1
	}
	r.parallel = n
	return r
}
func (r *Runner) WithFailFast(b bool) *Runner         { r.failFast = b; return r }
func (r *Runner) Covered() map[string]map[string]bool { return r.covered }

// ---- Suite execution ----

func clone(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (r *Runner) RunSuite(ctx context.Context, suite *ir.TestSuite) (*SuiteResult, error) {
	if suite == nil {
		return nil, errors.New("nil suite")
	}

	startSuite := time.Now()
	res := &SuiteResult{Passed: true, Scenarios: make([]ScenarioResult, len(suite.Scenarios))}

	parallel := r.parallel
	if r.failFast {
		parallel = 1
	}
	if parallel < 1 {
		parallel = 1
	}

	if parallel == 1 {
		for i, sc := range suite.Scenarios {
			scRes := r.runScenario(ctx, sc)
			if !scRes.Passed {
				res.Passed = false
			}
			res.Scenarios[i] = scRes
			if r.failFast && !scRes.Passed {
				res.Scenarios = res.Scenarios[:i+1]
				res.DurationMs = float64(time.Since(startSuite).Milliseconds())
				return res, nil
			}
		}
		res.DurationMs = float64(time.Since(startSuite).Milliseconds())
		return res, nil
	}

	type job struct {
		idx int
		sc  ir.Scenario
	}
	type result struct {
		idx int
		sc  ScenarioResult
	}

	jobs := make(chan job)
	results := make(chan result)

	for w := 0; w < parallel; w++ {
		go func() {
			for j := range jobs {
				results <- result{idx: j.idx, sc: r.runScenario(ctx, j.sc)}
			}
		}()
	}
	go func() {
		for i, sc := range suite.Scenarios {
			jobs <- job{idx: i, sc: sc}
		}
		close(jobs)
	}()

	for collected := 0; collected < len(suite.Scenarios); collected++ {
		rx := <-results
		if !rx.sc.Passed {
			res.Passed = false
		}
		res.Scenarios[rx.idx] = rx.sc
	}

	res.DurationMs = float64(time.Since(startSuite).Milliseconds())
	return res, nil
}

func (r *Runner) runScenario(ctx context.Context, sc ir.Scenario) ScenarioResult {
	vars := clone(r.baseVars)
	if vars == nil {
		vars = map[string]string{}
	}
	vars["uuid"] = newUUID()
	vars["now"] = time.Now().UTC().Format(time.RFC3339)

	startSc := time.Now()
	scRes := ScenarioResult{Name: sc.Name, Passed: true}

	// Setup (best-effort)
	if err := r.runActions(ctx, sc.Setup, vars); err != nil {
		scRes.Passed = false
	}

	// Steps
	for _, st := range sc.Steps {
		stepRes := StepResult{Passed: true}
		req := expandRequest(st.Request, vars)

		// BEFORE hooks
		for _, hk := range st.Hooks {
			if strings.ToLower(hk.When) != "before" {
				continue
			}
			out, err := hooks.RunProcessHook(ctx, "before", hk, hooks.Input{
				Vars:    clone(vars),
				Request: &req,
			})
			if err != nil {
				stepRes.Passed = false
				stepRes.Errors = append(stepRes.Errors, fmt.Sprintf("hook(before) error: %v", err))
				continue
			}
			// apply returned vars
			for k, v := range out.Vars {
				if v != "" {
					vars[k] = v
				}
			}
			// apply request patch (if any)
			if out.Request != nil {
				if out.Request.URL != "" {
					req.URL = out.Request.URL
				}
				if out.Request.Method != "" {
					req.Method = strings.ToUpper(out.Request.Method)
				}
				for k, v := range out.Request.Headers {
					if req.Headers == nil {
						req.Headers = map[string]string{}
					}
					req.Headers[k] = v
				}
				if out.Request.Body != nil {
					req.Body = out.Request.Body
				}
			}
			// add hook-declared errors
			if len(out.Errors) > 0 {
				stepRes.Passed = false
				stepRes.Errors = append(stepRes.Errors, out.Errors...)
			}
		}

		// Capture request details for report (after hooks have possibly mutated it)
		stepRes.Method = req.Method
		stepRes.URL = req.URL
		stepRes.ReqHeaders = clone(req.Headers)
		stepRes.ReqBody = stringifyBody(req.Body)

		// Guard unresolved vars in URL (clear error instead of bad URL)
		if unresolved := findUnresolved(req.URL); len(unresolved) > 0 {
			stepRes.Passed = false
			stepRes.Errors = append(stepRes.Errors,
				fmt.Sprintf("unresolved variables in URL: %s (define via --env or use ${VAR|default})",
					strings.Join(unresolved, ", ")))
			scRes.Passed = false
			scRes.Steps = append(scRes.Steps, stepRes)
			continue
		}

		startStep := time.Now()
		status, body, respHdrs, err := r.doRequest(ctx, req)
		stepRes.DurationMs = float64(time.Since(startStep).Milliseconds())

		// Capture response
		stepRes.StatusCode = status
		stepRes.RespHeaders = respHdrs
		stepRes.RespBody = limitBody(body, 64<<10) // 64KB cap in report

		if err != nil {
			stepRes.Passed = false
			stepRes.Errors = append(stepRes.Errors, fmt.Sprintf("request error: %v", err))
		}

		// AFTER hooks
		for _, hk := range st.Hooks {
			if strings.ToLower(hk.When) != "after" {
				continue
			}
			raw := json.RawMessage(body) // may be non-JSON; still pass through
			out, err := hooks.RunProcessHook(ctx, "after", hk, hooks.Input{
				Vars:    clone(vars),
				Request: &req,
				Response: &hooks.Resp{
					Status:  status,
					Headers: respHdrs,
					Body:    raw,
				},
			})
			if err != nil {
				stepRes.Passed = false
				stepRes.Errors = append(stepRes.Errors, fmt.Sprintf("hook(after) error: %v", err))
				continue
			}
			for k, v := range out.Vars {
				if v != "" {
					vars[k] = v
				}
			}
			if len(out.Errors) > 0 {
				stepRes.Passed = false
				stepRes.Errors = append(stepRes.Errors, out.Errors...)
			}
		}

		// Parse JSON body (best-effort) for expectations
		var jsonBody map[string]any
		if len(body) > 0 {
			_ = json.Unmarshal(body, &jsonBody)
		}

		// Expectations (including contract)
		for _, exp := range st.Expect {
			ok, msg := r.evalExpectation(
				exp, status, jsonBody, vars,
				req.Method, req.URL, respHdrs, body,
			)
			if !ok {
				stepRes.Passed = false
				stepRes.Errors = append(stepRes.Errors, msg)
			}
		}

		if !stepRes.Passed {
			scRes.Passed = false
		}
		scRes.Steps = append(scRes.Steps, stepRes)
	}

	_ = r.runActions(ctx, sc.Teardown, vars)
	scRes.TeardownRan = true
	scRes.DurationMs = float64(time.Since(startSc).Milliseconds())

	return scRes
}

func (r *Runner) runActions(ctx context.Context, acts []ir.Action, vars map[string]string) error {
	for _, a := range acts {
		if a.Request == nil {
			continue
		}
		_, _, _, err := r.doRequest(ctx, expandRequest(*a.Request, vars))
		if err != nil {
			return err
		}
	}
	return nil
}

// ---- HTTP ----

func (r *Runner) doRequest(ctx context.Context, req ir.Request) (int, []byte, map[string][]string, error) {
	tmo := time.Duration(req.TimeoutMs) * time.Millisecond
	if tmo <= 0 {
		tmo = 10 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, tmo)
	defer cancel()

	var body io.Reader
	switch b := req.Body.(type) {
	case nil:
	case string:
		body = bytes.NewBufferString(b)
	case map[string]any, []any:
		buf, err := json.Marshal(b)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("json marshal body: %w", err)
		}
		body = bytes.NewBuffer(buf)
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("unsupported body type %T", req.Body)
		}
		body = bytes.NewBuffer(buf)
	}

	httpReq, err := http.NewRequestWithContext(cctx, req.Method, req.URL, body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("new request: %w", err)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, resp.Header, nil
}

// ---- Expectations ----

func (r *Runner) evalExpectation(
	exp ir.Expectation,
	status int,
	jsonBody map[string]any,
	vars map[string]string,
	method string,
	url string,
	respHeaders map[string][]string,
	rawBody []byte,
) (bool, string) {
	switch exp.Type {
	case ir.ExpectStatus:
		want, ok := exp.Value.(int)
		if !ok {
			if f, fok := exp.Value.(float64); fok {
				want = int(f)
				ok = true
			}
		}
		if !ok {
			return false, "status expectation has non-integer value"
		}
		if status != want {
			return false, fmt.Sprintf("status: got %d, want %d", status, want)
		}
		return true, ""

	case ir.ExpectJSONPath:
		path := strings.TrimPrefix(exp.Target, "$.")
		got, ok := jsonBody[path]
		if !ok {
			return false, fmt.Sprintf("jsonPath: %s not found", exp.Target)
		}
		want := exp.Value
		if ws, ok := want.(string); ok {
			want = interpolate(ws, vars)
		}
		if gs, ok := got.(string); ok {
			if gs != want {
				return false, fmt.Sprintf("jsonPath %s: got %v, want %v", exp.Target, gs, want)
			}
			return true, ""
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return false, fmt.Sprintf("jsonPath %s: got %v, want %v", exp.Target, got, want)
		}
		return true, ""

	case ir.ExpectContract:
		if r.contractV == nil {
			return false, "contract: requested but no OpenAPI spec configured"
		}
		path, mth, err := r.contractV.ValidateResponse(context.Background(), method, url, status, respHeaders, rawBody)
		if err != nil {
			return false, fmt.Sprintf("contract: %v", err)
		}
		if r.covered[mth] == nil {
			r.covered[mth] = map[string]bool{}
		}
		r.covered[mth][path] = true
		return true, ""

	default:
		return false, fmt.Sprintf("unknown expectation type: %s", exp.Type)
	}
}

// ---- Interpolation (with defaults + unresolved guard) ----

var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandRequest(rq ir.Request, vars map[string]string) ir.Request {
	rq.URL = interpolate(rq.URL, vars)
	for k, v := range rq.Headers {
		rq.Headers[k] = interpolate(v, vars)
	}
	rq.Body = walkInterpolate(rq.Body, vars)
	rq.Method = strings.ToUpper(rq.Method)
	return rq
}

func walkInterpolate(v any, vars map[string]string) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return interpolate(x, vars)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = walkInterpolate(vv, vars)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = walkInterpolate(x[i], vars)
		}
		return out
	default:
		return v
	}
}

// ${KEY|default} supported; if missing and no default, leaves ${KEY} intact (so we can error clearly)
func interpolate(s string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-1]
		key, def := inner, ""
		if i := strings.Index(inner, "|"); i >= 0 {
			key, def = inner[:i], inner[i+1:]
		}
		if v, ok := vars[key]; ok && v != "" {
			return v
		}
		if def != "" {
			return def
		}
		return m
	})
}

func findUnresolved(s string) []string {
	var out []string
	for _, m := range varPattern.FindAllStringSubmatch(s, -1) {
		key := m[1]
		if i := strings.Index(key, "|"); i >= 0 {
			continue
		} // had default
		out = append(out, "${"+key+"}")
	}
	return out
}

// ---- small helpers ----

func newUUID() string {
	now := time.Now().UnixNano()
	return fmt.Sprintf("%x", now)
}

func stringifyBody(b any) string {
	if b == nil {
		return ""
	}
	switch x := b.(type) {
	case string:
		return x
	default:
		buf, err := json.MarshalIndent(x, "", "  ")
		if err != nil {
			raw, _ := json.Marshal(x)
			return string(raw)
		}
		return string(buf)
	}
}

func limitBody(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "\n...[truncated]..."
}
