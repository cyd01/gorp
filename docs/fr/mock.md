# Listener mock

Le listener `mock` répond directement aux requêtes, sans appeler de service
upstream. La requête sélectionne un scénario nommé avec
`X-Mock-Scenario`. Les scénarios peuvent être configurés au démarrage ou
modifiés à chaud via l'API d'administration.

## Configuration statique

```yaml
listeners:
  - name: test-api
    type: mock
    address: ":8081"
    mock:
      scenarios:
        user-found:
          match:
            method: GET
            path: /users/42
            headers:
              X-Tenant: demo
          response:
            status: 200
            headers:
              Content-Type: application/json
            body: '{"id":42,"name":"Ada"}'
        unavailable:
          response:
            status: 503
            headers:
              Retry-After: "30"
            body: service unavailable
```

Les critères `match.method`, `match.path` et `match.headers` sont facultatifs ;
ceux qui sont définis doivent tous correspondre. Le statut statique par défaut
est `200`, le corps et les headers sont vides (un `Content-Type` textuel est
ajouté par défaut).

```bash
curl -H 'X-Mock-Scenario: user-found' \
  -H 'X-Tenant: demo' \
  http://127.0.0.1:8081/users/42
```

Sans `X-Mock-Scenario`, le listener répond `400`. Un scénario inconnu ou dont
les critères ne correspondent pas répond `404`.

`X-Mock-Scenario` accepte aussi les jokers de `MatchWildcard` (`*`, `?` et
les classes `[abc]`). Le nom exact est prioritaire. Un motif qui correspond à
plusieurs scénarios est rejeté avec `400` plutôt que de choisir arbitrairement.

## Paramètres dans le chemin

`match.path` accepte des paramètres nommés sous la forme `:nom`, chacun
occupant un segment complet. Par exemple `/users/:id` correspond à
`/users/42`. Une fois le scénario sélectionné, le code Yaegi peut lire la
valeur avec l'API Go standard :

```go
id := response.Request.PathValue("id")
```

Les paramètres sont définis après les middlewares du listener ; ils sont
disponibles dans `response.code`, pas dans ces middlewares.

## Réponse calculée avec Yaegi

Pour générer une réponse plus complexe, définissez `response.code` à la place
de `status`, `headers` et `body`. Le code est évalué avec Yaegi et doit définir
`dynamic.Middleware`, de signature
`func(*http.Response) error`. La requête entrante est disponible via
`response.Request` ; le code construit le statut, les headers et le corps de la
réponse en modifiant `response`.

```yaml
listeners:
  - name: test-api
    type: mock
    address: ":8081"
    mock:
      scenarios:
        greet:
          match:
            method: GET
            path: /greet
          response:
            code: |
              package dynamic

              import (
                "fmt"
                "io"
                "net/http"
                "strings"
              )

              func Middleware(response *http.Response) error {
                name := response.Request.URL.Query().Get("name")
                if name == "" {
                  name = "world"
                }
                response.StatusCode = http.StatusOK
                response.Header.Set("Content-Type", "text/plain; charset=utf-8")
                response.Body = io.NopCloser(strings.NewReader(
                  fmt.Sprintf("Hello, %s!", name),
                ))
                return nil
              }
```

Le code est compilé au démarrage ; une erreur de compilation empêche le
serveur de démarrer. Les scénarios ajoutés par l'API d'administration sont
compilés avant d'être enregistrés. `response.code` ne peut pas être combiné
avec `response.status`, `response.headers` ou `response.body`.

Le code dynamique s'exécute avec les privilèges du processus GORP. Ne chargez
que des configurations provenant de sources de confiance. Un statut ou des
headers invalides, ou une erreur pendant l'exécution, produit une réponse
`500`.

`response.Body` doit être un `io.ReadCloser`, pas un `[]byte`. Pour utiliser
le résultat de `json.Marshal`, enveloppez les bytes, par exemple avec
`io.NopCloser(bytes.NewReader(body))`. Un panic du code Yaegi est intercepté et
produit également une réponse `500`.

## Overrides par requête

Les headers suivants peuvent modifier la réponse d'un scénario pour une seule
requête, y compris une réponse calculée par du code :

- `X-Mock-Status` remplace le statut (valeurs acceptées : `200` à `599`) ;
- `X-Mock-Body` remplace le corps ;
- `X-Mock-Response-Header-<Nom>` remplace ou ajoute un header de réponse.

Les overrides sont appliqués après le code dynamique. Les headers de framing
HTTP comme `Content-Length` et `Transfer-Encoding` ne peuvent pas être définis
ainsi.

```bash
curl -H 'X-Mock-Scenario: unavailable' \
  -H 'X-Mock-Status: 429' \
  -H 'X-Mock-Body: try again' \
  -H 'X-Mock-Response-Header-Retry-After: 5' \
  http://127.0.0.1:8081/any-path
```

`X-Mock-Body` convient aux petits corps ; utilisez un scénario configuré ou
calculé pour les réponses plus grandes.

## API d'administration

L'admin permet de consulter, créer/remplacer et supprimer un scénario. L'API
est disponible uniquement pour les listeners mock configurés et hérite des
middlewares de l'admin. Protégez donc cet endpoint.

```bash
curl -X PUT http://127.0.0.1:9090/mocks/test-api/scenarios/user-found \
  -H 'Content-Type: application/json' \
  -d '{"match":{"method":"GET","path":"/users/42"},"response":{"status":200,"body":"found"}}'
curl http://127.0.0.1:9090/mocks/test-api/scenarios
curl -X DELETE http://127.0.0.1:9090/mocks/test-api/scenarios/user-found
```

Un scénario créé via l'API est conservé en mémoire et ne survit pas au
redémarrage. Le format JSON du `PUT` utilise les mêmes champs `match` et
`response` que la configuration, y compris `response.code`.

## TLS et middlewares

Le listener accepte la configuration TLS standard (certificat, clé, options
TLS et mTLS), ainsi que les middlewares HTTP de listener. Les middlewares de
requête s'exécutent avant la sélection du scénario ; les middlewares de réponse
restent réservés aux routes HTTP.

```yaml
listeners:
  - name: secure-mock
    type: mock
    address: ":8443"
    tls:
      cert_file: /etc/gorp/tls/server.crt
      key_file: /etc/gorp/tls/server.key
    middlewares:
      - name: add_request_headers
        config:
          headers:
            X-Mock-Environment: test
    mock:
      scenarios:
        healthy:
          match:
            path: /health
            headers:
              X-Mock-Environment: test
          response:
            body: OK
```

Le listener mock ne prend pas en charge les backends ni les routes TCP.
