# Middlewares

This section describes the middlewares that can modify requests or responses.

## Middleware Configuration

A middleware definition includes a name and an optional configuration block.
Request middlewares can be applied at the **listener** or **route** level.
Response middlewares can be applied at the **route** level only.

## Request Middlewares

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
```

> The correlation ID can be used by a following `logging` middleware, so it
> must be defined **before** it.

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

## Response Middlewares

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
seconds. Additional requests receive HTTP status `429`.

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
