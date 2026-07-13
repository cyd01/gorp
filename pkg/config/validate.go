package config

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var listenerTypes = map[string]bool{
	"http": true, "https": true, "dynamic": true, "http3": true,
	"httpmulti": true, "multi": true, "tcp": true, "unix": true,
	"unix_tcp": true, "proxy": true, "proxy_tls": true, "socks5": true,
}
var selectorTypes = map[string]bool{
	"round-robin": true, "roundrobin": true, "rr": true,
	"random": true, "rand": true, "first-alive": true, "first": true,
	"alive": true, "header": true, "cookie": true, "ip": true,
	"source-ip": true, "sourceip": true, "ip-hash": true, "iphash": true,
	"leastconnection": true, "leastconnections": true, "least-connection": true,
	"least-connections": true, "power-of-two": true, "poweroftwo": true,
	"p2c": true, "query": true, "query-param": true, "queryparam": true,
}

var middlewareFields = map[string]map[string]bool{
	"add_prefix": {"prefix": true}, "addprefix": {"prefix": true},
	"ip_allow_list": {"sources": true}, "ipallowlist": {"sources": true},
	"ip_white_list": {"sources": true}, "ipwhitelist": {"sources": true},
	"delay":                  {"duration": true},
	"add_request_headers":    {"headers": true},
	"set_request_headers":    {"headers": true},
	"modify_request_headers": {"headers": true},
	"remove_request_headers": {"headers": true},
	"correlation_id":         {"type": true},
	"distributed_tracing":    {}, "tracing": {}, "trace_context": {}, "cors": {},
	"log": {}, "logger": {}, "logging": {},
	"rate":         {"max_requests": true, "window": true, "extensions": true},
	"rate_limit":   {"max_requests": true, "window": true, "extensions": true},
	"rate_limiter": {"max_requests": true, "window": true, "extensions": true},
	"basic_auth":   {"realm": true, "users": true}, "auth_basic": {"realm": true, "users": true},
	"token_auth":      {"realm": true, "tokens": true, "header": true, "prefix": true},
	"auth_token":      {"realm": true, "tokens": true, "header": true, "prefix": true},
	"auth_openid":     {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true},
	"openid_auth":     {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true},
	"auth_oidc":       {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true},
	"oidc_auth":       {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true},
	"digest_auth":     {"realm": true, "users": true, "nonce_timeout": true},
	"auth_digest":     {"realm": true, "users": true, "nonce_timeout": true},
	"secure_access":   {"path": true, "key": true, "redirect": true, "duration": true},
	"secureaccess":    {"path": true, "key": true, "redirect": true, "duration": true},
	"access_secure":   {"path": true, "key": true, "redirect": true, "duration": true},
	"dynamic_request": {"code": true}, "request_dynamic": {"code": true},
	"openapi": {"file": true}, "openapi_validate": {"file": true}, "openapi_validation": {"file": true},
	"add_response_headers": {"headers": true}, "set_response_headers": {"headers": true},
	"modify_response_headers": {"headers": true}, "remove_response_headers": {"headers": true},
	"add_location_prefix": {"prefix": true}, "add_response_location_prefix": {"prefix": true},
	"set_location_host": {"host": true}, "replace_location_host": {"host": true},
	"set_response_location_host": {"host": true},
	"dynamic_response":           {"code": true}, "response_dynamic": {"code": true},
	"response_affinity": {"header": true},
}

var responseMiddlewares = map[string]bool{
	"add_response_headers": true, "set_response_headers": true,
	"modify_response_headers": true, "remove_response_headers": true,
	"add_location_prefix": true, "add_response_location_prefix": true,
	"set_location_host": true, "replace_location_host": true,
	"set_response_location_host": true, "dynamic_response": true,
	"response_dynamic": true, "response_affinity": true,
}

