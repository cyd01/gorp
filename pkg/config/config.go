package config

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Admin     AdminConfig `yaml:"admin,omitempty" json:"admin,omitempty"`
	Listeners []Listener  `yaml:"listeners" json:"listeners"`
	Services  []Service   `yaml:"services,omitempty" json:"services,omitempty"`
	Routes    []Route     `yaml:"routes,omitempty" json:"routes,omitempty"`
	Directory string      `yaml:"directory,omitempty" json:"directory,omitempty"`
}

type AdminConfig struct {
	Address     string             `yaml:"address" json:"address"`
	Enabled     bool               `yaml:"enabled" json:"enabled"`
	TLS         TLSConfig          `yaml:"tls,omitempty" json:"tls,omitempty"`
	Middlewares []MiddlewareConfig `yaml:"middlewares,omitempty" json:"middlewares,omitempty"`
}

type Listener struct {
	Name               string             `yaml:"name" json:"name"`
	Type               string             `yaml:"type" json:"type"`
	Address            string             `yaml:"address" json:"address"`
	ReadHeaderTimeout  string             `yaml:"read_header_timeout,omitempty" json:"read_header_timeout,omitempty"`
	WriteTimeout       string             `yaml:"write_timeout,omitempty" json:"write_timeout,omitempty"`
	IdleTimeout        string             `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	MaxConnections     int64              `yaml:"max_connections,omitempty" json:"max_connections,omitempty"`
	MaxRequestBodySize int64              `yaml:"max_request_body_size,omitempty" json:"max_request_body_size,omitempty"`
	TLS                TLSConfig          `yaml:"tls,omitempty" json:"tls,omitempty"`
	Users              Users              `yaml:"users,omitempty" json:"users,omitempty"`
	Log                bool               `yaml:"log,omitempty" json:"log,omitempty"`
	Proxy              *BackendProxy      `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Backends           []Backend          `yaml:"backends,omitempty" json:"backends,omitempty"`
	Routes             []TCPRoute         `yaml:"routes,omitempty" json:"routes,omitempty"`
	InitialBytes       []TCPByteCheck     `yaml:"initial_bytes,omitempty" json:"initial_bytes,omitempty"`
	Middlewares        []MiddlewareConfig `yaml:"middlewares,omitempty" json:"middlewares,omitempty"`
	Mock               *MockConfig        `yaml:"mock,omitempty" json:"mock,omitempty"`
	MockSrv            *MockSrvConfig     `yaml:"mocksrv,omitempty" json:"mocksrv,omitempty"`
}

type MockConfig struct {
	Default   *MockScenario           `yaml:"default,omitempty" json:"default,omitempty"`
	Scenarios map[string]MockScenario `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
}

type MockScenario struct {
	Match    MockMatch    `yaml:"match,omitempty" json:"match,omitempty"`
	Response MockResponse `yaml:"response" json:"response"`
}

type MockMatch struct {
	Method  string            `yaml:"method,omitempty" json:"method,omitempty"`
	Path    string            `yaml:"path,omitempty" json:"path,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type MockSrvConfig struct {
	Routes map[string]MockSrvRoute `yaml:"routes,omitempty" json:"routes,omitempty"`
}

type MockSrvRoute struct {
	Match     MockMatch               `yaml:"match" json:"match"`
	Scenarios map[string]MockResponse `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
	Default   *MockResponse           `yaml:"default,omitempty" json:"default,omitempty"`
}

type MockResponse struct {
	Status  int               `yaml:"status,omitempty" json:"status,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`
	Code    string            `yaml:"code,omitempty" json:"code,omitempty"`
}

// TCPByteCheck describes the acceptable values for one byte in a TCP stream.
// Any accepts every value; Values restrict the byte to the listed values.
type TCPByteCheck struct {
	Any    bool
	Values []byte
}

func (check *TCPByteCheck) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Value == "*" {
		check.Any = true
		return nil
	}

	if node.Kind == yaml.SequenceNode {
		values := make([]byte, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := parseTCPByte(item)
			if err != nil {
				return fmt.Errorf("invalid TCP byte values: %w", err)
			}
			values = append(values, value)
		}
		if len(values) == 0 {
			return fmt.Errorf("TCP byte values must not be empty")
		}
		check.Values = values
		return nil
	}

	value, err := parseTCPByte(node)
	if err != nil {
		return fmt.Errorf("invalid TCP byte value: %w", err)
	}
	check.Values = []byte{value}
	return nil
}

