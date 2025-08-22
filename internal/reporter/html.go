package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"sea-qa/internal/executor"
)

// --- Primary HTML renderer (unchanged) ---

func WriteHTML(w io.Writer, suiteName string, res *executor.SuiteResult) error {
	var sb strings.Builder

	sb.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	sb.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	sb.WriteString(`<title>sea-qa Report — ` + html.EscapeString(suiteName) + `</title>`)
	sb.WriteString(`<style>
:root { --ok:#0a0; --bad:#b00; --muted:#666; --chip:#eee; --line:#e5e5e5; }
body{font-family:system-ui,Segoe UI,Roboto,Arial,sans-serif;margin:24px;line-height:1.45}
h1{margin:0 0 12px}
h2{margin:0 0 8px;font-size:1.05rem}
.summary{display:flex;gap:12px;align-items:center;margin:12px 0 18px}
.pass{color:var(--ok)} .fail{color:var(--bad)}
.badge{display:inline-block;padding:2px 8px;border-radius:999px;background:var(--chip);font-size:.85rem}
.card{border:1px solid var(--line);border-radius:12px;padding:16px;margin:12px 0}
.step{margin:6px 0}
details>summary{cursor:pointer;list-style:none}
details>summary::-webkit-details-marker{display:none}
summary {padding:6px 0}
pre{background:#f8f8f8;padding:12px;border-radius:8px;overflow:auto;max-height:320px;margin:8px 0 0;white-space:pre-wrap}
.muted{color:var(--muted)}
hr{border:0;border-top:1px solid var(--line);margin:20px 0}
.small{font-size:.85rem}
.kv{margin-top:6px}
</style></head><body>`)

	// Header
	sb.WriteString(`<h1>` + html.EscapeString(suiteName) + `</h1>`)
	sb.WriteString(`<div class="summary">`)
	sb.WriteString(`<div>Status: <strong class="` + statusClass(res.Passed) + `">` + tern(res.Passed, "PASS", "FAIL") + `</strong></div>`)
	sb.WriteString(chip("Duration: " + ms(res.DurationMs)))
	sb.WriteString(chip("Scenarios: " + strconv.Itoa(len(res.Scenarios))))
	sb.WriteString(`</div><hr>`)

	// Scenarios
	for _, sc := range res.Scenarios {
		sb.WriteString(`<div class="card">`)
		sb.WriteString(`<h2>` + html.EscapeString(sc.Name) + ` — ` + badgeStatus(sc.Passed) + ` ` + chip(ms(sc.DurationMs)) + `</h2>`)

		for i, st := range sc.Steps {
			sb.WriteString(`<div class="step">`)
			sb.WriteString(`<details ` + tern(!st.Passed, "open", "") + `>`)
			sb.WriteString(`<summary>Step ` + strconv.Itoa(i+1) + ` • ` + html.EscapeString(strings.ToUpper(st.Method)) + ` ` + html.EscapeString(st.URL) + ` • status ` + strconv.Itoa(st.StatusCode) + ` ` + badgeStatus(st.Passed) + ` ` + chip(ms(st.DurationMs)) + `</summary>`)

			// Errors
			if len(st.Errors) > 0 {
				sb.WriteString(`<pre>`)
				for _, e := range st.Errors {
					sb.WriteString(html.EscapeString(e) + "\n")
				}
				sb.WriteString(`</pre>`)
			} else {
				sb.WriteString(`<div class="small muted">No errors.</div>`)
			}

			// Request
			sb.WriteString(`<div class="small muted" style="margin-top:10px;">Request</div>`)
			if st.Method != "" || st.URL != "" {
				sb.WriteString(`<pre>` + html.EscapeString(strings.ToUpper(st.Method)+" "+st.URL) + `</pre>`)
			}
			if len(st.ReqHeaders) > 0 {
				sb.WriteString(`<pre class="kv">` + html.EscapeString(kvBlock(st.ReqHeaders)) + `</pre>`)
			}
			if st.ReqBody != "" {
				sb.WriteString(`<pre>` + html.EscapeString(prettyJSON(st.ReqBody)) + `</pre>`)
			}

			// Response
			sb.WriteString(`<div class="small muted" style="margin-top:10px;">Response</div>`)
			if len(st.RespHeaders) > 0 {
				sb.WriteString(`<pre class="kv">` + html.EscapeString(hdrBlock(st.RespHeaders)) + `</pre>`)
			}
			if st.RespBody != "" {
				sb.WriteString(`<pre>` + html.EscapeString(prettyJSON(st.RespBody)) + `</pre>`)
			}

			sb.WriteString(`</details>`)
			sb.WriteString(`</div>`)
		}
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`</body></html>`)
	_, err := io.WriteString(w, sb.String())
	return err
}

// --- Helper that guarantees HTML matches the on-disk results.json ---

func WriteHTMLFromJSONPath(w io.Writer, suiteName, resultsJSONPath string) error {
	data, err := os.ReadFile(resultsJSONPath)
	if err != nil {
		return fmt.Errorf("read results.json: %w", err)
	}
	var res executor.SuiteResult
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("decode results.json: %w", err)
	}
	return WriteHTML(w, suiteName, &res)
}

func statusClass(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func badgeStatus(ok bool) string {
	if ok {
		return `<span class="badge pass">PASS</span>`
	}
	return `<span class="badge fail">FAIL</span>`
}

func chip(text string) string {
	return `<span class="badge">` + html.EscapeString(text) + `</span>`
}

func ms(v float64) string { return fmt.Sprintf("%.0f ms", v) }

func tern[T ~string](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func kvBlock(h map[string]string) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(h[k])
		b.WriteByte('\n')
	}
	return b.String()
}

func hdrBlock(h map[string][]string) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(h[k], ", "))
		b.WriteByte('\n')
	}
	return b.String()
}

func prettyJSON(s string) string {
	var buf bytes.Buffer
	var raw any
	if json.Unmarshal([]byte(s), &raw) == nil {
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		_ = enc.Encode(raw)
		return strings.TrimRight(buf.String(), "\n")
	}
	return s
}
