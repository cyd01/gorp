package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/cyd01/gorp/pkg/backend"
	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/dynamic"
	"github.com/cyd01/gorp/pkg/helper"
	"github.com/cyd01/gorp/pkg/routing"
	"github.com/cyd01/gorp/pkg/selector"
)

// Build constructs a new server instance from the configuration.
//
// This function does not start any listeners; it only prepares the objects
// required for execution.
func Build(cfg *config.Config) (*Server, error) {
	routes, err := buildRoutes(cfg)
	if err != nil {
		return nil, err
	}

	router := buildRouter(routes, cfg.Directory)

	return New(cfg, router), nil
}

func buildRouter(routes []*routing.Route, directory string) *routing.Router {
	return routing.New(routes, directory)
}

// buildMiddlewareForListener creates middleware for a specific listener from the configuration.
func buildMiddlewareForListener(middlewareConfigs []config.MiddlewareConfig) ([]routing.Middleware, error) {
	return buildMiddlewareForRoute(middlewareConfigs)
}

// buildMiddlewareForResponse creates response middleware for a specific route from the configuration.
func buildMiddlewareForResponse(middlewareConfigs []config.MiddlewareConfig) ([]backend.ModifyResponse, error) {
	middlewares := make([]backend.ModifyResponse, 0)
	builtins := routing.NewBuiltinMiddleware()

	for _, mc := range middlewareConfigs {
		switch strings.ToLower(mc.Name) {
		case "add_response_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.AddResponseHeaders(headerMap))
			}
		case "set_response_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.SetResponseHeaders(headerMap))
			}
		case "modify_response_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.ModifyResponseHeaders(headerMap))
			}
		case "remove_response_headers":
			if names, err := helper.ParseStringSlice(mc.Config, "headers"); err == nil && len(names) > 0 {
				middlewares = append(middlewares, builtins.RemoveResponseHeaders(names))
			}
		case "add_location_prefix", "add_response_location_prefix":
			if prefix := helper.ParseStringOrDefault(mc.Config, "prefix", ""); prefix != "" {
				middlewares = append(middlewares, builtins.AddResponseLocationPrefix(prefix))
			}
		case "set_location_host", "replace_location_host", "set_response_location_host":
			if host := helper.ParseStringOrDefault(mc.Config, "host", ""); host != "" {
				middlewares = append(middlewares, builtins.SetResponseLocationHost(host))
			}
		case "dynamic_response", "response_dynamic":
			source := helper.ParseStringOrDefault(mc.Config, "code", "")
			modifier, err := dynamic.Response(source)
			if err != nil {
				return nil, fmt.Errorf("dynamic response middleware: %w", err)
			}
			middlewares = append(middlewares, modifier)
		default:
		}
	}
	return middlewares, nil
}

