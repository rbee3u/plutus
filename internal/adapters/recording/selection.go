package recording

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

// Endpoint declares one allowlisted interface, production defaults and necessary parameters.
type Endpoint struct {
	Request            fixture.Request
	Signed             bool
	Required           []string
	ValidateParameters func(fixture.Request) error
}
type Catalogue map[string]Endpoint

func (c Catalogue) Add(request fixture.Request, signed bool) {
	c[fixture.Name(request.Path, "")] = Endpoint{Request: request, Signed: signed}
}

func (c Catalogue) Find(request fixture.Request) (Endpoint, bool) {
	ep, ok := c[fixture.Name(request.Path, "")]
	return ep, ok && ep.Request.Service == request.Service && ep.Request.Method == request.Method && ep.Request.Path == request.Path
}

func (c Catalogue) Validate(request fixture.Request) error {
	if err := request.Validate(); err != nil {
		return err
	}
	ep, ok := c.Find(request)
	if !ok {
		return errors.New("unknown read-only request")
	}
	params := request.Query
	if ep.Request.Form != nil {
		params = request.Form
	}
	for _, key := range ep.Required {
		if len(params[key]) == 0 || params.Get(key) == "" {
			return fmt.Errorf("missing required business parameter %q", key)
		}
	}
	if ep.ValidateParameters != nil {
		return ep.ValidateParameters(request)
	}
	return nil
}

func parameters(values []string) (url.Values, error) {
	out := url.Values{}
	for _, value := range values {
		key, text, ok := strings.Cut(value, "=")
		if !ok || key == "" || fixture.AuthenticationParameter(key) {
			return nil, errors.New("business parameters must be key=value without authentication fields")
		}
		out.Add(key, text)
	}
	return out, nil
}

func SelectRequest(c Catalogue, name string, query, form []string) (fixture.Request, error) {
	ep, ok := c[name]
	if !ok {
		return fixture.Request{}, fmt.Errorf("unknown read-only interface %q; use --list-requests", name)
	}
	q, err := parameters(query)
	if err != nil {
		return fixture.Request{}, err
	}
	f, err := parameters(form)
	if err != nil {
		return fixture.Request{}, err
	}
	request := ep.Request
	request.Query, request.Form = business(request.Query), business(request.Form)
	if request.Method == "WS" && (len(q) > 0 || len(f) > 0) {
		return request, errors.New("logical WebSocket snapshots do not take HTTP parameters")
	}
	if len(f) > 0 && ep.Request.Form == nil {
		return request, errors.New("this interface does not accept --form")
	}
	if len(q) > 0 && ep.Request.Form != nil {
		return request, errors.New("this interface takes business parameters in --form")
	}
	for key, values := range q {
		request.Query[key] = values
	}
	for key, values := range f {
		request.Form[key] = values
	}
	return request, c.Validate(request)
}
