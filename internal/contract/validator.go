package contract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

type Validator struct {
	doc    *openapi3.T
	router routers.Router
}

func LoadFromFile(path string) (*Validator, error) {
	loader := &openapi3.Loader{IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return build(doc)
}

func LoadFromBytes(b []byte) (*Validator, error) {
	loader := &openapi3.Loader{IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromData(b)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return build(doc)
}

func build(doc *openapi3.T) (*Validator, error) {
	// Strict: if the spec is invalid, fail fast with a clear message.
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate spec: %w", err)
	}
	r, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("router: %w", err)
	}
	return &Validator{doc: doc, router: r}, nil
}

func (v *Validator) Doc() *openapi3.T { return v.doc }

// ValidateResponse validates (method, url, status, headers, body) against the spec.
// Returns (templatedPath, method) for coverage accounting.
func (v *Validator) ValidateResponse(
	ctx context.Context,
	method string,
	rawURL string,
	status int,
	header map[string][]string,
	body []byte,
) (routePath string, routeMethod string, err error) {

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse url: %w", err)
	}
	hdr := http.Header(header)
	req := &http.Request{
		Method: method,
		URL:    u,
		Header: hdr,
	}

	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		return "", "", fmt.Errorf("route not found: %w", err)
	}

	rvi := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    &openapi3filter.Options{},
	}

	rsp := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: rvi,
		Status:                 status,
		Header:                 hdr,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                &openapi3filter.Options{},
	}

	if err := openapi3filter.ValidateResponse(ctx, rsp); err != nil {
		return route.Path, route.Method, err
	}
	return route.Path, route.Method, nil
}
