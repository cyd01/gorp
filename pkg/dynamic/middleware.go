package dynamic

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const packageName = "dynamic"

// Request evaluates source and returns a request middleware.
// The source must define dynamic.Middleware with signature:
// func(http.Handler) http.Handler.
func Request(source string) (func(http.Handler) http.Handler, error) {
	value, err := evaluate(source, "Middleware")
	if err != nil {
		return nil, err
	}
	middleware, ok := value.Interface().(func(http.Handler) http.Handler)
	if !ok {
		return nil, fmt.Errorf("dynamic request Middleware has incompatible signature")
	}
	return middleware, nil
}

// Response evaluates source and returns a response modifier.
// The source must define dynamic.Middleware with signature:
// func(*http.Response) error.
func Response(source string) (func(*http.Response) error, error) {
	value, err := evaluate(source, "Middleware")
	if err != nil {
		return nil, err
	}
	modifier, ok := value.Interface().(func(*http.Response) error)
	if !ok {
		return nil, fmt.Errorf("dynamic response Middleware has incompatible signature")
	}
	return modifier, nil
}

func evaluate(source, symbol string) (reflect.Value, error) {
	if source == "" {
		return reflect.Value{}, fmt.Errorf("dynamic middleware source is empty")
	}

	interpreter := interp.New(interp.Options{})
	interpreter.Use(stdlib.Symbols)
	if _, err := interpreter.Eval(source); err != nil {
		return reflect.Value{}, fmt.Errorf("evaluate dynamic middleware: %w", err)
	}

	value, err := interpreter.Eval(packageName + "." + symbol)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("load dynamic middleware symbol: %w", err)
	}
	if !value.IsValid() || !value.CanInterface() {
		return reflect.Value{}, fmt.Errorf("dynamic middleware symbol is invalid")
	}
	return value, nil
}
