# External listener integration tests

Run the end-to-end checks from the repository root or any other directory:

```bash
tests/listener-integration.sh
```

The script builds GORP, starts local echo backends, generates a temporary CA,
server and client certificates (including a revoked client certificate), and
removes the processes and files it created on exit. It requires Go, `curl`,
OpenSSL, Python 3, `netcat`, and `timeout`. All listeners bind to loopback and
the test uses dynamically allocated ports.

The shell prints a `PASS` line for each successful HTTP status, response field,
TCP exchange, or expected rejection. A failure prints the captured response
and the tail of the GORP log. The script also exercises the admin `/stop`
endpoint for graceful shutdown.

## Exit status

The script exits with status **0** only when all tests that can run in the
current environment pass. A failed assertion, failed command, missing
prerequisite, or cleanup error makes it exit nonzero; the `EXIT` handler
preserves a test failure's status while still stopping processes and removing
temporary files. HTTP/3 is reported as `SKIP` when curl lacks HTTP/3 support;
that unsupported check is not counted as a failure, so status 0 means all
executed checks passed, not that a skipped protocol was verified.

## What the integration script tests

### HTTP routing and listener behavior

- **HTTP route and backend:** `GET /echo/plain` reaches the admin `/echo`
  backend through the configured service and reports the stripped request path.
- **Controllable backend response:** requests through `/selector/code` use the
  admin `/code` endpoint to return a chosen status (`418`), body and response
  header.
- **Selectors:** dedicated routes use three backends targeting `/code`; the
  `response_affinity` middleware reports the selected backend index. The tests
  verify round-robin rotation, first-alive selection, direct header affinity,
  cookie/query/IP hashing, random selection across the pool, and successful
  selections from least-connections and power-of-two.
- **Prefix stripping:** `GET /api/echo/stripped` is forwarded as `/stripped`.
- **Request-header middleware:** the route adds `X-Integration-Test` before
  proxying; the echo response must contain it.
- **Response-header middleware:** the same route adds
  `X-Integration-Response` to the response.
- **Host forwarding:** a custom `Host` sent to the listener is visible as
  `X-Forwarded-Host` at the echo backend. This does not test host-based route
  selection.
- **Request-path middleware:** `add_prefix` is checked by adding `/echo` after
  route-prefix stripping and confirming the echo backend receives the adjusted
  path.
- **Request-header middleware:** `set_request_headers` and
  `modify_request_headers` replace client-supplied values; `remove_request_headers`
  removes a supplied header before proxying.
- **Authentication middleware:** `basic_auth` rejects missing and invalid
  credentials with `401` and accepts valid credentials; `token_auth` rejects a
  missing token with `401` and accepts the configured bearer token.
- **Correlation ID middleware:** `correlation_id` generates a UUIDv7 when no
  ID is sent and preserves a caller-provided `X-Correlation-Id`.
- **Distributed tracing middleware:** a valid W3C `traceparent` is generated,
  sent upstream and returned in the response.
- **CORS middleware:** an `OPTIONS` preflight returns `200` and includes
  `Access-Control-Allow-Origin: *`.
- **IP allow-list middleware:** a request from the local test client is
  accepted by the configured `127.0.0.1/32` allow-list. A denied source address
  is not tested.
- **Rate-limit middleware:** two requests from the test client succeed and the
  third request in the configured window receives `429`.
- **Dynamic request middleware:** the Go middleware from the configuration
  sets `X-From-Dynamic-Request`; the echo backend must receive it.
- **Dynamic response middleware:** the Go response modifier sets
  `X-From-Dynamic-Response`; curl checks the header returned to the client.
- **Secure access middleware:** accessing the protected resource without a
  cookie receives `410`; requesting the configured unlock path receives a
  redirect and signed cookie; sending that cookie grants access to the
  protected resource.
- **Response-header middleware:** `set_response_headers` replaces
  `Cache-Control`, `modify_response_headers` sets `Expires`, and
  `remove_response_headers` removes `Cache-Control`.
- **Response Location middleware:** a backend-generated `302` Location path is
  prefixed with `/public`, and a backend-generated `301` Location host is
  replaced with `public.example:8443`. Curl does not follow these redirects;
  the tests assert the returned status and `Location` header.
- **OpenAPI middleware:** a temporary contract allows `GET /allowed`, which is
  forwarded to the backend; `POST /allowed` is rejected with `405` and a
  request to the undocumented `/not-in-contract` path is rejected with `404`.
- **OpenID authentication middleware:** a temporary GORP HTTP listener uses
  the directory fallback to serve the provider discovery document and JWKS.
  The test checks a missing token and an invalid signature are rejected with
  `401`, then verifies a correctly signed RS256 token with the expected issuer
  and audience is accepted. `remove_header` is enabled and checked at the echo
  backend.
