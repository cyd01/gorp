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