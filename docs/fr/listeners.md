# Listeners

Un **listener** définit comment GORP reçoit les connexions. Le champ `type` détermine le protocole.

| Type | Usage |
|---|---|
| `http` | HTTP/1.1 classique |
| `https` | HTTPS |
| `dynamic` | HTTPS avec certificat généré dynamiquement par SNI |
| `http3` | HTTP/3 sur QUIC |
| `httpmulti` / `multi` | HTTP en clair + TLS sur le même port |
| `tcp` | TCP brut, avec TLS/SNI optionnel |
| `unix` | HTTP sur socket Unix |
| `unix_tcp` | TCP sur socket Unix |
| `proxy` | proxy HTTP |
| `proxy_tls` | proxy HTTP sous TLS |
| `socks5` | proxy SOCKS5 |

Un type inconnu est traité comme `http`.

## HTTP

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"
```

Le listener utilise les routes HTTP globales.

### CONNECT HTTP

Un listener HTTP accepte aussi `CONNECT`. Le client ne choisit pas librement la destination : GORP utilise le backend de la route sélectionnée et ouvre le tunnel vers celui-ci.

## HTTPS

```yaml
listeners:
  - name: https
    type: https
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

Voir [TLS et mTLS](tls.md).

## HTTP/3

```yaml
listeners:
  - name: http3
    type: http3
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

Le listener HTTP/3 utilise QUIC. Le port doit donc être accessible en UDP.

## HTTP multi-protocole

`httpmulti` (ou `multi`) permet de partager un port entre HTTP en clair et TLS. GORP regarde le début de la connexion pour déterminer si elle commence par un handshake TLS.

```yaml
listeners:
  - name: multi
    type: httpmulti
    address: ":8080"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

Ce mode accepte HTTP/1.1, HTTP/2 en clair et, lorsque TLS est utilisé, HTTP/1.1 et HTTP/2 négociés par TLS. Il peut être utile lorsqu'un même port doit accepter plusieurs formes de trafic HTTP.

## HTTPS dynamique

Le type `dynamic` utilise une **CA** pour générer à la volée un certificat correspondant au SNI demandé :

```yaml
listeners:
  - name: intercept
    type: dynamic
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/ca/ca.crt"
      key_file: "/etc/gorp/ca/ca.key"
      key_passphrase: "change-me"
```

Ici `cert_file` et `key_file` désignent la CA de signature, pas un certificat serveur classique. Les clients doivent faire confiance à cette CA.

Un certificat généré est associé au nom SNI. Le cache conserve jusqu'à 1024 certificats et chaque certificat généré est valable 24 heures.

`ca_file` peut en plus servir à imposer un certificat client. Le chemin `dynamic` ne configure pas de CRL/OCSP pour ces certificats clients.

## Socket Unix HTTP

```yaml
listeners:
  - name: local-http
    type: unix
    address: "/run/gorp/http.sock"
```

Le socket est supprimé à l'arrêt. Un socket obsolète est remplacé au démarrage ; un fichier ordinaire ou un socket encore actif n'est pas écrasé.

Les permissions du système de fichiers déterminent qui peut accéder au listener.

## TCP

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    backends:
      - name: broker
        url: "tcp://127.0.0.1:1883"
```

Un listener TCP sélectionne aléatoirement un backend de son pool. Les routes HTTP, middlewares HTTP et selectors HTTP ne s'appliquent pas à ce mode.

## TCP TLS

Le même listener peut terminer TLS avant de transmettre le flux TCP :

```yaml
listeners:
  - name: secure-tcp
    type: tcp
    address: ":9443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
    backends:
      - name: service
        url: "tcp://127.0.0.1:9000"
```

Sur un listener TCP TLS, GORP peut inspecter le SNI avant le handshake complet et choisir un pool spécifique. Voir [TCP, TLS et SNI](tcp.md).

## Socket Unix TCP

```yaml
listeners:
  - name: local-tcp
    type: unix_tcp
    address: "/run/gorp/service.sock"
    backends:
      - name: service
        url: "tcp://127.0.0.1:9000"
```

## Limites et timeouts HTTP

Ces champs s'appliquent aux listeners HTTP/HTTPS/Unix HTTP et aux handlers HTTP construits par GORP :

```yaml
listeners:
  - name: public
    type: https
    address: ":443"
    read_header_timeout: "5s"
    write_timeout: "60s"
    idle_timeout: "120s"
    max_connections: 500
    max_request_body_size: 10485760
```

- `read_header_timeout` : temps maximal de lecture des headers ;
- `write_timeout` : délai d'écriture HTTP ;
- `idle_timeout` : délai d'inactivité des connexions HTTP ;
- `max_connections` : nombre maximal de requêtes HTTP actives traitées simultanément par ce handler ;
- `max_request_body_size` : limite vérifiée lorsque `Content-Length` est connu.

Une requête dépassant `max_request_body_size` reçoit `413 Request Entity Too Large`. Un corps avec `Content-Length: -1` n'est pas bloqué par ce contrôle.

Pour TCP, `max_connections` limite réellement le nombre de connexions acceptées.