func parseTCPByte(node *yaml.Node) (byte, error) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		value := []byte(node.Value)
		if len(value) != 1 {
			return 0, fmt.Errorf("expected a single-byte character or number, got %q", node.Value)
		}
		return value[0], nil
	}

	var value uint8
	if err := node.Decode(&value); err != nil {
		return 0, err
	}
	return value, nil
}

// Users accepts either a mapping or a list of one-entry mappings in YAML.
type Users map[string]string

func (users *Users) UnmarshalYAML(node *yaml.Node) error {
	parsed := make(Users)
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index < len(node.Content); index += 2 {
			var username, password string
			if err := node.Content[index].Decode(&username); err != nil {
				return err
			}
			if err := node.Content[index+1].Decode(&password); err != nil {
				return err
			}
			parsed[username] = password
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
				return fmt.Errorf("users list items must contain exactly one username and password")
			}
			var username, password string
			if err := item.Content[0].Decode(&username); err != nil {
				return err
			}
			if err := item.Content[1].Decode(&password); err != nil {
				return err
			}
			parsed[username] = password
		}
	default:
		return fmt.Errorf("users must be a mapping or a list of mappings")
	}
	*users = parsed
	return nil
}

// TCPRoute maps one or more SNI hostnames to a backend pool.
type TCPRoute struct {
	Name     string     `yaml:"name" json:"name"`
	Hosts    []string   `yaml:"hosts" json:"hosts"`
	TLS      *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
	Backends []Backend  `yaml:"backends" json:"backends"`
}

type TLSConfig struct {
	CertFile      string `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile       string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty" json:"key_passphrase,omitempty" sensitive:"true"`
	MinVersion    string `yaml:"min_version,omitempty" json:"min_version,omitempty"`
	// Custom corporate CA file for mTLS
	CAFile string `yaml:"ca_file,omitempty" json:"ca_file,omitempty"`
	// CRL revocation list file.
	CRLFile string `yaml:"crl_file,omitempty" json:"crl_file,omitempty"`
	// OCSP responder URL. If CRL is configured, it takes precedence.
	OCSPURL string `yaml:"ocsp_url,omitempty" json:"ocsp_url,omitempty"`
}

type Service struct {
	Name     string    `yaml:"name" json:"name"`
	Backends []Backend `yaml:"backends" json:"backends"`
}

