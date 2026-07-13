# Middlewares

This section describes the middlewares that can modify requests or responses.

## Middleware Configuration

A middleware definition includes a name and an optional configuration block.
Request middlewares can be applied at the **listener** or **route** level.
Response middlewares can be applied at the **route** level only.

## Request Middlewares

### OpenAPI request validation

The `openapi` middleware validates the request path, method, parameters, and
request body against an OpenAPI contract before forwarding it. The contract is
loaded once when the configuration is built and supports OpenAPI 3.1 through
the underlying validator.

```yaml
routes:
  - name: api
    prefix: "/api"
    middlewares:
      - name: openapi
        config:
          file: ./openapi.yaml
    backends:
      - name: api
        url: "http://127.0.0.1:8001"
```

`file` may also be an HTTP(S) or `file://` URL. Invalid contracts fail during
startup. Requests that do not match an operation return `404` or `405`, while
requests with invalid parameters or bodies return `400`.

### Authentication

#### Basic Authentication

To protect an entire listener with Basic Authentication:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: basic_auth
        config:
          realm: "Restricted"
          users:
            alice: secret
            bob: password
```

To protect a specific path, apply it at the route level:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: public
    prefix: "/public"
    backends:
      - name: public_backend
        url: "http://localhost:8001"
  - name: private
    prefix: "/private"
    middlewares:
      - name: basic_auth
        config:
          realm: "Restricted"
          users:
            alice: secret
            bob: password
    backends:
      - name: private_backend
        url: "http://localhost:8001"
```

In this example, the backend is the same for the two routes, but `/public` is unprotected, whereas `/private` is protected with basic authentication.

#### OpenID Connect

To validate an OpenID token against an OpenID provider, use:

```yaml
middlewares:
  - name: auth_openid
    config:
      issuer: "https://issuer.example.com"
      audience: "api://default"
      algorithm: "RS256"
      header: "Authorization"
      prefix: "Bearer"
      keys:
        - |
          -----BEGIN PUBLIC KEY-----
          ...
          -----END PUBLIC KEY-----
        - |
          -----BEGIN PUBLIC KEY-----
          ...
          -----END PUBLIC KEY-----
```

If no keys are provided, the middleware attempts to retrieve them from the
provider's discovery endpoint.

This middleware is a JWT gate, not just a bearer-token presence check. Before
allowing the request through, it verifies all of the following:

- the request contains the configured header and prefix (`Authorization` +
  `Bearer ` by default);
- the token has exactly three JWT segments (`header.payload.signature`);
- the JWT header is valid JSON and the `alg` claim matches the configured
  algorithm (default `RS256`, case-insensitive);
