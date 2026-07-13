package config

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
	Backends           []Backend          `yaml:"backends,omitempty" json:"backends,omitempty"`
	Routes             []TCPRoute         `yaml:"routes,omitempty" json:"routes,omitempty"`
	Middlewares        []MiddlewareConfig `yaml:"middlewares,omitempty" json:"middlewares,omitempty"`
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
	KeyPassphrase string `yaml:"key_passphrase,omitempty" json:"key_passphrase,omitempty"`
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
	Name            string        `yaml:"name" json:"name"`
	URL             string        `yaml:"url" json:"url"`
	Timeout         string        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ConnectTimeout  string        `yaml:"connect_timeout,omitempty" json:"connect_timeout,omitempty"`
	HeadersTimeout  string        `yaml:"headers_timeout,omitempty" json:"headers_timeout,omitempty"`
	ResponseTimeout string        `yaml:"response_timeout,omitempty" json:"response_timeout,omitempty"`
	IdleTimeout     string        `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	MaxConnections  int64         `yaml:"max_connections,omitempty" json:"max_connections,omitempty"`
	Compression     bool          `yaml:"compression,omitempty" json:"compression,omitempty"`
	ForceTLS        bool          `yaml:"force_tls,omitempty" json:"force_tls,omitempty"`
	TLS             BackendTLS    `yaml:"tls,omitempty" json:"tls,omitempty"`
	Auth            *BackendAuth  `yaml:"auth,omitempty" json:"auth,omitempty"`
	Proxy           *BackendProxy `yaml:"proxy,omitempty" json:"proxy,omitempty"`
}

type BackendProxy struct {
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

type BackendAuth struct {
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Header   string `yaml:"header,omitempty" json:"header,omitempty"`
	Prefix   string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	// AWS Signature Version 4 (for S3 and other AWS services)
	AccessKeyID     string `yaml:"access_key_id,omitempty" json:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty" json:"secret_access_key,omitempty"`
	Region          string `yaml:"region,omitempty" json:"region,omitempty"`
	Service         string `yaml:"service,omitempty" json:"service,omitempty"` // e.g. "s3", "ec2"
}

type BackendTLS struct {
	// Ignore certificate validation
	Insecure bool `yaml:"insecure" json:"insecure"`
	// Custom corporate CA
	CAFile string `yaml:"ca_file,omitempty" json:"ca_file,omitempty"`
	// TLS SNI
	ServerName string `yaml:"server_name,omitempty" json:"server_name,omitempty"`
	CertFile   string `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile    string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
}
