# depot

Sert les fichiers déposés par les clients (`GET /files/<nom>`).

- `files/` : ouverture d'un fichier déposé, sous le dossier des dépôts.
- Un client peut déposer une archive, décompressée telle quelle dans son dossier (liens symboliques compris).
- Tests : `go test ./...`
