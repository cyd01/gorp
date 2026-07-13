# Démarrer rapidement

## 1. Construire GORP

Le projet est un programme Go classique :

```bash
go build -o gorp ./cmd/gorp
```

Vérifiez également l'état du projet avec :

```bash
go test ./...
```

## 2. Premier reverse proxy HTTP

Créez `config.yaml` :

```yaml
listeners:
  - name: public-http
    type: http
    address: ":8080"

routes:
  - name: application
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Démarrez :

```bash
./gorp -config config.yaml
```

L'application doit maintenant être accessible via :

```bash
curl -i http://127.0.0.1:8080/
```

## 3. Ajouter deux backends

Un même route peut distribuer les requêtes entre plusieurs backends :

```yaml
routes:
  - name: application
    prefix: "/"
    backends:
      - name: app-1
        url: "http://10.0.0.11:8000"
      - name: app-2
        url: "http://10.0.0.12:8000"
```

Sans `selector`, GORP utilise le **round-robin**.

## 4. Exposer HTTPS

```yaml
listeners:
  - name: public-https
    type: https
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"

routes:
  - name: application
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Test :

```bash
curl -k https://127.0.0.1:8443/
```

## 5. Protéger une route

```yaml
routes:
  - name: public
    prefix: "/public"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"

  - name: admin
    prefix: "/admin"
    middlewares:
      - name: basic_auth
        config:
          realm: "Administration"
          users:
            admin: "change-me"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Test :

```bash
curl -u admin:change-me http://127.0.0.1:8080/admin/
```

## 6. Utiliser un service partagé

Si plusieurs routes utilisent le même pool :

```yaml
services:
  - name: frontend
    backends:
      - name: frontend-1
        url: "http://10.0.0.11:8000"
      - name: frontend-2
        url: "http://10.0.0.12:8000"

routes:
  - name: web
    prefix: "/"
    service: frontend
```

Une route peut utiliser soit `service`, soit son propre tableau `backends`.

## 7. Utiliser plusieurs fichiers

```bash
./gorp -config-dir /etc/gorp/conf.d
```

Seuls les fichiers `.yaml` et `.yml` sont chargés. Ils sont lus dans l'ordre lexicographique de leur nom.

Exemple :

```text
/etc/gorp/conf.d/
├── 00-admin.yaml
├── 10-listeners.yaml
├── 20-services.yaml
└── 30-routes.yaml
```

## 8. Vérifier l'installation

Si l'admin listener est activé :

```bash
curl http://127.0.0.1:9090/health
curl http://127.0.0.1:9090/ready
```

Le premier doit répondre `OK`. Le second indique si GORP est prêt à servir.

Pour les métriques :

```bash
curl http://127.0.0.1:9090/metrics
```

## 9. Arrêt propre

`SIGINT` et `SIGTERM` déclenchent un arrêt gracieux avec un délai de 30 secondes. L'endpoint `/stop` de l'admin listener déclenche également cet arrêt.