- **Delay middleware and connection limit:** a delayed request occupies the
  only configured HTTP connection slot; a concurrent request must receive
  `503`, and the delayed request must then finish with `200`.
- **Directory fallback:** requesting the generated `sample.txt` returns its
  known content when no route matches.
- **Request-body limit:** a POST larger than the configured limit receives
  `413`.
- **Unix HTTP listener:** a request through the generated Unix socket reaches
  the echo backend.
- **HTTPS and mutual TLS:** a client certificate signed by the generated CA is
  accepted; a missing certificate and a certificate listed in the generated
  CRL are rejected.
- **Dynamic HTTPS:** a request using a generated-CA-signed, SNI-specific
  certificate reaches the echo backend.
- **HTTP multi listener:** both cleartext HTTP and HTTPS work on the same
  listener address.
- **HTTP/3:** tested only when the installed curl advertises HTTP/3 support.
  Otherwise the script prints `SKIP`; it does not claim that HTTP/3 passed.

### Forward proxies

- **HTTP proxy authentication:** an unauthenticated request is rejected with
  `407`; valid Basic credentials allow an absolute-URL HTTP request.
- **HTTP proxy CONNECT:** an HTTPS request is tunneled through the proxy to the
  HTTPS listener, including its client-certificate check.
- **TLS-wrapped HTTP proxy:** an HTTP request works through the TLS proxy with
  trusted proxy certificate and valid proxy credentials.
- **TLS proxy CONNECT:** HTTPS CONNECT also works through the TLS-wrapped
  proxy.
- **SOCKS5:** an HTTP request succeeds with valid username/password
  authentication; invalid credentials are rejected.
- **HTTP proxy cascade:** a request traverses the configured local proxy to a
  second local proxy and then to the echo endpoint.

### TCP listeners

- **Raw TCP:** bytes sent through the listener are echoed by the plaintext
  backend.
- **Initial-byte validation:** an allowed prefix is forwarded; an invalid
  prefix closes the connection without reaching the echo backend.
- **TLS termination:** the TCP listener accepts a TLS connection and tunnels
  its plaintext payload to a non-TLS backend.
- **SNI routing:** `plain.test` and `secure.test` select distinct TCP backends.
- **TLS to backend:** `force_tls` connects to a TLS echo backend using the
  generated CA and configured server name.

## Middleware and selector coverage

**No: this script does not test every middleware.** It checks the middleware
behaviors marked **Tested** below. Selector strategies are all
explicitly configured and exercised, but the least-connections and
power-of-two checks only verify successful selection from the configured pool:
the fast `/code` backend does not create a controlled load difference. The IP
selector is checked from loopback only. These external checks do not replace Go
unit tests.

| Middleware | External script |
|---|---|
| `add_request_headers` | Tested |
| `add_response_headers` | Tested |
| `delay` | Tested |
| `add_prefix` / `addprefix` | Tested |
| `ip_allow_list` / `ipallowlist` / `ip_white_list` / `ipwhitelist` | Tested |
| `set_request_headers` | Tested |
| `modify_request_headers` | Tested |
| `remove_request_headers` | Tested |
| `correlation_id` | Tested |
| `distributed_tracing` / `tracing` / `trace_context` | Tested |
| `cors` | Tested |
| `log` / `logger` / `logging` | Not Tested |
| `rate` / `rate_limit` / `rate_limiter` | Tested |
| `dynamic_request` / `request_dynamic` | Tested |
| `dynamic_response` / `response_dynamic` | Tested |
| `secure_access` / `secureaccess` / `access_secure` | Tested |
| `basic_auth` / `auth_basic` | Tested |
| `token_auth` / `auth_token` | Tested |
| `auth_openid` / `openid_auth` / `auth_oidc` / `oidc_auth` | Tested |
| `digest_auth` / `auth_digest` | Not Tested |
| `openapi` / `openapi_validate` / `openapi_validation` | Tested |
| `set_response_headers` | Tested |
| `modify_response_headers` | Tested |
| `remove_response_headers` | Tested |
| `add_location_prefix` / `add_response_location_prefix` | Tested |
| `set_location_host` / `replace_location_host` / `set_response_location_host` | Tested |
| `response_affinity` | Tested |

| Selector | External script |
|---|---|
| `round-robin` | Tested: rotates across three backends |
| `random` | Tested: all three backends are selected across 30 requests |
| `first-alive` | Tested: selects the first available backend; unavailable-backend failover is not covered |
| `header` | Tested: opaque backend affinity index selects the requested backend |
| `cookie` | Tested: configured cookie hashes to the expected backend |
| `ip` / `source-ip` / `ip-hash` | Tested: loopback source IP hashes to the expected backend |
| `least-connections` | Tested: 10 successful selections from the configured pool; load balancing is not asserted |
| `power-of-two` | Tested: 10 successful selections from the configured pool; load balancing is not asserted |
| `query-param` | Tested: configured query parameter hashes to the expected backend |
