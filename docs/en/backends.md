# Backends and Services

A backend is a destination. A route can define its backends inline or refer to
a reusable service.

## Basic backend

```yaml
routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## Reuse a backend pool

```yaml
services:
  - name: api
    backends:
      - name: api-1
        url: "http://10.0.0.21:8080"
      - name: api-2
        url: "http://10.0.0.22:8080"

routes:
  - name: users
    prefix: "/users"
    service: api

  - name: orders
    prefix: "/orders"
    service: api
```

If a route has both `backends` and `service`, its inline `backends` take
precedence. See [backend selection](selectors.md) to choose among multiple
destinations.

## Connection limits and timeouts

Set `max_connections` to limit concurrent work assigned to one backend. New
requests are rejected with `503 Service Unavailable` when the limit is reached.

```yaml
- name: app
  url: "http://127.0.0.1:8000"
  max_connections: 50
```

Backend timeout fields accept Go duration strings:

| Field | Default behavior |
|---|---|
| `timeout` | 30 s |
| `connect_timeout` | `timeout`, otherwise 30 s |
| `headers_timeout` | `timeout`, otherwise 30 s |
| `response_timeout` | Declared configuration field; no global response deadline is currently applied |
| `idle_timeout` | 90 s |

```yaml
- name: slow-api
  url: "http://10.0.0.20:8000"
  timeout: "20s"
  connect_timeout: "2s"
  headers_timeout: "5s"
  idle_timeout: "60s"
```

`timeout: 0` does not disable timeouts; unset or zero values fall back to the
configured `timeout` or the defaults above. The current HTTP transport applies
connection, response-header, and idle-connection timeouts. It does not apply a
global response deadline from `response_timeout`.

## Compression

```yaml
- name: app
  url: "http://127.0.0.1:8000"
  compression: true
```

This enables gzip negotiation with the upstream HTTP server. It is a boolean
switch, not a configurable list of compression algorithms.

## Upstream TLS

For a backend using a private CA, configure `ca_file` and, if needed,
`server_name`:

```yaml
- name: secure-app
  url: "https://app.internal.example:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    server_name: "app.internal.example"
```

To present a client certificate for upstream mTLS:

```yaml
- name: mtls-api
  url: "https://api.internal.example:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    cert_file: "/etc/gorp/tls/client.crt"
    key_file: "/etc/gorp/tls/client.key"
```

`insecure: true` disables certificate verification. Avoid it in production:

```yaml
tls:
  insecure: true
```

See [TLS and mTLS](tls.md) for the full TLS configuration.

## Upstream authentication

### Basic

```yaml
- name: app
  url: "https://api.example.com"
  auth:
    type: basic
    username: apiuser
    password: "change-me"
```

### Token

```yaml
- name: api
  url: "https://api.example.com"
  auth:
    type: token
    token: "<token>"
```

By default, GORP sends the token in the `Authorization` header using the
Bearer scheme. To use a custom header:

```yaml
auth:
  type: token
  token: "<token>"
  header: "X-API-Key"
  prefix: ""
```

### AWS Signature Version 4

```yaml
- name: s3
  url: "https://bucket.s3.eu-west-3.amazonaws.com"
  auth:
    type: aws
    access_key_id: "<access-key-id>"
    secret_access_key: "<secret-access-key>"
    region: "eu-west-3"
    service: "s3"
```

GORP signs each request. The same mechanism can be used with other AWS
services that support Signature Version 4.

## Outbound proxy

A backend can send its upstream requests through an HTTP, HTTPS, or SOCKS5
proxy:

```yaml
- name: external
  url: "https://www.example.com"
  proxy:
    url: "http://proxy.internal:3128"
    username: proxyuser
    password: "change-me"
```

## Circuit breaker

The built-in HTTP backend circuit breaker opens after five failures for about
30 seconds, then permits a recovery attempt. Backend `5xx` responses also count
as failures. Selectors that check backend availability skip unavailable
backends. The threshold and duration are not configurable.

## TCP backends

TCP listeners use `tcp://` URLs:

```yaml
listeners:
  - name: database
    type: tcp
    address: ":5432"
    backends:
      - name: database
        url: "tcp://10.0.0.20:5432"
        force_tls: true
        tls:
          ca_file: "/etc/gorp/ca/db-ca.crt"
          server_name: "db.internal"
```

`force_tls: true` secures the connection from GORP to the TCP backend. TCP
backends are selected randomly from the listener or matching SNI route pool.
