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
