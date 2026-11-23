# Listener `mock`

Le listener `mock` répond directement aux requêtes HTTP, sans appeler de
service upstream. Chaque réponse est définie par un scénario. L'en-tête
`X-Mock-Scenario` sélectionne le scénario à utiliser.

## Configuration

La configuration du listener contient un mapping `scenarios` indexé par nom.
Chaque scénario contient un objet `match` facultatif et une `response`.
Tous les critères présents dans `match` doivent correspondre à la requête.

```yaml
listeners:
  - name: test-api
    type: mock
    address: ":8081"
    mock:
      default:
        match:
          path: /users/:id
        response:
          status: 200
          headers:
            Content-Type: application/json
          body: '{"id":"fallback"}'
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

Les trois critères de `match` sont facultatifs :

- `method` compare exactement la méthode HTTP ;
- `path` compare le chemin URL segment par segment. Un segment `:nom`
  capture une valeur non vide, par exemple `/users/:id` correspond à
  `/users/42`. Les autres segments sont littéraux ; `*` et `?` ne sont pas
  interprétés comme jokers dans ce type de listener ; `**` correspond à un ou plusieurs segments de chemin ;
- `headers` compare exactement les valeurs des en-têtes indiqués. Le nom
  d'en-tête est insensible à la casse.

## Choisir un scénario

Envoyez le nom du scénario dans `X-Mock-Scenario` :

```bash
curl -H 'X-Mock-Scenario: user-found' \
  -H 'X-Tenant: demo' \
  http://127.0.0.1:8081/users/42
```

Le nom exact est recherché en premier. Si aucun nom exact n'existe, la valeur
de `X-Mock-Scenario` est interprétée comme un motif wildcard Go (`*`, `?` et
les classes `[abc]`) et comparée aux noms des scénarios. Le motif doit
correspondre à un seul scénario ; zéro correspondance produit `404`, plusieurs
correspondances produisent `400`.

Une fois le scénario choisi, ses critères `match` sont évalués. Si un critère
ne correspond pas, la réponse est `404`. Sans en-tête `X-Mock-Scenario`, le
listener répond `400`, sauf si un scénario `default` est défini. Ce défaut est
également utilisé quand le nom demandé ne sélectionne aucun scénario ou quand
le scénario sélectionné ne correspond pas. Les critères `match` du défaut
doivent eux aussi correspondre ; sinon la réponse est `404`. Une sélection
wildcard ambiguë reste une erreur `400`, même si un défaut existe.

## Réponses statiques

Une réponse statique utilise `status`, `headers` et `body` :

- `status` est facultatif et vaut `200` par défaut. Les valeurs valides sont
  `200` à `599` ;
- `headers` est une map d'en-têtes de réponse. Les en-têtes de framing HTTP,
  tels que `Content-Length` et `Transfer-Encoding`, ne sont pas acceptés ;
- `body` est le corps texte de la réponse. Il est vide par défaut.

Si aucun `Content-Type` n'est défini, le listener ajoute
`text/plain; charset=utf-8`. Les réponses `HEAD`, `204`, `205` et `304` sont
envoyées sans corps.

## Réponse calculée avec Yaegi

Pour générer une réponse, définissez `response.code` à la place de
`response.status`, `response.headers` et `response.body`. Le code Yaegi doit
déclarer `dynamic.Middleware`, de signature
`func(*http.Response) error`. La requête est accessible via
`response.Request`; la réponse se construit en modifiant
`response.StatusCode`, `response.Header` et `response.Body`.

```yaml
listeners:
  - name: dynamic-api
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

Le code est compilé au démarrage. Une erreur de compilation empêche le
serveur de démarrer. Les scénarios ajoutés par l'API d'administration sont
compilés avant d'être enregistrés. `response.code` ne peut pas être combiné
avec `response.status`, `response.headers` ou `response.body`.

`response.Body` doit être un `io.ReadCloser`, pas un `[]byte`. Pour renvoyer
le résultat de `json.Marshal`, utilisez par exemple
`io.NopCloser(bytes.NewReader(body))`. Le listener lit et ferme le body après
l'exécution du code. Les erreurs et les panics Yaegi sont interceptés et
renvoient `500`; les statuts et en-têtes générés sont validés avant l'envoi.

