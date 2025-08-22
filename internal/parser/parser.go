package parser

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"sea-qa/internal/ir"
)

var ErrValidation = errors.New("validation error")

type Parser struct{}

func New() *Parser { return &Parser{} }

// ParseBytes parses YAML (or JSON) into IR and validates it.
func (p *Parser) ParseBytes(b []byte) (*ir.TestSuite, error) {
	var suite ir.TestSuite

	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // fail on unknown fields

	if err := dec.Decode(&suite); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if err := validateSuite(&suite); err != nil {
		return nil, err
	}

	// Normalize HTTP methods
	for i := range suite.Scenarios {
		for j := range suite.Scenarios[i].Steps {
			m := suite.Scenarios[i].Steps[j].Request.Method
			suite.Scenarios[i].Steps[j].Request.Method = strings.ToUpper(m)
		}
	}
	return &suite, nil
}

// --- validation helpers ---

func validateSuite(s *ir.TestSuite) error {
	if s.Name == "" {
		return wrapValidation("suite.name must not be empty")
	}
	if len(s.Scenarios) == 0 {
		return wrapValidation("suite.scenarios must not be empty")
	}
	for i := range s.Scenarios {
		if err := validateScenario(&s.Scenarios[i], i); err != nil {
			return err
		}
	}
	return nil
}

func validateScenario(sc *ir.Scenario, idx int) error {
	if sc.Name == "" {
		return wrapValidation(fmt.Sprintf("scenario[%d].name must not be empty", idx))
	}
	if len(sc.Steps) == 0 {
		return wrapValidation(fmt.Sprintf("scenario[%d].steps must not be empty", idx))
	}
	for j := range sc.Steps {
		if err := validateStep(&sc.Steps[j], idx, j); err != nil {
			return err
		}
	}
	return nil
}

func validateStep(st *ir.Step, i, j int) error {
	if st.Request.Method == "" {
		return wrapValidation(fmt.Sprintf("scenario[%d].step[%d].request.method must not be empty", i, j))
	}
	if st.Request.URL == "" {
		return wrapValidation(fmt.Sprintf("scenario[%d].step[%d].request.url must not be empty", i, j))
	}
	return nil
}

func wrapValidation(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidation, msg)
}
