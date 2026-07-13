# Configuration Reload

When the source is a local file or a local directory, GORP monitors changes and rebuilds the server.

## Single File

```bash
./gorp -config /etc/gorp/config.yaml
```

A detected modification, creation, or deletion on this file triggers a reload.

## Directory

```bash
./gorp -config-dir /etc/gorp/conf.d
```

GORP monitors the directory and reacts to changes in `.yaml` and `.yml` files.

## Reload Workflow

The process is:

```text
file modified
      |
      v
read configuration
      |
      v
build new server
      |
      +---- error ----> old configuration kept
      |
      v
graceful shutdown of the old instance
      |
      v
startup of the new instance
```

The new configuration is therefore not activated if it cannot be loaded or built.

## Deployment by fragments example

```text
/etc/gorp/conf.d/
├── 00-admin.yaml
├── 10-public.yaml
├── 20-services.yaml
├── 30-routes.yaml
└── 90-directory.yaml
```

`LoadDir` reads YAML files in alphabetical order and concatenates their elements.

### `00-admin.yaml`

```yaml
admin:
  enabled: true
  address: ":9090"
```

### `20-services.yaml`

```yaml
services:
  - name: app
    backends:
      - name: app-1
        url: "http://10.0.0.11:8000"
      - name: app-2
        url: "http://10.0.0.12:8000"
```

### `30-routes.yaml`

```yaml
routes:
  - name: app
    prefix: "/"
    service: app
```

## If the new file is invalid

GORP logs the loading or building error and continues with the current configuration. Then correct the file: a new modification will trigger the process again.

## Practical limits

Reloading rebuilds the listeners. It is not an atomic modification of each parameter of an existing server: the old instance is stopped and then the new one is started.

Therefore, plan critical changes with a suitable window and monitor `/ready` during an automated deployment.
