# Selectors

This section describes the algorithms used to select a backend from a group of
available servers.

## Selection Configuration

A selector is configured on an HTTP route. The type is case-insensitive; if
it is omitted or unrecognized, GORP uses round-robin. The `config` map carries
the field required by affinity selectors.

## Basic Selectors

### Round Robin

Distributes requests sequentially among available backends. This is the
default selector.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: backend1
        url: "http://localhost:8001"
      - name: backend2
        url: "http://localhost:8002"
    selector:
      type: RoundRobin
```

### Random

Distributes requests randomly among available backends.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: backend1
        url: "http://localhost:8001"
      - name: backend2
        url: "http://localhost:8002"
    selector: 
      type: random
```

### Least Connections

Routes each request to the backend with the fewest active connections.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: backend1
        url: "http://localhost:8001"
      - name: backend2
        url: "http://localhost:8002"
    selector:
     type: LeastConnection
```

### First Alive

Selects the first available backend in configuration order. This is useful
when one backend should be preferred and the others should act as failover.

```yaml
routes:
  - name: preferred
    prefix: /
    selector:
      type: first-alive
    backends:
      - name: primary
        url: "http://localhost:8001"
      - name: standby
        url: "http://localhost:8002"
```

Aliases: `first`, `alive`.

## Affinity Selectors

### Selector Based on Source IP Address

Requests with the same source IP hash to the same backend index while the
backend pool remains unchanged and that backend is available. The selector
uses the network peer address (`RemoteAddr`), not `X-Forwarded-For`.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
     type: SourceIP
```

### Selector Based on a Header Value

This selector calculates a hash of a header value. Requests with the same
value are sent to the same backend index while the pool remains unchanged and
the selected backend is available. The default header name is `Affinity`.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
      type: header
      config:
        name: Affinity
```

### Selector Based on a Cookie or Query Parameter

These selectors work like the header selector, using a cookie or query
parameter value instead. The default cookie or query parameter name is
`SESSIONID`. If the configured value is absent, a random value is used for
that selection, so no stable affinity is guaranteed.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
      type: cookie
      config:
        name: SESSIONID
```

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
      type: QueryParam
      config:
        name: SESSIONID
```

## Advanced Selection Logic

More complex selection logic.

### Power of Two Choices

Selects two available backends at random, then chooses the one with the
fewest active connections. This reduces the cost of global load comparison
while generally balancing better than random selection alone.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: backend1
        url: "http://localhost:8001"
      - name: backend2
        url: "http://localhost:8002"
      - name: backend3
        url: "http://localhost:8003"
    selector:
      type: PowerOfTwo
```

Aliases: `poweroftwo`, `p2c`.

## Supported names

| Selector | Accepted names |
|---|---|
| Round robin | default, `round-robin`, `roundrobin`, `rr` |
| Random | `random`, `rand` |
| First available | `first-alive`, `first`, `alive` |
| Least connections | `least-connections`, `leastconnection`, `leastconnections`, `least-connection` |
| Header affinity | `header` |
| Cookie affinity | `cookie` |
| Source IP affinity | `ip`, `source-ip`, `sourceip`, `ip-hash`, `iphash` |
| Query parameter affinity | `query`, `query-param`, `queryparam` |
| Power of Two Choices | `power-of-two`, `poweroftwo`, `p2c` |

Selectors that check availability skip backends marked unavailable by the
backend circuit breaker. If no backend is selectable, the route returns an
error rather than silently selecting an unavailable destination.

---
