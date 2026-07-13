# Quick start

## 1. Build GORP

The project is a standard Go program:

```bash
go build -o gorp ./cmd/gorp
```

Also check the project status with:

```bash
go test ./...
```

## 2. First HTTP reverse proxy

Create `config.yaml`:

```yaml
listeners:
  - name: public-http
    type: http
    address: ":8080"

routes:
  - name: application
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Start:

```bash
./gorp -config config.yaml
```

The application should now be accessible via:

```bash
curl -i http://127.0.0.1:8080/
```

## 3. Add two backends

A single route can distribute requests among multiple backends:

```yaml
routes:
  - name: application
    prefix: "/"
    backends:
      - name: app-1
        url: "http://10.0.0.11:8000"
      - name: app-2
        url: "http://10.0.0.12:8000"
```

Without a `selector`, GORP uses **round-robin**.

## 4. Expose HTTPS

```yaml
listeners:
  - name: public-https
    type: https
    address: ":8443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"

routes:
  - name: application
    prefix: "/"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"
```

Test:

```bash
curl -k https://127.0.0.1:8443/
```

## 5. Protect a route

```yaml
routes:
  - name: public
    prefix: "/public"
    backends:
      - name: app
        url: "http://127.0.0.1:8000"

  - name: admin
    prefix: "/admin"
```