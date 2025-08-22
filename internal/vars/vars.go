package vars

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadJSONFiles(paths []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}

		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		for k, v := range m {
			switch x := v.(type) {
			case string:
				out[k] = x
			default:
				out[k] = fmt.Sprint(x) // coerce numbers/bools to string
			}
		}
	}
	return out, nil
}