// buildMiddlewareForRoute creates middleware for a specific route from the configuration.
func buildMiddlewareForRoute(middlewareConfigs []config.MiddlewareConfig) ([]routing.Middleware, error) {
	middlewares := make([]routing.Middleware, 0)
	builtins := routing.NewBuiltinMiddleware()

	for _, mc := range middlewareConfigs {
		switch strings.ToLower(mc.Name) {
		case "add_prefix", "addprefix":
			if prefix := helper.ParseStringOrDefault(mc.Config, "prefix", ""); len(prefix) > 0 {
				middlewares = append(middlewares, builtins.AddPrefix(prefix))
			}

		case "ip_allow_list", "ipallowlist", "ip_white_list", "ipwhitelist":
			if cidr, err := helper.ParseStringSlice(mc.Config, "sources"); err == nil && len(cidr) > 0 {
				middlewares = append(middlewares, builtins.IPWhitelist(cidr))
			}

		case "delay":
			if duration, err := helper.ParseDuration(mc.Config, "duration", 0); err == nil && duration > 0 {
				middlewares = append(middlewares, builtins.Delay(duration))
			}

		case "add_request_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.AddRequestHeaders(headerMap))
			}

		case "set_request_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.SetRequestHeader(headerMap))
			}

		case "modify_request_headers":
			if headerMap, err := helper.ParseStringMap(mc.Config, "headers"); err == nil && len(headerMap) > 0 {
				middlewares = append(middlewares, builtins.ModifyRequestHeaders(headerMap))
			}

		case "remove_request_headers":
			if names, err := helper.ParseStringSlice(mc.Config, "headers"); err == nil && len(names) > 0 {
				middlewares = append(middlewares, builtins.RemoveRequestHeaders(names))
			}

		case "correlation_id":
			middlewares = append(middlewares, builtins.CorrelationID())

		case "cors":
			middlewares = append(middlewares, builtins.CORS())

		case "log", "logger", "logging":
			middlewares = append(middlewares, builtins.Logging())

		case "rate", "rate_limit", "rate_limiter":
			maxReqs := int(100)
			window := time.Duration(60) * time.Second
			maxReqs = helper.ParseInt(mc.Config, "max_requests", maxReqs)
			if val, err := helper.ParseDuration(mc.Config, "window", window); err == nil {
				window = val
			}
			middlewares = append(middlewares, builtins.RateLimitSimple(maxReqs, window))

		case "basic_auth", "auth_basic":
			realm := helper.ParseStringOrDefault(mc.Config, "realm", "Restricted")
			users, err := helper.ParseStringMap(mc.Config, "users")
			if err != nil || len(users) == 0 {
				return nil, fmt.Errorf("basic_auth requires a non-empty users map")
			}
			middlewares = append(middlewares, builtins.BasicAuth(realm, users))

		case "token_auth", "auth_token":
			realm := helper.ParseStringOrDefault(mc.Config, "realm", "Restricted")
			tokens, err := helper.ParseStringSlice(mc.Config, "tokens")
			if err != nil || len(tokens) == 0 {
				return nil, fmt.Errorf("token_auth requires a non-empty tokens list")
			}
			header := helper.ParseStringOrDefault(mc.Config, "header", "Authorization")
			prefix := helper.ParseStringOrDefault(mc.Config, "prefix", "Bearer ")
			middlewares = append(middlewares, builtins.TokenAuth(realm, tokens, header, prefix))

		case "auth_openid", "openid_auth", "auth_oidc", "oidc_auth":
			issuer := helper.ParseStringOrDefault(mc.Config, "issuer", "")
			audience := helper.ParseStringOrDefault(mc.Config, "audience", "")
			header := helper.ParseStringOrDefault(mc.Config, "header", "Authorization")
			prefix := helper.ParseStringOrDefault(mc.Config, "prefix", "Bearer ")
			algorithm := helper.ParseStringOrDefault(mc.Config, "algorithm", "RS256")
			keys, err := helper.ParseStringSlice(mc.Config, "keys")
			if err != nil || len(keys) == 0 {
				keys, err = helper.GetJWKSPEM(issuer)
				if err != nil {
					return nil, fmt.Errorf("auth_openid requires a non-empty key list")
				}
			}
			middlewares = append(middlewares, builtins.OpenIDAuth(issuer, audience, header, prefix, algorithm, keys))

		case "digest_auth", "auth_digest":
			realm := helper.ParseStringOrDefault(mc.Config, "realm", "Restricted")
			users, err := helper.ParseStringMap(mc.Config, "users")
			if err != nil || len(users) == 0 {
				return nil, fmt.Errorf("digest_auth requires a non-empty users map")
			}
			timeout, _ := helper.ParseDuration(mc.Config, "nonce_timeout", 5*time.Minute)
			middlewares = append(middlewares, builtins.DigestAuth(realm, users, timeout))

		case "secure_access", "secureaccess", "access_secure":
			path := helper.ParseStringOrDefault(mc.Config, "path", "")
			if len(path) == 0 {
				return nil, fmt.Errorf("secure_access requires a non-empty path")
			}
			key := helper.ParseStringOrDefault(mc.Config, "key", "this-is-a-very-secure-key")
			redirect := helper.ParseStringOrDefault(mc.Config, "redirect", "/")
			duration, err := helper.ParseDuration(mc.Config, "duration", time.Duration(15)*time.Minute)
			if err != nil {
				duration = time.Duration(15) * time.Minute
			}
			middlewares = append(middlewares, builtins.Secure(path, key, redirect, duration))

		case "dynamic_request", "request_dynamic":
			source := helper.ParseStringOrDefault(mc.Config, "code", "")
			middleware, err := dynamic.Request(source)
			if err != nil {
				return nil, fmt.Errorf("dynamic request middleware: %w", err)
			}
			middlewares = append(middlewares, middleware)

			/*
				default:
					return nil, fmt.Errorf("unknown middleware: %s", mc.Name)
			*/
		}
	}

	return middlewares, nil
}

