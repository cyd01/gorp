package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var listenerTypes = map[string]bool{
	"http": true, "https": true, "dynamic": true, "http3": true,
	"httpmulti": true, "multi": true, "tcp": true, "unix": true,
	"unix_tcp": true, "proxy": true, "proxy_tls": true, "socks5": true, "mock": true, "mocksrv": true,
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
	"remove_prefix": {"prefix": true},
	"rewrite_path":  {"regex": true, "replacement": true},
	"redirect":      {"status": true, "host": true, "regex": true, "replacement": true},
	"ip_allow_list": {"sources": true}, "ipallowlist": {"sources": true},
	"ip_white_list": {"sources": true}, "ipwhitelist": {"sources": true},
	"method_allow_list": {"methods": true}, "method_deny_list": {"methods": true},
	"delay":                  {"duration": true},
	"add_request_headers":    {"headers": true},
	"set_request_headers":    {"headers": true},
	"modify_request_headers": {"headers": true},
	"remove_request_headers": {"headers": true},
	"correlation_id":         {"type": true},
	"distributed_tracing":    {}, "tracing": {}, "trace_context": {}, "cors": {},
	"prefer_header": {}, "prefer": {},
	"log": {}, "logger": {}, "logging": {},
	"rate":         {"max_requests": true, "window": true, "extensions": true},
	"rate_limit":   {"max_requests": true, "window": true, "extensions": true},
	"rate_limiter": {"max_requests": true, "window": true, "extensions": true},
	"basic_auth":   {"realm": true, "users": true}, "auth_basic": {"realm": true, "users": true},
	"token_auth":      {"realm": true, "tokens": true, "header": true, "prefix": true},
	"auth_token":      {"realm": true, "tokens": true, "header": true, "prefix": true},
	"auth_openid":     {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true, "client_id": true, "client_secret": true, "redirect_uri": true, "callback_path": true, "cookie_name": true},
	"openid_auth":     {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true, "client_id": true, "client_secret": true, "redirect_uri": true, "callback_path": true, "cookie_name": true},
	"auth_oidc":       {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true, "client_id": true, "client_secret": true, "redirect_uri": true, "callback_path": true, "cookie_name": true},
	"oidc_auth":       {"issuer": true, "audience": true, "header": true, "prefix": true, "algorithm": true, "remove_header": true, "keys": true, "client_id": true, "client_secret": true, "redirect_uri": true, "callback_path": true, "cookie_name": true},
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
	"response_throttle": {"bytes_per_second": true},
}

var responseMiddlewares = map[string]bool{
	"add_response_headers": true, "set_response_headers": true,
	"modify_response_headers": true, "remove_response_headers": true,
	"add_location_prefix": true, "add_response_location_prefix": true,
	"set_location_host": true, "replace_location_host": true,
	"set_response_location_host": true, "dynamic_response": true,
	"response_dynamic": true, "response_affinity": true,
	"response_throttle": true,
}

