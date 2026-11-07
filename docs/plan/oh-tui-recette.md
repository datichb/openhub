# Recette manuelle de la TUI (à compléter au fil des lots tui-*)

## Wiki et briques (tui-wiki-bricks)
1. Lancer un `onboarding` qui produit plus de 4 propositions. Sur la fiche, ouvrir « Propositions du wiki (N) : décider… » : la liste défile (« 2/9 · 2 de plus ↓ »).
2. Sur la liste : `y`, puis `n` (motif vide puis `Entrée`) → « ✗ refusée » ; `e` avec ⏎, puis `e` avec `$EDITOR` (terminal suspendu puis rendu) ; `s` sur une acceptée, puis `A`.
3. `Échap` → fiche à jour ; `Entrée` → formulaire de décision (valideur et vérification compris) ; passer le checkpoint → acceptées appliquées dans le commit.
4. Lancer `doc-sync`, quitter sans commit, ouvrir « Wiki du projet » : sections, `/`, `m`, `r` ; une décision apparaît aussi dans `oh wiki pending`.
5. Proposition périmée (supprimer la ligne visée) : section « Périmées », `y` refusé avec un message.
6. « Propositions d'équipe (team-state) » : titre, palette (« team-state »), `y`, `n` confirmé, bouton « Refuser » confirmé ; sortir de la vue pendant un accept : pas de plantage.
7. Briques avec un catalogue d'équipe : une brique qui remplace le hub (« équipe ⟳ hub »), une non scellée, une avec problème, date de publication ; `f`, `/`, `Entrée` ; aucune écriture possible.

## Fiche de lancement, nettoyage, Doctor (tui-launch)
1. Omnibar : `feature` propose « run spec » et « run feature → implement-spec (avec tickets) » ; la seconde ouvre la fiche avec le bon titre ; `orchestrator` n'ouvre plus spec.
2. Lancer `feature` sur un ticket depuis le tableau : implement-spec s'ouvre ; la ligne Spec montre la dernière session spec ; le récap liste le bon agent.
3. `libre` : choisir `debugger` ; le récap le montre en agent d'entrée ; la session démarre bien sur lui.
4. « Session libre avec un agent… » : le choix est repris dans la fiche.
5. review / merge-conflicts : une branche inexistante avertit sans bloquer ; skill-refresh : « abc » dans un champ entier bloque.
6. review-feedback avec une MR : `base` est calculé (pas `main`).
7. Récap d'une session existante : `R` et `T` marchent, `r` et `t` ne lancent rien (aide du récap : « R reprendre · T transférer »).
8. Vue « Nettoyage », section « Données d'oh » (`oh gc`) : l'aperçu ne supprime rien ; `d` puis Annuler ne supprime rien ; `A` supprime puis affiche le résultat ; avec un nettoyage du démon en cours : message « occupé ».
9. Doctor : `f` sur Beads (hook à retirer), sur les orphelins, sur une configuration invalide avec sauvegarde ; `o` puis `r` : la version publiée n'est plus vérifiée sur le réseau ; `f` sur « Nettoyage (oh gc) » ouvre « Données d'oh », sur « Anciens déploiements » (projet avec des restes) ouvre « Anciens déploiements ».
10. Largeur 80 colonnes : les aides longues passent à la ligne sans chevaucher le champ suivant.

## Sessions, fan-out, lot (tui-sessions)
1. Checkpoint dont la vérification échoue, `y` sur la décision : la fiche s'ouvre sans message d'erreur ; Corriger avec une consigne ; Passer outre (motif vide refusé, puis confirmation) ; `override` et motif dans `oh session trace`.
2. Même chose sur `cp-merge` d'un fan-out : pas de « Passer outre… », ni dans « Décider… ».
3. Checkpoint avec `approver:` d'équipe et une autre identité git, `n` : valideur attendu et ⚠.
4. Fan-out réel `implement-spec -i fanout=true` : enfants sous la parente, enfant en attente dans « À traiter ».
5. Onglet Fan-out : `p` pilote actif (« tourne déjà ») ; tuer le pilote puis `p` (journal `~/.oh/sessions/<id>/fanout.log`) ; `Entrée` sur un enfant.
6. Après `cp-merge` ou session arrêtée, `d` : comparer l'aperçu à `oh fanout clean --dry-run [--discard]`, cocher, Nettoyer…, confirmer, puis `git branch` / `git worktree list` ; avant la fin, `d` refuse.
7. Trace d'une longue session : défilement, `/cp-merge`, `n`/`N` qui bouclent, `g`/`G`, `Échap`.
8. Rapports : sélection, `Entrée`, recherche, `Échap` deux fois.
9. Détail d'une session terminée avec `record:` et `next:` : sorties, suites, marque `next:` dans `e`.
10. `B` sur un enfant : filtre sur son checkpoint, `/` tous puis un, ligne ✗ (`y` refusé, `Entrée` fiche), `n` avec consigne, `C` ; seules les lignes filtrées répondues ; `Échap` annule.
11. `d` / `R` (en veille, distante prête) dans la liste ; `D`, `c`, `g` ne font plus rien (`g` = début). Dans les onglets et le lot, `?` et `Ctrl+P` marchent (plus seulement `F1`).

