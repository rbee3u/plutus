package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

var requestContract = sync.OnceValue(func() routers.Router {
	spec, err := api.GetSpec()
	if err != nil {
		panic(err)
	}
	router, err := legacy.NewRouter(spec)
	if err != nil {
		panic(err)
	}
	// The library decoder accepts a JSON stream; an HTTP JSON body must contain
	// exactly one value, including for PATCH's explicit-null interpretation.
	openapi3filter.RegisterBodyDecoder("application/json", decodeContractJSON)
	return router
})

func decodeContractJSON(body io.Reader, _ http.Header, _ *openapi3.SchemaRef, _ openapi3filter.EncodingFn) (any, error) {
	decoder := json.NewDecoder(body)
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("the request body must contain a single JSON value")
	}
	return value, nil
}

// validateContract applies the owned schema without defaults that alter PATCH or
// idempotency input. Authentication and CSRF remain the handlers' responsibility.
func validateContract() gin.HandlerFunc {
	router := requestContract()
	options := &openapi3filter.Options{
		AuthenticationFunc:      openapi3filter.NoopAuthenticationFunc,
		SkipSettingDefaults:     true,
		SchemaValidationOptions: []openapi3.SchemaValidationOption{openapi3.EnableFormatValidation(), openapi3.WithStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForUUIDOfRFC9562))},
	}
	return func(c *gin.Context) {
		route, params, err := router.FindRoute(c.Request)
		if err != nil {
			c.Next()
			return
		}
		input := &openapi3filter.RequestValidationInput{
			Request: c.Request, PathParams: params, Route: route,
			Options: options,
		}
		if err := openapi3filter.ValidateRequest(c.Request.Context(), input); err != nil {
			body := newProblem(c, http.StatusBadRequest, "validation_failed", "Request validation failed", "The request does not match the API contract.")
			field := "body"
			code, message := "invalid", "Supply a value matching the declared schema."
			var requestErr *openapi3filter.RequestError
			if errors.As(err, &requestErr) && requestErr.Parameter != nil {
				field = requestErr.Parameter.In + "." + requestErr.Parameter.Name
			}
			var schemaErr *openapi3.SchemaError
			if errors.As(err, &schemaErr) {
				if path := schemaErr.JSONPointer(); len(path) > 0 {
					field += "." + strings.Join(path, ".")
				}
				code = schemaErr.SchemaField
				// Schema reasons omit input values; SchemaError.Error includes
				// the value and schema and must never enter responses or logs.
				message = schemaErr.Reason
			}
			fields := []api.FieldError{{Field: field, Code: code, Message: message}}
			body.FieldErrors = &fields
			c.Header("Content-Type", "application/problem+json")
			c.JSON(http.StatusBadRequest, body)
			c.Abort()
			return
		}
		c.Next()
	}
}

// parseEditVersion accepts the canonical unsigned decimal form, without labels.
func parseEditVersion(value string, allowZero bool) (uint64, bool) {
	version, err := strconv.ParseUint(value, 10, 64)
	return version, err == nil && (allowZero || version > 0) && strconv.FormatUint(version, 10) == value
}