var requiredMiddlewareFields = map[string][]string{
	"basic_auth": {"users"}, "auth_basic": {"users"},
	"token_auth": {"tokens"}, "auth_token": {"tokens"},
	"digest_auth": {"users"}, "auth_digest": {"users"},
	"add_prefix": {"prefix"}, "addprefix": {"prefix"},
	"remove_prefix": {"prefix"},
	"rewrite_path":  {"regex"},
	"ip_allow_list": {"sources"}, "ipallowlist": {"sources"},
	"ip_white_list": {"sources"}, "ipwhitelist": {"sources"},
	"method_allow_list": {"methods"}, "method_deny_list": {"methods"},
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
	"response_throttle": {"bytes_per_second"},
	"openapi":           {"file"}, "openapi_validate": {"file"}, "openapi_validation": {"file"},
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
		if listener.Type == "mock" {
			if listener.Mock == nil {
				add("%s.mock is required for mock listeners", path)
			} else {
				if listener.Mock.Default != nil {
					if err := ValidateMockScenario(*listener.Mock.Default); err != nil {
						add("%s.mock.default: %v", path, err)
					}
				}
				for name, scenario := range listener.Mock.Scenarios {
					if !validMockScenarioName(name) {
						add("%s.mock.scenarios contains invalid name %q", path, name)
					}
					if err := ValidateMockScenario(scenario); err != nil {
						add("%s.mock.scenarios[%q]: %v", path, name, err)
					}
				}
			}
			if len(listener.Backends) > 0 || len(listener.Routes) > 0 || listener.Proxy != nil ||
				len(listener.Users) > 0 || listener.Log || len(listener.InitialBytes) > 0 {
				add("%s mock listeners do not support backends, TCP routes, or proxy settings", path)
			}
			if listener.MockSrv != nil {
				add("%s.mocksrv is only valid for mocksrv listeners", path)
			}
		} else if listener.Type == "mocksrv" {
			if listener.MockSrv == nil {
				add("%s.mocksrv is required for mocksrv listeners", path)
			} else {
				for name, route := range listener.MockSrv.Routes {
					if !validMockScenarioName(name) {
						add("%s.mocksrv.routes contains invalid name %q", path, name)
					}
					if err := ValidateMockSrvRoute(route); err != nil {
						add("%s.mocksrv.routes[%q]: %v", path, name, err)
					}
				}
			}
			if len(listener.Backends) > 0 || len(listener.Routes) > 0 || listener.Proxy != nil ||
				len(listener.Users) > 0 || listener.Log || len(listener.InitialBytes) > 0 {
				add("%s mocksrv listeners do not support backends, TCP routes, or proxy settings", path)
			}
			if listener.Mock != nil {
				add("%s.mock is only valid for mock listeners", path)
			}
		} else if listener.Mock != nil {
			add("%s.mock is only valid for mock listeners", path)
		} else if listener.MockSrv != nil {
			add("%s.mocksrv is only valid for mocksrv listeners", path)
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

// ValidateMockScenario checks the fields that can be configured at startup or
// changed through the admin API.
func ValidateMockScenario(scenario MockScenario) error {
	if scenario.Response.Code != "" &&
		(scenario.Response.Status != 0 || len(scenario.Response.Headers) > 0 || scenario.Response.Body != "") {
		return fmt.Errorf("response.code cannot be combined with response.status, response.headers, or response.body")
	}
	if method := scenario.Match.Method; method != "" {
		if _, err := http.NewRequest(method, "http://example.com", nil); err != nil {
			return fmt.Errorf("match.method %q is invalid", method)
		}
	}
	if path := scenario.Match.Path; path != "" {
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
			return fmt.Errorf("match.path must be an absolute path without query or fragment")
		}
		parameterNames := make(map[string]bool)
		for _, segment := range strings.Split(path, "/") {
			if !strings.HasPrefix(segment, ":") {
				continue
			}
			name := strings.TrimPrefix(segment, ":")
			if !validMockPathParameterName(name) {
				return fmt.Errorf("match.path parameter %q must be a non-empty identifier", segment)
			}
			if parameterNames[name] {
				return fmt.Errorf("match.path parameter %q is duplicated", name)
			}
			parameterNames[name] = true
		}
	}
	for name, value := range scenario.Match.Headers {
		if !validHTTPToken(name) {
			return fmt.Errorf("match.headers contains invalid header name %q", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("match.headers.%s must not contain newlines", name)
		}
	}
	if status := scenario.Response.Status; status != 0 && (status < 200 || status > 599) {
		return fmt.Errorf("response.status must be between 200 and 599")
	}
	for name, value := range scenario.Response.Headers {
		if !validHTTPToken(name) {
			return fmt.Errorf("response.headers contains invalid header name %q", name)
		}
		switch strings.ToLower(name) {
		case "connection", "content-length", "keep-alive", "proxy-authenticate",
			"proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
			return fmt.Errorf("response.headers.%s is not supported", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("response.headers.%s must not contain newlines", name)
		}
	}
	return nil
}

func ValidateMockSrvRoute(route MockSrvRoute) error {
	match := route.Match
	if match.Method == "" && match.Path == "" && len(match.Headers) == 0 {
		return fmt.Errorf("match must define at least one of method, path, or headers")
	}
	if err := ValidateMockScenario(MockScenario{Match: match}); err != nil {
		return err
	}
	if match.Path != "" {
		if _, err := filepath.Match(match.Path, match.Path); err != nil {
			return fmt.Errorf("match.path has an invalid wildcard pattern: %w", err)
		}
	}
	for name, value := range match.Headers {
		if _, err := filepath.Match(value, value); err != nil {
			return fmt.Errorf("match.headers.%s has an invalid wildcard pattern: %w", name, err)
		}
	}
	for name, response := range route.Scenarios {
		if !validMockScenarioName(name) {
			return fmt.Errorf("scenarios contains invalid name %q", name)
		}
		if err := ValidateMockScenario(MockScenario{Response: response}); err != nil {
			return fmt.Errorf("scenarios[%q]: %w", name, err)
		}
	}
	if route.Default != nil {
		if err := ValidateMockScenario(MockScenario{Response: *route.Default}); err != nil {
			return fmt.Errorf("default: %w", err)
		}
	}
	return nil
}

func validMockPathParameterName(name string) bool {
	if name == "" || !((name[0] >= 'a' && name[0] <= 'z') ||
		(name[0] >= 'A' && name[0] <= 'Z') || name[0] == '_') {
		return false
	}
	for i := 1; i < len(name); i++ {
		char := name[i]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

func validHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", char) {
			continue
		}
		return false
	}
	return true
}

func validMockScenarioName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		strings.TrimSpace(name) == name && !strings.ContainsAny(name, "/\\?#\r\n")
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
		"host": true, "regex": true, "replacement": true, "client_id": true,
		"client_secret": true, "redirect_uri": true, "callback_path": true, "cookie_name": true,
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
		case key == "sources" || key == "tokens" || key == "keys" || key == "extensions" || key == "headers" || key == "methods":
			if values, ok := value.([]any); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.%s must be a string list", path, key))
			} else {
				for i, item := range values {
					method, isString := item.(string)
					if !isString {
						*problems = append(*problems, fmt.Sprintf("%s.config.%s[%d] must be a string", path, key, i))
					} else if key == "methods" && !validHTTPMethod(method) {
						*problems = append(*problems, fmt.Sprintf("%s.config.methods[%d] must be a valid HTTP method token", path, i))
					}
				}
			}
		case key == "remove_header":
			if _, ok := value.(bool); !ok {
				*problems = append(*problems, fmt.Sprintf("%s.config.remove_header must be a boolean", path))
			}
		case name == "redirect" && key == "status":
			status, valid := integerValue(value)
			if !valid || !validRedirectStatus(status) {
				*problems = append(*problems, fmt.Sprintf("%s.config.status must be one of 301, 302, 303, 307, or 308", path))
			}
		}
	}
	for _, field := range requiredMiddlewareFields[name] {
		value, exists := middleware.Config[field]
		if !exists || emptyValue(value) {
			*problems = append(*problems, fmt.Sprintf("%s.config.%s is required", path, field))
		}
	}
	if name == "rewrite_path" {
		if _, exists := middleware.Config["replacement"]; !exists {
			*problems = append(*problems, fmt.Sprintf("%s.config.replacement is required", path))
		}
		if pattern, ok := middleware.Config["regex"].(string); ok {
			if _, err := regexp.Compile(pattern); err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.config.regex is invalid: %v", path, err))
			}
		}
	}
	if name == "redirect" {
		status, exists := middleware.Config["status"]
		if !exists || emptyValue(status) {
			*problems = append(*problems, fmt.Sprintf("%s.config.status is required", path))
		}

		host, hasHost := middleware.Config["host"].(string)
		pattern, hasPattern := middleware.Config["regex"].(string)
		_, hasReplacement := middleware.Config["replacement"]
		if hasHost && host != "" {
			parsed, err := url.Parse("//" + host)
			if err != nil || parsed.Host != host || parsed.User != nil ||
				parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				*problems = append(*problems, fmt.Sprintf("%s.config.host must be a hostname with an optional port", path))
			}
		}
		if hasPattern {
			if pattern == "" {
				*problems = append(*problems, fmt.Sprintf("%s.config.regex must not be empty", path))
			} else if _, err := regexp.Compile(pattern); err != nil {
				*problems = append(*problems, fmt.Sprintf("%s.config.regex is invalid: %v", path, err))
			}
			if !hasReplacement {
				*problems = append(*problems, fmt.Sprintf("%s.config.replacement is required when regex is set", path))
			}
		} else if hasReplacement {
			*problems = append(*problems, fmt.Sprintf("%s.config.replacement requires regex", path))
		}
		if (!hasHost || host == "") && (!hasPattern || pattern == "") {
			*problems = append(*problems, fmt.Sprintf("%s.config must define host or regex", path))
		}
	}
	if name == "auth_openid" || name == "openid_auth" || name == "auth_oidc" || name == "oidc_auth" {
		if emptyValue(middleware.Config["keys"]) && emptyValue(middleware.Config["issuer"]) {
			*problems = append(*problems, fmt.Sprintf("%s.config must define keys or issuer", path))
		}
		if !emptyValue(middleware.Config["client_id"]) && emptyValue(middleware.Config["redirect_uri"]) {
			*problems = append(*problems, fmt.Sprintf("%s.config.redirect_uri is required when client_id is configured", path))
		}
		if !emptyValue(middleware.Config["redirect_uri"]) && emptyValue(middleware.Config["client_id"]) {
			*problems = append(*problems, fmt.Sprintf("%s.config.client_id is required when redirect_uri is configured", path))
		}
		if !emptyValue(middleware.Config["client_id"]) && emptyValue(middleware.Config["issuer"]) {
			*problems = append(*problems, fmt.Sprintf("%s.config.issuer is required for interactive OpenID login", path))
		}
		for _, field := range []string{"client_secret", "callback_path", "cookie_name"} {
			if !emptyValue(middleware.Config[field]) && emptyValue(middleware.Config["client_id"]) {
				*problems = append(*problems, fmt.Sprintf("%s.config.client_id is required when %s is configured", path, field))
			}
		}
	}
	if name == "rate" || name == "rate_limit" || name == "rate_limiter" {
		if value, ok := middleware.Config["max_requests"]; ok {
			if number, valid := integerValue(value); !valid || number <= 0 {
				*problems = append(*problems, fmt.Sprintf("%s.config.max_requests must be a positive integer", path))
			}
		}
	}
	if name == "response_throttle" {
		if value, ok := middleware.Config["bytes_per_second"]; ok {
			if number, valid := integerValue(value); !valid || number <= 0 {
				*problems = append(*problems, fmt.Sprintf("%s.config.bytes_per_second must be a positive integer", path))
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

func validHTTPMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, char := range []byte(method) {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(char)) {
			continue
		}
		return false
	}
	return true
}

func validRedirectStatus(status int64) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
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
