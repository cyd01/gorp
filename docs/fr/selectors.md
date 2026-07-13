# Sélection des backends

Lorsqu'une route possède plusieurs backends, le selector détermine lequel reçoit la requête.

Syntaxe :

```yaml
selector:
  type: round-robin
```

Si aucun selector n'est configuré, GORP utilise **round-robin**.

## Round-robin

Distribue les requêtes successivement entre les backends disponibles.

```yaml
selector:
  type: round-robin
```

Aliases : `roundrobin`, `rr`.

## Random

Choisit un backend aléatoire parmi les backends disponibles.

```yaml
selector:
  type: random
```

Alias : `rand`.

## First alive

Prend le premier backend disponible dans l'ordre de configuration.

```yaml
selector:
  type: first-alive
```

Aliases : `first`, `alive`.

Utile lorsqu'un backend doit être préféré tant qu'il est disponible, avec bascule vers les suivants.

## Least connections

Choisit le backend ayant le moins de connexions actives. En cas d'égalité, un candidat est choisi aléatoirement.

```yaml
selector:
  type: least-connections
```

Aliases : `leastconnection`, `leastconnections`, `least-connection`.

## Power of Two Choices

Choisit deux backends disponibles au hasard et prend celui qui possède le moins de connexions actives.

```yaml
selector:
  type: power-of-two
```

Aliases : `poweroftwo`, `p2c`.

C'est un compromis entre le coût d'un calcul de charge global et la simplicité du random.

## Affinité par header

Le contenu d'un header sert de clé de hash.

```yaml
selector:
  type: header
  config:
    name: "X-Tenant-ID"
```

Un même `X-Tenant-ID` est dirigé vers le même index de backend tant que le pool reste identique et que le backend sélectionné reste disponible.

Sans header, GORP utilise une valeur aléatoire.

## Affinité par IP source

```yaml
selector:
  type: ip
```

L'adresse IP extraite de `RemoteAddr` est utilisée comme clé de hash.

Aliases : `source-ip`, `sourceip`, `ip-hash`, `iphash`.

> Le selector utilise l'adresse réseau réellement vue par GORP ; il ne lit pas automatiquement `X-Forwarded-For`.

## Affinité par cookie

```yaml
selector:
  type: cookie
  config:
    name: "SESSIONID"
```

La valeur du cookie est utilisée comme clé de hash. Sans cookie, une valeur aléatoire est générée pour cette sélection.

Le cookie n'est pas créé ni modifié par GORP : le client doit donc normalement fournir lui-même une valeur stable si vous voulez une affinité persistante.

## Query

La configuration accepte :

```yaml
selector:
  type: query
  config:
    name: "tenant"
```

Dans l'état actuel du code, le factory `query` utilise la même implémentation que le selector cookie. Si vous avez besoin d'une affinité par paramètre d'URL, vérifiez cette version du projet avant de déployer ce mode : l'implémentation dédiée `query_param` existe dans le package mais n'est pas utilisée par le factory actuel.

## Disponibilité des backends

Les selectors excluent les backends marqués indisponibles par le mécanisme de circuit breaker. Si aucun backend n'est disponible, la route renvoie `502 Bad Gateway` ou `503 Service Unavailable` selon l'erreur rencontrée.

## Exemple complet

```yaml
routes:
  - name: api
    prefix: "/api"
    selector:
      type: least-connections
    backends:
      - name: api-1
        url: "http://10.0.0.11:8080"
        max_connections: 100
      - name: api-2
        url: "http://10.0.0.12:8080"
        max_connections: 100
```
