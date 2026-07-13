# Administration et métriques

Le listener d'administration est un serveur HTTP séparé des listeners de trafic.

## Activer l'admin

```yaml
admin:
  enabled: true
  address: ":9090"
```

## Protéger l'admin

Les middlewares de l'admin s'appliquent à ses endpoints :

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

Pour un endpoint public, évitez d'exposer directement `/stop` et `/config` sur Internet.

## HTTPS et mTLS

```yaml
admin:
  enabled: true
  address: ":9444"
  tls:
    cert_file: /etc/gorp/tls/admin.crt
    key_file: /etc/gorp/tls/admin.key
    ca_file: /etc/gorp/tls/admin-clients-ca.crt
```

Le bloc TLS admin accepte également `crl_file`, `ocsp_url` et `min_version`.

## Endpoints

| Endpoint | Fonction |
|---|---|
| `/health` | santé du processus, répond `200 OK` et `OK` |
| `/ready` | état de readiness |
| `/metrics` | métriques Prometheus |
| `/config` | configuration courante sérialisée en JSON |
| `/echo/...` | inspection/echo de la requête |
| `/stop` | déclenche un arrêt gracieux |

## Health et readiness

```bash
curl http://127.0.0.1:9090/health
curl http://127.0.0.1:9090/ready
```

`/health` reste une vérification simple du serveur. `/ready` passe à l'état prêt après le démarrage des listeners et revient à non prêt lors de l'arrêt.

## Lire la configuration courante

```bash
curl http://127.0.0.1:9090/config
```

La réponse est du JSON correspondant à la configuration chargée.

## Arrêt gracieux

```bash
curl http://127.0.0.1:9090/stop
```

GORP répond avant de lancer l'arrêt. Le serveur dispose ensuite d'environ 30 secondes pour terminer proprement ses serveurs et connexions.

## Prometheus

```bash
curl http://127.0.0.1:9090/metrics
```

Les principales métriques sont :

```text
gorp_proxy_requests_total
gorp_proxy_request_duration_seconds
gorp_proxy_backend_errors_total
gorp_proxy_active_connections

gorp_listener_requests_total
gorp_listener_request_duration_seconds
gorp_listener_active_connections
```

Les métriques proxy utilisent notamment `backend`, `method` et `status`. Les métriques listener utilisent notamment `listener`, `type`, `method` et `status`.

## Exemple de scraping Prometheus

```yaml
scrape_configs:
  - job_name: gorp
    static_configs:
      - targets:
          - "127.0.0.1:9090"
```

Si l'admin est en HTTPS ou protégé par une authentification, configurez le job Prometheus en conséquence.