var requiredMiddlewareFields = map[string][]string{
	"basic_auth": {"users"}, "auth_basic": {"users"},
	"token_auth": {"tokens"}, "auth_token": {"tokens"},
	"digest_auth": {"users"}, "auth_digest": {"users"},
	"add_prefix": {"prefix"}, "addprefix": {"prefix"},
	"ip_allow_list": {"sources"}, "ipallowlist": {"sources"},
	"ip_white_list": {"sources"}, "ipwhitelist": {"sources"},
	"delay":               {"duration"},
	"add_request_headers": {"headers"}, "set_request_headers": {"headers"},
	"modify_request_headers": {"headers"}, "remove_request_headers": {"headers"},
	"add_response_headers": {"headers"}, "set_response_headers": {"headers"},
	"modify_response_headers": {"headers"}, "remove_response_headers": {"headers"},
	"add_location_prefix": {"prefix"}, "add_response_location_prefix": {"prefix"},
	"set_location_host": {"host"}, "replace_location_host": {"host"},
	"set_response_location_host": {"host"},
	"secure_access":              {"path"}, "secureaccess": {"path"}, "access_secure": {"path"},
	"dynamic_request": {"code"}, "request_dynamic": {"code"},
	"dynamic_response": {"code"}, "response_dynamic": {"code"},
	"openapi": {"file"}, "openapi_validate": {"file"}, "openapi_validation": {"file"},
}

// IsKnownMiddleware reports whether name identifies a configured middleware.
func IsKnownMiddleware(name string) bool {
	_, ok := middlewareFields[strings.ToLower(name)]
	return ok
}

// IsResponseMiddleware reports whether name modifies an upstream response.
func IsResponseMiddleware(name string) bool {
	return responseMiddlewares[strings.ToLower(name)]
}

