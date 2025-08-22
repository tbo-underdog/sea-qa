package contract

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type OpSig struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type StatusDiff struct {
	A []string `json:"a"` // status codes in doc A
	B []string `json:"b"` // status codes in doc B
}

// JSON-friendly representation of a status change for a single op
type StatusChange struct {
	Method string   `json:"method"`
	Path   string   `json:"path"`
	A      []string `json:"a"`
	B      []string `json:"b"`
}

type DiffReport struct {
	Added         []OpSig        `json:"added"`          // present in B, not in A
	Removed       []OpSig        `json:"removed"`        // present in A, not in B
	ChangedStatus []StatusChange `json:"changed_status"` // same op, different status sets
}

func DiffDocs(a, b *openapi3.T) DiffReport {
	opsA := listOps(a)
	opsB := listOps(b)

	setA := toSet(opsA)
	setB := toSet(opsB)

	var added, removed []OpSig
	for _, op := range opsB {
		if !setA[op] {
			added = append(added, op)
		}
	}
	for _, op := range opsA {
		if !setB[op] {
			removed = append(removed, op)
		}
	}

	var changed []StatusChange
	// Compare status code sets for intersection of ops
	for _, op := range opsA {
		if setB[op] {
			as := statusSet(a, op)
			bs := statusSet(b, op)
			if !equalStrSet(as, bs) {
				changed = append(changed, StatusChange{
					Method: op.Method,
					Path:   op.Path,
					A:      toSortedSlice(as),
					B:      toSortedSlice(bs),
				})
			}
		}
	}

	sortOps(added)
	sortOps(removed)
	sortChanges(changed)

	return DiffReport{
		Added:         added,
		Removed:       removed,
		ChangedStatus: changed,
	}
}

func listOps(doc *openapi3.T) []OpSig {
	var out []OpSig
	if doc == nil || doc.Paths == nil {
		return out
	}
	for p, pi := range doc.Paths.Map() {
		if pi == nil {
			continue
		}
		if pi.Get != nil {
			out = append(out, OpSig{"GET", p})
		}
		if pi.Post != nil {
			out = append(out, OpSig{"POST", p})
		}
		if pi.Put != nil {
			out = append(out, OpSig{"PUT", p})
		}
		if pi.Delete != nil {
			out = append(out, OpSig{"DELETE", p})
		}
		if pi.Patch != nil {
			out = append(out, OpSig{"PATCH", p})
		}
		if pi.Head != nil {
			out = append(out, OpSig{"HEAD", p})
		}
		if pi.Options != nil {
			out = append(out, OpSig{"OPTIONS", p})
		}
		if pi.Trace != nil {
			out = append(out, OpSig{"TRACE", p})
		}
	}
	return out
}

func statusSet(doc *openapi3.T, op OpSig) map[string]bool {
	set := map[string]bool{}
	if doc == nil || doc.Paths == nil {
		return set
	}
	pi := doc.Paths.Value(op.Path)
	if pi == nil {
		return set
	}
	var rs *openapi3.Responses
	switch strings.ToUpper(op.Method) {
	case "GET":
		if pi.Get != nil {
			rs = pi.Get.Responses
		}
	case "POST":
		if pi.Post != nil {
			rs = pi.Post.Responses
		}
	case "PUT":
		if pi.Put != nil {
			rs = pi.Put.Responses
		}
	case "DELETE":
		if pi.Delete != nil {
			rs = pi.Delete.Responses
		}
	case "PATCH":
		if pi.Patch != nil {
			rs = pi.Patch.Responses
		}
	case "HEAD":
		if pi.Head != nil {
			rs = pi.Head.Responses
		}
	case "OPTIONS":
		if pi.Options != nil {
			rs = pi.Options.Responses
		}
	case "TRACE":
		if pi.Trace != nil {
			rs = pi.Trace.Responses
		}
	}
	if rs == nil {
		return set
	}
	for code := range rs.Map() {
		set[code] = true
	}
	return set
}

func toSet(ops []OpSig) map[OpSig]bool {
	m := map[OpSig]bool{}
	for _, o := range ops {
		m[o] = true
	}
	return m
}

func equalStrSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func toSortedSlice(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortOps(ops []OpSig) {
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})
}

func sortChanges(ch []StatusChange) {
	sort.Slice(ch, func(i, j int) bool {
		if ch[i].Path == ch[j].Path {
			return ch[i].Method < ch[j].Method
		}
		return ch[i].Path < ch[j].Path
	})
}
