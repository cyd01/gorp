# HTTP Routing

HTTP routes determine which backend receives a request.

## A minimal route

```yaml
routes:
  - name: app
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

## Available criteria

A route can use:

- `prefix`: path prefix;
- `paths`: wildcard patterns on the full path or its last component;
- `hosts`: wildcard patterns on `Host`;
- `endpoints`: restriction to the listener name;
- `service` or `backends`: destination.

Routes with a longer `prefix` are sorted before shorter prefixes during configuration loading.

## Prefixes

```yaml
routes:
  - name: api
    prefix: "/api"
    backends:
      - name: api
        url: "http://127.0.0.1:8001"
```

`/api/users` will match this route.

The test is a simple `HasPrefix`: `/api2` will therefore also match `/api`. If you need a strict boundary, use an appropriate `paths` pattern.

## Stripping the prefix

```yaml
routes:
  - name: api
    prefix: "/api"
    strip_prefix: true
    backends:
      - name: api
        url: "http://127.0.0.1:8001"
```

A request:

```text
GET /api/users/42
```

is sent to the backend with:

```text
GET /users/42
```

If stripping leaves a path without `/`, GORP adds one.

## Preserving the original Host

By default, the reverse proxy defines the upstream URL from `backend.url`. With:

```yaml
preserve_host: true
```

the `Host` header sent to the backend remains the one received from the client.

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

The reverse proxy also adds the `X-Forwarded-*` headers handled by `net/http/httputil`.

## Host-based routing

```yaml
routes:
  - name: shop
    hosts:
      - "shop.example.com"
    backends:
      - name: shop
```