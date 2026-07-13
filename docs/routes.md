# Routes

This section describes how to define routes that map incoming requests to
specific backends or actions.

## Route Configuration

### HTTP/1.1 CONNECT

Routes can handle HTTP/1.1 `CONNECT` requests. The selected backend must have
a URL with a host and port, for example `tcp://127.0.0.1:5432`. After the
proxy returns `200 Connection Established`, the request and response bodies
are relayed as an opaque bidirectional TCP stream. HTTP response middlewares
do not apply to data inside the tunnel.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
routes:
  - name: database
    prefix: "/"
    middlewares:
      - name: basic_auth
        config:
          users:
            admin: change-me
    backends:
      - name: database_backend
        url: "tcp://127.0.0.1:5432"
```

A route definition includes a name, a prefix or a list of hosts, and optional
settings for `strip_prefix`, `preserve_host`, `selector`, `middlewares`, and
`backends`.

Routes may either declare their backends inline or reference a reusable service
by `service` name. The service model is useful when several routes share the
same backend pool.

For more information, see [Backends](backends.md).

By default, routes apply to every listener. To restrict a route to specific
listeners, add their listener names to `endpoints`:

```yaml
routes:
  - name: secure-api
    prefix: /api
    endpoints:
      - https
      - https-admin
    backends:
      - name: api
        url: http://127.0.0.1:8001
```

The endpoint values are listener `name` values. A route with an `endpoints`
list is ignored by all other listeners; omitting the field keeps the global
behavior.

### Basic Route with Prefix

A simple mapping of a path to a backend.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

### Reusable Service

When several routes point to the same backend pool, define the backend list once
under `services` and reference it by name from the route.

```yaml
services:
  - name: frontend
    backends:
      - name: app1
        url: "http://localhost:8001"
      - name: app2
        url: "http://localhost:8002"

routes:
  - name: main
    prefix: "/"
    service: frontend
```

This is equivalent to declaring the same backend list directly on each route. The
legacy inline `backends` format remains fully supported for backward compatibility.

### Strip Prefix

```yaml
routes:
  - name: simple
    prefix: "/private"
    strip_prefix: true
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

Paths such as `/private/xxx` will be transformed into `/xxx`.

### Route with Hosts

```yaml
routes:
  - name: simple
    hosts:
      - "www.example.com"
      - "*.api.example.com"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

Hostnames support wildcards such as `*`, `?`, and `[0-9]`.

### Preserve Host

By default, the `Host` header in the upstream request is set to the host of
the selected backend. To preserve the original `Host` from the client,
set `preserve_host` to `true`.

```yaml
routes:
  - name: simple
    prefix: "/"
    preserve_host: true
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
```

## Advanced Routing Features

### Selector Integration

To apply a selection algorithm to a route, see the [selectors](selectors.md) page.

### Middleware Integration

To apply middleware to a route, see the [middlewares](middlewares.md) page.

---