func buildRoutes(cfg *config.Config) ([]*routing.Route, error) {
	routes := []*routing.Route{}
	if cfg == nil {
		return nil, fmt.Errorf("no configuration")
	}
	servicesByName := make(map[string][]config.Backend, len(cfg.Services))
	for _, s := range cfg.Services {
		servicesByName[s.Name] = s.Backends
	}

	for _, r := range cfg.Routes {
		backends := []*backend.Backend{}

		// Get response middleware for the route from the configuration.
		responseMiddlewares, err := buildMiddlewareForResponse(r.Middlewares)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", r.Name, err)
		}

		backendsConfig := r.Backends
		if len(backendsConfig) == 0 && r.Service != "" {
			backendsConfig = servicesByName[r.Service]
		}
		if len(backendsConfig) == 0 {
			return nil, fmt.Errorf("route %s: no backends configured and no service named %q found", r.Name, r.Service)
		}

		for _, b := range backendsConfig {
			baseTimeout := 30 * time.Second
			if len(b.Timeout) > 0 {
				if d, err := time.ParseDuration(b.Timeout); err == nil {
					baseTimeout = d
				}
			}

			connectTimeout := baseTimeout
			if len(b.ConnectTimeout) > 0 {
				if d, err := time.ParseDuration(b.ConnectTimeout); err == nil {
					connectTimeout = d
				}
			}
			headersTimeout := baseTimeout
			if len(b.HeadersTimeout) > 0 {
				if d, err := time.ParseDuration(b.HeadersTimeout); err == nil {
					headersTimeout = d
				}
			}
			responseTimeout := baseTimeout
			if len(b.ResponseTimeout) > 0 {
				if d, err := time.ParseDuration(b.ResponseTimeout); err == nil {
					responseTimeout = d
				}
			}
			idleTimeout := 90 * time.Second
			if len(b.IdleTimeout) > 0 {
				if d, err := time.ParseDuration(b.IdleTimeout); err == nil {
					idleTimeout = d
				}
			}

			transportCfg := backend.TransportConfig{
				Timeout:         baseTimeout,
				ConnectTimeout:  connectTimeout,
				HeadersTimeout:  headersTimeout,
				ResponseTimeout: responseTimeout,
				IdleTimeout:     idleTimeout,
				Compression:     b.Compression,
				Insecure:        b.TLS.Insecure,
				CAFile:          b.TLS.CAFile,
				CertFile:        b.TLS.CertFile,
				KeyFile:         b.TLS.KeyFile,
				ServerName:      b.TLS.ServerName,
			}
			if b.Auth != nil {
				transportCfg.Auth = &backend.AuthConfig{
					Type:            b.Auth.Type,
					Username:        b.Auth.Username,
					Password:        b.Auth.Password,
					Token:           b.Auth.Token,
					Header:          b.Auth.Header,
					Prefix:          b.Auth.Prefix,
					AccessKeyID:     b.Auth.AccessKeyID,
					SecretAccessKey: b.Auth.SecretAccessKey,
					Region:          b.Auth.Region,
					Service:         b.Auth.Service,
				}
			}
			if b.Proxy != nil && b.Proxy.URL != "" {
				transportCfg.Proxy = &backend.ProxyConfig{
					URL:      b.Proxy.URL,
					Username: b.Proxy.Username,
					Password: b.Proxy.Password,
				}
			}

			transport, err := backend.NewTransport(transportCfg)
			if err != nil {
				return nil, fmt.Errorf("backend %s: %w", b.Name, err)
			}

			be, err := backend.New(b.Name, b.URL, r.PreserveHost, transport)
			if err != nil {
				return nil, err
			}

			be.ConnectTimeout = connectTimeout
			be.HeadersTimeout = headersTimeout
			be.ResponseTimeout = responseTimeout
			be.IdleTimeout = idleTimeout

			// Add response middleware to the backend.
			if len(responseMiddlewares) > 0 {
				be.ModifyResponse = responseMiddlewares
			}

			backends = append(backends, be)
		}

		// Add middleware for this specific route.
		middlewares, err := buildMiddlewareForRoute(r.Middlewares)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", r.Name, err)
		}

		routes = append(routes, &routing.Route{
			Name:        r.Name,
			Prefix:      r.Prefix,
			StripPrefix: r.StripPrefix,
			Hosts:       r.Hosts,
			Endpoints:   r.Endpoints,
			Selector:    selector.New(r.Selector, backends),
			Middleware:  middlewares,
		})
	}

	return routes, nil
}