type Route struct {
	Name         string             `yaml:"name" json:"name"`
	Prefix       string             `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	StripPrefix  bool               `yaml:"strip_prefix" json:"strip_prefix"`
	PreserveHost bool               `yaml:"preserve_host" json:"preserve_host"`
	Selector     SelectorConfig     `yaml:"selector,omitempty" json:"selector,omitempty"`
	Hosts        []string           `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	Paths        []string           `yaml:"paths,omitempty" json:"paths,omitempty"`
	Endpoints    []string           `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
	Service      string             `yaml:"service,omitempty" json:"service,omitempty"`
	Backends     []Backend          `yaml:"backends,omitempty" json:"backends,omitempty"`
	Middlewares  []MiddlewareConfig `yaml:"middlewares,omitempty" json:"middlewares,omitempty"`
}

// SelectorConfig describes a backend selector to apply.
type SelectorConfig struct {
	Type   string                 `yaml:"type" json:"type"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// MiddlewareConfig describes a middleware to apply.
type MiddlewareConfig struct {
	Name   string                 `yaml:"name" json:"name"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

type Backend struct {
	Name           string             `yaml:"name" json:"name"`
	URL            string             `yaml:"url" json:"url"`
	Timeout        string             `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ConnectTimeout string             `yaml:"connect_timeout,omitempty" json:"connect_timeout,omitempty"`
	HeadersTimeout string             `yaml:"headers_timeout,omitempty" json:"headers_timeout,omitempty"`
	IdleTimeout    string             `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	MaxConnections int64              `yaml:"max_connections,omitempty" json:"max_connections,omitempty"`
	Retries        int                `yaml:"retries,omitempty" json:"retries,omitempty"`
	RetryBackoff   string             `yaml:"retry_backoff,omitempty" json:"retry_backoff,omitempty"`
	HealthCheck    *HealthCheckConfig `yaml:"health_check,omitempty" json:"health_check,omitempty"`
	Compression    bool               `yaml:"compression,omitempty" json:"compression,omitempty"`
	ForceTLS       bool               `yaml:"force_tls,omitempty" json:"force_tls,omitempty"`
	TLS            BackendTLS         `yaml:"tls,omitempty" json:"tls,omitempty"`
	Auth           *BackendAuth       `yaml:"auth,omitempty" json:"auth,omitempty"`
	Proxy          *BackendProxy      `yaml:"proxy,omitempty" json:"proxy,omitempty"`
}

type HealthCheckConfig struct {
	Path     string `yaml:"path,omitempty" json:"path,omitempty"`
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

type BackendProxy struct {
	URL      string   `yaml:"url,omitempty" json:"url,omitempty"`
	Username string   `yaml:"username,omitempty" json:"username,omitempty"`
	Password string   `yaml:"password,omitempty" json:"password,omitempty" sensitive:"true"`
	Hosts    []string `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	NoProxy  []string `yaml:"no_proxy,omitempty" json:"no_proxy,omitempty"`
}

type BackendAuth struct {
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty" sensitive:"true"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty" sensitive:"true"`
	Header   string `yaml:"header,omitempty" json:"header,omitempty"`
	Prefix   string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	// AWS Signature Version 4 (for S3 and other AWS services)
	AccessKeyID     string `yaml:"access_key_id,omitempty" json:"access_key_id,omitempty" sensitive:"true"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty" json:"secret_access_key,omitempty" sensitive:"true"`
	Region          string `yaml:"region,omitempty" json:"region,omitempty"`
	Service         string `yaml:"service,omitempty" json:"service,omitempty"` // e.g. "s3", "ec2"
}

// BackendTLS contains TLS configuration for a backend.
type BackendTLS struct {
	// Ignore certificate validation
	Insecure bool `yaml:"insecure" json:"insecure"`
	// Custom corporate CA
	CAFile string `yaml:"ca_file,omitempty" json:"ca_file,omitempty"`
	// TLS SNI
	ServerName    string `yaml:"server_name,omitempty" json:"server_name,omitempty"`
	CertFile      string `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile       string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty" json:"key_passphrase,omitempty" sensitive:"true"`
}

var sensitiveKeys = map[string]bool{
	"client_secret": true,
	"password":      true,
	"token":         true,
	"access_key":    true,
	"secret_key":    true,
	"key":           true,
}

// MaskSensitive returns a copy of the config with sensitive fields masked.
func (c *Config) MaskSensitive() interface{} {
	return maskSensitive(reflect.ValueOf(*c))
}

func maskSensitive(v reflect.Value) interface{} {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return maskSensitive(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return maskSensitive(v.Elem())
	case reflect.Struct:
		// We create a map to represent the masked struct for JSON encoding
		// This avoids issues with modifying the original struct or needing a separate masked struct type
		maskedMap := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			value := v.Field(i)

			// Determine the JSON tag name
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				jsonTag = field.Name
			} else {
				// Handle omitempty
				if strings.Contains(jsonTag, "omitempty") {
					jsonTag = strings.Split(jsonTag, ",")[0]
				}
			}

			// Skip omitempty fields if they are zero values
			if strings.Contains(field.Tag.Get("json"), "omitempty") && isZeroValue(value) {
				continue
			}

			isSensitive := field.Tag.Get("sensitive") == "true"

			if isSensitive {
				maskedMap[jsonTag] = "***"
			} else {
				maskedMap[jsonTag] = maskSensitive(value)
			}
		}
		return maskedMap
	case reflect.Slice:
		sliceLen := v.Len()
		maskedSlice := make([]interface{}, sliceLen)
		for i := 0; i < sliceLen; i++ {
			maskedSlice[i] = maskSensitive(v.Index(i))
		}
		return maskedSlice
	case reflect.Map:
		maskedMap := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprint(key.Interface())
			if sensitiveKeys[strings.ToLower(keyStr)] {
				maskedMap[keyStr] = "***"
			} else {
				maskedMap[keyStr] = maskSensitive(v.MapIndex(key))
			}
		}
		return maskedMap
	default:
		return v.Interface()
	}
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Func, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	}
	// For other types, use the default zero value check
	return v.Interface() == reflect.Zero(v.Type()).Interface()
}
