---
name: verification-before-completion
description: "Utiliser avant de déclarer un travail fini (gate de complétion, pre-review, checkpoint de commit) : lancer toutes les vérifications du projet, montrer leur sortie, revérifier la demande."
---

# Skill — Vérifier avant de déclarer fini

« Ça passe » est une affirmation : elle ne vaut que par la sortie de la commande qui le montre, lancée **après ta dernière modification**. Ce qui n'a pas été lancé n'est pas vérifié.

## 1. Trouver les vraies vérifications du projet

Les tests du paquet modifié ne suffisent pas. Relève les commandes que le projet exige, dans cet ordre :

1. **CI** : `.github/workflows/*.yml`, `.gitlab-ci.yml`, `Jenkinsfile`, `azure-pipelines.yml`, `.circleci/` ; chaque étape `run:` ou `script:` est une vérification (lint, vet, typecheck, tests avec drapeaux ou étiquettes, build).
2. **Scripts du projet** : cibles du `Makefile` (`check`, `lint`, `test`, `verify`), scripts de `package.json` (`lint`, `typecheck`, `test`, `build`), `tox.ini`, `noxfile.py`, `pyproject.toml`, hooks de pre-commit.
3. **Conventions** : `docs/wiki/technical/conventions.md`, `CONVENTIONS.md`, `CONTRIBUTING.md`.
4. **À défaut**, celles de la stack, sur **tout** le projet : Go `go build ./...`, `go vet ./...`, `go test ./...` ; TypeScript `tsc --noEmit`, lint, tests ; Python `ruff check`, `mypy` s'il est configuré, `pytest` ; Rust `cargo clippy`, `cargo test`.

Une commande que tes permissions refusent : ne la contourne pas, note-la comme non lancée.

## 2. Tout lancer, sur tout le projet

- Lance chaque vérification sur l'ensemble du dépôt (`./...`, racine du projet), pas seulement sur les fichiers ou le paquet modifiés : un changement de type, de signature ou de comportement casse ailleurs.
- Reprends les drapeaux de la CI (`-race`, `-tags integration`, `--strict`) : la CI les passe, toi aussi.
- Relance tout après chaque correction, jusqu'à ce que toutes les vérifications passent dans la même série.

## 3. Lire la sortie, traiter chaque échec

- Lis la sortie entière, pas seulement le code de retour : avertissements de vet ou du linter, tests ignorés (`skip`), « no tests to run », fichiers non trouvés.
- Un échec dans un fichier que tu n'as pas touché vient souvent de ton changement (appelant, analyse statique, test d'intégration) : corrige-le, il fait partie du travail.
- Un échec déjà présent avant ton travail : montre-le (même échec sans tes changements, ou historique), ne le corrige pas hors périmètre et signale-le.
- Jamais de test supprimé, désactivé ou affaibli, ni de règle de lint coupée pour faire passer une vérification.

## 4. Revérifier la demande

Relis la demande et les critères d'acceptance, un par un : comportement, contraintes explicites (API inchangée, signature, format, performance), fichiers à ne pas toucher. Pour chacun, la preuve (test, sortie, ligne du diff). Une contrainte que ta solution ne tient pas : change de solution ou signale l'écart ; ne la contourne pas en silence.

## 5. Montrer la preuve

Dans ton rapport (ou le `summary` du checkpoint de commit), une ligne par vérification :

```
- `npm run lint` : OK (0 avertissement)
- `npx tsc --noEmit` : OK
- `npm test` : OK — 48 tests, 0 ignoré
- `npm run e2e` (étape de la CI) : non lancé — refusé par les permissions
```

Pas de « tout passe » sans ces lignes ; pas de « devrait passer ».
