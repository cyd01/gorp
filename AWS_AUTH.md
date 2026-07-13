# AWS Signature Version 4 Authentication

## Vue d'ensemble

L'authentification AWS a été implémentée pour permettre au proxy de frontal des services AWS comme S3, EC2, Lambda, etc.

**Type d'authentification:** `aws` (AWS Signature Version 4)

## Architecture

### Composants

1. **`internal/backend/auth_aws.go`** - Implémentation de la signature AWS V4
   - `AWSSignerV4` - Structure pour gérer la signature
   - `SignRequest()` - Signe une requête HTTP

2. **Modifications dans `transport.go`**
   - Extension de `AuthConfig` avec les paramètres AWS
   - Cas "aws" dans le switch du `authTransport`

3. **Modifications dans `config.go`**
   - Extension de `BackendAuth` pour les paramètres AWS

4. **Modifications dans `server/builder.go`**
   - Mappage des paramètres AWS lors de la construction

## Configuration

### Paramètres YAML

```yaml
routes:
  - name: mon-api-s3
    prefix: "/s3"
    backends:
      - name: s3-backend
        url: "https://bucket.s3.region.amazonaws.com"
        auth:
          type: "aws"                    # Authentification AWS
          access_key_id: "AKIA..."       # AWS Access Key ID
          secret_access_key: "wJal..."   # AWS Secret Access Key
          region: "us-east-1"            # Région AWS
          service: "s3"                  # Service (s3, ec2, etc.)
```

### Paramètres

| Paramètre | Description | Exemple |
|-----------|-------------|---------|
| `type` | Type d'authentification | `aws` |
| `access_key_id` | AWS Access Key ID | `AKIAIOSFODNN7EXAMPLE` |
| `secret_access_key` | AWS Secret Access Key | `wJalrXUtnFEMI/K7MDENG/...` |
| `region` | Région AWS | `us-east-1`, `us-west-2`, `eu-west-1` |
| `service` | Service AWS à proxifier | `s3`, `ec2`, `lambda`, `dynamodb` |

## Processus de signature AWS V4

### 1. Préparation des headers
- `X-Amz-Date`: Timestamp actuel en format UTC (YYYYMMDDTHHMMSSZ)
- `X-Amz-Content-Sha256`: SHA256 du corps de la requête
- `Host`: Hôte de la requête

### 2. Requête canonique

Construction d'une représentation standardisée:
```
METHOD
CANONICAL_PATH
CANONICAL_QUERY_STRING
CANONICAL_HEADERS

SIGNED_HEADERS
HASHED_PAYLOAD
```

### 3. Chaîne à signer

```
AWS4-HMAC-SHA256
TIMESTAMP
CREDENTIAL_SCOPE
HASHED_CANONICAL_REQUEST
```

Où `CREDENTIAL_SCOPE` = `DATESTAMP/REGION/SERVICE/aws4_request`

### 4. Signature

Dérivation progressive par HMAC-SHA256:
```
kDate = HMAC-SHA256("AWS4" + SecretKey, Date)
kRegion = HMAC-SHA256(kDate, Region)
kService = HMAC-SHA256(kRegion, Service)
kSigning = HMAC-SHA256(kService, "aws4_request")
Signature = Hex(HMAC-SHA256(kSigning, StringToSign))
```

### 5. Header Authorization

```
Authorization: AWS4-HMAC-SHA256 Credential=AccessKeyID/CredentialScope, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=HEX_SIGNATURE
```

## Flux de la requête

```
1. Client → Proxy: Requête HTTP
2. Proxy: Clône la requête
3. Proxy: AuthTransport détecte type="aws"
4. Proxy: AWSSignerV4.SignRequest()
   - Calcule le hash du corps
   - Construit la requête canonique
   - Calcule la signature AWS V4
   - Ajoute les headers Authorization et X-Amz-*
5. Proxy: Envoie la requête signée au backend AWS
6. Backend AWS: Valide la signature
7. Backend AWS: Exécute l'action (GET objet S3, etc.)
8. Backend AWS: Retourne réponse au proxy
9. Proxy → Client: Retourne réponse
```

## Exemples de configuration

### S3 (Virtual-hosted style)

```yaml
backends:
  - name: s3-bucket
    url: "https://my-bucket.s3.us-east-1.amazonaws.com"
    auth:
      type: "aws"
      access_key_id: "AKIA..."
      secret_access_key: "wJal..."
      region: "us-east-1"
      service: "s3"
```

### S3 (Path style)

```yaml
backends:
  - name: s3-path
    url: "https://s3.us-east-1.amazonaws.com/my-bucket"
    auth:
      type: "aws"
      access_key_id: "AKIA..."
      secret_access_key: "wJal..."
      region: "us-east-1"
      service: "s3"
```

### EC2

```yaml
backends:
  - name: ec2-api
    url: "https://ec2.us-west-2.amazonaws.com"
    auth:
      type: "aws"
      access_key_id: "AKIA..."
      secret_access_key: "wJal..."
      region: "us-west-2"
      service: "ec2"
```

## Headers HTTP ajoutés

La signature AWS V4 ajoute les headers suivants à chaque requête:

| Header | Description |
|--------|-------------|
| `Authorization` | Contient la signature et les credentials |
| `X-Amz-Date` | Timestamp de la requête (UTC) |
| `X-Amz-Content-Sha256` | Hash SHA256 du corps de la requête |

Tous les headers (y compris ces derniers) sont inclus dans le calcul de la signature pour garantir l'intégrité de la requête.

## Considérations de sécurité

### Credentials
- **Ne jamais** commettre les credentials en dur dans le code
- Utiliser des variables d'environnement
- Utiliser un fichier de configuration sécurisé avec permissions `600`
- Préférer les rôles IAM et credentials temporaires

### Rotation
- Mettre à jour les access keys régulièrement
- Désactiver les anciennes clés immédiatement
- Monitorer les tentatives d'accès échouées

### Permissions IAM
- Accorder uniquement les permissions minimales requises
- Utiliser des bucket policies pour S3
- Auditer les logs CloudTrail

## Limitations et notes

1. **Requêtes avec corps**: Le body est entièrement lu en mémoire pour calculer son hash
2. **URLs complexes**: Vérifie les paramètres de query sont correctement encodés
3. **Horodatage**: Les requêtes doivent être envoyées dans les 15 minutes de la signature
4. **Région**: La région doit correspondre à celle du service AWS

## Debugging

Pour vérifier la signature, on peut:
1. Activer la logging des headers Authorization
2. Comparer avec les calculs AWS SDK
3. Vérifier les timestamps avec le serveur AWS

## Services AWS supportés

- **S3**: `service: "s3"`
- **EC2**: `service: "ec2"`
- **Lambda**: `service: "lambda"`
- **DynamoDB**: `service: "dynamodb"`
- **Elasticache**: `service: "elasticache"`
- **ElasticLoadBalancing**: `service: "elasticloadbalancing"`
- Tous les autres services AWS utilisant AWS Signature Version 4

## Références

- [AWS Signature Version 4](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)
- [Calcul de signature](https://docs.aws.amazon.com/general/latest/gr/aws4-signed-request-examples.html)
- [Services supportés](https://docs.aws.amazon.com/general/latest/gr/aws-service-information.html)
