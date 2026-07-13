#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/gorp-listener-integration.XXXXXX")
GORP_PID=
OIDC_PID=
ECHO_PIDS=()
CURL=(curl --silent --show-error --max-time 8)
export NO_PROXY=127.0.0.1,localhost
export no_proxy=$NO_PROXY

fail() {
	TEST_FAILED=1
	printf 'FAIL: %s\n' "$*" >&2
	if [[ -f "$WORK/response" ]]; then
		printf '%s\n' '--- response body ---' >&2
		cat "$WORK/response" >&2
		printf '\n' >&2
	fi
	exit 1
}

cleanup() {
	local status=$? pid
	trap - EXIT ERR
	if [[ -n "$GORP_PID" ]] && kill -0 "$GORP_PID" 2>/dev/null; then
		curl --silent --max-time 2 "http://127.0.0.1:$ADMIN_PORT/stop" >/dev/null 2>&1 || true
		sleep 0.2
		kill "$GORP_PID" 2>/dev/null || true
		wait "$GORP_PID" 2>/dev/null || true
	fi
	for pid in "${ECHO_PIDS[@]}"; do
		kill "$pid" 2>/dev/null || true
		wait "$pid" 2>/dev/null || true
	done
	if [[ -n "$OIDC_PID" ]] && kill -0 "$OIDC_PID" 2>/dev/null; then
		kill "$OIDC_PID" 2>/dev/null || true
		wait "$OIDC_PID" 2>/dev/null || true
	fi
	if [[ ${TEST_FAILED:-0} == 1 && -f "$WORK/gorp.log" ]]; then
		printf '\n--- gorp log ---\n' >&2
		tail -80 "$WORK/gorp.log" >&2
	fi
	if [[ ${TEST_FAILED:-0} == 1 && -f "$WORK/oidc-server.log" ]]; then
		printf '\n--- OpenID directory-fallback server log ---\n' >&2
		tail -40 "$WORK/oidc-server.log" >&2
	fi
	if ! rm -rf -- "$WORK"; then
		printf 'Failed to remove temporary directory %s\n' "$WORK" >&2
		status=1
	fi
	exit "$status"
}
trap cleanup EXIT

on_error() {
	TEST_FAILED=1
	printf 'Integration test failed at line %s\n' "$1" >&2
	exit 1
}
trap 'on_error "$LINENO"' ERR

for command in curl openssl python3 nc timeout go; do
	command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

