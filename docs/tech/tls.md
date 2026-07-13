# TLS Configuration

This section describes Transport Layer Security (TLS) configuration for
incoming connections (downstream) and outgoing backend requests (upstream).

## Downstream TLS

Configuration for securing the connection between the client and the proxy.

### Standard downstream TLS

Basic certificate and key configuration.

```yaml
listeners:
  - name: https
    type: https
    address: ":1443"
    tls:
      cert_file: server.crt
      key_file: server.key
```

### Minimum TLS version

All listeners that accept downstream TLS support `min_version`. Supported
values are `TLS1.0`, `TLS1.1`, `TLS1.2`, and `TLS1.3`. If omitted, the
listener keeps its existing default.

```yaml
listeners:
  - name: https
    type: https
    address: ":1443"
    tls:
      cert_file: server.crt
      key_file: server.key
      min_version: TLS1.2
```

This setting applies to HTTPS, dynamic HTTPS, TLS forward proxy, HTTP/3,
HTTP multi, TCP TLS, and the admin TLS listener. For a TCP route with its own
certificate, it can also be set in that route's `tls` block.

When client certificate authentication is enabled with `ca_file`, the HTTPS,
HTTP/3, and dynamic HTTPS listeners add the verified client certificate to the
upstream request as `X-Forwarded-Client-Cert`. The value is a URL-escaped PEM
certificate. Any client-supplied value is replaced.

### mTLS Configuration

To authenticate clients using mutual TLS (mTLS), provide the certificate file
of the certificate authority that signs the client certificates.

```yaml
listeners:
  - name: https
    type: https
    address: ":1443"
    tls:
      cert_file: server.crt
      key_file: server.key
      ca_file: ca.crt
```

### mTLS Configuration with revocation list or OCSP

To revoke client certificates, provide either a certificate revocation list (CRL) or an OCSP responder URL.
If both are configured, the CRL takes precedence.

```yaml
listeners:
  - name: https
    type: https
    address: ":1443"
    tls:
      cert_file: server.crt
      key_file: server.key
      ca_file: ca.crt
      crl_file: ca.crl
      ocsp_url: https://ocsp.example.com
```

## Upstream TLS

Configuration for securing the connection between the proxy and the backend servers.

### Standard upstream TLS

Basic certificate and key configuration.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "https://localhost:8001"
        force_tls: true
```

### Accept insecure connections

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "https://localhost:8001"
        tls:
          insecure: true
```

### Use a private CA certificate

If the backend uses a private certificate authority, provide the CA
certificate:

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "https://localhost:8001"
        tls:
          ca_file: ca.crt
```

### Add a Server Name (SNI)

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "https://localhost:8001"
        tls:
          server_name: localhost
```

### mTLS configuration

Client authentication (mTLS) with a TLS backend:

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "https://localhost:8001"
        tls:
          cert_file: client.crt
          key_file: client.key
```

---
