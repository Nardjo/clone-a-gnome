# clone-a-gnome

Un utilitaire CLI en Go pour cloner rapidement une base PostgreSQL distante dans un conteneur Docker local prêt à l’emploi.

## Fonctionnalités clés

- Validation de l’URL source (`postgres://` ou `postgresql://`) et extraction des métadonnées utiles.
- Provision automatique d’un conteneur `postgres:16-alpine` avec mot de passe temporaire généré.
- Import en continu via `pg_dump` → `docker exec psql` sans fichier intermédiaire.
- Journalisation lisible des grandes étapes et rappel pour arrêter le conteneur.
- Filtrage simple via `--exclude-table` et script d’anonymisation post-import avec `--anonymize-script`.
- Réutilisation optionnelle d’un conteneur existant via `--reuse` avec persistance des paramètres sensibles.

## Prérequis

- Docker (CLI `docker` accessible dans le `PATH`).
- Outils PostgreSQL côté client (`pg_dump`, `psql`).
- Go 1.21+ pour la compilation.

## Installation

```bash
go build -o clone-a-gnome ./cmd/clone-a-gnome
```

## Utilisation

```bash
./clone-a-gnome <URL_SOURCE>
```

Exemple :

```bash
./clone-a-gnome postgresql://user:secret@ep-sample.eu-west-3.aws.neon.tech/mydb
```

### Options principales

- `--container-name` : nom du conteneur Docker (défaut `clone-a-gnome`).
- `--local-db` : nom de la base créée dans le conteneur local (défaut `localdb`).
- `--port` : port local à mapper (0 = auto, défaut).
- `--exclude-table` : exclure une table du dump (répéter le drapeau pour plusieurs tables).
- `--anonymize-script` : chemin d’un script SQL exécuté après l’import dans le conteneur.
- `--reuse` : réutilise un conteneur précédemment créé (mot de passe, port et base sont relus depuis `~/.config/clone-a-gnome/state.json`).
- `--log-level` : `debug`, `info`, `warn`, `error`.

### Réutilisation d’un conteneur existant

1. Lancer une première fois sans `--reuse` pour provisionner et importer la base.
2. Le mot de passe, le port et le nom de base sont stockés dans `~/.config/clone-a-gnome/state.json`.
3. Relancer avec `--reuse` pour rejouer un import sur le même conteneur (et donc conserver le même port/URL).

### Exécution d’un script d’anonymisation

Préparez un fichier `anonymize.sql`, puis :

```bash
./clone-a-gnome <URL_SOURCE> --anonymize-script anonymize.sql
```

Le script est exécuté à l’intérieur du conteneur juste après l’import.

## Arrêt et nettoyage

Le CLI rappelle la commande à exécuter :

```bash
docker stop <container-name>
```

Pour repartir de zéro, supprimez le conteneur puis effacez l’entrée correspondante dans `~/.config/clone-a-gnome/state.json`.

## Développement

- `go build ./...`
- `go test ./...`