Le code Yaegi s'exécute avec les privilèges du processus GORP. N'utilisez que
des configurations provenant de sources de confiance.

## Lire la requête et les paramètres de chemin

Dans le code dynamique, `response.Request` donne accès à la requête :

- paramètre d'un segment `:nom` : `request.PathValue("nom")` ;
- query string : `request.URL.Query().Get("page")`, ou
  `request.URL.Query()["tag"]` pour toutes les valeurs d'un paramètre répété ;
- en-tête : `request.Header.Get("User-Agent")`, ou `Values` pour plusieurs
  valeurs ;
- corps : `io.ReadAll(request.Body)`, en traitant l'erreur retournée ;
- fragment URL : il n'est pas transmis dans une requête HTTP ; il n'est donc
  pas disponible côté serveur.

Les paramètres `:nom` ne sont renseignés qu'après l'exécution des middlewares
du listener. Ils sont accessibles dans `response.code`, mais pas dans ces
middlewares. Le body de requête n'est pas lu par le handler avant l'exécution
de Yaegi ; un middleware qui l'a déjà consommé peut toutefois le rendre
indisponible.

## Overrides par requête

Ces en-têtes remplacent des éléments de la réponse pour une seule requête :

- `X-Mock-Status` remplace le statut ; seules les valeurs de `200` à `599`
  sont acceptées ;
- `X-Mock-Body` remplace le corps ;
- `X-Mock-Response-Header-<Nom>` remplace ou ajoute un en-tête de réponse.

Les overrides sont appliqués après la réponse statique ou le code Yaegi.
Les en-têtes de framing HTTP ne peuvent pas être modifiés ainsi.

```bash
curl -H 'X-Mock-Scenario: unavailable' \
  -H 'X-Mock-Status: 429' \
  -H 'X-Mock-Body: try again' \
  -H 'X-Mock-Response-Header-Retry-After: 5' \
  http://127.0.0.1:8081/any-path
```

## API d'administration

Quand l'administration est activée, les scénarios nommés peuvent être listés,
créés/remplacés et supprimés :

- `GET /mocks/{listener}/scenarios`
- `PUT /mocks/{listener}/scenarios/{scenario}`
- `DELETE /mocks/{listener}/scenarios/{scenario}`

```bash
curl -X PUT http://127.0.0.1:9090/mocks/test-api/scenarios/user-found \
  -H 'Content-Type: application/json' \
  -d '{"match":{"method":"GET","path":"/users/42"},"response":{"status":200,"body":"found"}}'
curl http://127.0.0.1:9090/mocks/test-api/scenarios
curl -X DELETE http://127.0.0.1:9090/mocks/test-api/scenarios/user-found
```

Le corps JSON du `PUT` reprend les champs `match` et `response` de la
configuration. Les requêtes sont limitées à 1 Mio, les champs inconnus sont
refusés et le changement n'est enregistré qu'après validation et compilation.
Les modifications sont en mémoire et ne survivent pas au redémarrage.
L'API hérite des middlewares d'administration ; protégez ces endpoints.

Le scénario `default` est défini dans la configuration et n'est pas modifiable
par cette API.

## TLS et middlewares

Le listener accepte la configuration TLS standard (certificat, clé, options
TLS et mTLS) et les middlewares HTTP configurés au niveau du listener. Les
middlewares de requête s'exécutent avant le choix du scénario et son
matching ; ils peuvent donc modifier les en-têtes utilisés par `match`.
Les middlewares de réponse restent réservés aux routes HTTP.

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

Ce listener ne prend pas en charge les backends ni les routes TCP.

---

# Listener `mocksrv`

Le listener `mocksrv` répond directement aux requêtes HTTP, sans appeler de
service upstream. Il commence par sélectionner une route à partir de la
méthode, du chemin et des en-têtes de la requête. Chaque route fournit des
réponses nommées, sélectionnées ensuite par `X-Mock-Scenario`, et peut définir
une réponse `default`.

