# TODO

À mon avis, les fonctionnalités prioritaires manquantes sont :

## Observabilité

- métriques Prometheus : requêtes, latence, codes HTTP, erreurs backend, connexions actives ;
- logs structurés avec méthode, route, backend sélectionné et durée ;
- endpoint /metrics.

## Health checks actifs

- vérification périodique des backends ;
- retrait automatique des backends défaillants ;
- réintégration après récupération.

## Retries configurables

- nombre de tentatives ;
- méthodes sûres uniquement par défaut (GET, HEAD, etc.) ;
- backoff et limite de durée globale.

## Validation stricte de configuration

- détecter au démarrage les routes impossibles, ports dupliqués, TLS incomplet, middlewares invalides ;
- afficher toutes les erreurs de configuration en une fois ;
- éviter les configurations silencieusement ignorées.

## Limites et protection

- taille maximale de requête ;
- délais séparés : connexion, headers, réponse, idle ;
- limite de connexions par listener ou backend ;
- rate limiter réellement sûr en concurrence.

## Gestion des certificats

- rechargement automatique des certificats sans redémarrer les listeners ;
- support ACME/Let’s Encrypt ;
- rotation des certificats.

## Configuration dynamique plus robuste

- reload atomique ;
- possibilité de conserver l’ancien serveur si la nouvelle configuration ne peut pas démarrer ;
- éviter une coupure entre l’arrêt de l’ancien serveur et le démarrage du nouveau.

## Support proxy HTTP complet

- WebSocket et autres upgrades à tester plus largement ;
- HTTP CONNECT si nécessaire ;
- compression et décompression optionnelles.


La prochaine fonctionnalité que je choisirais est l’observabilité, suivie des health checks actifs. Sans métriques ni état fiable des backends, il devient difficile de diagnostiquer les problèmes en production.

---

API GATEWAYS – FONCTIONS
Gestion du trafic
◆ Routage intelligent: Dirige les requêtes vers les zones appropriées
◆ Load balancing: Distribue la charge entre plusieurs instances
◆ Rate limiting: Contrôle le nombre de requêtes par utilisateur/IP

Sécurité
◆ Authentification/Autorisation: Vérifie l'identité et les permissions (coarse-grained)
◆ Validation des requêtes: Contrôle la structure et le contenu
◆ Protection DDoS: Filtre les attaques malveillantes

Transformation des données
◆ Protocol translation: Convertit entre différents protocoles (REST/GraphQL/gRPC)
◆ Format conversion: Transforme JSON/XML selon les besoins
◆ Request/Response mapping: Adapte les structures de données

Observabilité
◆ Logging centralisé: Enregistre toutes les interactions
◆ Métriques en temps réel: Surveille performance et utilisation
◆ Tracing distribué: Suit les requêtes à travers les services

Optimisation
◆ Caching: Met en cache les réponses fréquentes
◆ Compression: Réduit la taille des données transmises
◆ Connection pooling: Réutilise les connexions backend


API GATEWAYS – SÉCURITÉ
Authentification & Autorisation
◆ OAuth 2.0/OpenID Connect : Standard pour l'authentification déléguée
◆ JWT (JSON Web Tokens) : Tokens stateless avec claims intégrés
◆ API Keys : Identification simple pour les APIs internes
◆ mTLS (Mutual TLS) : Authentification bidirectionnelle pour les services critiques
◆ RBAC/ABAC : Contrôle d'accès basé sur les rôles ou attributs

Protection contre les Attaques
◆ Rate Limiting : Limitation par IP, utilisateur ou API key
◆ Throttling adaptatif : Ajustement dynamique selon la charge
◆ CORS (Cross-Origin Resource Sharing) : Contrôle des origines autorisées
◆ Input Validation : Validation des payloads et paramètres
◆ SQL/NoSQL Injection Prevention : Sanitisation des requêtes

Chiffrement & Intégrité
◆ TLS 1.3 : Chiffrement en transit obligatoire
◆ Payload Encryption : Chiffrement des données sensibles
◆ Message Signing : Vérification de l'intégrité avec HMAC/RSA
◆ Certificate Pinning : Validation stricte des certificats

Architecture Zero Trust
◆ Micro-segmentation : Isolation réseau granulaire
◆ Least Privilege : Accès minimal nécessaire
◆ Continuous Verification : Validation permanente des identités
◆ Context-aware Security : Décisions basées sur le contexte (géolocalisation, device, etc.)

Monitoring & Détection
◆ Anomaly Detection : Détection de comportements suspects
◆ Security Headers : HSTS, CSP, X-Frame-Options
◆ Audit Logging : Traçabilité complète des accès
◆ SIEM Integration : Corrélation avec les outils de sécurité
◆ Real-time Alerting : Notifications immédiates des incidents

---