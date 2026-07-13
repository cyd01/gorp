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

---
