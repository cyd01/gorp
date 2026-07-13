# Listeners

A **listener** defines how GORP receives connections. The `type` field determines the protocol.

| Type | Usage |
|---|---|
| `http` | Classic HTTP/1.1 |
| `https` | HTTPS |
| `dynamic` | HTTPS with dynamically generated certificate via SNI |
| `http3` | HTTP/3 over QUIC |
| `httpmulti` / `multi` | Plain HTTP + TLS on the same port |
| `tcp` | Raw TCP, with optional TLS/SNI |
| `unix` | HTTP over Unix socket |
| `unix_tcp` | TCP over Unix socket |
| `proxy` | HTTP proxy |
| `proxy_tls` | HTTP proxy under TLS |
| `socks5` | SOCKS5 proxy |

An unknown type is treated as `http`.

## HTTP

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
```

The listener uses global HTTP routes.

### HTTP CONNECT

An HTTP listener also accepts `CONNECT`. The client does not freely choose the destination: GORP uses the backend of the selected route and opens the tunnel to it.

## HTTPS

```yaml
listeners:
  - name: https
    type: https
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

See [TLS and mTLS](tls.md).

## HTTP/3

```yaml
listeners:
  - name: http3
    type: http3
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

The HTTP/3 listener uses QUIC. The port must therefore be accessible via UDP.

## Unix domain sockets

Use `unix` for HTTP over a Unix socket. The `address` is the socket path:

```yaml
listeners:
  - name: local-http
    type: unix
    address: "/run/gorp/http.sock"
```

The HTTP listener uses the same routes and middlewares as a TCP HTTP listener.
GORP removes its socket file during shutdown and cleans up a stale socket at
startup; it does not overwrite an active socket or a regular file. Filesystem
permissions control which local users can connect.

Use `unix_tcp` for a raw TCP stream over a Unix socket; its backend and
optional TLS/SNI routing work like a `tcp` listener. See
[TCP, TLS, and SNI](tcp.md).

## HTTP multi-protocol

`httpmulti` (or `multi`) allows sharing a port between plain HTTP and TLS. GORP looks at the start of the connection to determine if it begins with a TLS handshake.

```yaml
listeners:
  - name: multi
    type: httpmulti
    address: ":8080"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

This mode accepts HTTP/1.1, HTTP/2 in plain text and, when TLS is used, HTTP/1.1 and HTTP/2 negotiated via TLS. It can be useful when the same port must accept multiple forms of HTTP traffic.

## Dynamic HTTPS

The `dynamic` type uses a **CA** to generate a certificate on the fly corresponding to the requested SNI:

```yaml
listeners:
  - name: intercept
    type: dynamic
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/ca/ca.crt"
      key_file: "/etc/gorp/ca/ca.key"
      key_passphrase: "change-me"
```

Here `cert_file` and `key_file` designate the signing CA, not a classic server certificate. Clients must trust this CA.

A generated certificate is associated with the SNI name. The cache keeps up to 1024 certificates and each generated certificate is valid for 24 hours.

`ca_file` can additionally be used to enforce a client certificate. The `dynamic` path does not configure CRL/OCSP for these client certificates.

## HTTP limits and timeouts

HTTP listeners support per-listener timeouts, an active-request limit, and a
request-body size limit:

```yaml
listeners:
  - name: public
    type: https
    address: ":443"
    read_header_timeout: "5s"
    write_timeout: "60s"
    idle_timeout: "120s"
    max_connections: 500
    max_request_body_size: 10485760
```

- `read_header_timeout`, `write_timeout`, and `idle_timeout` configure the
  corresponding HTTP server timeouts.
- `max_connections` limits concurrently handled HTTP requests for this
  listener. A value of `0` disables this limit. For TCP listeners, it limits
  accepted TCP connections instead.
- `max_request_body_size` is in bytes. When `Content-Length` is known and
  exceeds the limit, GORP responds with `413 Request Entity Too Large`. A
  request without a known content length is not rejected by this check.
