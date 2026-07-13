# Comprendre la configuration

GORP accepte une configuration YAML ou JSON. Le même modèle est utilisé quel que soit le format.

## Charger la configuration

La priorité est :

1. `-config` ;
2. `-config-dir` ;
3. variables d'environnement ;
4. fichiers par défaut si aucun paramètre n'est fourni.

### Fichier

```bash
./gorp -config /etc/gorp/config.yaml
```

### Répertoire

```bash
./gorp -config-dir /etc/gorp/conf.d
```

### Variables d'environnement

Pour `-config` : `CONFIG`, `GORP_CONFIG`, `PROXY_CONFIG`.

Pour `-config-dir` : `GORP_CONFIG_DIR`, `PROXY_CONFIG_DIR`.

`-config` et `-config-dir` sont exclusifs.

## Sources de configuration

La fonction de lecture de fichier accepte également les sources supportées par le helper de lecture du projet, notamment les URLs HTTP(S), les fichiers locaux et le contenu fourni indirectement par une variable d'environnement. Pour une installation de production, un fichier local ou un répertoire local est généralement le choix le plus simple à administrer.

## Structure générale

```yaml
admin: {}
directory: "/srv/www"

listeners: []
services: []
routes: []
```

## Exemple complet

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
    service: application

directory: "/srv/gorp/public"
```

## YAML multi-documents

Un fichier YAML peut contenir plusieurs documents séparés par `---`. Les éléments `listeners`, `services` et `routes` sont concaténés.

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

## Durées

Les champs de durée utilisent le format Go :

```text
500ms
5s
2m
1h30m
```

## Secrets

Les mots de passe, tokens et clés AWS sont actuellement des valeurs de configuration ordinaires. GORP ne fournit pas de gestionnaire de secrets intégré. Protégez donc les fichiers et limitez leurs permissions.
