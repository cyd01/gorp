# GORP

`gorp` is a lightweight reverse proxy for HTTP and TCP, written in [Go](https://go.dev/).

## Features

- [Listeners](listeners.md)
  - [x] **HTTP**
  - [x] **HTTPS**
  - [x] **HTTPS** with dynamic server certificate generation
  - [x] **HTTP/3**
  - [x] **TCP**
  - [x] **TCP** with **TLS**
  - [x] **HTTP proxy**
- [Middlewares](middlewares.md)
  - [x] request and response headers
  - [x] response `Location` prefixing for `strip_prefix`
  - [x] response `Location` host replacement
  - [x] correlation IDs (**UUIDs**)
  - [x] logging and rate limiting
  - [x] Basic, token, digest, and OpenID Connect authentication
  - [x] dynamic Go request and response middleware
- [HTTP routing](routes.md), based on the request path or `Host` header
- SNI-based routing for TLS-enabled TCP listeners
- [Backend selectors](selectors.md)
  - [x] round robin, random, and first available
  - [x] cookie, header, source IP, and query parameter affinity
  - [x] least connections and power of two choices
- [Backend authentication](backends.md)
  - [x] Basic, token, and **AWS** authentication
- [TLS](tls.md)
  - [x] **TLS** for upstream
  - [x] **TLS** for downstream
  - [x] private certificate authorities for downstream connections
  - [x] downstream client authentication (**mTLS**)
  - [x] upstream client authentication (**mTLS**)
  - [x] certificate revocation lists (**CRLs**)
  - [x] **TLS** for **TCP** endpoints
- [Directory fallback](directory.md)
- [Admin listener](admin.md)
- [Outbound proxy](proxy.md)
- [x] Prometheus metrics for listeners and backends on the admin listener
- [x] Hot configuration reload

## Usage

```shell
$ gorp -h
Usage of ./gorp:
  -config string
        configuration file
```

The configuration source is read from the `-config` command-line option or from one of these environment variables:

- `CONFIG`
- `GORP_CONFIG`
- `PROXY_CONFIG`

The configuration source can be:

- an HTTP or HTTPS URL
- a local file
- an environment variable name
- the configuration content itself

The configuration format can be either YAML or JSON. All examples in this
documentation use YAML.

The top-level configuration structure is:

```yaml
admin: ...
directory: ...
listeners: ...
services: ...
routes: ...
```

When the configuration source is a local file, it is reloaded automatically
when the file changes, and the servers are restarted.

---