- the signature verifies against at least one configured public key (or keys
  fetched from the provider's JWKS/discovery document);
- if `exp` is present, the token is not expired (`now > exp` is rejected);
- if `nbf` is present, the token is not yet valid (`now < nbf` is rejected);
- if `issuer` is configured, the `iss` claim must match it exactly;
- if `audience` is configured, the token `aud` claim must contain that value
  (single string or array of strings are both accepted);

If any check fails, the middleware replies with `401 Unauthorized` and sets a
`WWW-Authenticate: Bearer realm="OpenID", error="invalid_token"` header.

### Logging

#### Correlation ID

Adds a correlation ID to the `X-Correlation-Id` request header if one is not
already present. The value is a UUID.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: correlation_id
        config:
          type: uuid
```

Accepted types are `uuid`, `uuidV4`, `uuidV6`, `uuidV7`. Default is `uuid` (same as `uuidV4`).

> The correlation ID can be used by a following `logging` middleware, so it
> must be defined **before** it.

#### Distributed tracing

The `distributed_tracing` middleware propagates the W3C Trace Context through
the `traceparent` header and creates a new span for the proxy request:

```yaml
listeners:
  - name: https
    type: https
    address: ":8443"
    middlewares:
      - name: distributed_tracing
```

When a valid `traceparent` is received, its `TraceID` is preserved and its
incoming `SpanID` becomes the `ParentSpanID` of the proxy span. The proxy then
generates a new `SpanID` and forwards a new `traceparent` to the backend. When
no valid context is received, a new `TraceID` and `SpanID` are generated.

The middleware preserves the W3C trace flags. It does not export spans to a
tracing collector; it only creates and propagates the request context.

#### Logger

To enable request logging:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: correlation_id
      - name: logging
```

### Request headers

#### Set, Add, Modify, or Remove a Request Header

- Set the endpoint name:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: set_request_headers
        config:
          headers:
            X-Endpoint: http
```

> Header values must be strings for request header middlewares (`add`, `set`,
> and `modify`). Quote numeric values, for example, `"123"`.

- Add languages to the `Accept-Language` header. The middleware can be
  repeated:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: add_request_headers
        config:
          headers:
            Accept-Language: en-GB
      - name: add_request_headers
        config:
          headers:
            Accept-Language: en-US
```

- Modify the `User-Agent` header:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: modify_request_headers
        config:
          headers:
            User-Agent: gorp-0.9
```

- Remove the `Accept` header:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: remove_request_headers
        config:
          headers:
            - Accept
```

### Dynamic Request Middleware

The `dynamic_request` middleware evaluates Go source code with Yaegi. The
source must use package `dynamic` and define a `Middleware` function with the
signature `func(http.Handler) http.Handler`.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
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

Dynamic middleware is evaluated when the configuration is loaded. A syntax,
compilation, or signature error rejects the configuration.

## Response Middlewares

### Responses headers

#### Set, Add, Modify, or Remove a Response Header

- Add an informational header:

```yaml
routes:
  - name: simple
    prefix: "/"
    middlewares:
      - name: add_response_headers
        config:
          headers:
            X-From: my-backends
```

> Header values must be strings for all response header middlewares (`add`,
> `set`, and `modify`). Quote numeric values, for example, `"123"`.

- Modify the cache expiration:

```yaml
routes:
  - name: simple
    prefix: "/"
    middlewares:
      - name: modify_response_headers
        config:
          headers:
            Expires: "3600"
```

- Remove the server name:

```yaml
routes:
  - name: simple
    prefix: "/"
    middlewares:
      - name: remove_response_headers
        config:
          headers:
            - Server
```

#### Add a prefix to Location

The `add_location_prefix` middleware prefixes the path in a `Location`
response header. It supports both paths and absolute HTTP(S) URIs.
Use the same prefix as the route's `prefix` when the backend redirects to a
path that was hidden by `strip_prefix`.

```yaml
routes:
  - name: simple
    prefix: "/private"
    middlewares:
      - name: add_location_prefix
        config:
          prefix: "/private"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

#### Set the host in Location

The `set_location_host` middleware replaces the host in an absolute HTTP(S)
`Location` header. Relative paths and other URI schemes are left unchanged.

```yaml
routes:
  - name: simple
    prefix: "/private"
    middlewares:
      - name: set_location_host
        config:
          host: "public.example.com"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

### Dynamic Response Middleware

The `dynamic_response` middleware evaluates Go source code with Yaegi. The
source must use package `dynamic` and define a `Middleware` function with the
signature `func(*http.Response) error`.

```yaml
routes:
  - name: simple
    prefix: "/"
    middlewares:
      - name: dynamic_response
        config:
          code: |
            package dynamic

            import "net/http"

            func Middleware(response *http.Response) error {
                response.Header.Set("X-From-Dynamic", "true")
                return nil
            }
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

Dynamic response middleware runs before the response is sent to the client.

> Dynamic middleware executes code from the configuration with the privileges
> of the GORP process. Only load configuration from trusted sources.

## HTTP Protocol Middlewares

### Add Prefix

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: add_prefix
        config:
          prefix: "/start"
```

### CORS

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: cors
```

## Other Middlewares

### Delay

Adds a two-second delay before sending the response.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: delay
        config:
          duration: 2000ms
```

### Rate Limiter

Adds a rate limiter configured for a maximum of five requests every five
seconds. Additional requests receive HTTP status `429 Too Many Requests`.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: rate_limiter
        config:
          max_requests: 5
          window: 5s
```

### Allow IP Addresses by CIDR

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: ip_white_list
        config:
          sources:
            - 10.0.0.0/8
            - 172.16.0.0/12
            - 192.168.0.0/16
```

### Secure Access

`secure_access` is a very simple security middleware. All accesses are denied unless a previous call the `path` url is done.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
    middlewares:
      - name: secure_access
        config:
          path: /u
          key: secret-key
          redirect: /
          duration: 15m
```

---
