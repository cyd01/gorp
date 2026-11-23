# `mock` listener

The `mock` listener responds directly to HTTP requests without calling an
upstream service. Each response is defined by a scenario. The
`X-Mock-Scenario` request header selects the scenario to use.

## Configuration

The listener configuration contains a `scenarios` map keyed by scenario name.
Each scenario has an optional `match` object and a `response`. Every criterion
present in `match` must match the request.

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

All three `match` criteria are optional:

- `method` exactly matches the HTTP method;
- `path` matches the URL path segment by segment. A `:name` segment captures
  a non-empty value; for example, `/users/:id` matches `/users/42`. Other
  segments are literals; `*` and `?` are not treated as wildcards by this
  listener; `**` matches one or more path segments;
- `headers` exactly matches the specified request-header values. Header names
  are case-insensitive.

## Selecting a scenario

Send the scenario name in `X-Mock-Scenario`:

```bash
curl -H 'X-Mock-Scenario: user-found' \
  -H 'X-Tenant: demo' \
  http://127.0.0.1:8081/users/42
```

An exact name is checked first. If no exact name exists, the value of
`X-Mock-Scenario` is treated as a Go wildcard pattern (`*`, `?`, and
`[abc]` character classes) and compared with scenario names. The pattern must
match exactly one scenario; no match returns `404`, and multiple matches
return `400`.

After selecting a scenario, its `match` criteria are evaluated. If any
criterion fails, the response is `404`. Without `X-Mock-Scenario`, the
listener returns `400` unless a `default` scenario is configured. The default
is also used when the requested name selects no scenario or when the selected
scenario does not match. The default's own `match` criteria must also match;
otherwise the response is `404`. An ambiguous wildcard selection remains a
`400` error even when a default exists.

## Static responses

A static response uses `status`, `headers`, and `body`:

- `status` is optional and defaults to `200`. Valid values are `200` through
  `599`;
- `headers` is a map of response headers. HTTP framing headers such as
  `Content-Length` and `Transfer-Encoding` are not supported;
- `body` is the response text and defaults to empty.

If `Content-Type` is not set, the listener adds
`text/plain; charset=utf-8`. Responses to `HEAD`, `204`, `205`, and `304`
requests do not include a body.

## Generating a response with Yaegi

To generate a response, set `response.code` instead of `response.status`,
`response.headers`, and `response.body`. Yaegi source must define
`dynamic.Middleware` with the signature `func(*http.Response) error`. The
incoming request is available through `response.Request`; build the response
by modifying `response.StatusCode`, `response.Header`, and `response.Body`.

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

The code is compiled at startup. A compilation error prevents the server
from starting. Scenarios added through the admin API are compiled before
being registered. `response.code` cannot be combined with `response.status`,
`response.headers`, or `response.body`.

`response.Body` must be an `io.ReadCloser`, not a `[]byte`. To return bytes
from `json.Marshal`, wrap them with, for example,
`io.NopCloser(bytes.NewReader(body))`. The listener reads and closes the body
after the code runs. Yaegi errors and panics are recovered and return `500`;
generated statuses and headers are validated before they are sent.

Yaegi code runs with the privileges of the GORP process. Only use
configuration from trusted sources.

## Reading the request and path parameters

Dynamic code can inspect the request through `response.Request`:

- named `:name` path segment: `request.PathValue("name")`;
- query string: `request.URL.Query().Get("page")`, or
  `request.URL.Query()["tag"]` for all values of a repeated parameter;
- request header: `request.Header.Get("User-Agent")`, or `Values` for
  multiple values;
- request body: `io.ReadAll(request.Body)`, handling the returned error;
- URL fragment: fragments are not sent in HTTP requests, so they are not
  available on the server.

Path parameters are set after listener middlewares run. They are available in
`response.code`, but not in those middlewares. The handler does not read the
request body before invoking Yaegi; a middleware that has already consumed
the body may make it unavailable.

## Per-request overrides

These request headers replace parts of the response for one request:

- `X-Mock-Status` replaces the status; values from `200` through `599` are
  accepted;
- `X-Mock-Body` replaces the response body;
- `X-Mock-Response-Header-<Name>` replaces or adds a response header.

Overrides are applied after the static response or Yaegi code. HTTP framing
headers cannot be changed this way.

```bash
curl -H 'X-Mock-Scenario: unavailable' \
  -H 'X-Mock-Status: 429' \
  -H 'X-Mock-Body: try again' \
  -H 'X-Mock-Response-Header-Retry-After: 5' \
  http://127.0.0.1:8081/any-path
```

## Admin API

When admin is enabled, named scenarios can be listed, created/replaced, and
deleted:

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

The `PUT` JSON body uses the same `match` and `response` fields as
configuration. Requests are limited to 1 MiB, unknown fields are rejected,
and changes are stored only after validation and compilation. Changes are
in-memory and do not survive a restart. The API inherits admin middlewares;
protect these endpoints.

The `default` scenario is configured in the listener and cannot be changed
through this API.

## TLS and middleware

The listener accepts standard TLS configuration (certificate, key, TLS
options, and mTLS) and HTTP listener middlewares. Request middlewares run
before scenario selection and matching, so they can modify headers used by
`match`. Response middlewares remain route-only.

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

