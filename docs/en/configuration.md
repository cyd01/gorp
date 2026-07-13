# Understanding configuration

GORP accepts YAML or JSON configuration. The same model is used regardless of the format.
Unknown fields are rejected, and inconsistent values or references (listeners,
routes, services, middleware, and backends) are reported before listeners start.

## Loading configuration

The priority is:

1. `-config`;
2. `-config-dir`;
3. environment variables;
4. default files if no parameter is provided.

### File

```bash
./gorp -config /etc/gorp/config.yaml
```

### Directory

```bash
./gorp -config-dir /etc/gorp/conf.d
```

### Environment variables

For `-config`: `CONFIG`, `GORP_CONFIG`, `PROXY_CONFIG`.

For `-config-dir`: `GORP_CONFIG_DIR`, `PROXY_CONFIG_DIR`.

`-config` and `-config-dir` are mutually exclusive.

## Configuration sources

The file reading function also accepts sources supported by the project's reading helper, including HTTP(S) URLs, local files, and content provided indirectly via an environment variable. For a production installation, a local file or a local directory is generally the simplest choice to administer.

## General structure

```yaml
admin: {}
directory: "/srv/www"

listeners: []
services: []
routes: []
```

## Complete example

```yaml
admin:
  enabled: true
  address: ":9090"

listeners:
  - name: https
    type: https
    address: ":443"
    read_header_timeout: "5s"
    write_timeout: "60s"
    idle_timeout: "120s"
    max_connections: 500
    max_request_body_size: 10485760
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
    middlewares:
      - name: correlation_id
      - name: logging

services:
  - name: application
    backends:
      - name: app-1
        url: "http://10.0.0.11:8080"
      - name: app-2
        url: "http://10.0.0.12:8080"

routes:
  - name: api
    prefix: "/api"
    strip_prefix: true
    service: application
    selector:
      type: power-of-two
    middlewares:
      - name: rate_limit
        config:
          max_requests: 120
          window: "1m"

  - name: admin
    prefix: "/admin"
    middlewares:
      - name: basic_auth
        config:
          realm: "Admin"
          users:
            admin: "change-me"
```

## Multiple YAML documents

A YAML file can contain multiple documents separated by `---`. GORP combines
their `listeners`, `services`, and `routes` entries:

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
---
services:
  - name: app
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
---
routes:
  - name: root
    prefix: "/"
    service: app
```

## Durations and secrets

Duration values use Go syntax, for example `500ms`, `5s`, `2m`, or `1h30m`.
Passwords, tokens, and AWS keys are ordinary configuration values; GORP has
no built-in secrets manager. Restrict configuration file permissions and
avoid exposing secrets to other local users.