mapfile -t PORTS < <(python3 - <<'PY'
import socket

sockets = []
for _ in range(20):
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    sockets.append(sock)
for sock in sockets:
    print(sock.getsockname()[1])
for sock in sockets:
    sock.close()
PY
)
if [[ ${#PORTS[@]} != 20 ]]; then
	fail "could not allocate temporary ports"
fi
ADMIN_PORT=${PORTS[0]}
HTTP_PORT=${PORTS[1]}
HTTPS_PORT=${PORTS[2]}
DYNAMIC_PORT=${PORTS[3]}
MULTI_PORT=${PORTS[4]}
HTTP3_PORT=${PORTS[5]}
PROXY_PORT=${PORTS[6]}
PROXY_TLS_PORT=${PORTS[7]}
SOCKS_PORT=${PORTS[8]}
PROXY_INNER_PORT=${PORTS[9]}
PROXY_CASCADE_PORT=${PORTS[10]}
TCP_PORT=${PORTS[11]}
TCP_TLS_PORT=${PORTS[12]}
TCP_CHECK_PORT=${PORTS[13]}
TCP_SNI_PORT=${PORTS[14]}
TCP_FORCE_TLS_PORT=${PORTS[15]}
ECHO_PLAIN_PORT=${PORTS[16]}
ECHO_ROUTE_PORT=${PORTS[17]}
ECHO_TLS_PORT=${PORTS[18]}
OIDC_PORT=${PORTS[19]}

CA_CERT="$WORK/ca.crt"
CA_KEY="$WORK/ca.key"
SERVER_CERT="$WORK/server.crt"
SERVER_KEY="$WORK/server.key"

openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 2 \
	-subj "/CN=GORP integration test CA" \
	-addext "basicConstraints=critical,CA:TRUE" \
	-addext "keyUsage=critical,keyCertSign,cRLSign" \
	-keyout "$CA_KEY" -out "$CA_CERT" >/dev/null 2>&1

openssl req -new -newkey rsa:2048 -sha256 -nodes \
	-subj "/CN=localhost" -keyout "$SERVER_KEY" -out "$WORK/server.csr" >/dev/null 2>&1
cat >"$WORK/server.ext" <<'EOF'
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:localhost,DNS:service.test,DNS:plain.test,DNS:secure.test,IP:127.0.0.1
EOF
openssl x509 -req -sha256 -days 2 -in "$WORK/server.csr" \
	-CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial \
	-extfile "$WORK/server.ext" -out "$SERVER_CERT" >/dev/null 2>&1

mkdir "$WORK/newcerts"
: >"$WORK/index.txt"
printf '1000\n' >"$WORK/ca.srl"
printf '1000\n' >"$WORK/crlnumber"
cat >"$WORK/ca.cnf" <<EOF
[ ca ]
default_ca = CA_default
[ CA_default ]
database = $WORK/index.txt
new_certs_dir = $WORK/newcerts
certificate = $CA_CERT
private_key = $CA_KEY
serial = $WORK/ca.srl
crlnumber = $WORK/crlnumber
default_md = sha256
default_days = 2
default_crl_days = 1
policy = policy
[ policy ]
commonName = supplied
[ client_cert ]
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature
extendedKeyUsage = clientAuth
subjectAltName = DNS:client.test
[ crl_ext ]
authorityKeyIdentifier = keyid:always
EOF

for client in allowed revoked; do
	openssl req -new -newkey rsa:2048 -sha256 -nodes \
		-subj "/CN=$client" -keyout "$WORK/$client.key" -out "$WORK/$client.csr" >/dev/null 2>&1
	openssl ca -batch -config "$WORK/ca.cnf" -extensions client_cert \
		-in "$WORK/$client.csr" -out "$WORK/$client.crt" >/dev/null 2>&1
done
openssl ca -batch -config "$WORK/ca.cnf" -revoke "$WORK/revoked.crt" >/dev/null 2>&1
openssl ca -batch -config "$WORK/ca.cnf" -gencrl -out "$WORK/revoked.crl" >/dev/null 2>&1

mkdir "$WORK/www"
printf 'directory fallback works\n' >"$WORK/www/sample.txt"
cat >"$WORK/openapi.yaml" <<'EOF'
openapi: 3.1.0
info:
  title: Integration test API
  version: 1.0.0
paths:
  /allowed:
    get:
      responses:
        "200":
          description: OK
EOF

OIDC_ISSUER="http://127.0.0.1:$OIDC_PORT/oidc"
OIDC_JWKS_URI="$OIDC_ISSUER/jwks.json"
mkdir -p "$WORK/www/oidc/.well-known"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
	-out "$WORK/oidc-private.pem" >/dev/null 2>&1
python3 - "$WORK/oidc-private.pem" "$WORK/www/oidc/jwks.json" \
	"$WORK/oidc-valid.jwt" "$WORK/oidc-invalid.jwt" "$OIDC_ISSUER" "$OIDC_JWKS_URI" <<'PY'
import base64
import json
import subprocess
import sys
import time

private_key, jwks_path, valid_path, invalid_path, issuer, jwks_uri = sys.argv[1:]
modulus_line = subprocess.run(
    ["openssl", "rsa", "-in", private_key, "-noout", "-modulus"],
    check=True,
    capture_output=True,
    text=True,
).stdout.strip()
modulus = bytes.fromhex(modulus_line.partition("=")[2]).lstrip(b"\0")
b64url = lambda value: base64.urlsafe_b64encode(value).rstrip(b"=").decode("ascii")
with open(jwks_path, "w", encoding="utf-8") as jwks:
    json.dump(
        {"keys": [{"kty": "RSA", "use": "sig", "alg": "RS256",
                   "kid": "integration-key", "n": b64url(modulus), "e": "AQAB"}]},
        jwks,
    )
with open(
    sys.argv[2].rsplit("/", 1)[0] + "/.well-known/openid-configuration",
    "w",
    encoding="utf-8",
) as discovery:
    json.dump({"issuer": issuer, "jwks_uri": jwks_uri}, discovery)

header = b64url(json.dumps({"alg": "RS256", "typ": "JWT"}, separators=(",", ":")).encode())
claims = b64url(
    json.dumps(
        {"iss": issuer, "aud": "integration-client", "iat": int(time.time()),
         "exp": int(time.time()) + 600},
        separators=(",", ":"),
    ).encode()
)
signing_input = f"{header}.{claims}".encode()
signature = subprocess.run(
    ["openssl", "dgst", "-sha256", "-sign", private_key],
    input=signing_input,
    check=True,
    capture_output=True,
).stdout
with open(valid_path, "w", encoding="ascii") as token:
    token.write(f"{signing_input.decode()}.{b64url(signature)}\n")
with open(invalid_path, "w", encoding="ascii") as token:
    token.write(f"{signing_input.decode()}.{b64url(bytes(len(signature)))}\n")
PY

cat >"$WORK/oidc-server.yaml" <<EOF
directory: "$WORK/www"
listeners:
  - name: oidc-static
    type: http
    address: "127.0.0.1:$OIDC_PORT"
EOF

start_echo() {
	local port=$1 prefix=$2 cert=${3:-} key=${4:-}
	if [[ -n "$cert" ]]; then
		python3 "$ROOT/tests/tcp-echo.py" "$port" "$prefix" "$cert" "$key" >"$WORK/echo-$port.log" 2>&1 &
	else
		python3 "$ROOT/tests/tcp-echo.py" "$port" "$prefix" >"$WORK/echo-$port.log" 2>&1 &
	fi
	ECHO_PIDS+=("$!")
}
start_echo "$ECHO_PLAIN_PORT" "plain:"
start_echo "$ECHO_ROUTE_PORT" "route:"
start_echo "$ECHO_TLS_PORT" "secure:" "$SERVER_CERT" "$SERVER_KEY"

go -C "$ROOT" build -o "$WORK/gorp" ./cmd/gorp
"$WORK/gorp" -config "$WORK/oidc-server.yaml" >"$WORK/oidc-server.log" 2>&1 &
OIDC_PID=$!
for _ in $(seq 1 80); do
	if curl --silent --show-error --max-time 2 --noproxy '*' \
		"$OIDC_ISSUER/.well-known/openid-configuration" -o "$WORK/oidc-discovery.json" 2>/dev/null \
		&& curl --silent --show-error --max-time 2 --noproxy '*' \
			"$OIDC_JWKS_URI" -o "$WORK/oidc-jwks.json" 2>/dev/null; then
		break
	fi
	if ! kill -0 "$OIDC_PID" 2>/dev/null; then
		fail "local OpenID directory-fallback server did not start"
	fi
	sleep 0.1
done
python3 - "$WORK/oidc-discovery.json" "$WORK/oidc-jwks.json" <<'PY' || fail "local OpenID discovery or JWKS document is invalid"
import json
import sys

with open(sys.argv[1], encoding="utf-8") as discovery_file:
    discovery = json.load(discovery_file)
with open(sys.argv[2], encoding="utf-8") as jwks_file:
    jwks = json.load(jwks_file)
if not discovery.get("jwks_uri") or not jwks.get("keys"):
    raise SystemExit(1)
PY

cat >"$WORK/config.yaml" <<EOF
admin:
  enabled: true
  address: "127.0.0.1:$ADMIN_PORT"
directory: "$WORK/www"
listeners:
  - name: http
    type: http
    address: "127.0.0.1:$HTTP_PORT"
    read_header_timeout: 2s
    write_timeout: 5s
    idle_timeout: 5s
    max_connections: 1
    max_request_body_size: 8
  - name: https
    type: https
    address: "127.0.0.1:$HTTPS_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
      ca_file: "$CA_CERT"
      crl_file: "$WORK/revoked.crl"
  - name: dynamic
    type: dynamic
    address: "127.0.0.1:$DYNAMIC_PORT"
    tls:
      cert_file: "$CA_CERT"
      key_file: "$CA_KEY"
  - name: multi
    type: multi
    address: "127.0.0.1:$MULTI_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
  - name: http3
    type: http3
    address: "127.0.0.1:$HTTP3_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
  - name: unix-http
    type: unix
    address: "$WORK/http.sock"
  - name: forward-proxy
    type: proxy
    address: "127.0.0.1:$PROXY_PORT"
    users:
      alice: secret
  - name: tls-forward-proxy
    type: proxy_tls
    address: "127.0.0.1:$PROXY_TLS_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
    users:
      alice: secret
  - name: socks5
    type: socks5
    address: "127.0.0.1:$SOCKS_PORT"
    users:
      - alice: secret
  - name: cascade-upstream
    type: proxy
    address: "127.0.0.1:$PROXY_INNER_PORT"
  - name: cascade-proxy
    type: proxy
    address: "127.0.0.1:$PROXY_CASCADE_PORT"
    proxy:
      url: "http://127.0.0.1:$PROXY_INNER_PORT"
      hosts:
        - "127.0.0.1"
  - name: tcp
    type: tcp
    address: "127.0.0.1:$TCP_PORT"
    backends:
      - name: raw-echo
        url: "tcp://127.0.0.1:$ECHO_PLAIN_PORT"
  - name: tcp-tls
    type: tcp
    address: "127.0.0.1:$TCP_TLS_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
    backends:
      - name: tls-terminated-echo
        url: "tcp://127.0.0.1:$ECHO_PLAIN_PORT"
  - name: tcp-check
    type: tcp
    address: "127.0.0.1:$TCP_CHECK_PORT"
    initial_bytes:
      - "P"
      - "I"
    backends:
      - name: checked-echo
        url: "tcp://127.0.0.1:$ECHO_PLAIN_PORT"
  - name: tcp-sni
    type: tcp
    address: "127.0.0.1:$TCP_SNI_PORT"
    tls:
      cert_file: "$SERVER_CERT"
      key_file: "$SERVER_KEY"
    backends:
      - name: fallback
        url: "tcp://127.0.0.1:$ECHO_PLAIN_PORT"
    routes:
      - name: plain-sni
        hosts:
          - "plain.test"
        backends:
          - name: plain-sni-backend
            url: "tcp://127.0.0.1:$ECHO_PLAIN_PORT"
      - name: secure-sni
        hosts:
          - "secure.test"
        backends:
          - name: secure-sni-backend
            url: "tcp://127.0.0.1:$ECHO_ROUTE_PORT"
  - name: tcp-force-tls
    type: tcp
    address: "127.0.0.1:$TCP_FORCE_TLS_PORT"
    backends:
      - name: tls-upstream
        url: "tcp://127.0.0.1:$ECHO_TLS_PORT"
        force_tls: true
        tls:
          ca_file: "$CA_CERT"
          server_name: localhost
services:
  - name: admin-echo
    backends:
      - name: admin-echo
        url: "http://127.0.0.1:$ADMIN_PORT"
  - name: selector-backends
    backends:
      - name: selector-backend-0
        url: "http://127.0.0.1:$ADMIN_PORT/code"
      - name: selector-backend-1
        url: "http://127.0.0.1:$ADMIN_PORT/code"
      - name: selector-backend-2
        url: "http://127.0.0.1:$ADMIN_PORT/code"
  - name: openapi-backend
    backends:
      - name: openapi-backend
        url: "http://127.0.0.1:$ADMIN_PORT/code/200"
routes:
  - name: api
    prefix: "/api"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: add_request_headers
        config:
          headers:
            X-Integration-Test: passed
      - name: add_response_headers
        config:
          headers:
            X-Integration-Response: passed
  - name: delayed
    prefix: "/slow"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: delay
        config:
          duration: 2s
  - name: add-prefix
    prefix: "/mw/prefix"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: add_prefix
        config:
          prefix: "/echo"
  - name: basic-auth
    prefix: "/mw/basic"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: basic_auth
        config:
          realm: "Integration"
          users:
            alice: secret
  - name: token-auth
    prefix: "/mw/token"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: token_auth
        config:
          tokens:
            - integration-token
  - name: set-request-header
    prefix: "/mw/set-request"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: set_request_headers
        config:
          headers:
            X-Request-Set: configured
  - name: modify-request-header
    prefix: "/mw/modify-request"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: modify_request_headers
        config:
          headers:
            X-Request-Modified: configured
  - name: remove-request-header
    prefix: "/mw/remove-request"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: remove_request_headers
        config:
          headers:
            - X-Request-Removed
  - name: correlation-id
    prefix: "/mw/correlation"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: correlation_id
        config:
          type: uuidv7
  - name: distributed-tracing
    prefix: "/mw/tracing"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: distributed_tracing
  - name: cors
    prefix: "/mw/cors"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: cors
  - name: ip-allow-list
    prefix: "/mw/ip-allow"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: ip_allow_list
        config:
          sources:
            - 127.0.0.1/32
  - name: rate-limit
    prefix: "/mw/rate"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: rate_limit
        config:
          max_requests: 2
          window: 30s
  - name: dynamic-request
    prefix: "/mw/dynamic-request"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: dynamic_request
        config:
          code: |
            package dynamic

            import "net/http"

            func Middleware(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                    r.Header.Set("X-From-Dynamic-Request", "true")
                    next.ServeHTTP(w, r)
                })
            }
  - name: dynamic-response
    prefix: "/mw/dynamic-response"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: dynamic_response
        config:
          code: |
            package dynamic

            import "net/http"

            func Middleware(response *http.Response) error {
                response.Header.Set("X-From-Dynamic-Response", "true")
                return nil
            }
  - name: secure-access
    prefix: "/mw/secure"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: secure_access
        config:
          path: "/unlock"
          key: "integration-secret"
          redirect: "/mw/secure/resource"
          duration: 1m
      - name: add_prefix
        config:
          prefix: "/echo"
  - name: set-response-header
    prefix: "/mw/set-response"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: set_response_headers
        config:
          headers:
            Cache-Control: configured
  - name: modify-response-header
    prefix: "/mw/modify-response"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: modify_response_headers
        config:
          headers:
            Expires: configured
  - name: remove-response-header
    prefix: "/mw/remove-response"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: remove_response_headers
        config:
          headers:
            - Cache-Control
  - name: add-location-prefix
    prefix: "/mw/location-prefix"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: add_location_prefix
        config:
          prefix: "/public"
  - name: set-location-host
    prefix: "/mw/location-host"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: set_location_host
        config:
          host: "public.example:8443"
  - name: openapi
    prefix: "/mw/openapi"
    strip_prefix: true
    service: openapi-backend
    middlewares:
      - name: openapi
        config:
          file: "$WORK/openapi.yaml"
  - name: openid
    prefix: "/mw/openid"
    strip_prefix: true
    service: admin-echo
    middlewares:
      - name: auth_openid
        config:
          issuer: "$OIDC_ISSUER"
          audience: integration-client
          algorithm: RS256
          remove_header: true
  - name: echo
    prefix: "/echo"
    service: admin-echo
  - name: selector-code
    prefix: "/selector/code"
    strip_prefix: true
    service: selector-backends
    selector:
      type: first-alive
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-round-robin
    prefix: "/selector/round-robin"
    strip_prefix: true
    service: selector-backends
    selector:
      type: round-robin
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-random
    prefix: "/selector/random"
    strip_prefix: true
    service: selector-backends
    selector:
      type: random
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-first-alive
    prefix: "/selector/first-alive"
    strip_prefix: true
    service: selector-backends
    selector:
      type: first-alive
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-header
    prefix: "/selector/header"
    strip_prefix: true
    service: selector-backends
    selector:
      type: header
      config:
        name: X-Select-Backend
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-cookie
    prefix: "/selector/cookie"
    strip_prefix: true
    service: selector-backends
    selector:
      type: cookie
      config:
        name: selector-session
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-ip
    prefix: "/selector/ip"
    strip_prefix: true
    service: selector-backends
    selector:
      type: ip
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-least-connections
    prefix: "/selector/least-connections"
    strip_prefix: true
    service: selector-backends
    selector:
      type: least-connections
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-power-of-two
    prefix: "/selector/power-of-two"
    strip_prefix: true
    service: selector-backends
    selector:
      type: power-of-two
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
  - name: selector-query
    prefix: "/selector/query"
    strip_prefix: true
    service: selector-backends
    selector:
      type: query-param
      config:
        name: tenant
    middlewares:
      - name: response_affinity
        config:
          header: X-Selector-Backend
EOF

"$WORK/gorp" -config "$WORK/config.yaml" >"$WORK/gorp.log" 2>&1 &
GORP_PID=$!

wait_http() {
	local url=$1
	for _ in $(seq 1 80); do
		if "${CURL[@]}" --noproxy '*' "$url" >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.1
	done
	return 1
}
wait_http "http://127.0.0.1:$ADMIN_PORT/health" || fail "admin listener did not start"
wait_http "http://127.0.0.1:$HTTP_PORT/echo/ready" || fail "HTTP listener did not start"

assert_status() {
	local expected=$1
	shift
	local actual request
	request=${!#}
	actual=$("${CURL[@]}" -D "$WORK/headers" -o "$WORK/response" -w '%{http_code}' "$@") || fail "curl request failed: $*"
	[[ "$actual" == "$expected" ]] || fail "HTTP status was $actual, expected $expected ($*)"
	printf '  PASS: HTTP %s %s\n' "$expected" "$request"
}

assert_json_field() {
	local field=$1 expected=$2
	python3 - "$WORK/response" "$field" "$expected" <<'PY' || fail "JSON field $field did not equal $expected"
import json
import sys

with open(sys.argv[1], encoding="utf-8") as response:
    value = json.load(response)
for part in sys.argv[2].split("."):
    value = value[part]
if isinstance(value, list):
    if sys.argv[3] not in value:
        raise SystemExit(1)
elif str(value) != sys.argv[3]:
    raise SystemExit(1)
PY
	printf '  PASS: JSON field %s = %s\n' "$field" "$expected"
}

assert_json_missing() {
	local field=$1
	python3 - "$WORK/response" "$field" <<'PY' || fail "JSON field $field was unexpectedly present"
import json
import sys

with open(sys.argv[1], encoding="utf-8") as response:
    value = json.load(response)
for part in sys.argv[2].split("."):
    if not isinstance(value, dict) or part not in value:
        raise SystemExit(0)
    value = value[part]
raise SystemExit(1)
PY
	printf '  PASS: JSON field %s is absent\n' "$field"
}

assert_json_matches() {
	local field=$1 pattern=$2
	python3 - "$WORK/response" "$field" "$pattern" <<'PY' || fail "JSON field $field did not match $pattern"
import json
import re
import sys

with open(sys.argv[1], encoding="utf-8") as response:
    value = json.load(response)
for part in sys.argv[2].split("."):
    value = value[part]
if isinstance(value, list):
    value = value[0] if value else ""
if not re.fullmatch(sys.argv[3], str(value)):
    raise SystemExit(1)
PY
	printf '  PASS: JSON field %s matches the expected format\n' "$field"
}

assert_contains() {
	grep -Fq -- "$2" "$1" || fail "expected '$2' in $1"
	printf '  PASS: response contains %s\n' "$2"
}

assert_not_contains() {
	grep -Fqi -- "$2" "$1" && fail "unexpected '$2' in $1"
	printf '  PASS: response does not contain %s\n' "$2"
}

assert_header_matches() {
	local header=$1 pattern=$2
	python3 - "$WORK/headers" "$header" "$pattern" <<'PY' || fail "header $header did not match $pattern"
import re
import sys

with open(sys.argv[1], encoding="latin-1") as headers:
    for line in headers:
        name, separator, value = line.partition(":")
        if separator and name.lower() == sys.argv[2].lower():
            if re.fullmatch(sys.argv[3], value.strip()):
                raise SystemExit(0)
            break
raise SystemExit(1)
PY
	printf '  PASS: response header %s matches the expected format\n' "$header"
}

assert_header_equals() {
	local header=$1 expected=$2
	python3 - "$WORK/headers" "$header" "$expected" <<'PY' || fail "header $header did not equal $expected"
import sys

with open(sys.argv[1], encoding="latin-1") as headers:
    for line in headers:
        name, separator, value = line.partition(":")
        if separator and name.lower() == sys.argv[2].lower():
            if value.strip() == sys.argv[3]:
                raise SystemExit(0)
            break
raise SystemExit(1)
PY
	printf '  PASS: response header %s = %s\n' "$header" "$expected"
}

read_selector_index() {
	python3 - "$WORK/headers" <<'PY' || fail "selector affinity header is missing or invalid"
import re
import sys

with open(sys.argv[1], encoding="latin-1") as headers:
    for line in headers:
        name, separator, value = line.partition(":")
        if separator and name.lower() == "x-selector-backend":
            match = re.fullmatch(r"__gorp_backend_index:([0-2])", value.strip())
            if match:
                print(match.group(1))
                raise SystemExit(0)
            break
raise SystemExit(1)
PY
}

hash_index() {
	python3 - "$1" <<'PY'
import sys

value = 2166136261
for byte in sys.argv[1].encode():
    value = ((value ^ byte) * 16777619) & 0xFFFFFFFF
print(value % 3)
PY
}

printf 'Test: HTTP routing, middleware, directory fallback and body limit\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/echo/plain"
assert_json_field path /plain
assert_status 200 "http://127.0.0.1:$HTTP_PORT/api/echo/stripped"
assert_json_field path /stripped
assert_json_field headers.X-Integration-Test passed
printf 'Test: admin /code controls backend status, headers and body\n'
assert_status 418 -H "X-Body: selector controlled body" \
	-H "X-Headers: X-Code-Controlled:yes" \
	"http://127.0.0.1:$HTTP_PORT/selector/code/418"
assert_contains "$WORK/response" "selector controlled body"
assert_header_equals X-Code-Controlled yes

printf 'Test: configured HTTP selectors\n'
for index in 0 1 2; do
	assert_status 201 -H "X-Body: selector-controlled" \
		"http://127.0.0.1:$HTTP_PORT/selector/round-robin/201"
	assert_contains "$WORK/response" "selector-controlled"
	assert_header_equals X-Selector-Backend "__gorp_backend_index:$index"
done
assert_status 202 "http://127.0.0.1:$HTTP_PORT/selector/first-alive/202"
assert_header_equals X-Selector-Backend "__gorp_backend_index:0"
assert_status 202 -H "X-Select-Backend: __gorp_backend_index:2" \
	"http://127.0.0.1:$HTTP_PORT/selector/header/202"
assert_header_equals X-Selector-Backend "__gorp_backend_index:2"
cookie_index=$(hash_index selector-cookie-key)
assert_status 202 -b "selector-session=selector-cookie-key" \
	"http://127.0.0.1:$HTTP_PORT/selector/cookie/202"
assert_header_equals X-Selector-Backend "__gorp_backend_index:$cookie_index"
ip_index=$(hash_index 127.0.0.1)
assert_status 202 "http://127.0.0.1:$HTTP_PORT/selector/ip/202"
assert_header_equals X-Selector-Backend "__gorp_backend_index:$ip_index"
query_index=$(hash_index selector-query-key)
assert_status 202 \
	"http://127.0.0.1:$HTTP_PORT/selector/query/202?tenant=selector-query-key"
assert_header_equals X-Selector-Backend "__gorp_backend_index:$query_index"
for selector in least-connections power-of-two; do
	for _ in $(seq 1 10); do
		assert_status 202 "http://127.0.0.1:$HTTP_PORT/selector/$selector/202" >/dev/null
		selected_index=$(read_selector_index)
		[[ "$selected_index" =~ ^[0-2]$ ]] || fail "$selector selected invalid backend index $selected_index"
	done
	printf '  PASS: %s returns a configured backend for 10 requests\n' "$selector"
done
: >"$WORK/random-indexes"
for _ in $(seq 1 30); do
	assert_status 202 "http://127.0.0.1:$HTTP_PORT/selector/random/202" >/dev/null
	read_selector_index >>"$WORK/random-indexes"
done
python3 - "$WORK/random-indexes" <<'PY' || fail "random selector did not sample all three backends"
import sys

with open(sys.argv[1], encoding="ascii") as indexes:
    if set(indexes.read().split()) != {"0", "1", "2"}:
        raise SystemExit(1)
PY
printf '  PASS: random selector samples all three backends in 30 requests\n'
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/api/response-header"
assert_contains "$WORK/headers" "X-Integration-Response: passed"
assert_status 200 -H "Host: custom.test" "http://127.0.0.1:$HTTP_PORT/echo/host"
assert_json_field headers.X-Forwarded-Host custom.test
printf 'Test: request-path and request-header middleware\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/prefix/prefix-check"
assert_json_field path /prefix-check
assert_status 200 -H "X-Request-Set: client-value" \
	"http://127.0.0.1:$HTTP_PORT/mw/set-request/echo/set-header"
assert_json_field headers.X-Request-Set configured
assert_status 200 -H "X-Request-Modified: client-value" \
	"http://127.0.0.1:$HTTP_PORT/mw/modify-request/echo/modify-header"
assert_json_field headers.X-Request-Modified configured
assert_status 200 -H "X-Request-Removed: must-not-reach-backend" \
	"http://127.0.0.1:$HTTP_PORT/mw/remove-request/echo/remove-header"
assert_json_missing headers.X-Request-Removed
printf 'Test: authentication middleware\n'
assert_status 401 "http://127.0.0.1:$HTTP_PORT/mw/basic/echo/basic-no-auth"
assert_status 401 -u alice:wrong "http://127.0.0.1:$HTTP_PORT/mw/basic/echo/basic-bad-auth"
assert_status 200 -u alice:secret "http://127.0.0.1:$HTTP_PORT/mw/basic/echo/basic-auth"
assert_json_field path /basic-auth
assert_status 401 "http://127.0.0.1:$HTTP_PORT/mw/token/echo/token-no-auth"
assert_status 200 -H "Authorization: Bearer integration-token" \
	"http://127.0.0.1:$HTTP_PORT/mw/token/echo/token-auth"
assert_json_field path /token-auth
printf 'Test: correlation ID, tracing, CORS and source-IP allow list\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/correlation/echo/correlation-generated"
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/correlation/echo/correlation-generated" \
	|| fail "correlation ID request failed"
assert_header_matches X-Correlation-Id '[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}'
assert_status 200 -H "X-Correlation-Id: supplied-correlation-id" \
	"http://127.0.0.1:$HTTP_PORT/mw/correlation/echo/correlation-preserved"
assert_json_field headers.X-Correlation-Id supplied-correlation-id
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/tracing/echo/trace"
assert_json_matches headers.Traceparent '00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}'
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/tracing/echo/trace" || fail "tracing request failed"
assert_contains "$WORK/headers" "Traceparent: 00-"
assert_status 200 -X OPTIONS -H "Origin: https://client.example" \
	"http://127.0.0.1:$HTTP_PORT/mw/cors/preflight"
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -X OPTIONS -H "Origin: https://client.example" \
	-D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/cors/preflight" || fail "CORS preflight request failed"
assert_contains "$WORK/headers" "Access-Control-Allow-Origin: *"
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/ip-allow/echo/allowed"
assert_json_field path /allowed
printf 'Test: per-route rate limit\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/rate/echo/one"
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/rate/echo/two"
assert_status 429 "http://127.0.0.1:$HTTP_PORT/mw/rate/echo/three"
printf 'Test: dynamic request and response middleware\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/dynamic-request/echo/request"
assert_json_field headers.X-From-Dynamic-Request true
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/dynamic-response/echo/response" \
	|| fail "dynamic response request failed"
assert_contains "$WORK/headers" "X-From-Dynamic-Response: true"
printf 'Test: secure_access cookie and redirect flow\n'
assert_status 410 "http://127.0.0.1:$HTTP_PORT/mw/secure/resource"
assert_status 302 -c "$WORK/secure.cookies" \
	"http://127.0.0.1:$HTTP_PORT/mw/secure/unlock"
assert_status 200 -b "$WORK/secure.cookies" \
	"http://127.0.0.1:$HTTP_PORT/mw/secure/resource"
assert_json_field path /resource
printf 'Test: response-header middleware\n'
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/set-response/echo/headers" || fail "set response header request failed"
assert_contains "$WORK/headers" "Cache-Control: configured"
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/modify-response/echo/headers" || fail "modify response header request failed"
assert_contains "$WORK/headers" "Expires: configured"
: >"$WORK/headers"
"${CURL[@]}" --noproxy '*' -D "$WORK/headers" -o "$WORK/response" \
	"http://127.0.0.1:$HTTP_PORT/mw/remove-response/echo/headers" || fail "remove response header request failed"
assert_not_contains "$WORK/headers" "Cache-Control:"
printf 'Test: response Location middleware\n'
assert_status 302 -H "X-Location: https://origin.example/old/path?source=test" \
	"http://127.0.0.1:$HTTP_PORT/mw/location-prefix/code/302"
assert_header_equals Location "https://origin.example/public/old/path?source=test"
assert_status 301 -H "X-Location: https://origin.example/redirect?next=%2Fhome" \
	"http://127.0.0.1:$HTTP_PORT/mw/location-host/code/301"
assert_header_equals Location "https://public.example:8443/redirect?next=%2Fhome"
printf 'Test: OpenAPI method and path validation\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/mw/openapi/allowed"
assert_contains "$WORK/response" OK
assert_status 405 -X POST "http://127.0.0.1:$HTTP_PORT/mw/openapi/allowed"
assert_status 404 "http://127.0.0.1:$HTTP_PORT/mw/openapi/not-in-contract"
printf 'Test: OpenID discovery, JWKS and JWT authentication\n'
assert_status 401 "http://127.0.0.1:$HTTP_PORT/mw/openid/echo/no-token"
oidc_invalid_token=$(<"$WORK/oidc-invalid.jwt")
assert_status 401 -H "Authorization: Bearer $oidc_invalid_token" \
	"http://127.0.0.1:$HTTP_PORT/mw/openid/echo/invalid-token"
oidc_valid_token=$(<"$WORK/oidc-valid.jwt")
assert_status 200 -H "Authorization: Bearer $oidc_valid_token" \
	"http://127.0.0.1:$HTTP_PORT/mw/openid/echo/valid-token"
assert_json_field path /valid-token
assert_json_missing headers.Authorization
printf 'Test: HTTP connection limit\n'
"${CURL[@]}" --noproxy '*' -o "$WORK/slow-response" -w '%{http_code}' \
	"http://127.0.0.1:$HTTP_PORT/slow/echo/held" >"$WORK/slow-status" &
slow_request_pid=$!
limited_status=
for _ in $(seq 1 30); do
	limited_status=$("${CURL[@]}" --noproxy '*' -o /dev/null -w '%{http_code}' \
		"http://127.0.0.1:$HTTP_PORT/echo/concurrent") || fail "concurrent HTTP request failed"
	if [[ "$limited_status" == 503 ]]; then
		break
	fi
	sleep 0.05
done
[[ "$limited_status" == 503 ]] || fail "HTTP max_connections did not reject a concurrent request"
printf '  PASS: max_connections rejects a concurrent request with HTTP 503\n'
wait "$slow_request_pid"
[[ $(<"$WORK/slow-status") == 200 ]] || fail "delayed HTTP request did not complete successfully"
printf '  PASS: delayed HTTP request completes after the active slot is released\n'
assert_status 200 "http://127.0.0.1:$HTTP_PORT/sample.txt"
assert_contains "$WORK/response" "directory fallback works"
assert_status 413 -X POST --data-binary "123456789" "http://127.0.0.1:$HTTP_PORT/echo/too-large"
assert_status 200 --unix-socket "$WORK/http.sock" "http://localhost/echo/unix"
assert_json_field path /unix

printf 'Test: HTTPS client authentication and certificate revocation\n'
assert_status 200 --cacert "$CA_CERT" --cert "$WORK/allowed.crt" --key "$WORK/allowed.key" \
	"https://localhost:$HTTPS_PORT/echo/mtls"
assert_json_field path /mtls
if "${CURL[@]}" --noproxy '*' --cacert "$CA_CERT" -o "$WORK/no-client" \
	"https://localhost:$HTTPS_PORT/echo/no-client" >/dev/null 2>&1; then
	fail "HTTPS accepted a connection without the required client certificate"
fi
printf '  PASS: HTTPS rejects a client without a certificate\n'
if "${CURL[@]}" --noproxy '*' --cacert "$CA_CERT" --cert "$WORK/revoked.crt" \
	--key "$WORK/revoked.key" -o "$WORK/revoked" \
	"https://localhost:$HTTPS_PORT/echo/revoked" >/dev/null 2>&1; then
	fail "HTTPS accepted a revoked client certificate"
fi
printf '  PASS: HTTPS rejects a revoked client certificate\n'

printf 'Test: dynamic TLS and shared HTTP/TLS listener\n'
assert_status 200 --cacert "$CA_CERT" --resolve "service.test:$DYNAMIC_PORT:127.0.0.1" \
	"https://service.test:$DYNAMIC_PORT/echo/dynamic"
assert_json_field path /dynamic
assert_status 200 "http://127.0.0.1:$MULTI_PORT/echo/multi-clear"
assert_status 200 -k "https://localhost:$MULTI_PORT/echo/multi-tls"

printf 'Test: HTTP proxy, CONNECT, proxy TLS, SOCKS5 and cascade\n'
assert_status 407 --noproxy "" -x "http://127.0.0.1:$PROXY_PORT" \
	"http://127.0.0.1:$ADMIN_PORT/echo/proxy-no-auth"
assert_status 200 --noproxy "" -x "http://127.0.0.1:$PROXY_PORT" --proxy-user alice:secret \
	"http://127.0.0.1:$ADMIN_PORT/echo/proxy"
assert_json_field path /proxy
assert_status 200 --noproxy "" -x "http://127.0.0.1:$PROXY_PORT" --proxy-user alice:secret \
	--cacert "$CA_CERT" --cert "$WORK/allowed.crt" --key "$WORK/allowed.key" \
	"https://localhost:$HTTPS_PORT/echo/proxy-connect"
assert_json_field path /proxy-connect
assert_status 200 --noproxy "" -x "https://localhost:$PROXY_TLS_PORT" \
	--proxy-cacert "$CA_CERT" --proxy-user alice:secret \
	"http://127.0.0.1:$ADMIN_PORT/echo/proxy-tls"
assert_json_field path /proxy-tls
assert_status 200 --noproxy "" -x "https://localhost:$PROXY_TLS_PORT" \
	--proxy-cacert "$CA_CERT" --proxy-user alice:secret \
	--cacert "$CA_CERT" --cert "$WORK/allowed.crt" --key "$WORK/allowed.key" \
	"https://localhost:$HTTPS_PORT/echo/proxy-tls-connect"
assert_json_field path /proxy-tls-connect
assert_status 200 --noproxy "" --socks5-hostname "127.0.0.1:$SOCKS_PORT" \
	--proxy-user alice:secret "http://127.0.0.1:$ADMIN_PORT/echo/socks"
assert_json_field path /socks
if "${CURL[@]}" --noproxy "" --socks5-hostname "127.0.0.1:$SOCKS_PORT" \
	--proxy-user alice:wrong -o "$WORK/socks-auth-failure" \
	"http://127.0.0.1:$ADMIN_PORT/echo/socks-auth-failure" >/dev/null 2>&1; then
	fail "SOCKS5 accepted invalid credentials"
fi
printf '  PASS: SOCKS5 rejects invalid credentials\n'
assert_status 200 --noproxy "" -x "http://127.0.0.1:$PROXY_CASCADE_PORT" \
	"http://127.0.0.1:$ADMIN_PORT/echo/cascade"
assert_json_field path /cascade

tcp_roundtrip() {
	local input=$1 port=$2
	printf '%s' "$input" | timeout 8 nc -q 1 -w 4 127.0.0.1 "$port"
}

assert_tcp_response() {
	local expected=$1 port=$2 input=$3 actual
	actual=$(tcp_roundtrip "$input" "$port") || fail "TCP roundtrip failed on port $port"
	[[ "$actual" == "$expected" ]] || fail "TCP response '$actual' did not equal '$expected'"
	printf '  PASS: TCP port %s echoes %s\n' "$port" "$expected"
}

tls_tcp_roundtrip() {
	local port=$1 server_name=$2 expected=$3
	python3 - "$CA_CERT" "$port" "$server_name" "$expected" <<'PY'
import socket
import ssl
import sys

context = ssl.create_default_context(cafile=sys.argv[1])
with socket.create_connection(("127.0.0.1", int(sys.argv[2])), timeout=5) as raw:
    with context.wrap_socket(raw, server_hostname=sys.argv[3]) as connection:
        connection.settimeout(5)
        connection.sendall(b"PING")
        expected = sys.argv[4].encode()
        received = bytearray()
        while len(received) < len(expected):
            chunk = connection.recv(len(expected) - len(received))
            if not chunk:
                break
            received.extend(chunk)
        if bytes(received) != expected:
            raise SystemExit(f"received {bytes(received)!r}, expected {expected!r}")
        sys.stdout.buffer.write(received)
PY
}

printf 'Test: raw TCP, TLS termination, initial-byte checks, SNI and TLS upstream\n'
assert_tcp_response "plain:PING" "$TCP_PORT" "PING"
assert_tcp_response "plain:PING" "$TCP_CHECK_PORT" "PING"
invalid_tcp=$(printf 'NOPE' | timeout 5 nc -q 1 -w 3 127.0.0.1 "$TCP_CHECK_PORT" || true)
[[ -z "$invalid_tcp" ]] || fail "TCP listener forwarded bytes rejected by initial_bytes"
printf '  PASS: TCP initial_bytes closes a connection with an invalid prefix\n'
tls_tcp_response=$(tls_tcp_roundtrip "$TCP_TLS_PORT" localhost "plain:PING")
[[ "$tls_tcp_response" == "plain:PING" ]] || fail "TLS-terminated TCP did not echo through its backend"
printf '  PASS: TCP TLS listener terminates TLS and tunnels to a plaintext backend\n'
tls_tcp_response=$(tls_tcp_roundtrip "$TCP_SNI_PORT" plain.test "plain:PING")
[[ "$tls_tcp_response" == "plain:PING" ]] || fail "TCP SNI plain.test selected the wrong backend"
printf '  PASS: TCP SNI plain.test selects the plain-echo backend\n'
tls_tcp_response=$(tls_tcp_roundtrip "$TCP_SNI_PORT" secure.test "route:PING")
[[ "$tls_tcp_response" == "route:PING" ]] || fail "TCP SNI secure.test selected the wrong backend"
printf '  PASS: TCP SNI secure.test selects the route-specific backend\n'
assert_tcp_response "secure:PING" "$TCP_FORCE_TLS_PORT" "PING"

if curl --version | grep -q 'HTTP3'; then
	printf 'Test: HTTP/3\n'
	assert_status 200 --http3-only --cacert "$CA_CERT" \
		"https://localhost:$HTTP3_PORT/echo/http3"
	assert_json_field path /http3
else
	printf '  SKIP: HTTP/3 (installed curl has no HTTP/3 support)\n'
fi

printf 'Test: admin /stop and graceful shutdown\n'
"${CURL[@]}" --noproxy '*' "http://127.0.0.1:$ADMIN_PORT/stop" >/dev/null
for _ in $(seq 1 80); do
	if ! kill -0 "$GORP_PID" 2>/dev/null; then
		break
	fi
	sleep 0.1
done
if kill -0 "$GORP_PID" 2>/dev/null; then
	fail "GORP did not stop after the admin /stop request"
fi
wait "$GORP_PID"
GORP_PID=
printf '  PASS: admin /stop shuts GORP down gracefully\n'

printf 'PASS: listener integration tests completed\n'
