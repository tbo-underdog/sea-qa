package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type CoverageReport struct {
	Total        int      `json:"total"`
	Covered      int      `json:"covered"`
	Percent      float64  `json:"percent"`
	CoveredSet   []string `json:"covered_set"`
	UncoveredSet []string `json:"uncovered_set"`
}

// covered is: method -> pathTemplate -> true
func WriteCoverage(w io.Writer, doc *openapi3.T, covered map[string]map[string]bool) error {
	all := allOps(doc)
	cset := flattenCovered(covered)

	var coveredCount int
	var coveredList []string
	var uncoveredList []string

	for _, op := range all {
		if cset[op] {
			coveredCount++
			coveredList = append(coveredList, op)
		} else {
			uncoveredList = append(uncoveredList, op)
		}
	}
	sort.Strings(coveredList)
	sort.Strings(uncoveredList)

	rep := CoverageReport{
		Total:        len(all),
		Covered:      coveredCount,
		Percent:      pct(coveredCount, len(all)),
		CoveredSet:   coveredList,
		UncoveredSet: uncoveredList,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func ComputeCoverage(doc *openapi3.T, covered map[string]map[string]bool) CoverageReport {
	all := allOps(doc)
	cset := flattenCovered(covered)

	var coveredCount int
	var coveredList []string
	var uncoveredList []string

	for _, op := range all {
		if cset[op] {
			coveredCount++
			coveredList = append(coveredList, op)
		} else {
			uncoveredList = append(uncoveredList, op)
		}
	}
	sort.Strings(coveredList)
	sort.Strings(uncoveredList)

	return CoverageReport{
		Total:        len(all),
		Covered:      coveredCount,
		Percent:      pct(coveredCount, len(all)),
		CoveredSet:   coveredList,
		UncoveredSet: uncoveredList,
	}
}

func allOps(doc *openapi3.T) []string {
	var out []string
	if doc == nil || doc.Paths == nil {
		return out
	}
	for p, pi := range doc.Paths.Map() { // <-- use Map() with kin-openapi v0.126.0
		if pi == nil {
			continue
		}
		if pi.Get != nil {
			out = append(out, sig("GET", p))
		}
		if pi.Post != nil {
			out = append(out, sig("POST", p))
		}
		if pi.Put != nil {
			out = append(out, sig("PUT", p))
		}
		if pi.Delete != nil {
			out = append(out, sig("DELETE", p))
		}
		if pi.Patch != nil {
			out = append(out, sig("PATCH", p))
		}
		if pi.Head != nil {
			out = append(out, sig("HEAD", p))
		}
		if pi.Options != nil {
			out = append(out, sig("OPTIONS", p))
		}
		if pi.Trace != nil {
			out = append(out, sig("TRACE", p))
		}
	}
	return out
}

func flattenCovered(m map[string]map[string]bool) map[string]bool {
	out := map[string]bool{}
	for method, paths := range m {
		for path := range paths {
			out[sig(strings.ToUpper(method), path)] = true
		}
	}
	return out
}

func sig(method, path string) string { return fmt.Sprintf("%s %s", method, path) }

func pct(n, d int) float64 {
	if d == 0 {
		return 100.0
	}
	return float64(n) * 100.0 / float64(d)
}
