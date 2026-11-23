# HTTP mock listeners

The listener configurations and request-processing rules are documented
below.

# `mock` listener

## Configuration model

`Listener.Type == "mock"` requires `Listener.Mock`. `MockConfig.Scenarios` is
a map keyed by scenario name. Each `MockScenario` has an optional `Match` and
a `Response`.

`MockMatch` fields are optional:

- `Method` is an exact HTTP method;
- `Path` is matched by slash-separated segment equality, except a segment
  beginning with `:` captures a non-empty value. This path syntax does not
  implement `*` or `?` wildcards;
- `Headers` maps request header names to exact values. Header names are
  matched case-insensitively.

All configured criteria must match after the scenario is selected. Scenario
names are exact by default. If the `X-Mock-Scenario` value is not an exact
name, `routing.MatchWildcard` is applied to configured names. One match
selects the response; zero matches return `404`, and multiple matches return
`400`.

`MockConfig.Default` is an optional `*MockScenario`, with the same `Match` and
`Response` shape. It is selected when `X-Mock-Scenario` is absent, its value
selects no scenario, or the selected named scenario fails its request match.
The default's own match criteria are evaluated and must match, or the request
returns `404`. Without a default, a missing scenario header returns `400`;
unknown names and non-matching scenario criteria return `404`. An ambiguous
wildcard selection remains `400`.

## Response formats

`MockResponse` has two mutually exclusive representations:

- static: `Status`, `Headers`, and `Body`;
- dynamic: `Code`, Yaegi source defining `dynamic.Middleware` with signature
  `func(*http.Response) error`.

Static status defaults to `200` and must be in `200..599`. Static body and
headers default to empty. If no `Content-Type` is returned, the handler sets
`text/plain; charset=utf-8`. The handler suppresses bodies for `HEAD`, `204`,
`205`, and `304`.

The dynamic function receives a prepared `*http.Response`, including the
current request in `Request`. It may set `StatusCode`, `Header`, and `Body`.
`Body` must be an `io.ReadCloser`; it is read and closed by the handler.
Generated status and headers are validated before use. Dynamic errors, panics,
invalid statuses, and invalid headers return `500`. Yaegi source runs with
the process's privileges and must be trusted.

Configured scenario and default code is compiled while building the server.
Compilation errors prevent server startup. Admin scenario updates are
validated and compiled before the in-memory scenario is replaced. Dynamic
`Code` is mutually exclusive with static status, headers, and body.

## Request data in dynamic code

The dynamic function reads the original request via `response.Request`:

- named path captures are available from `Request.PathValue("name")`;
- query parameters use `Request.URL.Query()`;
- headers use `Request.Header.Get` or `Values`;
- request bodies can be read using `io.ReadAll(Request.Body)` and the error
  must be handled;
- URL fragments are not sent over HTTP and are not available to the server.

Path values are set after listener middlewares, so they are unavailable to
those middlewares and available to `response.code`. The mock handler does not
consume the request body before invoking dynamic code; a request middleware
may already have consumed it.

For `json.Marshal` output, assign an `io.ReadCloser`, for example
`io.NopCloser(bytes.NewReader(body))`, not the returned `[]byte`.

## Per-request overrides

Overrides apply after static response setup or dynamic execution:

- `X-Mock-Status`: replace the status with a value in `200..599`;
- `X-Mock-Body`: replace the response body;
- `X-Mock-Response-Header-<Name>`: replace/add a response header.

HTTP framing headers, including `Content-Length`, `Transfer-Encoding`, and
`Connection`, are not supported as response headers or overrides.

## Admin API

The admin mux exposes these endpoints for configured mock listeners:

- `GET /mocks/{listener}/scenarios` lists named scenarios;
- `PUT /mocks/{listener}/scenarios/{scenario}` creates or replaces a named
  scenario;
- `DELETE /mocks/{listener}/scenarios/{scenario}` deletes a named scenario.

The `PUT` body has the `MockScenario` JSON shape. The request is limited to
1 MiB, rejects unknown fields and trailing JSON objects, validates the
scenario, compiles Yaegi source, then updates the scenario atomically.
Changes are in-memory only. The default scenario is configured at startup and
is not managed through these scenario endpoints. The endpoints pass through
admin middlewares.

## TLS and middleware

The listener uses the regular HTTP/HTTPS server setup. It supports TLS,
including mTLS, and listener-level HTTP middleware. Request middleware runs
before scenario selection and matching; response middleware remains
route-only. Backends and TCP routes are not supported.

# `mocksrv` listener

## Configuration model

