package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sea-qa/internal/contract"
	"sea-qa/internal/executor"
	"sea-qa/internal/ir"
	"sea-qa/internal/parser"
	"sea-qa/internal/reporter"
	"sea-qa/internal/vars"
)

func main() {
	var (
		// run mode
		spec        = flag.String("spec", "", "Path to YAML/JSON test suite")
		outDir      = flag.String("out", "reports", "Output directory for artifacts")
		name        = flag.String("name", "", "Optional suite name override")
		envPaths    = flag.String("env", "", "Comma-separated JSON env files (e.g., env/dev.json,env/ci.json)")
		jsonOut     = flag.Bool("json", true, "Write JSON results")
		junitOut    = flag.Bool("junit", true, "Write JUnit XML results")
		htmlOut     = flag.Bool("html", true, "Write HTML report")
		verbose     = flag.Bool("v", false, "Verbose: print failure details")
		openapiPath = flag.String("openapi", "", "Path to OpenAPI (YAML/JSON) for contract checks & coverage")
		covMin      = flag.Float64("coverage-min", -1, "Fail if coverage percent < this threshold (requires OpenAPI)")
		parallel    = flag.Int("parallel", 1, "Number of scenarios to execute in parallel")
		failFast    = flag.Bool("fail-fast", false, "Stop after first failing scenario (forces --parallel=1)")
		includeTags = flag.String("include-tags", "", "Comma-separated tags to include (OR semantics)")
		excludeTags = flag.String("exclude-tags", "", "Comma-separated tags to exclude (OR semantics)")

		// diff mode
		diffA = flag.String("diff-a", "", "Contract diff: path to OpenAPI A (enables diff mode)")
		diffB = flag.String("diff-b", "", "Contract diff: path to OpenAPI B (enables diff mode)")
	)
	flag.Parse()

	// ---- Contract diff mode (no --spec required) ----
	if *diffA != "" || *diffB != "" {
		if *diffA == "" || *diffB == "" {
			fail("both --diff-a and --diff-b are required for contract diff mode")
		}
		runContractDiff(*diffA, *diffB, *outDir)
		return
	}

	// ---- Regular test execution ----
	if *spec == "" {
		fail("missing --spec")
	}

	data, err := os.ReadFile(*spec)
	if err != nil {
		fail("read spec: %v", err)
	}

	p := parser.New()
	suite, err := p.ParseBytes(data)
	if err != nil {
		fail("parse: %v", err)
	}
	if *name != "" {
		suite.Name = *name
	}

	// tag filtering (optional)
	if *includeTags != "" || *excludeTags != "" {
		inc := splitCSV(*includeTags)
		exc := splitCSV(*excludeTags)
		suite.Scenarios = filterByTags(suite.Scenarios, inc, exc)
		if len(suite.Scenarios) == 0 {
			fail("no scenarios left after tag filtering")
		}
	}

	// env vars (optional)
	var baseVars map[string]string
	if *envPaths != "" {
		paths := strings.Split(*envPaths, ",")
		baseVars, err = vars.LoadJSONFiles(paths)
		if err != nil {
			fail("load env: %v", err)
		}
	}

	// Resolve OpenAPI file: flag wins; else suite.openapi (relative to spec)
	var openapiFile string
	if *openapiPath != "" {
		openapiFile = *openapiPath
	} else if suite.OpenAPI != "" {
		specDir := filepath.Dir(*spec)
		if filepath.IsAbs(suite.OpenAPI) {
			openapiFile = suite.OpenAPI
		} else {
			openapiFile = filepath.Join(specDir, suite.OpenAPI)
		}
	}

	// Fail-fast enforces sequential execution
	if *failFast && *parallel != 1 {
		*parallel = 1
	}

	// Runner
	r := executor.NewWithVars(baseVars).WithParallel(*parallel).WithFailFast(*failFast)

	// Contract (strict)
	var v *contract.Validator
	if openapiFile != "" {
		v, err = contract.LoadFromFile(openapiFile)
		if err != nil {
			fail("openapi load: %v", err)
		}
		r = r.WithContract(v)
	}

	// Execute
	res, err := r.RunSuite(context.Background(), suite)
	if err != nil {
		fail("execute: %v", err)
	}

	// Artifacts
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail("mkdir out: %v", err)
	}

	// Always compute suite name for outputs
	outSuiteName := suite.Name
	if outSuiteName == "" {
		outSuiteName = "sea-qa"
	}

	// JSON (and remember the path for HTML parity)
	var jsonPath string
	if *jsonOut {
		jsonPath = filepath.Join(*outDir, "results.json")
		writeOrDie(jsonPath, func(f *os.File) error {
			return reporter.WriteJSON(f, res)
		})
	}

	// JUnit
	if *junitOut {
		writeOrDie(filepath.Join(*outDir, "junit.xml"), func(f *os.File) error {
			return reporter.WriteJUnit(f, outSuiteName, res)
		})
	}

	// HTML — if JSON is enabled, render from results.json to guarantee parity
	if *htmlOut {
		htmlPath := filepath.Join(*outDir, "report.html")
		if *jsonOut && jsonPath != "" {
			writeOrDie(htmlPath, func(f *os.File) error {
				return reporter.WriteHTMLFromJSONPath(f, outSuiteName, jsonPath)
			})
		} else {
			// fallback: render directly from memory
			writeOrDie(htmlPath, func(f *os.File) error {
				return reporter.WriteHTML(f, outSuiteName, res)
			})
		}
	}

	// Coverage report + optional gate
	if v != nil {
		writeOrDie(filepath.Join(*outDir, "coverage.json"), func(f *os.File) error {
			return reporter.WriteCoverage(f, v.Doc(), r.Covered())
		})
		if *covMin >= 0 {
			rep := reporter.ComputeCoverage(v.Doc(), r.Covered())
			if rep.Percent+1e-9 < *covMin {
				fmt.Fprintf(os.Stderr, "coverage gate failed: got %.2f%%, need >= %.2f%%\n", rep.Percent, *covMin)
				fmt.Println("FAIL")
				os.Exit(1)
			}
		}
	}

	// Failure summary (or verbose print)
	if !res.Passed || *verbose {
		for _, sc := range res.Scenarios {
			if sc.Passed {
				continue
			}
			fmt.Fprintf(os.Stderr, "\nScenario FAILED: %s\n", sc.Name)
			for i, st := range sc.Steps {
				if st.Passed {
					continue
				}
				fmt.Fprintf(os.Stderr, "  Step %d: status=%d\n", i+1, st.StatusCode)
				for _, e := range st.Errors {
					fmt.Fprintf(os.Stderr, "    - %s\n", e)
				}
			}
		}
	}

	if res.Passed {
		fmt.Println("PASS")
		os.Exit(0)
	}
	fmt.Println("FAIL")
	os.Exit(1)
}

