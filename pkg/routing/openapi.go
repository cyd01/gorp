package routing

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// OpenAPIMiddleware loads an OpenAPI contract and validates every request
// against its matching operation before passing it to the next handler.
func OpenAPIMiddleware(filename string) (Middleware, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, fmt.Errorf("openapi middleware requires a non-empty file")
	}

	loader := openapi3.NewLoader()
	var document *openapi3.T
	var err error
	if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") || strings.HasPrefix(filename, "file://") {
		location, parseErr := url.Parse(filename)
		if parseErr != nil {
			return nil, fmt.Errorf("parse OpenAPI file location: %w", parseErr)
		}
		document, err = loader.LoadFromURI(location)
	} else {
		document, err = loader.LoadFromFile(filename)
	}
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI contract %q: %w", filename, err)
	}
	if err := document.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI contract %q: %w", filename, err)
	}
	router, err := legacy.NewRouter(document)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router %q: %w", filename, err)
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				http.Error(w, fmt.Sprintf("OpenAPI route validation failed: %v", err), routeStatusCode(err))
				return
			}
			if err := openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    options,
			}); err != nil {
				http.Error(w, fmt.Sprintf("OpenAPI request validation failed: %v", err), http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func routeStatusCode(err error) int {
	if strings.Contains(strings.ToLower(err.Error()), "method") {
		return http.StatusMethodNotAllowed
	}
	return http.StatusNotFound
}
