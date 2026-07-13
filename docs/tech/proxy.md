# Outbound Proxy

This section describes how to configure an outbound proxy for backend
connections.

## Configuration

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend1
        url: "http://www.example.com:8001"
        proxy:
          url: http://proxy.example.com:8080
          username: proxyuser
          password: proxypassword
```

The proxy URL can use the `http`, `https`, or `socks5` scheme.

## Listener proxy cascading

The `proxy` and `proxy_tls` listeners can themselves forward traffic through
an upstream proxy using the same syntax:

```yaml
listeners:
  - name: outbound-proxy
    type: proxy
    address: ":3128"
    proxy:
      url: http://proxy.example.com:8080
      username: proxyuser
      password: proxypassword
```

The upstream URL may use `http`, `https`, or `socks5`. HTTP requests are sent
through the upstream proxy, and `CONNECT` requests create a tunnel through it.
The same `proxy` block can be added to a `proxy_tls` listener.

---
