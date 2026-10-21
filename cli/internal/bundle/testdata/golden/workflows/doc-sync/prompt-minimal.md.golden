Mode de workflow : manuel
Langue de réponse : fr

Synchronise le wiki du projet (`docs/wiki/`) avec le code, puis traite les propositions en file.

Projet : demo
Chemin : /src/demo

Tu n'écris que dans `docs/wiki/` (format `doc-wiki-protocol`, à charger avec l'outil `skill`). Tu ne modifies aucun fichier de code.

1. Appelle `wiki_check` avec `since_sync: true`. oh calcule, depuis la dernière synchronisation : les commits, les fichiers modifiés, les pages concernées et leur dérive (`drift_confirmed` : page `CONFIRMÉ` qui cite un fichier modifié ; `drift_god_node` : god node touché dont aucune page n'est à jour). Sans commit ni page concernée, dis-le et passe à l'étape 3.
2. Pour chaque page concernée (dérives d'abord) : lis-la (`wiki_read`) et le code cité (`git diff <base>..HEAD -- <fichier>`, `read`). Mets à jour ce qui a changé, au format du protocole (fin de section, ligne de tag, frontmatter `updated` et `agents`). Ce que tu as revérifié dans le code reçoit `CONFIRMÉ` avec `fichier:ligne` ; ce que tu ne peux pas revérifier passe en `DÉDUIT` ou `INCERTAIN`. Ne touche pas au champ `synced:` de `index.md` : oh l'écrit.
3. Appelle `wiki_pending` : ce sont les propositions des autres sessions (review, audit…). Vérifie chacune dans le code et prépare ta recommandation (accepter, modifier, refuser), avec une raison d'une ligne. Ne les applique pas toi-même : l'utilisateur décide à la carte du checkpoint.
4. Appelle `wiki_check` avec `since_sync: true` pour vérifier tes pages, et corrige les erreurs signalées sur elles.
5. Termine par `workflow_checkpoint` (`id: "cp-sync"`, `summary` : commits couverts, pages mises à jour, dérives restantes, recommandation pour chaque proposition `wp-…`). La carte montre les propositions en file : l'utilisateur les accepte, les modifie ou les refuse.
6. Après la validation : oh a appliqué les propositions acceptées et noté la synchronisation dans `docs/wiki/index.md` (sa réponse liste les fichiers). Commite le wiki seul : `git add docs/wiki`, puis `git commit -m "docs(wiki): synchronisation avec <dernier commit couvert>"`, hooks compris, jamais `--no-verify` ; avant le checkpoint, oh refuse tout `git commit`. Le push revient à l'utilisateur.
