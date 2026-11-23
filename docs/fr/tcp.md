# TCP, TLS et SNI

Les listeners `tcp` et `unix_tcp` servent à transporter des protocoles qui ne sont pas HTTP.

## TCP simple

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    backends:
      - name: mqtt-1
        url: "tcp://10.0.0.11:1883"
      - name: mqtt-2
        url: "tcp://10.0.0.12:1883"
```

Chaque nouvelle connexion choisit aléatoirement un backend.

## TCP TLS

```yaml
listeners:
  - name: mqtts
    type: tcp
    address: ":8883"
    tls:
      cert_file: server.crt
      key_file: server.key
    backends:
      - name: mqtt-1
        url: "tcp://10.0.0.11:1883"
```

Le TLS client → GORP est terminé par GORP avant le tunnel vers le backend.

## Routage par SNI

```yaml
listeners:
  - name: tls-router
    type: tcp
    address: ":443"
    tls:
      cert_file: default.crt
      key_file: default.key
    routes:
      - name: mqtt
        hosts:
          - "mqtt.example.com"
        backends:
          - name: mqtt
            url: "tcp://10.0.0.11:1883"

      - name: redis
        hosts:
          - "redis.example.com"
        backends:
          - name: redis
            url: "tcp://10.0.0.12:6379"

    backends:
      - name: fallback
        url: "tcp://10.0.0.13:9000"
```

Si aucun host SNI ne correspond, GORP utilise les `backends` du listener.

Les wildcards suivent `filepath.Match`. Pour TCP SNI, la comparaison est normalisée en minuscules.

## Certificat par route

Une route TCP peut définir son propre bloc `tls` avec `cert_file`, `key_file`, `ca_file`, `crl_file`, `ocsp_url` et `min_version`.

Cela permet par exemple de présenter un certificat différent selon le SNI.

## Vérifier les premiers octets

`initial_bytes` permet de filtrer le protocole avant d'ouvrir le backend.

Exemple MQTT :

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    initial_bytes:
      - [16]
      - "*"
      - [3, 4, 5]
    backends:
      - name: mqtt
        url: "tcp://127.0.0.1:1883"
```

Chaque entrée correspond à une position dans le début du flux :

- `16` : le byte doit valoir 16 ;
- `"*"` : n'importe quelle valeur ;
- `[3, 4, 5]` : l'une de ces valeurs ;
- `"M"` : le caractère ASCII `M`.

Exemple :

```yaml
initial_bytes:
  - "M"
  - "Q"
  - [3, 4]
```

GORP lit autant d'octets que le nombre de contrôles avant d'autoriser le tunnel. Un échec ferme la connexion.

Pour un listener TLS, le contrôle est effectué **après** le handshake TLS, donc sur les données déchiffrées.

## TLS côté backend

```yaml
backends:
  - name: secure
    url: "tcp://backend.internal:9000"
    force_tls: true
    tls:
      ca_file: backend-ca.crt
      server_name: backend.internal
```

`timeout` contrôle le délai de connexion TCP non TLS. Pour une connexion TLS, la construction du client TLS utilise la configuration de certificat/CA du backend.

## Limiter les connexions

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    max_connections: 1000
    backends:
      - name: mqtt
        url: "tcp://127.0.0.1:1883"
```

Contrairement au handler HTTP, cette limite est une vraie limite de connexions TCP acceptées.
