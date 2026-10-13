---
name: rtk-usage
description: "Utiliser quand le plugin RTK est chargé : formes rtk compactes des commandes courantes."
plugin: rtk
---

# RTK

RTK (Rust Token Killer) filtre les sorties des commandes CLI (60 à 90 % de tokens en moins sur les diffs, tests, logs et listings). Le plugin RTK d'OpenCode **réécrit automatiquement** les commandes bash : tu n'as pas besoin de les préfixer par `rtk`.

- Formes utiles hors réécriture : `rtk read <fichier>` (gros fichier), `rtk json <fichier> --keys-only` ou `--depth 2` (structure d'un JSON avant d'en lire une section), `rtk deps` (dépendances), `rtk tree src/ --depth 3`.
- Pas de `rtk` pour les commandes interactives (`npm init`, `git add -p`) ; inutile pour les commandes déjà compactes (`git status`) ou de modification (`git commit`, `npm install`).
- `rtk gain --project` (économies du projet) et `rtk discover` (commandes non optimisées) pour diagnostiquer ; une commande qui se comporte mal peut venir de la réécriture.
