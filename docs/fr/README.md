# GORP — documentation utilisateur

**GORP** est un reverse proxy multi-protocole écrit en Go. Il peut exposer des services HTTP/HTTPS, HTTP/2, HTTP/3, TCP/TLS, des sockets Unix, ainsi que des services de proxy HTTP(S) et SOCKS5.

Cette documentation est orientée **utilisation et configuration**. Elle part d'exemples concrets puis détaille les possibilités de chaque composant.

> Les exemples utilisent YAML. GORP accepte également JSON.

## Sommaire

- [Démarrer rapidement](getting-started.md)
- [Comprendre la configuration](configuration.md)
- [Listeners](listeners.md)
- [Routage HTTP](routing.md)
- [Backends et services](backends.md)
- [Sélection des backends](selectors.md)
- [Middlewares](middlewares.md)
- [TLS et mTLS](tls.md)
- [TCP, TLS et SNI](tcp.md)
- [Proxy HTTP et SOCKS5](proxies.md)
- [Administration et métriques](admin.md)
- [Rechargement de configuration](reload.md)
- [Référence de configuration](reference.md)
- [Dépannage](troubleshooting.md)

## En deux minutes

Le cas le plus simple est un serveur HTTP sur `:8080` qui transmet toutes les requêtes à une application sur `127.0.0.1:8000` :

```yaml
listeners:
  - name: http
    type: http
    address: ":8080"

routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Puis :

```bash
./gorp -config ./config.yaml
```

Test :

```bash
curl http://127.0.0.1:8080/
```

## Architecture mentale

```text
                          +-------------------+
 HTTP/HTTPS/HTTP3 ------->|                   |
 TCP/TLS ---------------->|       GORP        |-----> backend HTTP
 HTTP proxy ------------->|                   |-----> backend TCP
 SOCKS5 ----------------->|                   |
 Unix sockets ----------->|                   |
                          +-------------------+
                                   |
                                   +----> /metrics, /health, /ready
```

Le fichier de configuration décrit quatre éléments principaux :

- **listeners** : comment GORP accepte les connexions ;
- **routes** : où envoyer les requêtes HTTP ;
- **services** : pools de backends réutilisables par plusieurs routes ;
- **backends** : destinations réelles et paramètres de connexion.

## Principales possibilités

- HTTP/1.1 et HTTPS ;
- HTTP/2 et HTTP/3 ;
- écoute HTTP et TCP sur sockets Unix ;
- listener HTTP « multi » capable de distinguer HTTP en clair et TLS sur le même port ;
- TCP avec routage SNI ;
- proxy HTTP avec `CONNECT`, proxy TLS et SOCKS5 ;
- authentification Basic, Digest, Bearer/token et OpenID Connect ;
- authentification des connexions sortantes Basic, token et AWS Signature V4 ;
- TLS aval avec CA privée, mTLS, CRL et OCSP ;
- TLS amont avec CA privée et certificat client ;
- sélection round-robin, random, first-alive, affinité cookie/header/IP, least-connections et power-of-two choices ;
- limitation de connexions, taille de corps et débit ;
- réécriture de chemins et de `Location` ;
- middleware Go dynamique ;
- validation OpenAPI ;
- identifiants de corrélation et propagation W3C Trace Context ;
- métriques Prometheus ;
- rechargement automatique d'un fichier ou d'un répertoire de configuration.

## Sécurité : point de départ recommandé

Pour un service exposé sur Internet, utilisez HTTPS et, si nécessaire, une authentification applicative ou mTLS. Évitez de mettre des secrets directement dans un fichier de configuration lisible par tous les utilisateurs du système.

Le middleware `secure_access` est un mécanisme d'accès par cookie pratique pour certains usages, mais il ne remplace pas une authentification utilisateur complète.