// ---- Contract diff mode ----

func runContractDiff(aPath, bPath, outDir string) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail("mkdir out: %v", err)
	}
	a, err := contract.LoadFromFile(aPath)
	if err != nil {
		fail("openapi A load: %v", err)
	}
	b, err := contract.LoadFromFile(bPath)
	if err != nil {
		fail("openapi B load: %v", err)
	}

	rep := contract.DiffDocs(a.Doc(), b.Doc())

	out := filepath.Join(outDir, "contract-diff.json")
	writeOrDie(out, func(f *os.File) error {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	})

	// Console summary
	fmt.Printf("Contract diff (%s → %s)\n", aPath, bPath)
	if len(rep.Added) == 0 && len(rep.Removed) == 0 && len(rep.ChangedStatus) == 0 {
		fmt.Println("  No changes.")
	} else {
		if len(rep.Added) > 0 {
			fmt.Println("  Added:")
			for _, op := range rep.Added {
				fmt.Printf("    + %s %s\n", op.Method, op.Path)
			}
		}
		if len(rep.Removed) > 0 {
			fmt.Println("  Removed:")
			for _, op := range rep.Removed {
				fmt.Printf("    - %s %s\n", op.Method, op.Path)
			}
		}
		if len(rep.ChangedStatus) > 0 {
			fmt.Println("  Status changes:")
			for _, ch := range rep.ChangedStatus {
				fmt.Printf("    * %s %s: %v -> %v\n", ch.Method, ch.Path, ch.A, ch.B)
			}
		}
	}
	fmt.Printf("wrote %s\n", out)
}

// ---- helpers ----

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(2)
}

func writeOrDie(path string, fn func(*os.File) error) {
	f, err := os.Create(path)
	if err != nil {
		fail("create %s: %v", path, err)
	}
	defer f.Close()
	if err := fn(f); err != nil {
		fail("write %s: %v", path, err)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filterByTags(in []ir.Scenario, include, exclude []string) []ir.Scenario {
	if len(include) == 0 && len(exclude) == 0 {
		return in
	}
	toSet := func(ss []string) map[string]bool {
		m := map[string]bool{}
		for _, s := range ss {
			m[strings.ToLower(s)] = true
		}
		return m
	}
	inc, exc := toSet(include), toSet(exclude)
	hasAny := func(tags []string, m map[string]bool) bool {
		for _, t := range tags {
			if m[strings.ToLower(t)] {
				return true
			}
		}
		return false
	}
	out := make([]ir.Scenario, 0, len(in))
	for _, sc := range in {
		if len(inc) > 0 && !hasAny(sc.Tags, inc) {
			continue
		}
		if len(exc) > 0 && hasAny(sc.Tags, exc) {
			continue
		}
		out = append(out, sc)
	}
	return out
}
