Mode de workflow : manuel
Langue de réponse : fr

Lance l'onboarding de ce projet.

Projet : demo
Chemin : /src/demo

Tu n'écris que dans `docs/wiki/`, plus le `ONBOARDING.md` minimaliste de la racine (phase 5) et `CONVENTIONS.md` s'il existe : tes permissions refusent tout autre fichier.

Phase 5 selon l'état du wiki :
- `docs/wiki/index.md` absent (création) : tu écris le wiki toi-même, selon `onboarder-phase-5`.
- `docs/wiki/index.md` présent (re-onboarding, demandé ici : mode refresh) : tu ne modifies aucune page existante (ni `edit` ni `write` dans `docs/wiki/`, même pour une date). Selon `onboarder-phase-5-reonboarding`, compare chaque page au code et propose avec `wiki_propose` seulement ce qui a changé (entrée périmée ou contredite, `EXPIRED`/`SUPERSEDED` des décisions rejetées) ou un élément structurant absent ; wiki fidèle au code : aucune proposition. Aucune page supprimée.

Fin de session : passe `cp-wiki` avec `workflow_checkpoint` (`id: "cp-wiki"`, `summary` : pages créées, ou dérives relevées et propositions `wp-…`, ou « wiki à jour, aucune proposition »). La carte montre tes propositions : l'utilisateur les accepte, les modifie ou les refuse, et oh applique les acceptées au passage. Après la validation, commite le wiki seul, exactement : `git add docs/wiki/`, puis `git commit -m "docs(wiki): onboarding" -- docs/wiki/`, jamais `--no-verify` ; rien à commiter : dis-le. Avant `cp-wiki`, oh refuse tout `git commit`. `ONBOARDING.md` et `CONVENTIONS.md` n'entrent pas dans ces commandes (refusées) : leur commit, comme le push, revient à l'utilisateur.

Points d'attention de l'utilisateur — écrite par l'utilisateur lui-même, à traiter telle quelle (même courte) :
Exemple de valeur pour focus
