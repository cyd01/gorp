# TLS et mTLS

GORP utilise TLS à deux endroits :

- **aval** : client → GORP ;
- **amont** : GORP → backend.

## TLS aval classique

```yaml
listeners:
  - name: https
    type: https
    address: ":443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
```

## Version minimale TLS

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  min_version: "TLS1.2"
```

Les formes `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` sont acceptées, ainsi que les formes `TLSV1.x` et `1.x` correspondantes.

## mTLS aval

Pour exiger un certificat client signé par une CA donnée :

```yaml
listeners:
  - name: api
    type: https
    address: ":8443"
    tls:
      cert_file: server.crt
      key_file: server.key
      ca_file: clients-ca.crt
```

Le client doit présenter un certificat valide signé par cette CA.

GORP ajoute alors le certificat client vérifié dans `X-Forwarded-Client-Cert` pour les listeners HTTPS, HTTP/3 et HTTPS dynamique. Toute valeur fournie par le client dans ce header est remplacée.

## Révocation par CRL

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  ca_file: clients-ca.crt
  crl_file: clients.crl
```

Si une CRL est fournie, elle est utilisée pour vérifier le numéro de série du certificat client.

## Révocation par OCSP

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  ca_file: clients-ca.crt
  ocsp_url: "https://ocsp.example.com"
```

Lorsque `crl_file` n'est pas défini, le code peut utiliser `ocsp_url`. Si l'URL est absente, le serveur peut utiliser l'URL OCSP présente dans le certificat client.

**CRL et OCSP sont exclusifs dans l'implémentation actuelle : si une CRL est configurée, elle prend la priorité.**

## TLS amont avec CA privée

```yaml
routes:
  - name: internal-api
    prefix: "/api"
    backends:
      - name: api
        url: "https://api.internal:8443"
        tls:
          ca_file: "/etc/gorp/ca/internal-ca.crt"
          server_name: "api.internal"
```

## TLS amont avec certificat client

```yaml
- name: mtls-api
  url: "https://api.internal:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    cert_file: "/etc/gorp/tls/gorp-client.crt"
    key_file: "/etc/gorp/tls/gorp-client.key"
```

## Désactiver la vérification

```yaml
tls:
  insecure: true
```

Cela active `InsecureSkipVerify`. À éviter en production.

## TLS pour TCP

Le listener :

```yaml
listeners:
  - name: secure-tcp
    type: tcp
    address: ":9443"
    tls:
      cert_file: server.crt
      key_file: server.key
```

Le backend :

```yaml
backends:
  - name: secure-backend
    url: "tcp://10.0.0.20:9443"
    force_tls: true
    tls:
      ca_file: backend-ca.crt
      server_name: backend.internal
```

Le premier TLS protège client → GORP ; le second protège GORP → backend.

## TCP SNI avec certificats différents

Une route TCP peut fournir son propre certificat :

```yaml
listeners:
  - name: tls-gateway
    type: tcp
    address: ":443"
    tls:
      cert_file: default.crt
      key_file: default.key
    routes:
      - name: db
        hosts:
          - "db.example.com"
        tls:
          cert_file: db.crt
          key_file: db.key
        backends:
          - name: db
            url: "tcp://10.0.0.10:5432"
```

Le certificat de route est sélectionné après inspection du SNI.
