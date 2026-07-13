# Middlewares

Middlewares allow you to modify a request, control access, or modify the response.

## Where to place them?

### Listener

```yaml
listeners:
  - name: https
    type: https
    address: ":443"
    middlewares:
      - name: logging
```

The middleware is applied to all routes of this listener.

### Route

```yaml
routes:
  - name: admin
    prefix: "/admin"
    middlewares:
      - name: basic_auth
        config:
          users:
            admin: change-me
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Response middlewares are configured at the route level.

## Basic Authentication

```yaml
- name: basic_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
      bob: "password"
```

The client must provide `Authorization: Basic ...`. In case of failure, GORP returns `401` and `WWW-Authenticate`.

## Digest Authentication

```yaml
- name: digest_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
    nonce_timeout: "5m"
```

The mechanism uses Digest with `MD5` and `qop="auth"`. The nonce expires according to `nonce_timeout` (5 minutes by default).

## Token Authentication

```yaml
- name: token_auth
  config:
    realm: "API"
    tokens:
      - "token-one"
      - "token-two"
    header: "Authorization"
    prefix: "Bearer "
```

The header and prefix can be customized:

```yaml
header: "X-API-Key"
prefix: ""
```

## OpenID Connect / JWT

```yaml
- name: auth_openid
  config:
    issuer: "https://issuer.example.com"
    audience: "api://default"
    algorithm: "RS256"
    header: "Authorization"
    prefix: "Bearer "
    remove_header: false
    keys:
      - |
        -----BEGIN PUBLIC KEY-----
        ...
        -----END PUBLIC KEY-----
```

The middleware verifies the JWT structure, algorithm, signature, `exp`, `nbf`, `iss` and `aud` according to the provided parameters.

If `keys` is absent, GORP attempts to retrieve the public keys via the configured provider.

## IP Whitelist

```yaml
- name: ip_whitelist
  config:
    sources:
      - "10.0.0.0/8"
      - "192.168.10.0/24"
```

An address that does not match any CIDR receives `403 Forbidden`.

The middleware looks at `RemoteAddr`, not `X-Forwarded-For`.

## Rate limiting

```yaml
- name: rate_limit
  config:
    max_requests: 100
    window: "1m"
```

The counter is local to the process and indexed by source IP. Once the quota is exceeded, the response is `429 Too Many Requests`.

To limit only certain paths:

```yaml
- name: rate_limit
  config:
    max_requests: 20
    window: "1m"
    extensions:
      - "/api/*"
      - "*.json"
```

The name `extensions` is historical: it actually refers to wildcard path patterns.

Default values: `100` requests per `60s`.

## Correlation

```yaml
- name: correlation_id
  config:
    type: uuidV7
```

Accepted types: `uuid`, `uuidV4`, `uuidV6`, `uuidV7`.

The middleware adds `X-Correlation-Id` if the client has not provided it and also returns it in the response.

To ensure the logger uses this identifier, place `correlation_id` before `logging`.

## Logging

```yaml
- name: logging
```

Aliases: `log`, `logger`.

The logger writes the method, URI, remote address, any correlation ID, and the duration.

## Distributed tracing

```yaml
- name: distributed_tracing
```

Aliases: `tracing`, `trace_context`.

The middleware reads and propagates the W3C `traceparent` header. It creates a new Span ID and transmits a new `traceparent` to the backend.

It does not transmit spans to a collector: it only ensures the creation and propagation of the context.

## Request Headers

### Add

```yaml
- name: add_request_headers
  config:
    headers:
      X-Source: gorp
```

### Replace

```yaml
- name: set_request_headers
  config:
    headers:
      X-Source: gorp
```

### Modify

```yaml
- name: modify_request_headers
  config:
    headers:
      User-Agent: gorp
```

### Remove

```yaml
- name: remove_request_headers
  config:
    headers:
      - Accept
      - X-Debug
```

Header values must be strings.

## CORS

```yaml
- name: cors
```

The middleware adds CORS headers and responds directly with `200` to `OPTIONS` requests.

The current behavior allows `*` for `Access-Control-Allow-Origin` and enables `Access-Control-Allow-Credentials`.

## Path Prefixing

```yaml
- name: add_prefix
  config:
    prefix: "/internal"
```

The prefix is added to `r.URL.Path` before the rest of the chain.

## Artificial Delay

```yaml
- name: delay
  config:
    duration: "250ms"
```

Useful for testing and latency simulation.

## Secure access

```yaml
- name: secure_access
  config:
    path: "/unlock"
    key: "long-secret"
    redirect: "/"
    duration: "15m"
```

A request to `/unlock` obtains a signed cookie, then subsequent requests using this cookie can pass.

The cookie is tied to the User-Agent. Without a valid cookie, the middleware returns `410 Gone`.

## OpenAPI

```yaml
- name: openapi
  config:
    file: "/etc/gorp/openapi.yaml"
```

The contract is loaded during configuration construction. An invalid contract causes loading to fail.

Requests are checked according to path, method, parameters, and body. A non-existent operation can produce `404` or `405`, while a non-compliant request can produce `400`.

The file can also be referenced via HTTP(S) or `file://`.

## Dynamic Go Middleware

### Request

```yaml
- name: dynamic_request
  config:
    code: |
      package dynamic

      import "net/http"

      func Middleware(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              r.Header.Set("X-From-Dynamic", "true")
              next.ServeHTTP(w, r)
          })
      }
```

### Response

```yaml
- name: dynamic_response
  config:
    code: |
      package dynamic

      import "net/http"

      func Middleware(response *http.Response) error {
          response.Header.Set("X-From-Dynamic", "true")
          return nil
      }
```

The code is interpreted with Yaegi at configuration load time. A syntax, compilation, or signature error causes this configuration to fail.

> A dynamic middleware executes code provided by the configuration. Therefore, treat the configuration file as privileged code.

## Response Middlewares

### Add / Set / Modify

```yaml
- name: add_response_headers
  config:
    headers:
      X-Backend: application
```

Variants are:

- `add_response_headers`;
- `set_response_headers`;
- `modify_response_headers`;
- `remove_response_headers`.

### Location Prefixing

```yaml
- name: add_location_prefix
  config:
    prefix: "/private"
```

### Replace Location Host

```yaml
- name: set_location_host
  config:
    host: "public.example.com"
```

Relative URLs are not modified by `set_location_host`.
