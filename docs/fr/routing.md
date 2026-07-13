# Routage HTTP

Les routes HTTP déterminent quel backend reçoit une requête.

## Une route minimale

```yaml
routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## Critères disponibles

Une route peut utiliser :

- `prefix` : préfixe de chemin ;
- `paths` : motifs wildcard sur le chemin complet ou son dernier composant ;
- `hosts` : motifs wildcard sur `Host` ;
- `endpoints` : restriction au nom du listener ;
- `service` ou `backends` : destination.

Les routes avec un `prefix` plus long sont triées avant les préfixes plus courts lors du chargement de la configuration.

## Prefixes

```yaml
routes:
  - name: api
    prefix: "/api"
    backends:
      - name: api
        url: "http://127.0.0.1:8001"
```

`/api/users` correspondra à cette route.

Le test est un `HasPrefix` simple : `/api2` correspond donc également à `/api`. Si vous avez besoin d'une frontière stricte, utilisez un motif `paths` adapté.

## Supprimer le préfixe

```yaml
routes:
  - name: api
    prefix: "/api"
    strip_prefix: true
    backends:
      - name: api
        url: "http://127.0.0.1:8001"
```

Une requête :

```text
GET /api/users/42
```

est envoyée au backend avec :

```text
GET /users/42
```

Si la suppression laisse un chemin sans `/`, GORP en ajoute un.

## Conserver le Host original

Par défaut, le reverse proxy définit l'URL amont à partir de `backend.url`. Avec :

```yaml
preserve_host: true
```

le header `Host` envoyé au backend reste celui reçu du client.

```yaml
routes:
  - name: tenant
    hosts:
      - "app.example.com"
    preserve_host: true
    backends:
      - name: app
        url: "http://10.0.0.10:8000"
```

Le reverse proxy ajoute également les headers `X-Forwarded-*` gérés par `net/http/httputil`.

## Routage par Host

```yaml
routes:
  - name: shop
    hosts:
      - "shop.example.com"
    backends:
      - name: shop
        url: "http://10.0.0.11:8000"

  - name: api
    hosts:
      - "api.example.com"
    backends:
      - name: api
        url: "http://10.0.0.12:8000"
```

Les motifs utilisent le comportement de `filepath.Match`, notamment `*`, `?` et les classes `[...]`.

> Le `Host` HTTP est comparé tel qu'il est reçu. Si le client envoie un port, par exemple `example.com:8080`, le motif doit en tenir compte.

## Routage par chemin avec wildcard

```yaml
routes:
  - name: images
    paths:
      - "/images/*"
    backends:
      - name: media
        url: "http://127.0.0.1:8002"
```

Le motif est testé sur le chemin complet et sur `path.Base(path)`.

## Restreindre une route à un listener

`endpoints` contient les **noms** des listeners, pas leurs adresses :

```yaml
listeners:
  - name: public
    type: https
    address: ":443"
    tls:
      cert_file: server.crt
      key_file: server.key

  - name: internal
    type: http
    address: ":8080"

routes:
  - name: admin
    prefix: "/admin"
    endpoints:
      - internal
    backends:
      - name: admin
        url: "http://127.0.0.1:9000"
```

La route `admin` ne sera donc utilisée que par le listener `internal`.

## Ordre de sélection

Lorsqu'une requête arrive, GORP :

1. cherche d'abord une route dont `paths` correspond ;
2. si aucune ne correspond, cherche une route dont `prefix`, `hosts` ou la règle spéciale `CONNECT` correspond ;
3. ignore les routes dont `endpoints` ne contient pas le listener courant ;
4. transmet la requête au backend choisi.

Les routes sont triées par longueur de `prefix` au chargement. En cas de plusieurs routes ayant le même niveau de correspondance, leur ordre de configuration peut donc devenir important.

## Fallback sur un répertoire

Si aucune route ne correspond et que `directory` est configuré, GORP sert directement le fichier correspondant au chemin :

```yaml
directory: "/srv/gorp/public"
```

Par exemple `/logo.png` cherche `/srv/gorp/public/logo.png`.

Sans route correspondante et sans `directory`, GORP renvoie `404`.

## Redirections du backend

Si une application située derrière `strip_prefix` renvoie :

```text
Location: /login
```

le client public peut avoir besoin de recevoir `/private/login`. Utilisez :

```yaml
middlewares:
  - name: add_location_prefix
    config:
      prefix: "/private"
```

Pour remplacer le host d'une URL absolue :

```yaml
middlewares:
  - name: set_location_host
    config:
      host: "public.example.com"
```
