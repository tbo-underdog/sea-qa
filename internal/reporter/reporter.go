package reporter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"

	"sea-qa/internal/executor"
)

// -------- JSON --------

func WriteJSON(w io.Writer, res *executor.SuiteResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// -------- JUnit XML --------

// Minimal JUnit schema: testsuite -> testcase (+failure)
type junitTestsuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Testcase []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

func WriteJUnit(w io.Writer, suiteName string, res *executor.SuiteResult) error {
	var total, failures int
	var cases []junitTestcase

	for _, sc := range res.Scenarios {
		for i, st := range sc.Steps {
			total++
			tc := junitTestcase{
				Classname: sc.Name,
				Name:      fmt.Sprintf("step-%d", i+1),
				Time:      fmt.Sprintf("%.3f", st.DurationMs/1000.0),
			}
			if !st.Passed {
				failures++
				msg := "assertion failed"
				if len(st.Errors) > 0 {
					msg = st.Errors[0]
				}
				tc.Failure = &junitFailure{
					Message: msg,
					Type:    "AssertionError",
					Text:    joinErrs(st.Errors),
				}
			}
			cases = append(cases, tc)
		}
	}

	ts := junitTestsuite{
		Name:     suiteName,
		Tests:    total,
		Failures: failures,
		Time:     fmt.Sprintf("%.3f", res.DurationMs/1000.0),
		Testcase: cases,
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(ts)
}

func joinErrs(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	out := errs[0]
	for i := 1; i < len(errs); i++ {
		out += "\n" + errs[i]
	}
	return out
}
