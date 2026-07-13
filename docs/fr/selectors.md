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

Un même `X-Tenant-ID` est dirigé vers le même index de backend tant que le pool reste identique et que le backend sélectionné reste disponible. Si ce backend est marqué indisponible, GORP bascule sur le premier backend disponible suivant dans le pool.

Sans header, GORP utilise une valeur aléatoire.

Pour créer une affinité même si les backends ne gèrent pas eux-mêmes ce header, le middleware de réponse `response_affinity` peut ajouter un marqueur identifiant le backend sélectionné. Le client doit renvoyer ce header dans ses requêtes suivantes. Voir l'exemple dans la section [Middlewares de réponse](./middlewares.md).

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

## Affinité par paramètre de requête

La valeur du paramètre configuré sert de clé de hash :

```yaml
selector:
  type: query-param
  config:
    name: "tenant"
```

Aliases : `query`, `queryparam`.

Sans paramètre, GORP utilise une valeur aléatoire pour cette sélection.

## Disponibilité des backends

Les selectors excluent les backends marqués indisponibles par le mécanisme de circuit breaker. Les selectors d'affinité conservent le backend préféré lorsqu'il est disponible et basculent vers un autre backend sain s'il ne l'est pas. Après 30 secondes, le circuit autorise une seule tentative de sonde ; en cas d'échec, il se rouvre pour 30 secondes. Si aucun backend n'est disponible, la route renvoie une erreur.

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
