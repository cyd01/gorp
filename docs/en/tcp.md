# TCP, TLS, and SNI

The `tcp` and `unix_tcp` listeners are used to transport protocols that are not HTTP.

## Simple TCP

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    backends:
      - name: mqtt-1
        url: "tcp://10.0.0.11:1883"
      - name: mqtt-2
        url: "tcp://10.0.0.12:1883"
```

Each new connection randomly chooses a backend.

## TCP TLS

```yaml
listeners:
  - name: mqtts
    type: tcp
    address: ":8883"
    tls:
      cert_file: server.crt
      key_file: server.key
    backends:
      - name: mqtt-1
        url: "tcp://10.0.0.11:1883"
```

The client → GORP TLS is terminated by GORP before the tunnel to the backend.

## SNI Routing

```yaml
listeners:
  - name: tls-router
    type: tcp
    address: ":443"
    tls:
      cert_file: default.crt
      key_file: default.key
    routes:
      - name: mqtt
        hosts:
          - "mqtt.example.com"
        backends:
          - name: mqtt
            url: "tcp://10.0.0.11:1883"

      - name: redis
        hosts:
          - "redis.example.com"
        backends:
          - name: redis
            url: "tcp://10.0.0.12:6379"

    backends:
      - name: fallback
        url: "tcp://10.0.0.13:9000"
```

If no SNI host matches, GORP uses the listener's `backends`.

Wildcards follow `filepath.Match`. For TCP SNI, the comparison is normalized to lowercase.

## Route-specific Certificate

A TCP route can define its own `tls` block with `cert_file`, `key_file`, `ca_file`, `crl_file`, `ocsp_url`, and `min_version`.

This allows, for example, presenting a different certificate depending on the SNI.

## Checking initial bytes

`initial_bytes` allows filtering the protocol before opening the backend.

MQTT Example:

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    initial_bytes:
      - [16]
      - "*"
      - [3, 4, 5]
    backends:
      - name: mqtt
        url: "tcp://127.0.0.1:1883"
```

Each entry corresponds to a position at the start of the stream:

- `16` : the byte must be 16;
