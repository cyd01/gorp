# TODO

Fonctionnalités et améliorations qui restent à envisager :

## Retries

- ajouter une limite de durée totale pour les tentatives, en plus du nombre maximal et du backoff déjà configurés ;

## Gestion des certificats

- recharger automatiquement les certificats depuis leurs fichiers sans redémarrer les listeners ;
- intégrer ACME/Let's Encrypt et la rotation automatique des certificats.

## Rechargement de configuration

- démarrer et vérifier la nouvelle configuration avant de remplacer l'ancienne, afin d'éviter une coupure si les nouveaux listeners ne peuvent pas démarrer.

## Proxy et protocoles

- renforcer les tests des upgrades HTTP, notamment WebSocket, et du streaming de bout en bout ;
- étudier la prise en charge de protocoles supplémentaires, par exemple gRPC et GraphQL.

## Observabilité

- proposer une exportation des spans vers un collecteur de tracing ; le middleware actuel propage le contexte W3C mais n'exporte pas les spans ;
- intégrer les logs à un système centralisé et ajouter des événements d'audit dédiés.

## Sécurité avancée

- ajouter des politiques d'autorisation RBAC/ABAC ;
- étudier une limitation adaptative et une protection DDoS au-delà du rate limiting existant ;
- proposer un profil de sécurité TLS imposant TLS 1.3, plutôt que de seulement permettre sa configuration ;
- étudier le chiffrement applicatif des payloads, la signature générique des messages, le certificate pinning et des contrôles Zero Trust ;
- fournir une politique de sécurité pour les headers (HSTS, CSP, X-Frame-Options), au-delà de leur configuration manuelle.

## Cache et transformation

- mettre en cache les réponses lorsque les règles de validité permettent de le faire sans risque ;
- étudier la conversion de formats et le mapping des requêtes/réponses.
