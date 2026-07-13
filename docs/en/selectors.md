# Backend Selection

When a route has multiple backends, the selector determines which one receives the request.

Syntax:

```yaml
selector:
  type: round-robin
```

If no selector is configured, GORP uses **round-robin**.

## Round-robin

Distributes requests successively among the available backends.

```yaml
selector:
  type: round-robin
```

Aliases: `roundrobin`, `rr`.

## Random

Chooses a random backend from the available backends.

```yaml
selector:
  type: random
```

Alias: `rand`.

## First alive

Takes the first available backend in the configuration order.

```yaml
selector:
  type: first-alive
```

Aliases: `first`, `alive`.

Useful when one backend should be preferred as long as it is available, with failover to the next ones.

## Least connections

Chooses the backend with the fewest active connections. In case of a tie, a candidate is chosen randomly.

```yaml
selector:
  type: least-connections
```

Aliases: `leastconnection`, `leastconnections`, `least-connection`.

## Power of Two Choices

Randomly chooses two available backends and takes the one with the fewest active connections.

```yaml
selector:
  type: power-of-two
```

Aliases: `poweroftwo`, `p2c`.

This is a compromise between the cost of a global load calculation and the simplicity of random selection.

## Header affinity

The content of a header is used as a hash key.

```yaml
selector:
  type: header
  config:
    name: "X-Tenant-ID"
```

The same `X-Tenant-ID` is directed to the same backend index as long as the pool remains identical and the selected backend remains available. If that backend is marked unavailable, GORP falls back to the next available backend in the pool.

Without a header, GORP uses a random value.

To establish affinity when backends do not manage this header themselves, the `response_affinity` response middleware can add a marker identifying the selected backend. The client must send this header back on subsequent requests. See [Response affinity](./middlewares.md#response-affinity).

## Source IP affinity

```yaml
selector:
  type: ip
```

The IP address extracted from `RemoteAddr` is used as a hash key.

Aliases: `source-ip`, `sourceip`, `ip-hash`, `iphash`.

> The selector uses the network address actually seen by GORP; it does not automatically read `X-Forwarded-For`.

## Cookie affinity

The configured cookie value is used as the hash key:

```yaml
selector:
  type: cookie
  config:
    name: "SESSIONID"
```

Without a cookie, GORP uses a random value for that selection.

## Query parameter affinity

The configured query parameter value is used as the hash key:

```yaml
selector:
  type: query-param
  config:
    name: "tenant"
```

Aliases: `query`, `queryparam`.

Without the parameter, GORP uses a random value for that selection.

## Backend availability

Selectors exclude backends marked unavailable by the circuit breaker. Affinity
selectors keep the preferred backend while it is available and fall back to
another healthy backend otherwise. After 30 seconds, the breaker allows one
probe attempt; if that attempt fails, the circuit opens for another 30 seconds.
If no backend is available, the route returns an error.
