# HTTP and SOCKS5 Proxy

GORP can also function as an **outgoing proxy** in addition to its role as a reverse proxy.

## HTTP Proxy

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
```

For an HTTP request, the client must send an absolute URL:

```bash
curl -x http://127.0.0.1:3128 http://example.com/
```

For HTTPS, the client generally uses `CONNECT`:

```bash
curl -x http://127.0.0.1:3128 https://example.com/
```

The proxy then creates a TCP tunnel to the requested destination.

## Authenticating the Proxy

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    users:
      alice: "secret"
      bob: "password"
```

Users can also be written as a YAML list:

```yaml
users:
  - alice: "secret"
  - bob: "password"
```

The client must use `Proxy-Authorization: Basic ...`.

## Logging the Proxy

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    log: true
```

## HTTP Proxy under TLS

```yaml
listeners:
  - name: secure-proxy
    type: proxy_tls
    address: ":3129"
    tls:
      cert_file: proxy.crt
      key_file: proxy.key
    users:
      alice: "secret"
```

`proxy_tls` encrypts the transport between the client and the proxy. The proxy itself does not terminate client authentication via certificate in this mode: the TLS configuration of this listener uses the server certificate and key.

## Cascading Proxy

An HTTP proxy can forward its connections to another proxy:

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    proxy:
      url: "http://proxy-upstream.internal:8080"
      username: proxyuser
      password: change-me
      hosts:
        - "*.example.com"
        - "*.internal"
      no_proxy:
        - "direct.example.com"
```

The upstream proxy can use `http`, `https`, or `socks5`.

`hosts` and `no_proxy` are glob-pattern lists matched against the destination
hostname (without its port), case-insensitively. `hosts` selects destinations
to send through the upstream proxy; `no_proxy` takes precedence and sends
matching hostnames directly to their destination. If `hosts` is absent or
empty, all destinations use the upstream proxy except those in `no_proxy`. The
`*`, `?`, and character classes such as `[a-z]` are supported. The same rules
apply to HTTP requests and `CONNECT` tunnels, and are also available for
`proxy_tls`.

## SOCKS5

```yaml
listeners:
  - name: socks
    type: socks5
```