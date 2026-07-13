# Dépannage

## GORP refuse de démarrer

Commencez par valider la configuration avec le programme lui-même :

```bash
./gorp -config ./config.yaml
```

Les erreurs de configuration apparaissent dans les logs avant le démarrage.

## `no backends configured`

Une route doit avoir soit :

```yaml
backends:
  - name: app
    url: "http://127.0.0.1:8000"
```

soit :

```yaml
service: app
```

avec un service correspondant :

```yaml
services:
  - name: app
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## Le backend répond `502`

Vérifiez :

1. l'adresse et le port du backend ;
2. le protocole dans `url` ;
3. le certificat si HTTPS est utilisé ;
4. `tls.insecure`, `tls.ca_file` et `tls.server_name` ;
5. les timeouts ;
6. l'état du circuit breaker.

Testez directement le backend :

```bash
curl -v http://127.0.0.1:8000/
```

## Le backend répond `504`

Le code renvoie `504 Gateway Timeout` lorsqu'un timeout réseau/connexion est détecté par le reverse proxy.

Augmentez par exemple :

```yaml
connect_timeout: "5s"
headers_timeout: "15s"
```

## Le backend reçoit le mauvais Host

Par défaut, GORP reconstruit la destination à partir de l'URL du backend. Si l'application amont attend le Host public :

```yaml
preserve_host: true
```

## Une route ne correspond pas

Vérifiez :

- `prefix` ;
- `paths` ;
- `hosts` ;
- le port présent dans `Host` ;
- `endpoints` et le nom du listener.

Ajoutez temporairement :

```yaml
middlewares:
  - name: logging
```

## HTTPS échoue pendant le handshake

Vérifiez :

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  min_version: TLS1.2
```

Pour mTLS :

```yaml
ca_file: clients-ca.crt
```

Pour un certificat révoqué :

```yaml
crl_file: clients.crl
```

Si vous utilisez OCSP, assurez-vous que le serveur GORP peut joindre le responder.

## HTTP/3 ne répond pas

HTTP/3 utilise QUIC et donc UDP. Vérifiez le firewall et le port UDP, pas seulement TCP.

## Le listener multi refuse une connexion

`httpmulti` inspecte le premier octet de la connexion. Une connexion TLS doit commencer comme un handshake TLS. Une connexion non TLS est traitée comme HTTP/1.1 ou HTTP/2 en clair.

## Un proxy refuse `CONNECT`

Le proxy HTTP attend une destination dans `r.Host`. Vérifiez que le client utilise bien un proxy HTTP et non un reverse proxy.

Exemple :

```bash
curl -x http://127.0.0.1:3128 https://example.com/
```

## Un proxy demande une authentification

Ajoutez :

```yaml
users:
  - alice: secret
```

et utilisez les identifiants proxy côté client.

## Le reload ne prend pas la configuration

Vérifiez :

```bash
./gorp -config /etc/gorp/config.yaml
```

ou :

```bash
./gorp -config-dir /etc/gorp/conf.d
```

Seuls les changements surveillés sur la source locale déclenchent le reload. Une erreur de chargement ou de construction laisse l'ancienne configuration active.

## Vérifier l'état opérationnel

Activez l'admin :

```yaml
admin:
  enabled: true
  address: ":9090"
```

Puis :

```bash
curl -i http://127.0.0.1:9090/health
curl -i http://127.0.0.1:9090/ready
curl http://127.0.0.1:9090/metrics
```

## Un backend reste indisponible

Le circuit breaker actuel ouvre le circuit après plusieurs erreurs. Après environ 30 secondes, une récupération est tentée.

Surveillez :

```text
gorp_proxy_backend_errors_total
gorp_proxy_active_connections
```

et les logs contenant `circuit breaker`.
