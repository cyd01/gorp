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

- `16`: the byte must have value 16;
- `"*"`: any byte value is accepted;
- `[3, 4, 5]`: any of these byte values is accepted;
- `"M"`: the ASCII character `M` (a one-byte string).

The number of entries determines how many bytes GORP reads before opening the
backend connection. A failed check closes the client connection. The checked
bytes are replayed to the backend, so the application protocol still receives
its complete stream. For a TLS-enabled listener, checks run after the TLS
handshake against decrypted application data.

## TLS to the backend

To secure the GORP-to-backend connection, set `force_tls` on the TCP backend.
Optionally provide the trusted CA and the backend name used for SNI and
certificate verification:

```yaml
listeners:
  - name: secure-tcp
    type: tcp
    address: ":9443"
    backends:
      - name: backend
        url: "tcp://backend.internal:9000"
        force_tls: true
        tls:
          ca_file: "/etc/gorp/ca/backend-ca.crt"
          server_name: "backend.internal"
```

`timeout` controls the non-TLS TCP connection timeout. TLS backend connections
use the configured TLS certificate and CA settings.

## Connection limit

`max_connections` on a TCP listener limits accepted TCP connections:

```yaml
listeners:
  - name: mqtt
    type: tcp
    address: ":1883"
    max_connections: 1000
    backends:
      - name: mqtt
        url: "tcp://127.0.0.1:1883"
```

Unlike the HTTP listener limit, this is a limit on TCP connections, not
concurrent HTTP requests.
