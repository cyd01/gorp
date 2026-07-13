# Middlewares

Les middlewares permettent de modifier une requête, de contrôler l'accès ou de modifier la réponse.

## Où les placer ?

### Listener

```yaml
listeners:
  - name: https
    type: https
    address: ":443"
    middlewares:
      - name: logging
```

Le middleware est appliqué à toutes les routes de ce listener.

### Route

```yaml
routes:
  - name: admin
    prefix: "/admin"
    middlewares:
      - name: basic_auth
        config:
          users:
            admin: change-me
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Les middlewares de réponse sont configurés au niveau route.

## Authentification Basic

```yaml
- name: basic_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
      bob: "password"
```

Le client doit fournir `Authorization: Basic ...`. En cas d'échec, GORP renvoie `401` et `WWW-Authenticate`.

## Authentification Digest

```yaml
- name: digest_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
    nonce_timeout: "5m"
```

Le mécanisme utilise Digest avec `MD5` et `qop="auth"`. Le nonce expire selon `nonce_timeout` (5 minutes par défaut).

## Authentification par token

```yaml
- name: token_auth
  config:
    realm: "API"
    tokens:
      - "token-one"
      - "token-two"
    header: "Authorization"
    prefix: "Bearer "
```

Le header et le préfixe peuvent être personnalisés :

```yaml
header: "X-API-Key"
prefix: ""
```

## OpenID Connect / JWT

```yaml
- name: auth_openid
  config:
    issuer: "https://issuer.example.com"
    audience: "api://default"
    algorithm: "RS256"
    header: "Authorization"
    prefix: "Bearer "
    keys:
      - |
        -----BEGIN PUBLIC KEY-----
        ...
        -----END PUBLIC KEY-----
```

Le middleware vérifie la structure JWT, l'algorithme, la signature, `exp`, `nbf`, `iss` et `aud` selon les paramètres fournis.

Si `keys` est absent, GORP tente de récupérer les clés publiques via le fournisseur configuré.

## Liste blanche IP

```yaml
- name: ip_whitelist
  config:
    sources:
      - "10.0.0.0/8"
      - "192.168.10.0/24"
```

Une adresse qui ne correspond à aucun CIDR reçoit `403 Forbidden`.

Le middleware regarde `RemoteAddr`, pas `X-Forwarded-For`.

## Rate limiting

```yaml
- name: rate_limit
  config:
    max_requests: 100
    window: "1m"
```

Le compteur est local au processus et indexé par IP source. Une fois le quota dépassé, la réponse est `429 Too Many Requests`.

Pour limiter seulement certains chemins :

```yaml
- name: rate_limit
  config:
    max_requests: 20
    window: "1m"
    extensions:
      - "/api/*"
      - "*.json"
```

Le nom `extensions` est historique : il s'agit en réalité de motifs de chemin wildcard.

Valeurs par défaut : `100` requêtes par `60s`.

## Corrélation

```yaml
- name: correlation_id
  config:
    type: uuidV7
```

Types acceptés : `uuid`, `uuidV4`, `uuidV6`, `uuidV7`.

Le middleware ajoute `X-Correlation-Id` si le client ne l'a pas fourni et le renvoie également dans la réponse.

Pour que le logger utilise cet identifiant, placez `correlation_id` avant `logging`.

## Logging

```yaml
- name: logging
```

Aliases : `log`, `logger`.

Le logger écrit la méthode, l'URI, l'adresse distante, l'identifiant de corrélation éventuel et la durée.

## Distributed tracing

```yaml
- name: distributed_tracing
```

Aliases : `tracing`, `trace_context`.

Le middleware lit et propage le header W3C `traceparent`. Il crée un nouveau Span ID et transmet un nouveau `traceparent` au backend.

Il ne transmet pas les spans à un collecteur : il assure seulement la création et la propagation du contexte.

## Headers de requête

### Ajouter

```yaml
- name: add_request_headers
  config:
    headers:
      X-Source: gorp
```

### Remplacer

```yaml
- name: set_request_headers
  config:
    headers:
      X-Source: gorp
```

### Modifier

```yaml
- name: modify_request_headers
  config:
    headers:
      User-Agent: gorp
```

### Supprimer

```yaml
- name: remove_request_headers
  config:
    headers:
      - Accept
      - X-Debug
```

Les valeurs des headers doivent être des chaînes.

## CORS

```yaml
- name: cors
```

Le middleware ajoute les headers CORS et répond directement `200` aux requêtes `OPTIONS`.

Le comportement actuel autorise `*` pour `Access-Control-Allow-Origin` et active `Access-Control-Allow-Credentials`.

## Préfixer le chemin

```yaml
- name: add_prefix
  config:
    prefix: "/internal"
```

Le préfixe est ajouté à `r.URL.Path` avant la suite de la chaîne.

## Délai artificiel

```yaml
- name: delay
  config:
    duration: "250ms"
```

Pratique pour les tests et la simulation de latence.

## Secure access

```yaml
- name: secure_access
  config:
    path: "/unlock"
    key: "long-secret"
    redirect: "/"
    duration: "15m"
```

Une requête vers `/unlock` obtient un cookie signé, puis les requêtes suivantes utilisant ce cookie peuvent passer.

Le cookie est lié à l'User-Agent. Sans cookie valide, le middleware renvoie `410 Gone`.

## OpenAPI

```yaml
- name: openapi
  config:
    file: "/etc/gorp/openapi.yaml"
```

Le contrat est chargé lors de la construction de la configuration. Un contrat invalide fait échouer le chargement.

Les requêtes sont contrôlées selon le chemin, la méthode, les paramètres et le corps. Une opération inexistante peut produire `404` ou `405`, tandis qu'une requête non conforme peut produire `400`.

Le fichier peut également être référencé via HTTP(S) ou `file://`.

## Middleware Go dynamique

### Requête

```yaml
- name: dynamic_request
  config:
    code: |
      package dynamic

      import "net/http"

      func Middleware(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              r.Header.Set("X-From-Dynamic", "true")
              next.ServeHTTP(w, r)
          })
      }
```

### Réponse

```yaml
- name: dynamic_response
  config:
    code: |
      package dynamic

      import "net/http"

      func Middleware(response *http.Response) error {
          response.Header.Set("X-From-Dynamic", "true")
          return nil
      }
```

Le code est interprété avec Yaegi au chargement de la configuration. Une erreur de syntaxe, de compilation ou de signature fait échouer cette configuration.

> Un middleware dynamique exécute du code fourni par la configuration. Traitez donc le fichier de configuration comme du code privilégié.

## Middlewares de réponse

### Ajouter / définir / modifier

```yaml
- name: add_response_headers
  config:
    headers:
      X-Backend: application
```

Les variantes sont :

- `add_response_headers` ;
- `set_response_headers` ;
- `modify_response_headers` ;
- `remove_response_headers`.

### Préfixer Location

```yaml
- name: add_location_prefix
  config:
    prefix: "/private"
```

### Remplacer l'hôte de Location

```yaml
- name: set_location_host
  config:
    host: "public.example.com"
```

Les URLs relatives ne sont pas modifiées par `set_location_host`.
