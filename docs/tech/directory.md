# Directory Fallback

The top-level `directory` option is an HTTP fallback, not a listener or a
route. GORP first evaluates the configured HTTP routes. Only when no route
matches does it serve the requested path from this directory. Without a
matching route or a configured directory, the response is `404 Not Found`.

```yaml
directory: /var/www/html

listeners:
  - name: web
    type: http
    address: ":8080"

routes:
  - name: api
    prefix: /api
    backends:
      - name: api
        url: "http://127.0.0.1:8000"
```

In this example, `/api/...` is handled by the backend route; other paths are
served from `/var/www/html` using Go's standard HTTP file-serving behavior.
The fallback does not apply to TCP, proxy, or SOCKS5 listeners, and it does
not add route middleware to served files. Protect private files through
filesystem permissions and do not place secrets in the served directory.

---
