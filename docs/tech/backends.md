# Backends

This section describes how to define the backend servers to which the proxy
forwards traffic.

## Backend Configuration

A backend definition includes a name, a URL, and optional proxy,
authentication, TLS, and connection limit settings.

### Backend connection limit

Use `max_connections` to cap the number of active requests/connections using a
backend at the same time. A value of `0` means there is no limit.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        max_connections: 50
```

When a backend reaches its configured cap, new in-flight work is rejected with a
`503 Service Unavailable` response.

### Backend timeouts

The backend supports several timeout fields:

- `timeout` (legacy global fallback)
- `connect_timeout`
- `headers_timeout`
- `response_timeout`
- `idle_timeout`

Default behavior when a timeout is not configured is:

- `timeout = 30s`
- `connect_timeout = timeout` or `30s`
- `headers_timeout = timeout` or `30s`
- `response_timeout = timeout` or `30s`
- `idle_timeout = 90s`

Important: for backend timeout fields, a value of `0` is not treated as
"disable timeout". In the current implementation, `0` falls back to the legacy
`timeout` value, and if that is also unset, to the built-in defaults above.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        timeout: "20s"
        connect_timeout: "2s"
        headers_timeout: "4s"
        response_timeout: "15s"
        idle_timeout: "45s"
```

### Basic Backend

A simple backend configuration is:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

### Backend compression

The Go HTTP transport only supports gzip for negotiated backend compression.
Set `compression: true` to allow the proxy to advertise gzip support when
calling the upstream service.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        compression: true
```

This is a boolean switch only; the proxy does not implement a custom
compression algorithm list. If the upstream server does not advertise gzip or
is not compatible with it, the request behaves normally without any special
fallback.

### Backend with TLS

For backends that require a secure connection, see [TLS](tls.md).

### Backend with timeout

You can define a specific connection timeout for a backend. The default is
`30s`.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        timeout: 5s
```

### Backend with authentication

#### Basic Authentication

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        auth:
          type: basic
          username: username
          password: password
```

#### Token Authentication

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        auth:
          type: token
          token: eyJhbGciOiJIUzI1...
```

#### AWS Authentication

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: s3-backend
        url: "https://bucket.s3.region.amazonaws.com"
        auth:
          type: "aws"                    # AWS authentication
          access_key_id: "AKIA..."       # AWS Access Key ID
          secret_access_key: "wJal..."   # AWS Secret Access Key
          region: "us-east-1"            # AWS Region
          service: "s3"                  # AWS Service (s3, ec2, etc.)
```

## Outbound Proxy Configuration

Optional proxy settings for outbound requests.

### Simple Proxy Configuration

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        proxy:
          url: "http://127.0.0.1:3128"
```

### Proxy with Authentication

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
        proxy:
          url: "http://127.0.0.1:3128"
          username: username
          password: password
```

---
