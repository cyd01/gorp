# Selectors

This section describes the algorithms used to select a backend from a group of
available servers.

## Selection Configuration

A selector definition includes a type, such as `RoundRobin`, and optional
configuration parameters.

## Basic Selectors

### Round Robin

Distributes requests sequentially among available backends. This is the
default selector.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
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
      - name: simple_backend
        url: "http://localhost:8001"
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
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
     type: LeastConnection
```

## Sticky Selectors

### Selector Based on Source IP Address

All requests from the same IP address are directed to the same backend.

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

This selector calculates a hash of a header value (`Affinity` in the example
below). Requests with the same hash are sent to the same backend.

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
parameter value instead.

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

Selects two backends at random, then chooses the one with the fewest active
connections.

```yaml
routes:
  - name: simple
    prefix: "/"
    backends:
      - name: simple_backend
        url: "http://localhost:8001"
    selector:
      type: PowerOfTwo
```

---
