# Admin Listener

To enable the admin listener, add:

```yaml
admin:
  enabled: true
  address: ":9090"
```

For an unencrypted listener, use:

```yaml
admin:
  enabled: true
  address: ":9090"
  tls:
    cert_file: server.crt
    key_file: server.key
```

For a TLS listener, use:

The admin listener supports the same request middlewares as the other
listeners. For example, to require Basic Authentication on every admin
endpoint:

```yaml
admin:
  enabled: true
  address: ":9090"
  middlewares:
    - name: basic_auth
      config:
        realm: "Admin"
        users:
          admin: change-me
```

The middleware also protects `/health`, `/ready`, `/config`, and `/stop`.

The admin listener exposes the following endpoints:

| Endpoint | Description |
| --- | --- |
| `/config` | Returns the current configuration. |
| `/echo/` | Echoes the request content as JSON. |
| `/health` | Returns the health status. |
| `/ready` | Returns the readiness status. |
| `/metrics` | Exposes Prometheus metrics. |
| `/stop` | Initiates a graceful shutdown. |

The `/metrics` endpoint exposes listener metrics such as
`gorp_listener_requests_total`, `gorp_listener_request_duration_seconds`, and
`gorp_listener_active_connections`. Listener metrics include the `listener`,
`type`, `method`, and `status` labels where applicable. Backend-specific
metrics use the `gorp_proxy_*` names.

---