## Autres vues (tui-cleanup)
1. Workflows : `c` sur un brouillon ouvre le diff ; `x` sur un brouillon ne fait qu'un message ; `d` confirme (focus sur Annuler) puis supprime ; `x` sur un publié demande la raison.
2. Historique : `r` recharge ; `R` sur la version actuelle refusé ; `R` sur une ancienne version confirme puis restaure ; `c` ouvre le diff.
3. Éditeur : `d` sur un champ écrit confirme puis remet la valeur héritée, `u` annule ; `d` sur un nœud du graphe ; Suppr confirme aussi ; `Y` ouvre `$EDITOR`.
4. Board d'équipe : `/` ouvre le menu, « Rechercher un texte… » filtre ; `L` libère après confirmation ; `f` et `x` ne font plus rien.
5. Worktrees : `n` crée, `d` supprime, `M` nettoie les fusionnés (confirmé) ; libellés « principal » / « (détaché) ».
6. Détail d'équipe : `T` lance la découverte, `y` ne fait rien ; modifier un champ puis `r` demande confirmation ; synchro sans token → fenêtre « Token manquant » ; autre erreur → message.
7. MCP d'équipe : modifier puis `r` demande confirmation ; `w` sauvegarde.
8. Fournisseur : `d` confirme puis efface la configuration et la clé du trousseau.
9. Modèles (trois vues), patterns, policies, secrets, équipes, projets : `n` crée, `d` confirme ; Patterns : `y` valide ; Projets : `e` renomme.
10. TUI en anglais : parcourir ces vues, aucun texte français ni clé brute.
11. `oh dashboard` : panneaux traduits.

## Touches (tui-keys, complété par tui-docs)
1. Premier lancement après la mise à jour (effacer la préférence `tui.keys_notice` si le message a déjà été vu) : message « Nouvelles touches » une seule fois ; réaffichable depuis `?` ; la partie « Changé » liste les vues sur une ligne chacune (Sessions, Session existante, Propositions wiki, Workflows, Historique, Éditeur, Patterns, Projets, Board d'équipe, Worktrees, Détail d'équipe, Autres listes), lisible en 80 colonnes (fenêtre défilante).
2. `?` dans chaque vue : tableau cohérent avec la convention.
3. Même message en anglais : aucune clé brute.

## Passe finale (tui-docs)
1. Sessions, onglets Trace, Rapports, Fan-out : `?` ouvre l'aide (la section « Touches de la vue » montre celles de l'onglet), `Échap` la ferme et l'onglet garde les touches (`n`/`N` marchent encore) ; `Ctrl+P` ouvre la palette, `Échap` la ferme : le focus revient dans l'onglet (pas sur la liste) ; dans le lecteur d'un rapport aussi.
2. Ligne d'aide de l'omnibar : elle suit l'onglet (Trace : `/ rechercher · n/N…` ; Fan-out : `p`, `d`, `B`) et le lot (`C confirmer`) ; revient à celle de la liste après `Échap`.
3. Lot (`B`) : `?` et `Ctrl+P` disponibles ; `Échap` annule toujours le lot ; une commande lancée depuis la palette pendant le lot ne casse pas la vue au retour.
4. Formulaires (fiche de lancement, éditeur, assistants) : `?` s'écrit dans un champ, `Ctrl+P` ne fait rien, `F1` ouvre l'aide (inchangé).
5. Alias : palette `secu` → security-fix, `perf` → benchmark, `dbg` → debug, `feedback` et `retours` → review-feedback ; `oh run secu --help` (ou un lancement) résout le même workflow ; `dev`, `fast`, `rev` ne proposent rien.
6. Nettoyage : une seule entrée « Nettoyage » dans la palette (`gc`, `nettoyage`, `deploy-cleanup` y mènent) et dans les menus ; `Tab` passe d'« Anciens déploiements » à « Données d'oh » et la ligne d'aide suit ; projet avec des restes d'`oh deploy` : la section les liste, `c` diff, `d` → confirmation → toast « N projet(s) nettoyé(s) » puis « Aucun ancien déploiement » ; sans restes : message vert ; l'écran proposé une fois au démarrage est inchangé.
7. Vues touchées par la fondation (rendu inchangé) : Fan-out `p`/`B`, lot `C`, Détail d'équipe `T`, Board d'équipe `L`, Worktrees `M`, Éditeur `Y`, Doctor `f`/`o` ; MCP, Fournisseur, Détail d'équipe : « Espace » (« Space » en anglais) dans la ligne d'aide ; listes vides (rapports, fan-out, briques, propositions wiki) : texte « chargement » puis « aucun ».
8. TUI en anglais : vues « Cleanup », sections, aide ; aucune clé brute (131 clés retirées).
