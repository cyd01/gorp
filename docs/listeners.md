# Listeners

This section describes the listener types supported by the reverse proxy. A
listener defines how the proxy accepts incoming connections.

## HTTP and HTTPS Listeners

HTTP listeners also support HTTP/1.1 `CONNECT` requests. The route selects a
configured backend and opens a TCP tunnel to its host and port. This is a
reverse-proxy tunnel: the client cannot choose an arbitrary destination.

A listener definition includes a name, type, address, and optional TLS,
connection limits, or middleware settings.

### Listener connection limit

Use `max_connections` to cap the number of active connections accepted by a
listener. A value of `0` means there is no limit.

```yaml
listeners:
  - name: public
    type: http
    address: ":8080"
    max_connections: 200
```

This limit applies to the listener itself, regardless of how many backends are
configured behind it.

### Listener timeouts

For HTTP and HTTPS listeners, the following per-listener timeout fields are
supported:

- `read_header_timeout`
- `write_timeout`
- `idle_timeout`

If they are not provided, Go's default `http.Server` behavior is used: a zero
value means "no timeout". In practice, that means the effective default is:

- `read_header_timeout = 0` (disabled)
- `write_timeout = 0` (disabled)
- `idle_timeout = 0` (disabled)

Setting one of them to `0` is therefore equivalent to "disable the timeout" for
that listener phase.

```yaml
listeners:
  - name: public
    type: http
    address: ":8080"
    read_header_timeout: "5s"
    write_timeout: "30s"
    idle_timeout: "60s"
```

### Payload size limit

Use `max_request_body_size` to reject oversize request bodies before they are
forwarded upstream. The value is expressed in bytes, and a value of `0` disables
this protection.

```yaml
listeners:
  - name: public
    type: http
    address: ":8080"
    max_request_body_size: 10485760
```

When a client sends a request body larger than this limit, the listener responds
with HTTP `413 Request Entity Too Large`.

### Basic HTTP

A standard HTTP listener.

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
```

### Forward proxy

A `proxy` listener forwards HTTP requests sent with absolute URLs and supports
arbitrary HTTP `CONNECT` tunnels. It does not use the configured routes or
backends.

```yaml
listeners:
  - name: outbound-proxy
    type: proxy
    address: ":8080"
```

Configure clients with `http://127.0.0.1:8080` as their HTTP proxy. This is an
open proxy, so it should only be exposed to trusted networks or protected by
network access controls.

### HTTPS with TLS

An HTTPS listener with certificate configuration.

```yaml
listeners:
  - name: https
    type: https
    address: ":1443"
    tls:
      cert_file: server.crt
      key_file: server.key
```

See [TLS](tls.md) for complete TLS configuration.

### Dynamic HTTPS

The `dynamic` listener generates a short-lived certificate for each TLS **SNI** hostname and signs it with the configured **CA**. Clients must trust this **CA**.
The **CA** private key can be an encrypted legacy PEM key; set its passphrase with `key_passphrase`.

```yaml
listeners:
  - name: dynamic-https
    type: dynamic
    address: ":1445"
    tls:
      cert_file: ca.crt
      key_file: ca.key
      key_passphrase: "change-me"
```

SNI is required. The certificate is cached per hostname and expires after 24 hours. Only the 1024 last generated certificates are cached. `cert_file` and `key_file` are the signing CA certificate and private key, not an end-entity server certificate and key.

### HTTP/3

An HTTP/3 listener.

```yaml
listeners:
  - name: http3
    type: http3
    address: ":8443"
    tls:
      cert_file: server.crt
      key_file: server.key
```

An HTTP/3 listener must have a [TLS](tls.md) configuration.

## TCP Listeners

Plain TCP listeners do not use route definitions and select from backends
defined directly in the listener configuration. TLS-enabled TCP listeners can
add SNI-based routes.

TCP listeners are intended for non-HTTP traffic.

### Basic TCP

A standard TCP listener.

```yaml
listeners:
  - name: tcp
    type: tcp
    address: ":9080"
    backends:
      - name: simple_backend1
        url: "tcp://127.0.0.1:8001"
```

### TCP with downstream TLS

A TCP listener wrapped in TLS. When TLS is enabled, connections can be routed
by the SNI hostname requested by the client.

```yaml
listeners:
  - name: tcp_secure
    type: tcp
    address: ":9443"
    tls:
      cert_file: server.crt
      key_file: server.key
    backends:
      - name: simple_backend1
        url: "tcp://127.0.0.1:8001"
```

For more information about downstream TLS, see [TLS](tls.md).

#### Route TLS connections by SNI

Add `routes` to a TLS-enabled TCP listener. Each route contains one or more
host patterns and its own backend pool. Patterns support the same wildcards as
HTTP host routes. The listener-level `backends` remain the fallback pool when
the SNI hostname does not match a route.

```yaml
listeners:
  - name: tcp_secure
    type: tcp
    address: ":9443"
    tls:
      cert_file: server.crt
      key_file: server.key
    routes:
      - name: database
        hosts:
          - "db.example.com"
          - "*.db.example.com"
        tls:
          cert_file: example.crt
          key_file: example.key
        backends:
          - name: database_backend
            url: "tcp://127.0.0.1:5432"
      - name: cache
        hosts:
          - "cache.example.com"
        backends:
          - name: cache_backend
            url: "tcp://127.0.0.1:6379"
    backends:
      - name: default_backend
        url: "tcp://127.0.0.1:9000"
```

SNI routing is available only for TLS-enabled TCP listeners. SSL key and certificate are defined at listener level, but can be eventually overwritten at route level.  
Plain TCP listeners continue to select a backend from their listener-level pool.

### TCP with upstream TLS

```yaml
listeners:
  - name: tcp_secure
    type: tcp
    address: ":9080"
    backends:
      - name: simple_backend1
        url: "tcp://127.0.0.1:8001"
        force_tls: true
```

For more information about upstream TLS, see [TLS](tls.md).

## Admin Listener

The [`admin`](admin.md) listener can also be enabled.

```yaml
admin:
  enabled: true
  address: ":9090"
```

The admin listener supports [TLS](tls.md) configuration like the other
listeners. It also supports request middlewares for authentication and
access control.

---
