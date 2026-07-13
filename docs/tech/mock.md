# Mock listener

The `mock` listener is an HTTP server whose handler creates responses locally.
It does not create backend objects or pass requests through the proxy router.
TLS and listener request middlewares use the same server setup as the HTTPS and
HTTP listeners.

## Configuration model

`Listener.Mock.Scenarios` is a map keyed by the scenario name. A scenario has
optional request criteria (`method`, `path`, and exact header values) and a
response. A path can include named `:param` segments. Parameter names must be
unique within the pattern and match an identifier (`[A-Za-z_][A-Za-z0-9_]*`);
captured values are non-empty, single path segments. A response selects
exactly one representation:

- static fields: `status`, `headers`, and `body`;
- `response.code`: Yaegi source defining `dynamic.Middleware` as
  `func(*http.Response) error`.

The dynamic modifier receives a response with its `Request` field populated
with the current request. It can set `StatusCode`, `Header`, and `Body`;
`Body` must be an `io.ReadCloser`. Body readers are consumed and closed before
the handler writes the response. A
compiled modifier is stored with its scenario: configuration scenarios are
compiled while creating the server, and admin updates are compiled before
replacing the stored scenario. Invalid configured source causes server build
failure; invalid admin updates return `400`.

The static fields and `code` are mutually exclusive. Dynamic execution errors,
panics, invalid status codes, and invalid generated headers produce `500`.

## Request processing

The listener middleware chain runs before the mock handler, so configured
request headers and other request mutations are visible to both scenario
matching and response code. The handler then:

1. reads `X-Mock-Scenario` and obtains a snapshot of the named scenario;
2. evaluates the optional request criteria, matching literal path segments and
   capturing `:param` segments;
3. sets each captured value on the request with `Request.SetPathValue`, so code
   can read it with `response.Request.PathValue("param")`;
4. constructs the configured static response or applies its compiled dynamic
   modifier;
5. validates the generated status and headers;
6. applies per-request `X-Mock-Status`, `X-Mock-Body`, and
   `X-Mock-Response-Header-<Name>` overrides;
7. writes the response, suppressing the body for `HEAD`, `204`, `205`, and
   `304`.

Missing scenario selection returns `400`; unknown scenarios and non-matching
criteria return `404`. The scenario selector first checks for an exact name,
then applies `routing.MatchWildcard` to configured names. Patterns matching
multiple scenarios return `400`; this avoids depending on Go map iteration
order. The scenario map is protected by an RW mutex. Admin updates replace a
scenario atomically, so concurrent requests use either the old complete
scenario or the new complete scenario.

Listener middlewares run before mock matching and therefore do not see path
parameters assigned by the mock handler. Dynamic response code runs after
matching and can access them through `response.Request.PathValue`.

The dynamic evaluator runs user-supplied Go code with the privileges of the
GORP process. Treat code from configuration and admin updates as trusted input.
Generated headers are checked for valid field names/values and prohibited
framing headers before use.

## Admin API

The mock registry adds these routes only when at least one mock listener is
configured:

- `GET /mocks/{listener}/scenarios`
- `PUT /mocks/{listener}/scenarios/{scenario}`
- `DELETE /mocks/{listener}/scenarios/{scenario}`

These routes are installed on the admin mux and therefore pass through the
configured admin middleware chain. Updates are in-memory only. `PUT` limits the
JSON request body to 1 MiB, rejects unknown JSON fields, validates a single
JSON object, compiles dynamic code, and commits only after validation
succeeds.

Per-request overrides are applied after static response construction or code
execution. They cannot change HTTP framing headers such as `Content-Length`,
`Transfer-Encoding`, or `Connection`.
