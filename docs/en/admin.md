# Administration and Metrics

The admin listener is a separate HTTP server. It is disabled unless
`admin.enabled` is set to `true`.

## Enable the admin listener

```yaml
admin:
  enabled: true
  address: ":9090"
```

Keep this listener on a trusted network. In particular, do not expose `/stop`
or `/config` publicly.

## Protect admin endpoints

Admin middlewares apply to all admin endpoints:

```yaml
admin:
  enabled: true
  address: ":9090"
  middlewares:
    - name: basic_auth
      config:
        realm: "GORP admin"
        users:
          admin: "change-me"
```

You can also configure TLS and require client certificates:

```yaml
admin:
  enabled: true
  address: ":9444"
  tls:
    cert_file: /etc/gorp/tls/admin.crt
    key_file: /etc/gorp/tls/admin.key
    ca_file: /etc/gorp/tls/admin-clients-ca.crt
```

The admin TLS block also accepts `crl_file`, `ocsp_url`, and `min_version`.

## Endpoints

| Endpoint | Purpose |
|---|---|
| `/health` | Process health; responds with `200 OK` and `OK` |
| `/ready` | Readiness state |
| `/metrics` | Prometheus metrics |
| `/backends` | Names, destinations, and states of route backends |
| `/config` | Current configuration serialized as JSON |
| `/echo/...` | Request inspection and echo |
| `/stop` | Starts a graceful shutdown |

`/health` is a basic server health check. `/ready` becomes ready after the
listeners start and becomes not ready during shutdown.

```bash
curl -i http://127.0.0.1:9090/health
curl -i http://127.0.0.1:9090/ready
curl http://127.0.0.1:9090/config
```

## Backend status

```bash
curl http://127.0.0.1:9090/backends
```

The response is a JSON array containing each route backend's name, destination,
and state (`closed`, `open`, or `half-open`). Backends referenced through a
service are included. Reading the endpoint does not trigger a circuit-breaker
probe.

## Graceful shutdown

```bash
curl http://127.0.0.1:9090/stop
```

GORP responds before starting shutdown. It then allows about 30 seconds for
servers and connections to stop gracefully. `SIGINT` and `SIGTERM` also
trigger graceful shutdown.

## Prometheus metrics

```bash
curl http://127.0.0.1:9090/metrics
```

Common metrics include:

```text
gorp_proxy_requests_total
gorp_proxy_request_duration_seconds
gorp_proxy_backend_errors_total
gorp_proxy_active_connections

gorp_listener_requests_total
gorp_listener_request_duration_seconds
gorp_listener_active_connections
```

Proxy metrics use labels such as `backend`, `method`, and `status`. Listener
metrics use labels such as `listener`, `type`, `method`, and `status`.

Example Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: gorp
    static_configs:
      - targets:
          - "127.0.0.1:9090"
```

If the admin listener uses HTTPS or authentication, configure the Prometheus
job accordingly.
