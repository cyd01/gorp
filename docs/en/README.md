# GORP — user documentation

**GORP** is a multi-protocol reverse proxy written in Go. It can expose HTTP/HTTPS, HTTP/2, HTTP/3, TCP/TLS, Unix sockets, as well as HTTP(S) proxy and SOCKS5 services.

This documentation is oriented towards **usage and configuration**. It starts from concrete examples and then details the possibilities of each component.

> Examples use YAML. GORP also accepts JSON.

## Summary

- [Quick start](getting-started.md)
- [Understanding configuration](configuration.md)
- [Listeners](listeners.md)
- [HTTP Routing](routing.md)
- [Backends and services](backends.md)
- [Backend selection](selectors.md)
- [Middlewares](middlewares.md)
- [TLS and mTLS](tls.md)
- [TCP, TLS and SNI](tcp.md)
- [HTTP and SOCKS5 proxies](proxies.md)
- [Administration and metrics](admin.md)
- [Configuration reload](reload.md)
- [Configuration reference](reference.md)
- [Troubleshooting](troubleshooting.md)

## In two minutes

The simplest case is an HTTP server on `:8080` that forwards all requests to an application on `127.0.0.1:8000`:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"

routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Then:

```bash
./gorp -config ./config.yaml
```

Test:

```bash
curl http://127.0.0.1:8080/
```

## Mental Architecture

```text
                          +-------------------+
 HTTP/HTTPS/HTTP3 ------->|                   |
 TCP/TLS ---------------->|       GORP        |-----> backend HTTP
 HTTP proxy ------------->|                   |-----> backend TCP
 SOCKS5 ----------------->|                   |
 Unix sockets ----------->|                   |
                          +-------------------+
                                   |
                                   +----> /metrics, /health, /ready
```

The configuration file describes four main elements:

- **listeners**: how GORP accepts connections;
- **routes**: where to send HTTP requests;
- **services**: pools of backends reusable by multiple routes;
- **backends**: actual destinations and connection parameters.

## Main Capabilities

- HTTP/1.1 and HTTPS;
- HTTP/2 and HTTP/3;
- listening for HTTP and TCP on Unix sockets;
- "multi" HTTP listener capable of distinguishing plain HTTP and TLS on the same port;
- TCP with SNI routing;
- HTTP proxy with `CONNECT`, TLS proxy and SOCKS5;
- Basic, Digest, Bearer/token and OpenID Connect authentication;
- Basic, token and AWS Signature V4 authentication for outgoing connections;
- Upstream TLS with private CA, mTLS, CRL and OCSP;
- Downstream TLS with private CA and client certificate;
- round-robin, random, first-alive, cookie/header/IP affinity, least-connections and power-of-two choices selection;
- connection limiting, body size and rate limiting;
- path and `Location` rewriting;
- dynamic Go middleware;
- OpenAPI validation;
- correlation IDs and W3C Trace Context propagation;
- Prometheus metrics;
- automatic reloading of a configuration file or directory.

## Security: recommended starting point

For a service exposed to the Internet, use HTTPS and, if necessary, application authentication or mTLS. Avoid putting secrets directly in a configuration file readable by all system users.