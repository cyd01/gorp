# Backends et services

Un backend représente une destination réelle. Une route peut déclarer ses backends directement ou utiliser un `service` partagé.

## Backend HTTP minimal

```yaml
backends:
  - name: app
    url: "http://127.0.0.1:8000"
```

Dans une route :

```yaml
routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## Plusieurs backends

```yaml
routes:
  - name: app
    prefix: "/"
    selector:
      type: round-robin
    backends:
      - name: app-1
        url: "http://10.0.0.11:8000"
      - name: app-2
        url: "http://10.0.0.12:8000"
```

Le selector par défaut est round-robin.

## Services réutilisables

```yaml
services:
  - name: api
    backends:
      - name: api-1
        url: "http://10.0.0.21:8080"
      - name: api-2
        url: "http://10.0.0.22:8080"

routes:
  - name: users
    prefix: "/users"
    service: api

  - name: orders
    prefix: "/orders"
    service: api
```

Une route qui possède `backends` les utilise en priorité. `service` est utilisé lorsque la route n'a pas de backends locaux.

## Limiter un backend

```yaml
- name: app
  url: "http://127.0.0.1:8000"
  max_connections: 50
```

Lorsqu'il atteint sa limite, les nouvelles requêtes reçoivent `503 Service Unavailable`.

## Timeouts

Les valeurs par défaut actuelles sont :

| Champ | Valeur effective par défaut |
|---|---:|
| `timeout` | 30 s |
| `connect_timeout` | `timeout`, sinon 30 s |
| `headers_timeout` | `timeout`, sinon 30 s |
| `response_timeout` | `timeout`, sinon 30 s |
| `idle_timeout` | 90 s |

Exemple :

```yaml
- name: slow-api
  url: "http://10.0.0.20:8000"
  timeout: "20s"
  connect_timeout: "2s"
  headers_timeout: "5s"
  response_timeout: "15s"
  idle_timeout: "60s"
```

> `response_timeout` existe dans la configuration mais le transport HTTP actuel n'applique pas de deadline globale de réponse avec ce champ. Les timeouts de connexion, headers et connexions idle sont effectivement configurés par le transport.

## Compression

```yaml
- name: app
  url: "http://127.0.0.1:8000"
  compression: true
```

Cela permet au transport HTTP de négocier gzip avec le backend. Ce champ ne constitue pas une liste d'algorithmes de compression configurable.

## TLS amont

```yaml
- name: secure-app
  url: "https://app.internal.example:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    server_name: "app.internal.example"
```

Pour un backend utilisant un certificat auto-signé :

```yaml
- name: lab
  url: "https://10.0.0.20:8443"
  tls:
    insecure: true
```

`insecure: true` désactive la vérification du certificat. Réservez cette option aux environnements où c'est réellement nécessaire.

Pour présenter un certificat client :

```yaml
- name: mtls-api
  url: "https://api.internal.example:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    cert_file: "/etc/gorp/tls/client.crt"
    key_file: "/etc/gorp/tls/client.key"
```

## Authentification amont

### Basic

```yaml
- name: app
  url: "http://127.0.0.1:8000"
  auth:
    type: basic
    username: apiuser
    password: change-me
```

### Token

```yaml
- name: api
  url: "https://api.example.com"
  auth:
    type: token
    token: "secret-token"
```

Par défaut, GORP envoie `Authorization: Bearer <token>`.

Header et préfixe personnalisés :

```yaml
auth:
  type: token
  token: "secret"
  header: "X-API-Key"
  prefix: ""
```

### AWS Signature V4

```yaml
- name: s3
  url: "https://bucket.s3.eu-west-3.amazonaws.com"
  auth:
    type: aws
    access_key_id: "AKIA..."
    secret_access_key: "..."
    region: "eu-west-3"
    service: "s3"
```

La signature est calculée à chaque requête. Le même mécanisme peut être utilisé avec d'autres services AWS compatibles avec Signature V4.

## Proxy sortant d'un backend

```yaml
- name: external
  url: "https://www.example.com"
  proxy:
    url: "http://proxy.internal:3128"
```

Avec authentification :

```yaml
proxy:
  url: "http://proxy.internal:3128"
  username: proxyuser
  password: change-me
```

Les schémas `http`, `https` et `socks5` sont acceptés.

## Circuit breaker intégré

Le backend HTTP devient indisponible après plusieurs erreurs : le code actuel ouvre le circuit après **5 échecs**, le maintient ouvert pendant environ **30 secondes**, puis permet une tentative de récupération.

Les réponses HTTP `5xx` du backend contribuent également au compteur d'échecs. Lorsqu'un backend est indisponible, les selectors qui consultent `Available()` ne le sélectionnent plus.

Ce mécanisme est volontairement simple : il n'expose pas encore de réglages de seuil, durée ou fenêtre dans la configuration.

## Backends TCP

Pour les listeners `tcp` et `unix_tcp`, l'URL attendue est de la forme :

```yaml
backends:
  - name: database
    url: "tcp://127.0.0.1:5432"
```

`force_tls: true` chiffre alors la connexion entre GORP et le backend :

```yaml
- name: database
  url: "tcp://db.internal:5432"
  force_tls: true
  tls:
    ca_file: "/etc/gorp/ca/db-ca.crt"
    server_name: "db.internal"
```

Les backends TCP sont choisis aléatoirement dans le pool du listener ou de la route SNI correspondante.
