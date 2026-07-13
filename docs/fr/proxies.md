# Proxy HTTP et SOCKS5

GORP peut fonctionner comme **proxy sortant** en plus de son rôle de reverse proxy.

## Proxy HTTP

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
```

Pour une requête HTTP, le client doit envoyer une URL absolue :

```bash
curl -x http://127.0.0.1:3128 http://example.com/
```

Pour HTTPS, le client utilise généralement `CONNECT` :

```bash
curl -x http://127.0.0.1:3128 https://example.com/
```

Le proxy crée alors un tunnel TCP vers la destination demandée.

## Authentifier le proxy

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    users:
      alice: "secret"
      bob: "password"
```

Les utilisateurs peuvent également être écrits sous forme de liste YAML :

```yaml
users:
  - alice: "secret"
  - bob: "password"
```

Le client doit utiliser `Proxy-Authorization: Basic ...`.

## Journaliser le proxy

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    log: true
```

## Proxy HTTP sous TLS

```yaml
listeners:
  - name: secure-proxy
    type: proxy_tls
    address: ":3129"
    tls:
      cert_file: proxy.crt
      key_file: proxy.key
    users:
      alice: "secret"
```

`proxy_tls` chiffre le transport entre le client et le proxy. Le proxy lui-même ne termine pas l'authentification client par certificat dans ce mode : la configuration TLS de ce listener utilise le certificat et la clé serveur.

## Proxy en cascade

Un proxy HTTP peut transmettre ses connexions à un autre proxy :

```yaml
listeners:
  - name: proxy
    type: proxy
    address: ":3128"
    proxy:
      url: "http://proxy-upstream.internal:8080"
      username: proxyuser
      password: change-me
```

Le proxy amont peut utiliser `http`, `https` ou `socks5`.

Le même mécanisme est disponible sur `proxy_tls`.

## SOCKS5

```yaml
listeners:
  - name: socks
    type: socks5
    address: ":1080"
```

Le listener accepte les requêtes SOCKS5 `CONNECT` vers IPv4, IPv6 et noms DNS.

Test avec curl :

```bash
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

## Authentifier SOCKS5

```yaml
listeners:
  - name: socks
    type: socks5
    address: ":1080"
    users:
      - alice: "secret"
```

Si des utilisateurs sont configurés, GORP attend l'authentification SOCKS5 username/password. Sinon, le mode sans authentification est utilisé lorsque le client le propose.

## Attention à la différence reverse proxy / forward proxy

Dans un reverse proxy HTTP classique :

```text
client ---> GORP ---> backend configuré
```

Dans un forward proxy :

```text
client ---> GORP ---> destination choisie par le client
```

Le listener `proxy` est donc à protéger sérieusement. Un proxy ouvert sur Internet peut devenir un relais d'accès à des destinations internes.
