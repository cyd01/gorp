# Troubleshooting

## GORP refuses to start

Start by validating the configuration with the program itself:

```bash
./gorp -config ./config.yaml
```

Configuration errors appear in the logs before startup.

## `no backends configured`

A route must have either:

```yaml
backends:
  - name: app
    url: "http://127.0.0.1:8000"
```

or:

```yaml
service: app
```

with a corresponding service:

```yaml
services:
  - name: app
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## The backend responds with `502`

Check:

1. the backend address and port;
2. the protocol in `url`;
3. the certificate if HTTPS is used;
4. `tls.insecure`, `tls.ca_file`, and `tls.server_name`;
5. timeouts;
6. the state of the circuit breaker.

Test the backend directly:

```bash
curl -v http://127.0.0.1:8000/
```

## The backend responds with `504`

The code returns `504 Gateway Timeout` when a network/connection timeout is detected by the reverse proxy.

Increase, for example:

```yaml
connect_timeout: "5s"
headers_timeout: "15s"
```

## The backend receives the wrong Host

By default, GORP reconstructs the destination from the backend URL. If the upstream application expects the public Host:

```yaml
preserve_host: true
```

## A route does not match

Check:

- `prefix`;
- `paths`;
- `hosts`;
- the port present in `Host`;
- `endpoints` and the listener name.

Temporarily add:

```yaml
middlewares:
  - name: logging
```

## HTTPS fails during the handshake

Check:

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  min_version: TLS1.2
```