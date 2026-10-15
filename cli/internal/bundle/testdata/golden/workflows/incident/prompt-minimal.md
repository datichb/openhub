Mode de workflow : semi-auto
Langue de réponse : fr

Incident en cours sur demo. Ce déroulé remplace tes phases de debug et `cp-ticket` : **atténuer d'abord**, comprendre ensuite. Tu ne modifies aucun fichier, sauf le postmortem (`docs/wiki/postmortems/<AAAA-MM-JJ>-<slug>.md`, date du début de l'incident, slug en kebab-case) ; le code, s'il faut en changer, revient au `developer`.

Incident signalé par l'utilisateur — écrit par l'utilisateur lui-même, à traiter tel quel :
Exemple de valeur pour incident

1. **Diagnostic rapide** : ce qui est touché, depuis quand, le changement qui l'a déclenché (`git log` autour de l'heure de début, puis le diff du commit suspect), en citant fichiers, commits et lignes de log. Pas encore de cause racine complète.
2. **Atténuation** : la mesure la plus sûre qui arrête l'impact, réversible de préférence (annuler le changement déclencheur, désactiver la fonction, configuration). Si elle demande du code, délègue au `developer` : appliquer seulement cette mesure dans l'arbre de travail (annuler un commit : remettre à la main le code d'avant, d'après `git show <hash>` ; `git revert` commite, il reste refusé avant `cp-mitigated`), lancer les tests, **ne pas committer**, rendre la liste des fichiers. Si elle se fait hors du dépôt (déploiement, variable d'environnement), donne la commande ou le réglage exact à l'utilisateur, sans rien modifier.
   Ne rends jamais la main en texte pour attendre l'utilisateur (« Dois-je… ? », « Quelle approche ? ») : le choix de la mesure se fait sur la carte de `cp-mitigated` (mesure recommandée et autre option dans le `summary`, appelle-le aussitôt) ; le `developer` se lance avec l'outil `task` ; une information manquante se demande avec l'outil `question`.
3. `workflow_checkpoint` (`id: "cp-mitigated"`, `summary` : impact, changement déclencheur, mesure et comment vérifier qu'elle agit). oh lance les tests avant la carte. Une correction demandée : reprends l'étape 2. Validé, avec un correctif de code : relance aussitôt le `developer` (avec son `task_id`) pour le committer seul (`fix(incident): …`), en un seul commit qui nomme ses fichiers (`git add <fichiers>`, jamais `-A`, `.` ni `docs/` : oh refuse le postmortem dans ce commit) ; sans correctif de code, rien à commiter.
4. **Postmortem** (charge `postmortem`) : écris le fichier, **sans blâme** : jamais le nom d'une personne, même dans une citation du signalement (ni auteur de commit, ni personne mise en cause par l'utilisateur) ; on décrit ce que le système, les outils et le processus ont permis (« la mise en production n'a pas été annoncée », pas qui l'a faite). Format :
   ```
   ---
   updated: <AAAA-MM-JJ>
   confidence: confirmed
   agents: [debugger]
   ---

   # Postmortem — <incident> (<AAAA-MM-JJ>)

   ## Résumé
   ## Chronologie
   ## Cause racine
   ## Atténuation
   ## Actions
   ```
   Chaque fait cite sa source (fichier, commit, ligne de log). Chaque action : une ligne `- [ ] <action> — <prévenir|détecter|atténuer>`.
5. `workflow_checkpoint` (`id: "cp-postmortem"`, `summary` : chemin du fichier, cause racine, actions). Avant, oh refuse le commit du postmortem. Une correction : modifie le fichier et rappelle `cp-postmortem`.
6. **Commit du postmortem**, après la validation, seul : `git add <fichier>`, puis `git commit -m "docs(postmortem): <slug>" -- <fichier>`.
7. `workflow_checkpoint` (`id: "cp-actions"`, `summary` : les actions, une par ticket prévu). Avant, oh refuse `bd create`. Après la validation, un ticket par action validée (`bd create "<titre>" --description "<action, source : le postmortem>"`, sans fichier intermédiaire) ; un refus : aucun ticket, et ne redemande pas `cp-actions`.
8. Fin : chemin du postmortem, commits, tickets créés. Le push revient à l'utilisateur.