This listener does not support backends or TCP routes.

---

# `mocksrv` listener

The `mocksrv` listener responds directly to HTTP requests without calling an
upstream service. It first selects a route using the request method, path,
and headers. Each route provides named responses selected by
`X-Mock-Scenario` and can define a `default` response.

## Configuration

`mocksrv.routes` is a map keyed by route identifier. Each route must define at
least one of `match.method`, `match.path`, and `match.headers`. Every defined
criterion must match.

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

`match.method` exactly compares the HTTP method. `match.path` is a Go
wildcard pattern (`*`, `?`, `[abc]`) matched one path segment at a time;
wildcards do not cross `/`. `:name` path segments capture non-empty values.
Values in `match.headers` are also wildcard patterns; header names are
case-insensitive. `path` and `headers` are optional, but `match` cannot be
empty.

## Selecting a route

Every route whose criteria match is a candidate. Candidates are ranked in
this order:

1. the largest number of configured criteria (`method`, `path`, and each
   header count separately);
2. the largest number of exact criteria;
3. the largest number of literal characters in the patterns.

If several routes have the best score, the request is rejected with `400`
instead of depending on map iteration order. If no route matches, the
listener returns `404`.

For a `:name` path pattern, the captured value can be read in Yaegi with
`response.Request.PathValue("name")`.

## Selecting a response

After selecting a route, `X-Mock-Scenario` selects an entry in that route's
`scenarios` map. An exact name takes precedence; otherwise, `*`, `?`, and
`[abc]` character classes are used as wildcards. A pattern matching multiple
scenarios is rejected with `400`.

If `X-Mock-Scenario` is absent or does not select a scenario, the route's
`default` response is used when configured. Without a default, a missing
header returns `400`, and an unknown scenario returns `404`. The default is
a response, not an additional route, so it has no `match` criteria.
The fallback can also be defined as an entry named `default` in the
`scenarios` map; that entry is used when `X-Mock-Scenario` is absent or does
not select a scenario. If both the route-level `default` and
`scenarios.default` are set, the route-level field takes precedence. An
explicit `X-Mock-Scenario: default` always selects the named entry.

## Static responses

A `scenarios` entry or `default` can contain a static response:

- `status` is optional and defaults to `200`; valid values are `200` through
  `599`;
- `headers` is a response-header map. HTTP framing headers such as
  `Content-Length` and `Transfer-Encoding` are rejected;
- `body` is the response text and defaults to empty.

If `Content-Type` is not provided, `text/plain; charset=utf-8` is used. Bodies
are suppressed for `HEAD`, `204`, `205`, and `304`.

## Generating a response with Yaegi

A scenario entry or the default can use `code` instead of `status`,
`headers`, and `body`. Yaegi code defines
`dynamic.Middleware func(*http.Response) error`. The incoming request is
available in `response.Request`; the code sets the response status, headers,
and body.

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

The code is compiled at startup. The server does not start if configured code
fails to compile. `code` cannot be combined with `status`, `headers`, or
`body`. `response.Body` must be an `io.ReadCloser`; wrap `json.Marshal`
bytes with `io.NopCloser(bytes.NewReader(body))`. Errors and panics return
`500`; generated statuses and headers are validated before they are sent.

Yaegi code runs with the privileges of the GORP process. Only use trusted
source code.

## Reading request data

Dynamic code uses `response.Request`:

- captured `:name` path value: `request.PathValue("name")`;
- query string: `request.URL.Query().Get("page")`, or
  `request.URL.Query()["tag"]` for repeated values;
- headers: `request.Header.Get("User-Agent")` or `Values`;
- body: `io.ReadAll(request.Body)`, handling the returned error;
- URL fragment: fragments are not sent to an HTTP server.

Path values are set before dynamic code runs, but after listener middlewares.
The handler does not read the request body before dynamic code; a middleware
may already have consumed it.

## Per-request overrides

Named responses and `default` support these request overrides:

- `X-Mock-Status` overrides the status (values `200` through `599`);
- `X-Mock-Body` overrides the response body;
- `X-Mock-Response-Header-<Name>` replaces or adds a response header.

Overrides run after static response construction or Yaegi code. They cannot
change HTTP framing headers.

```bash
curl -H 'X-Mock-Scenario: hello' \
  -H 'X-Mock-Status: 202' \
  -H 'X-Mock-Body: accepted' \
  http://127.0.0.1:8082/greet
```

## Admin API

Routes are listed, created/replaced, and deleted as complete units:

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

The `PUT` JSON body contains `match`, `scenarios`, and optional `default`, as
in configuration. The route is atomically replaced after validation and
compilation of dynamic responses. Requests are limited to 1 MiB; unknown
fields and additional JSON objects are rejected. Changes are held in memory
only. The API inherits admin middlewares; protect these endpoints.

## TLS and middleware

The listener accepts standard TLS configuration, including mTLS, and HTTP
listener middlewares. Those middlewares run before matching; request changes
are visible to route criteria and Yaegi code. Response middlewares remain
route-only. This listener does not support backends or TCP routes.