// Validate checks configuration relationships and values that would otherwise
// be ignored or fail only after some listeners have started.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	listenerNames := make(map[string]bool, len(cfg.Listeners))
	listenerAddresses := make(map[string][]string, len(cfg.Listeners))
	for i, listener := range cfg.Listeners {
		path := fmt.Sprintf("listeners[%d]", i)
		if listener.Name == "" {
			add("%s.name is required", path)
		} else if listenerNames[listener.Name] {
			add("%s.name %q is duplicated", path, listener.Name)
		}
		listenerNames[listener.Name] = true
		if !listenerTypes[listener.Type] {
			add("%s.type %q is not supported", path, listener.Type)
		}
		if listener.Address == "" {
			add("%s.address is required", path)
		} else {
			network := listenerNetwork(listener.Type)
			if network == "unix" {
				key := network + "|" + listener.Address
				if previous, exists := listenerAddresses[key]; exists {
					add("%s.address %q conflicts with listener %q", path, listener.Address, previous[0])
				} else {
					listenerAddresses[key] = []string{listener.Name}
				}
			} else if host, port, ok := validateNetworkAddress(&problems, path+".address", listener.Address); ok && port != "0" {
				for key, existing := range listenerAddresses {
					parts := strings.SplitN(key, "|", 3)
					if len(parts) != 3 || parts[0] != network || parts[2] != port {
						continue
					}
					if hostsConflict(host, parts[1]) {
						add("%s.address %q conflicts with listener %q", path, listener.Address, existing[0])
						break
					}
				}
				key := network + "|" + host + "|" + port
				listenerAddresses[key] = append(listenerAddresses[key], listener.Name)
			}
		}
		if listener.MaxConnections < 0 || listener.MaxRequestBodySize < 0 {
			add("%s connection and request-body limits must not be negative", path)
		}
		validateDuration(&problems, path+".read_header_timeout", listener.ReadHeaderTimeout, false)
		validateDuration(&problems, path+".write_timeout", listener.WriteTimeout, false)
		validateDuration(&problems, path+".idle_timeout", listener.IdleTimeout, false)
		requiresTLS := listener.Type == "https" || listener.Type == "http3" ||
			listener.Type == "httpmulti" || listener.Type == "multi" ||
			listener.Type == "proxy_tls" || listener.Type == "dynamic"
		validateTLS(&problems, path+".tls", listener.TLS, requiresTLS)
		if len(listener.Backends) > 0 && listener.Type != "tcp" && listener.Type != "unix_tcp" {
			add("%s.backends is only valid for TCP listeners", path)
		}
		if len(listener.Routes) > 0 && listener.Type != "tcp" && listener.Type != "unix_tcp" {
			add("%s.routes is only valid for TCP listeners", path)
		}
		if (listener.Proxy != nil || len(listener.Users) > 0 || listener.Log) &&
			listener.Type != "proxy" && listener.Type != "proxy_tls" && listener.Type != "socks5" {
			add("%s proxy settings are only valid for proxy and SOCKS5 listeners", path)
		}
		if listener.Proxy != nil {
			validateProxy(&problems, path+".proxy", listener.Proxy.URL, listener.Proxy.Hosts, listener.Proxy.NoProxy)
		}
		for j, middleware := range listener.Middlewares {
			validateMiddleware(&problems, fmt.Sprintf("%s.middlewares[%d]", path, j), middleware, false)
		}
		for j, tcpRoute := range listener.Routes {
			routePath := fmt.Sprintf("%s.routes[%d]", path, j)
			if tcpRoute.Name == "" {
				add("%s.name is required", routePath)
			}
			if len(tcpRoute.Hosts) == 0 {
				add("%s.hosts must contain at least one SNI pattern", routePath)
			}
			for _, host := range tcpRoute.Hosts {
				validatePattern(&problems, routePath+".hosts", host)
			}
			if tcpRoute.TLS != nil {
				validateTLS(&problems, routePath+".tls", *tcpRoute.TLS, false)
			}
			for k, backend := range tcpRoute.Backends {
				validateBackend(&problems, fmt.Sprintf("%s.backends[%d]", routePath, k), backend, true)
			}
		}
		for j, backend := range listener.Backends {
			validateBackend(&problems, fmt.Sprintf("%s.backends[%d]", path, j), backend, true)
		}
	}

	serviceNames := make(map[string]bool, len(cfg.Services))
	for i, service := range cfg.Services {
		path := fmt.Sprintf("services[%d]", i)
		if service.Name == "" {
			add("%s.name is required", path)
		} else if serviceNames[service.Name] {
			add("%s.name %q is duplicated", path, service.Name)
		}
		serviceNames[service.Name] = true
		if len(service.Backends) == 0 {
			add("%s.backends must not be empty", path)
		}
		for j, backend := range service.Backends {
			validateBackend(&problems, fmt.Sprintf("%s.backends[%d]", path, j), backend, false)
		}
	}

	routeNames := make(map[string]bool, len(cfg.Routes))
	for i, route := range cfg.Routes {
		path := fmt.Sprintf("routes[%d]", i)
		if route.Name == "" {
			add("%s.name is required", path)
		} else if routeNames[route.Name] {
			add("%s.name %q is duplicated", path, route.Name)
		}
		routeNames[route.Name] = true
		if route.Prefix == "" && len(route.Hosts) == 0 && len(route.Paths) == 0 {
			add("%s must define at least one of prefix, hosts, or paths", path)
		}
		for _, pattern := range route.Hosts {
			validatePattern(&problems, path+".hosts", pattern)
		}
		for _, pattern := range route.Paths {
			validatePattern(&problems, path+".paths", pattern)
		}
		for _, endpoint := range route.Endpoints {
			if !listenerNames[endpoint] {
				add("%s.endpoints references unknown listener %q", path, endpoint)
			}
		}
		if route.Service != "" && !serviceNames[route.Service] {
			add("%s.service references unknown service %q", path, route.Service)
		}
		if route.Service != "" && len(route.Backends) > 0 {
			add("%s must not define both service and backends", path)
		}
		if len(route.Backends) == 0 && route.Service == "" {
			add("%s must define backends or reference a service", path)
		}
		if route.Selector.Type != "" && !selectorTypes[strings.ToLower(route.Selector.Type)] {
			add("%s.selector.type %q is not supported", path, route.Selector.Type)
		}
		selectorName := strings.ToLower(route.Selector.Type)
		acceptsSelectorName := selectorName == "header" || selectorName == "cookie" ||
			selectorName == "query" || selectorName == "query-param" || selectorName == "queryparam"
		for key, value := range route.Selector.Config {
			if key != "name" || !acceptsSelectorName {
				add("%s.selector.config contains unsupported field %q", path, key)
			} else if _, ok := value.(string); !ok {
				add("%s.selector.config.name must be a string", path)
			}
		}
		for j, middleware := range route.Middlewares {
			validateMiddleware(&problems, fmt.Sprintf("%s.middlewares[%d]", path, j), middleware, true)
		}
		for j, backend := range route.Backends {
			validateBackend(&problems, fmt.Sprintf("%s.backends[%d]", path, j), backend, false)
		}
	}
	for i, middleware := range cfg.Admin.Middlewares {
		validateMiddleware(&problems, fmt.Sprintf("admin.middlewares[%d]", i), middleware, false)
	}
	if cfg.Admin.Enabled {
		validateTLS(&problems, "admin.tls", cfg.Admin.TLS, false)
		if cfg.Admin.Address == "" {
			add("admin.address is required when admin.enabled is true")
		} else if host, port, ok := validateNetworkAddress(&problems, "admin.address", cfg.Admin.Address); ok && port != "0" {
			for key, existing := range listenerAddresses {
				parts := strings.SplitN(key, "|", 3)
				if len(parts) == 3 && parts[0] == "tcp" && parts[2] == port && hostsConflict(host, parts[1]) {
					add("admin.address %q conflicts with listener %q", cfg.Admin.Address, existing[0])
					break
				}
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n - %s", strings.Join(problems, "\n - "))
}

func validateNetworkAddress(problems *[]string, path, address string) (string, string, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		*problems = append(*problems, fmt.Sprintf("%s must be a host:port address: %v", path, err))
		return "", "", false
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		*problems = append(*problems, fmt.Sprintf("%s port %q must be between 0 and 65535", path, port))
		return "", "", false
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
		if ip.IsUnspecified() {
			host = "*"
		}
	} else {
		host = strings.ToLower(host)
	}
	if host == "" {
		host = "*"
	}
	return host, strconv.Itoa(number), true
}

func hostsConflict(first, second string) bool {
	return first == second || first == "*" || second == "*"
}

func listenerNetwork(listenerType string) string {
	switch listenerType {
	case "http3":
		return "udp"
	case "unix", "unix_tcp":
		return "unix"
	default:
		return "tcp"
	}
}

func validateProxy(problems *[]string, path, rawURL string, hosts, noProxy []string) {
	if rawURL == "" {
		*problems = append(*problems, fmt.Sprintf("%s.url is required", path))
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || !parsed.IsAbs() {
		*problems = append(*problems, fmt.Sprintf("%s.url must be an absolute proxy URL with a host", path))
	} else if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		*problems = append(*problems, fmt.Sprintf("%s.url scheme %q is not supported", path, parsed.Scheme))
	}
	for _, pattern := range hosts {
		validatePattern(problems, path+".hosts", pattern)
	}
	for _, pattern := range noProxy {
		validatePattern(problems, path+".no_proxy", pattern)
	}
}

func validateTLS(problems *[]string, path string, tls TLSConfig, required bool) {
	hasCert, hasKey := tls.CertFile != "", tls.KeyFile != ""
	if required && (!hasCert || !hasKey) {
		*problems = append(*problems, fmt.Sprintf("%s requires both cert_file and key_file", path))
	} else if hasCert != hasKey {
		*problems = append(*problems, fmt.Sprintf("%s must set cert_file and key_file together", path))
	}
	if tls.MinVersion != "" {
		version := strings.ToUpper(strings.ReplaceAll(tls.MinVersion, "TLSV", "TLS"))
		switch version {
		case "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3", "1.0", "1.1", "1.2", "1.3":
		default:
			*problems = append(*problems, fmt.Sprintf("%s.min_version %q is invalid", path, tls.MinVersion))
		}
	}
}

func validateBackend(problems *[]string, path string, backend Backend, tcp bool) {
	if backend.Name == "" {
		*problems = append(*problems, fmt.Sprintf("%s.name is required", path))
	}
	parsed, err := url.Parse(backend.URL)
	if err != nil || parsed.Host == "" || !parsed.IsAbs() {
		*problems = append(*problems, fmt.Sprintf("%s.url must be an absolute URL with a host", path))
	} else if tcp && parsed.Scheme != "tcp" {
		*problems = append(*problems, fmt.Sprintf("%s.url must use the tcp:// scheme", path))
	} else if !tcp && parsed.Scheme != "http" && parsed.Scheme != "https" {
		*problems = append(*problems, fmt.Sprintf("%s.url must use http:// or https://", path))
	} else {
		host := parsed.Hostname()
		if host == "" {
			*problems = append(*problems, fmt.Sprintf("%s.url must contain a hostname", path))
		}
		port := parsed.Port()
		if tcp || port != "" {
			if tcp {
				_, port, err = net.SplitHostPort(parsed.Host)
			}
			number, portErr := strconv.Atoi(port)
			if err != nil || portErr != nil || number < 1 || number > 65535 {
				*problems = append(*problems, fmt.Sprintf("%s.url must include a valid port between 1 and 65535", path))
			}
		}
	}
	if backend.MaxConnections < 0 {
		*problems = append(*problems, fmt.Sprintf("%s.max_connections must not be negative", path))
	}
	if backend.Retries < 0 || backend.Retries > 5 {
		*problems = append(*problems, fmt.Sprintf("%s.retries must be between 0 and 5", path))
	}
	validateDuration(problems, path+".timeout", backend.Timeout, false)
	validateDuration(problems, path+".connect_timeout", backend.ConnectTimeout, false)
	validateDuration(problems, path+".headers_timeout", backend.HeadersTimeout, false)
	validateDuration(problems, path+".idle_timeout", backend.IdleTimeout, false)
	validateDuration(problems, path+".retry_backoff", backend.RetryBackoff, true)
	validateTLS(problems, path+".tls", TLSConfig{CertFile: backend.TLS.CertFile, KeyFile: backend.TLS.KeyFile}, false)
	if tcp && backend.HealthCheck != nil {
		*problems = append(*problems, fmt.Sprintf("%s.health_check is only supported for HTTP backends", path))
	}
	if tcp && (backend.Retries > 0 || backend.RetryBackoff != "") {
		*problems = append(*problems, fmt.Sprintf("%s.retries is only supported for HTTP backends", path))
	}
	if backend.Proxy != nil {
		if len(backend.Proxy.Hosts) > 0 || len(backend.Proxy.NoProxy) > 0 {
			*problems = append(*problems, fmt.Sprintf("%s.proxy hosts and no_proxy are only supported on listener proxies", path))
		}
		validateProxy(problems, path+".proxy", backend.Proxy.URL, nil, nil)
	}
	if backend.Auth != nil {
		switch strings.ToLower(backend.Auth.Type) {
		case "basic", "token", "aws":
		default:
			*problems = append(*problems, fmt.Sprintf("%s.auth.type %q is not supported", path, backend.Auth.Type))
		}
		switch strings.ToLower(backend.Auth.Type) {
		case "basic":
			if backend.Auth.Username == "" || backend.Auth.Password == "" {
				*problems = append(*problems, fmt.Sprintf("%s.auth basic requires username and password", path))
			}
		case "token":
			if backend.Auth.Token == "" {
				*problems = append(*problems, fmt.Sprintf("%s.auth.token is required for token authentication", path))
			}
		case "aws":
			if backend.Auth.AccessKeyID == "" || backend.Auth.SecretAccessKey == "" ||
				backend.Auth.Region == "" || backend.Auth.Service == "" {
				*problems = append(*problems, fmt.Sprintf("%s.auth AWS requires access_key_id, secret_access_key, region, and service", path))
			}
		}
	}
	if backend.HealthCheck != nil {
		check := backend.HealthCheck
		if check.Path != "" && (!strings.HasPrefix(check.Path, "/") || strings.Contains(check.Path, "?") || strings.Contains(check.Path, "#")) {
			*problems = append(*problems, fmt.Sprintf("%s.health_check.path must be an absolute path without query or fragment", path))
		}
		validateDuration(problems, path+".health_check.interval", check.Interval, true)
		validateDuration(problems, path+".health_check.timeout", check.Timeout, true)
	}
}

func validateDuration(problems *[]string, path, value string, mustBePositive bool) {
	if value == "" {
		return
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 || (mustBePositive && duration == 0) {
		*problems = append(*problems, fmt.Sprintf("%s must be a valid positive duration", path))
	}
}

func validatePattern(problems *[]string, path, pattern string) {
	if _, err := filepath.Match(pattern, ""); err != nil {
		*problems = append(*problems, fmt.Sprintf("%s contains invalid pattern %q: %v", path, pattern, err))
	}
}

func validateMiddleware(problems *[]string, path string, middleware MiddlewareConfig, allowResponse bool) {
	name := strings.ToLower(middleware.Name)
	allowed, known := middlewareFields[name]
	if !known {
		*problems = append(*problems, fmt.Sprintf("%s.name %q is not supported", path, middleware.Name))
		return
	}
	if IsResponseMiddleware(name) && !allowResponse {
		*problems = append(*problems, fmt.Sprintf("%s.name %q is a response middleware and can only be used on a route", path, middleware.Name))
	}
	stringFields := map[string]bool{
		"prefix": true, "type": true, "realm": true, "header": true,
		"algorithm": true, "issuer": true, "audience": true, "key": true,
		"redirect": true, "path": true, "code": true, "file": true,
		"host": true,
	}
	for key, value := range middleware.Config {
		if !allowed[key] {
			*problems = append(*problems, fmt.Sprintf("%s.config contains unsupported field %q", path, key))
		}
		switch {
		case stringFields[key]:
			if _, ok := value.(string); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.%s must be a string", path, key))
			}
		case key == "users" || (key == "headers" && name != "remove_request_headers" && name != "remove_response_headers"):
			if values, ok := value.(map[string]any); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.%s must be a string map", path, key))
			} else {
				for mapKey, item := range values {
					if _, ok := item.(string); !ok {
						*problems = append(*problems, fmt.Sprintf("%s.config.%s.%s must be a string", path, key, mapKey))
					}
				}
			}
		case key == "sources" || key == "tokens" || key == "keys" || key == "extensions" || key == "headers":
			if values, ok := value.([]any); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.%s must be a string list", path, key))
			} else {
				for i, item := range values {
					if _, ok := item.(string); !ok {
						*problems = append(*problems, fmt.Sprintf("%s.config.%s[%d] must be a string", path, key, i))
					}
				}
			}
		case key == "remove_header":
			if _, ok := value.(bool); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.remove_header must be a boolean", path))
			}
		}
	}
	for _, field := range requiredMiddlewareFields[name] {
		value, exists := middleware.Config[field]
		if !exists || emptyValue(value) {
			*problems = append(*problems, fmt.Sprintf("%s.config.%s is required", path, field))
		}
	}
	if name == "auth_openid" || name == "openid_auth" || name == "auth_oidc" || name == "oidc_auth" {
		if emptyValue(middleware.Config["keys"]) && emptyValue(middleware.Config["issuer"]) {
			*problems = append(*problems, fmt.Sprintf("%s.config must define keys or issuer", path))
		}
	}
	if name == "rate" || name == "rate_limit" || name == "rate_limiter" {
		if value, ok := middleware.Config["max_requests"]; ok {
			if number, valid := integerValue(value); !valid || number <= 0 {
				*problems = append(*problems, fmt.Sprintf("%s.config.max_requests must be a positive integer", path))
			}
		}
	}
	for _, field := range []string{"duration", "window", "nonce_timeout"} {
		if value, ok := middleware.Config[field]; ok {
			text, valid := value.(string)
			if !valid {
				*problems = append(*problems, fmt.Sprintf("%s.config.%s must be a duration string", path, field))
				continue
			}
			validateDuration(problems, path+".config."+field, text, true)
		}
	}
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case uint64:
		return int64(typed), typed <= uint64(^uint64(0)>>1)
	case float64:
		return int64(typed), typed == float64(int64(typed))
	default:
		return 0, false
	}
}