## Configuration

`mocksrv.routes` est une map indexée par identifiant de route. Chaque route
doit définir au moins un critère parmi `match.method`, `match.path` et
`match.headers`. Tous les critères définis doivent correspondre.

```yaml
listeners:
  - name: test-mocksrv
    type: mocksrv
    address: ":8082"
    mocksrv:
      routes:
        users:
          match:
            method: GET
            path: /users/*
            headers:
              X-Tenant: tenant-*
          scenarios:
            found:
              status: 200
              headers:
                Content-Type: application/json
              body: '{"result":"found"}'
            dynamic:
              code: |
                package dynamic
                import (
                  "io"
                  "net/http"
                  "strings"
                )
                func Middleware(response *http.Response) error {
                  response.StatusCode = http.StatusCreated
                  response.Body = io.NopCloser(strings.NewReader(
                    response.Request.URL.Path,
                  ))
                  return nil
                }
          default:
            status: 404
            body: user scenario not selected
```

`match.method` compare exactement la méthode HTTP. `match.path` est un motif
wildcard Go (`*`, `?`, `[abc]`) comparé segment par segment : les jokers ne
traversent pas `/`. Les segments `:nom` capturent une valeur non vide. Les
valeurs de `match.headers` sont également des motifs wildcard ; les noms
d'en-tête sont insensibles à la casse. Les critères `path` et `headers` sont
facultatifs, mais un `match` ne peut pas être vide.

## Sélection de la route

Toutes les routes dont les critères correspondent sont candidates. La
priorité est déterminée dans cet ordre :

1. le plus grand nombre de critères définis (`method`, `path`, chaque
   en-tête compte séparément) ;
2. le plus grand nombre de critères exacts ;
3. le plus grand nombre de caractères littéraux dans les motifs.

Si plusieurs routes partagent le meilleur score, la requête est rejetée avec
`400` plutôt que de dépendre de l'ordre d'une map. Si aucune route ne
correspond, le listener répond `404`.

Pour un motif de chemin avec `:nom`, le paramètre capturé est disponible dans
le code Yaegi via `response.Request.PathValue("nom")`.

## Sélection de la réponse

Une fois la route sélectionnée, `X-Mock-Scenario` cherche une entrée de sa
map `scenarios`. Le nom exact est prioritaire ; en l'absence de nom exact,
`*`, `?` et les classes `[abc]` sont utilisés comme jokers. Une sélection qui
correspond à plusieurs scénarios est rejetée avec `400`.

Si `X-Mock-Scenario` est absent ou ne sélectionne aucun scénario, le champ
`default` de cette route est utilisé s'il existe. Sans défaut, l'absence de
l'en-tête produit `400` et un scénario inconnu produit `404`. Le défaut est
une réponse, pas une route supplémentaire ; il ne possède donc pas de
critères `match`. Pour définir le défaut dans la map `scenarios`, vous pouvez
également ajouter une entrée nommée `default`. Cette entrée est utilisée comme
repli en l'absence de `X-Mock-Scenario` ou si son nom ne sélectionne aucun
scénario. Si la route définit à la fois `default` et `scenarios.default`, le
champ `default` de la route est prioritaire. L'en-tête explicite
`X-Mock-Scenario: default` sélectionne toujours l'entrée nommée.

## Réponses statiques

Une valeur de `scenarios` ou le champ `default` peut contenir une réponse
statique :

- `status` est facultatif et vaut `200` par défaut ; valeurs valides de `200`
  à `599` ;
- `headers` est une map d'en-têtes de réponse. Les en-têtes de framing HTTP,
  tels que `Content-Length` et `Transfer-Encoding`, sont refusés ;
- `body` est le corps texte, vide par défaut.

Si `Content-Type` n'est pas fourni, `text/plain; charset=utf-8` est utilisé.
Le corps n'est pas envoyé pour `HEAD`, `204`, `205` et `304`.

## Réponse calculée avec Yaegi

