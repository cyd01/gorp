# Directory Fallback

When `directory` is configured, `gorp` falls back to a simple file server
when no route matches the request. For example:

```yaml
directory: /var/www/html
```

In this case, `gorp` serves static files from `/var/www/html` when no route
matches the incoming request.

---
