# Référence de configuration

Cette page rassemble les champs exposés directement par le modèle de configuration actuel.

## Racine

```yaml
admin: ...
listeners: ...
services: ...
routes: ...
directory: ...
```

| Champ | Type | Description |
|---|---|---|
| `admin` | objet | serveur d'administration |
| `listeners` | liste | points d'entrée |
| `services` | liste | pools réutilisables |
| `routes` | liste | routage HTTP |
| `directory` | chaîne | répertoire servi en fallback |

## `admin`

| Champ | Type | Description |
|---|---|---|
| `enabled` | booléen | active le listener admin |
| `address` | chaîne | adresse d'écoute |
| `tls` | objet | TLS aval admin |
| `middlewares` | liste | middlewares admin |

## `listeners`

| Champ | Type | Description |
|---|---|---|
| `name` | chaîne | nom logique, utilisé aussi par `endpoints` |
| `type` | chaîne | type de listener |
| `address` | chaîne | adresse ou chemin de socket |
| `read_header_timeout` | durée | timeout lecture headers HTTP |
| `write_timeout` | durée | timeout écriture HTTP |
| `idle_timeout` | durée | timeout idle HTTP |
| `max_connections` | entier | limite HTTP active ou TCP acceptée selon le type |
| `max_request_body_size` | entier | limite de corps HTTP en octets |
| `tls` | objet | TLS aval |
| `users` | mapping/liste | comptes pour proxy/SOCKS5 |
| `log` | booléen | logging spécifique proxy/SOCKS5 |
| `proxy` | objet | proxy amont pour `proxy`/`proxy_tls` |
| `backends` | liste | backends TCP |
| `routes` | liste | routes SNI TCP |
| `initial_bytes` | liste | contrôles des premiers octets TCP |
| `middlewares` | liste | middlewares HTTP du listener |

## Types de listener

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

| Champ | Description |
|---|---|
| `cert_file` | certificat serveur ou CA de signature pour `dynamic` |
| `key_file` | clé privée |
| `key_passphrase` | passphrase de certaines clés PEM utilisées par `dynamic` |
| `min_version` | version TLS minimale |
| `ca_file` | CA des certificats clients pour mTLS |
| `crl_file` | CRL client |
| `ocsp_url` | URL OCSP client |

## `service`

```yaml
services:
  - name: app
    backends:
      - ...
```

| Champ | Description |
|---|---|
| `name` | nom référencé par `route.service` |
| `backends` | pool de destinations |

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

| Champ | Description |
|---|---|
| `name` | nom de route |
| `prefix` | préfixe de chemin |
| `strip_prefix` | retire `prefix` avant l'envoi |
| `preserve_host` | conserve le Host client vers le backend |
| `selector` | stratégie de sélection |
| `hosts` | motifs Host |
| `paths` | motifs de chemin |
| `endpoints` | noms des listeners autorisés |
| `service` | service partagé |
| `backends` | pool local |
| `middlewares` | middlewares de route |

## `selector`

```yaml
selector:
  type: header
  config:
    name: X-Tenant-ID
```

Types reconnus par le factory :

```text
round-robin (ou défaut)
random
first-alive
header
cookie
ip
least-connections
power-of-two
query
```

## `backend`

| Champ | Description |
|---|---|
| `name` | nom du backend |
| `url` | URL HTTP(S), ou `tcp://...` pour TCP |
| `timeout` | timeout de base |
| `connect_timeout` | timeout de connexion |
| `headers_timeout` | timeout des headers de réponse |
| `response_timeout` | paramètre de timeout de réponse ; voir les limites de l'implémentation |
| `idle_timeout` | timeout des connexions idle |
| `max_connections` | nombre maximal d'utilisations simultanées |
| `compression` | active la négociation gzip amont |
| `force_tls` | active TLS pour les backends TCP |
| `tls` | paramètres TLS amont |
| `auth` | authentification amont |
| `proxy` | proxy sortant |

## `backend.tls`

| Champ | Description |
|---|---|
| `insecure` | désactive la vérification du certificat |
| `ca_file` | CA de confiance supplémentaire |
| `server_name` | SNI / nom vérifié |
| `cert_file` | certificat client |
| `key_file` | clé privée client |

## `backend.auth`

| Champ | Description |
|---|---|
| `type` | `basic`, `token` ou `aws` |
| `username` | utilisateur Basic |
| `password` | mot de passe Basic |
| `token` | token |
| `header` | header du token |
| `prefix` | préfixe du token |
| `access_key_id` | AWS access key |
| `secret_access_key` | AWS secret |
| `region` | région AWS |
| `service` | service AWS, par ex. `s3` |

## `backend.proxy`

```yaml
proxy:
  url: "http://proxy.internal:3128"
  username: proxyuser
  password: change-me
```

## `tcp route`

```yaml
routes:
  - name: db
    hosts:
      - "db.example.com"
    tls:
      cert_file: db.crt
      key_file: db.key
    backends:
      - name: db
        url: "tcp://127.0.0.1:5432"
```

Les routes TCP sont définies **dans le listener TCP** et servent au routage SNI.

## `middleware`

```yaml
middlewares:
  - name: logging
  - name: rate_limit
    config:
      max_requests: 100
      window: "1m"
```

`config` est une map libre dont les clés dépendent du middleware.
