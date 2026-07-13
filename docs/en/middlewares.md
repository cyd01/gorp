# Middlewares

Middlewares allow you to modify a request, control access, or modify the response.

## Where to place them?

### Listener

```yaml
listeners:
  - name: https
    type: https
    address: ":443"
    middlewares:
      - name: logging
```

The middleware is applied to all routes of this listener.

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

Response middlewares are configured at the route level.

## Basic Authentication

```yaml
- name: basic_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
      bob: "password"
```

The client must provide `Authorization: Basic ...`. In case of failure, GORP returns `401` and `WWW-Authenticate`.

## Digest Authentication

```yaml
- name: digest_auth
  config:
    realm: "Restricted"
    users:
      alice: "secret"
    nonce_timeout: "5m"
```

The mechanism uses Digest with `MD5` and `qop="auth"`. The nonce expires according to `nonce_timeout` (5 minutes by default).

## Token Authentication

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

The header and prefix can be customized:

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