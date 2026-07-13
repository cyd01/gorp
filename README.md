# GORP

`gorp`, a new lightweight reverse proxy for HTTP and TCP, written in [Go](https://go.dev/).

Here is the [user guide](docs/en/README.md).

## Features

- [Listeners](docs/en/listeners.md)
  - [x] **HTTP**
  - [x] **HTTPS**
  - [x] **HTTPS** with dynamic server certificate generation
  - [x] **HTTP/3**
  - [x] **TCP**
  - [x] **TCP** with **TLS**
  - [x] **HTTP proxy** (with cascade)
  - [x] **Socks5 proxy**
- [Middlewares](docs/en/middlewares.md)
  - [x] request and response headers
  - [x] response `Location` prefixing for `strip_prefix`
  - [x] response `Location` host replacement
  - [x] correlation IDs (**UUIDs**)
  - [x] logging and rate limiting
  - [x] Basic, token, digest, and OpenID Connect authentication
  - [x] dynamic Go request and response middleware
  - [x] OpenID Connect validation
  - [x] OpenAPI validation
  - [x] W3C distributed tracing
- [HTTP routing](docs/en/routing.md), based on the request path or `Host` header
- SNI-based routing for TLS-enabled TCP listeners
- [Backend selectors](docs/en/selectors.md)
  - [x] round robin, random, and first available
  - [x] cookie, header, source IP, and query parameter affinity
  - [x] least connections and power of two choices
- [Backend authentication](docs/en/backends.md)
  - [x] Basic, token, and **AWS** authentication
- [TLS](docs/en/tls.md)
  - [x] **TLS** for upstream
  - [x] **TLS** for downstream
  - [x] private certificate authorities for downstream connections
  - [x] downstream client authentication (**mTLS**)
  - [x] upstream client authentication (**mTLS**)
  - [x] certificate revocation lists (**CRLs**)
  - [x] **OCSP** (Online Certificate Status Protocol)
  - [x] **TLS** for **TCP** endpoints
- [Directory fallback](docs/tech/directory.md)
- [Admin listener](docs/en/admin.md)
- [Outbound proxy](docs/en/proxies.md)
- [x] Prometheus metrics for listeners and backends on the admin listener
- [x] Hot configuration [reload](docs/en/reload.md)

---