`Listener.Type == "mocksrv"` requires `Listener.MockSrv`. `MockSrvConfig.Routes`
is a map keyed by route identifier. Each `MockSrvRoute` contains:

- `Match`, which must define at least one of `Method`, `Path`, or `Headers`;
- `Scenarios`, a map from scenario names to `MockResponse`;
- optional `Default`, a `*MockResponse` used when no named response is
  selected.

Every configured match criterion is required to match. `Method` is exact.
`Path` uses `routing.MatchWildcard` per slash-separated segment, so `*`, `?`,
and `[abc]` do not span `/`. A `:name` path segment captures a non-empty
segment. Header names are case-insensitive, and configured header values use
the wildcard matcher.

## Route selection

The handler evaluates request matches before reading `X-Mock-Scenario`.
Candidates are ranked by:

1. number of configured criteria: method, path, and each header count;
2. number of exact criteria;
3. total count of literal characters in the match patterns.

The route with the highest score is selected. If multiple routes share that
score, the request returns `400` to avoid nondeterministic map-order
selection. If there are no matching routes, it returns `404`.

Path captures are set with `Request.SetPathValue` on the selected request.
Yaegi response code can read them with `response.Request.PathValue("name")`.

## Response selection

Once a route is selected, `X-Mock-Scenario` selects from that route's
`Scenarios`. Exact name matching has priority. Otherwise, wildcard matching
is performed against scenario names. A wildcard matching multiple names
returns `400`.

If the header is absent or selects no named response, the route's `Default`
is used when present. Without a default, a missing header returns `400` and
an unknown scenario returns `404`. The default is a response and has no
request `Match`. As an alternative, the `Scenarios` map may contain an entry
named `default`; it is used as the fallback when the header is absent or
selects no scenario. If both forms are configured, `MockSrvRoute.Default`
takes precedence for fallback requests. An explicit
`X-Mock-Scenario: default` still selects the named map entry.

## Response formats and execution

Each named scenario and the default use `MockResponse`, supporting the same
two formats:

- static `Status`, `Headers`, and `Body`;
- dynamic `Code` defining Yaegi `dynamic.Middleware` as
  `func(*http.Response) error`.

Static status defaults to `200`; valid values are `200..599`. Headers and
body default to empty; absent `Content-Type` becomes
`text/plain; charset=utf-8`. Bodies are suppressed for `HEAD`, `204`, `205`,
and `304`.

Dynamic code receives a prepared `*http.Response` with the current request
attached. It sets `StatusCode`, `Header`, and `Body`; the body must be an
`io.ReadCloser`, and the handler reads and closes it. Yaegi errors, panics,
invalid statuses, and invalid headers return `500`. Code is compiled during
server construction, and invalid configured code prevents startup.
`Code` cannot be combined with static status, headers, or body.

Dynamic code can inspect path values, query parameters, request headers, and
the request body via `response.Request`. Fragments are not transmitted in
HTTP. A listener middleware runs before matching and can consume the request
body; path captures are populated after middleware and before the response
code runs. As with all Yaegi code, dynamic source runs with the GORP process
privileges and must be trusted. `json.Marshal` bytes must be wrapped as an
`io.ReadCloser`, for example using `io.NopCloser(bytes.NewReader(body))`.

## Per-request overrides

The selected named response or default accepts:

- `X-Mock-Status` to replace status (`200..599`);
- `X-Mock-Body` to replace the body;
- `X-Mock-Response-Header-<Name>` to replace/add a response header.

Overrides are applied after static response construction or Yaegi execution.
HTTP framing headers such as `Content-Length`, `Transfer-Encoding`, and
`Connection` cannot be set.

## Admin API

The admin mux exposes these endpoints for configured mocksrv listeners:

- `GET /mocks/{listener}/routes` lists all routes;
- `PUT /mocks/{listener}/routes/{route}` creates or replaces a complete route;
- `DELETE /mocks/{listener}/routes/{route}` deletes a route.

The `PUT` body uses the `MockSrvRoute` JSON shape, including `match`,
`scenarios`, and optional `default`. It is limited to 1 MiB, rejects unknown
fields and trailing JSON objects, validates every response and compiles every
Yaegi function before atomically replacing the route. Changes are in-memory
only. The endpoints pass through admin middlewares.

## TLS and middleware

The listener uses the regular HTTP/HTTPS server setup. It supports TLS,
including mTLS, and listener-level HTTP middleware. Middleware runs before
route matching; request changes are visible to match criteria and dynamic
response code. Response middleware remains route-only. Backends and TCP
routes are not supported.