Une entrée de scénario ou le défaut peut utiliser `code` à la place de
`status`, `headers` et `body`. Le code Yaegi définit
`dynamic.Middleware func(*http.Response) error`. La requête est disponible
dans `response.Request`; le code renseigne le statut, les en-têtes et le body
de réponse.

```yaml
listeners:
  - name: dynamic-mocksrv
    type: mocksrv
    address: ":8082"
    mocksrv:
      routes:
        greet:
          match:
            method: GET
            path: /greet
          scenarios:
            hello:
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
          default:
            status: 404
            body: scenario not selected
```

Le code est compilé au démarrage. Le serveur ne démarre pas si le code
configuré ne compile pas. `code` ne peut pas être combiné avec `status`,
`headers` ou `body`. `response.Body` doit être un `io.ReadCloser`; enveloppez
les bytes de `json.Marshal` avec `io.NopCloser(bytes.NewReader(body))`. Les
erreurs et les panics produisent `500`; les en-têtes et statuts produits sont
validés avant envoi.

Le code Yaegi s'exécute avec les privilèges du processus GORP. N'utilisez que
des sources fiables.

## Accès aux données de requête

Le code dynamique utilise `response.Request` :

- chemin capturé par `:nom` : `request.PathValue("nom")` ;
- query string : `request.URL.Query().Get("page")`, ou
  `request.URL.Query()["tag"]` pour les valeurs répétées ;
- en-têtes : `request.Header.Get("User-Agent")` ou `Values` ;
- corps : `io.ReadAll(request.Body)` et traitement de l'erreur ;
- fragment : le fragment URL n'est pas transmis au serveur HTTP.

Les paramètres de chemin sont posés avant l'exécution de `code`, mais après
les middlewares du listener. Le handler ne lit pas le body de requête avant
le code dynamique. Un middleware peut toutefois avoir déjà consommé ce body.

## Overrides par requête

Les réponses nommées et `default` acceptent les mêmes overrides :

- `X-Mock-Status` remplace le statut (valeurs `200` à `599`) ;
- `X-Mock-Body` remplace le corps ;
- `X-Mock-Response-Header-<Nom>` remplace ou ajoute un en-tête.

Ils sont appliqués après la réponse statique ou le code Yaegi. Ils ne peuvent
pas modifier les en-têtes de framing HTTP.

```bash
curl -H 'X-Mock-Scenario: hello' \
  -H 'X-Mock-Status: 202' \
  -H 'X-Mock-Body: accepted' \
  http://127.0.0.1:8082/greet
```

## API d'administration

Les routes sont listées, créées/remplacées et supprimées en tant qu'unités
complètes :

- `GET /mocks/{listener}/routes`
- `PUT /mocks/{listener}/routes/{route}`
- `DELETE /mocks/{listener}/routes/{route}`

```bash
curl http://127.0.0.1:9090/mocks/test-mocksrv/routes
curl -X PUT http://127.0.0.1:9090/mocks/test-mocksrv/routes/users \
  -H 'Content-Type: application/json' \
  -d '{"match":{"method":"GET","path":"/users/*"},"scenarios":{"found":{"status":200,"body":"found"}},"default":{"status":404,"body":"not selected"}}'
curl -X DELETE http://127.0.0.1:9090/mocks/test-mocksrv/routes/users
```

Le JSON du `PUT` contient `match`, `scenarios` et éventuellement `default`,
comme dans la configuration. La route est remplacée atomiquement après
validation et compilation des réponses dynamiques. La requête est limitée à
1 Mio et les champs inconnus ou les objets JSON supplémentaires sont refusés.
Les modifications sont conservées en mémoire uniquement. L'API hérite des
middlewares d'administration ; protégez ces endpoints.

## TLS et middlewares

Le listener accepte la configuration TLS standard, y compris le mTLS, ainsi
que les middlewares HTTP configurés au niveau du listener. Ceux-ci s'exécutent
avant le matching et leurs modifications de requête sont visibles par les
critères de route et par le code Yaegi. Les middlewares de réponse restent
réservés aux routes HTTP. Ce listener ne prend pas en charge les backends ni
les routes TCP.
