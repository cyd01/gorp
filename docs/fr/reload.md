# Rechargement de configuration

Lorsque la source est un fichier local ou un répertoire local, GORP surveille les changements et reconstruit le serveur.

## Fichier unique

```bash
./gorp -config /etc/gorp/config.yaml
```

Une modification, création ou suppression détectée sur ce fichier déclenche un rechargement.

## Répertoire

```bash
./gorp -config-dir /etc/gorp/conf.d
```

GORP surveille le répertoire et réagit aux changements des fichiers `.yaml` et `.yml`.

## Déroulement d'un reload

Le processus est :

```text
fichier modifié
      |
      v
relecture de la configuration
      |
      v
construction du nouveau serveur
      |
      +---- erreur ----> ancienne configuration conservée
      |
      v
arrêt gracieux de l'ancienne instance
      |
      v
démarrage de la nouvelle instance
```

La nouvelle configuration n'est donc pas activée si elle ne peut pas être chargée ou construite.

## Exemple de déploiement par fragments

```text
/etc/gorp/conf.d/
├── 00-admin.yaml
├── 10-public.yaml
├── 20-services.yaml
├── 30-routes.yaml
└── 90-directory.yaml
```

`LoadDir` lit les fichiers YAML dans l'ordre alphabétique et concatène leurs éléments.

### `00-admin.yaml`

```yaml
admin:
  enabled: true
  address: ":9090"
```

### `20-services.yaml`

```yaml
services:
  - name: app
    backends:
      - name: app-1
        url: "http://10.0.0.11:8000"
      - name: app-2
        url: "http://10.0.0.12:8000"
```

### `30-routes.yaml`

```yaml
routes:
  - name: app
    prefix: "/"
    service: app
```

## Si le nouveau fichier est invalide

GORP journalise l'erreur de chargement ou de construction et continue avec la configuration courante. Corrigez ensuite le fichier : une nouvelle modification relancera le processus.

## Limites pratiques

Le reload reconstruit les listeners. Ce n'est pas une modification atomique de chaque paramètre d'un serveur existant : l'ancienne instance est arrêtée puis la nouvelle est démarrée.

Planifiez donc les changements critiques avec une fenêtre adaptée et surveillez `/ready` pendant un déploiement automatisé.
