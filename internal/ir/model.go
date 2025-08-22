package ir

// Expectation types (string constants for portability)
const (
	ExpectStatus   = "status"
	ExpectJSONPath = "jsonPath"
	ExpectContract = "contract"
)

type TestSuite struct {
	Name      string     `json:"name" yaml:"name"`
	OpenAPI   string     `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	Scenarios []Scenario `json:"scenarios" yaml:"scenarios"`
}

type Scenario struct {
	Name     string   `json:"name" yaml:"name"`
	Env      string   `json:"env,omitempty" yaml:"env,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Setup    []Action `json:"setup,omitempty" yaml:"setup,omitempty"`
	Steps    []Step   `json:"steps" yaml:"steps"`
	Teardown []Action `json:"teardown,omitempty" yaml:"teardown,omitempty"`
}

type Action struct {
	Name    string   `json:"name,omitempty" yaml:"name,omitempty"`
	Request *Request `json:"request,omitempty" yaml:"request,omitempty"`
}

type Step struct {
	Name    string        `json:"name,omitempty" yaml:"name,omitempty"`
	Request Request       `json:"request" yaml:"request"`
	Expect  []Expectation `json:"expect,omitempty" yaml:"expect,omitempty"`
	Hooks   []Hook        `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

type Request struct {
	Method    string            `yaml:"method"  json:"method"`
	URL       string            `yaml:"url"     json:"url"`
	Headers   map[string]string `yaml:"headers" json:"headers"`
	Body      any               `yaml:"body"    json:"body"`
	TimeoutMs int               `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
}

type Expectation struct {
	Type   string `json:"type" yaml:"type"`
	Target string `json:"target,omitempty" yaml:"target,omitempty"`
	Value  any    `json:"value,omitempty" yaml:"value,omitempty"`
}

type Hook struct {
	Type      string            `json:"type" yaml:"type"` // "process"
	When      string            `json:"when" yaml:"when"` // "before" | "after"
	Cmd       string            `json:"cmd" yaml:"cmd"`
	Args      []string          `json:"args,omitempty" yaml:"args,omitempty"`
	TimeoutMs int               `json:"timeoutMs,omitempty" yaml:"timeoutMs,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Redact    []string          `json:"redact,omitempty" yaml:"redact,omitempty"`
}
