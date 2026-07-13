# Configuration Reference

This page gathers the fields directly exposed by the current configuration model.

## Root

```yaml
admin: ...
listeners: ...
services: ...
routes: ...
directory: ...
```

| Field | Type | Description |
|---|---|---|
| `admin` | object | administration server |
| `listeners` | list | entry points |
| `services` | list | reusable pools |
| `routes` | list | HTTP routing |
| `directory` | string | fallback served directory |

## `admin`

| Field | Type | Description |
|---|---|---|
| `enabled` | boolean | enables the admin listener |
| `address` | string | listening address |
| `tls` | object | admin downstream TLS |
| `middlewares` | list | admin middlewares |

## `listeners`

| Field | Type | Description |
|---|---|---|
| `name` | string | logical name, also used by `endpoints` |
| `type` | string | listener type |
| `address` | string | address or socket path |
| `read_header_timeout` | duration | HTTP header read timeout |
| `write_timeout` | duration | HTTP write timeout |
| `idle_timeout` | duration | HTTP idle timeout |
| `max_connections` | integer | HTTP active limit or TCP accepted depending on type |
| `max_request_body_size` | integer | HTTP body limit in bytes |
| `tls` | object | downstream TLS |
| `users` | mapping/list | accounts for proxy/SOCKS5 |
| `log` | boolean | proxy/SOCKS5 specific logging |
| `proxy` | object | upstream proxy for `proxy`/`proxy_tls` |
| `backends` | list | TCP backends |
| `routes` | list | TCP SNI routes |
| `initial_bytes` | list | TCP initial bytes controls |
| `middlewares` | list | listener HTTP middlewares |

### `listener.proxy`

```yaml
proxy:
  url: "http://proxy-upstream.internal:8080"
  username: proxyuser
  password: "change-me"
  hosts:
    - "*.example.com"
  no_proxy:
    - "direct.example.com"
```

| Field | Type | Description |
|---|---|---|
| `url` | string | upstream proxy URL (`http`, `https`, or `socks5`) |
| `username`, `password` | string | upstream proxy credentials |
| `hosts` | list | destination hostname patterns sent through the upstream proxy |
| `no_proxy` | list | destination hostname patterns that bypass it |

## Listener Types

```text
http
https
dynamic
http3
httpmulti
multi
tcp
unix
unix_tcp
proxy
proxy_tls
socks5
```

## `tls`

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  key_passphrase: change-me
  min_version: TLS1.2
  ca_file: clients-ca.crt
  crl_file: clients.crl
  ocsp_url: https://ocsp.example.com
```

| Field | Description |
|---|---|
| `cert_file` | server certificate or signing CA for `dynamic` |
| `key_file` | private key |
| `key_passphrase` | passphrase for some PEM keys used by `dynamic` |
| `min_version` | minimum TLS version |
| `ca_file` | client certificate CA for mTLS |
| `crl_file` | client CRL |
| `ocsp_url` | client OCSP URL |

## `service`

```yaml
services:
  - name: app
    backends:
      - ...
```

| Field | Type | Description |
|---|---|---|
| `name` | string | name referenced by `route.service` |
| `backends` | list | reusable destination pool |

## `route`

```yaml
routes:
  - name: api
    prefix: /api
    strip_prefix: true
    preserve_host: true
    selector: ...
    hosts: [...]
    paths: [...]
    endpoints: [...]
    service: app
    backends: [...]
    middlewares: [...]
```

| Field | Type | Description |
|---|---|---|
| `name` | string | route name |
| `prefix` | string | path prefix |
| `strip_prefix` | boolean | remove `prefix` before forwarding |
| `preserve_host` | boolean | preserve the client `Host` header upstream |
| `selector` | object | backend selection strategy |
| `hosts` | list | wildcard patterns matched against the HTTP Host |
| `paths` | list | wildcard patterns matched against the path |
| `endpoints` | list | allowed listener names |
| `service` | string | name of a reusable service |
| `backends` | list | route-local backend pool; takes precedence over `service` |
| `middlewares` | list | route request/response middlewares |

## `selector`

```yaml
selector:
  type: header
  config:
    name: X-Tenant-ID
```

Supported types and aliases:

```text
round-robin (default), roundrobin, rr
random, rand
first-alive, first, alive
header
cookie
ip, source-ip, sourceip, ip-hash, iphash
least-connections, leastconnection, leastconnections, least-connection
power-of-two, poweroftwo, p2c
query, query-param, queryparam
```

`config.name` selects the header, cookie, or query parameter name. The
defaults are `Affinity` for headers and `SESSIONID` for cookies and query
parameters. See [Backend Selection](selectors.md) for behavior and examples.

## `backend`

| Field | Type | Description |
|---|---|---|
| `name` | string | logical backend name |
| `url` | string | HTTP(S) destination; TCP listeners use `tcp://host:port` |
| `timeout` | duration | legacy fallback for backend timeouts |
| `connect_timeout` | duration | connection timeout |
| `headers_timeout` | duration | response-header timeout |
| `response_timeout` | duration | declared field; no global response deadline is currently applied |
| `idle_timeout` | duration | idle connection timeout |
| `max_connections` | integer | concurrent request/connection limit |
| `compression` | boolean | enables gzip negotiation with HTTP backends |
| `force_tls` | boolean | enables TLS to TCP backends |
| `tls` | object | upstream TLS settings |
| `auth` | object | upstream authentication |
| `proxy` | object | outbound proxy settings |

Timeouts use Go duration syntax, such as `500ms`, `5s`, or `2m`. A zero or
omitted backend timeout falls back to `timeout`, then to the implementation
default. `response_timeout` is currently not enforced as a global response
deadline.

### `backend.tls`

| Field | Type | Description |
|---|---|---|
| `insecure` | boolean | disables upstream certificate verification; avoid in production |
| `ca_file` | string | additional trusted CA certificate |
| `server_name` | string | TLS SNI and verification name |
| `cert_file` | string | client certificate for upstream mTLS |
| `key_file` | string | client private key for upstream mTLS |

### `backend.auth`

| Field | Type | Description |
|---|---|---|
| `type` | string | `basic`, `token`, or `aws` |
| `username`, `password` | string | Basic credentials |
| `token` | string | token credential |
| `header` | string | custom token header |
| `prefix` | string | token prefix; defaults to the `Bearer` scheme |
| `access_key_id` | string | AWS access key ID |
| `secret_access_key` | string | AWS secret access key |
| `region` | string | AWS region |
| `service` | string | AWS service name, such as `s3` |

### `backend.proxy`

```yaml
proxy:
  url: "http://proxy.internal:3128"
  username: proxyuser
  password: "change-me"
```

The URL may use `http`, `https`, or `socks5`.
Backend proxy requests use `url`, `username`, and `password`; the `hosts` and
`no_proxy` routing lists are for cascaded `proxy` and `proxy_tls` listeners.

## TCP SNI route

TCP routes are declared inside a `tcp` or `unix_tcp` listener. They match
client SNI hostnames and select a TCP backend pool:

```yaml
listeners:
  - name: tls-gateway
    type: tcp
    address: ":443"
    tls:
      cert_file: default.crt
      key_file: default.key
    routes:
      - name: database
        hosts:
          - "db.example.com"
        tls:
          cert_file: db.crt
          key_file: db.key
        backends:
          - name: database
            url: "tcp://127.0.0.1:5432"
```

## `middleware`

```yaml
middlewares:
  - name: logging
  - name: rate_limit
    config:
      max_requests: 100
      window: "1m"
```

`config` is a free-form map whose keys depend on the middleware. Request
middlewares can be configured on listeners and routes; response middlewares
are configured on routes.
