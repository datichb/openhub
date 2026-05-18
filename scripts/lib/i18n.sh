#!/bin/bash
# i18n.sh — CLI string table for opencode-hub (EN / FR)
# Sourced by common.sh. Do not execute directly.
# Compatible with bash 3.2+ (no associative arrays).
#
# Usage:
#   t "key"           → prints the string in OC_LANG (default: en)
#   OC_LANG=fr t "key"
#
# Fallback chain: FR string → EN string → key itself

# Guard against double-sourcing
[ -n "${_I18N_LOADED:-}" ] && return 0
_I18N_LOADED=1

# ── t() — translation function ───────────────────────────────────────────────
# Translate a key according to OC_LANG (defaults to "en").
# Falls back to EN string, then to the key itself.
t() {
  local key="$1"
  local lang="${OC_LANG:-en}"

  if [ "$lang" = "fr" ]; then
    case "$key" in
      # ── Core / oc.sh ───────────────────────────────────────────────────────
      cmd.unknown)            printf '%s' "Commande inconnue" ;;
      subcmd.unknown)         printf '%s' "Sous-commande inconnue" ;;
      project_id.required)    printf '%s' "PROJECT_ID requis" ;;
      cancelled)              printf '%s' "Annulé" ;;
      no_modification)        printf '%s' "Aucune modification." ;;
      invalid_choice)         printf '%s' "Choix invalide" ;;
      deploy_later)           printf '%s' "Déployer plus tard : ./oc.sh deploy all" ;;
      deploy_apply_prompt)    printf '%s' "Appliquer le déploiement ? [Y/n] : " ;;
      deploy_cancelled)       printf '%s' "Déploiement annulé." ;;

      # ── cmd-help.sh ────────────────────────────────────────────────────────
      help.title)             printf '%s' "opencode-hub — gestionnaire d'agents IA" ;;
      help.usage)             printf '%s' "Usage :" ;;
      help.section.setup)     printf '%s' "Setup" ;;
      help.section.projects)  printf '%s' "Projets" ;;
      help.section.launch)    printf '%s' "Lancement" ;;
      help.section.analysis)  printf '%s' "Analyse" ;;
      help.section.maintenance) printf '%s' "Maintenance" ;;
      help.section.config)    printf '%s' "Configuration" ;;
      help.section.deploy_targets) printf '%s' "Cibles deploy" ;;
      help.section.agents)    printf '%s' "Agents" ;;
      help.section.skills)    printf '%s' "Skills externes (context7)" ;;
      help.section.beads)     printf '%s' "Beads (bd)" ;;
      help.section.examples)  printf '%s' "Exemples skills" ;;
      # Setup
      help.install.cmd)       printf '%s' "install" ;;
      help.install.desc)      printf '%s' "Installe les outils IA et Beads (bd)" ;;
      help.uninstall.cmd)     printf '%s' "uninstall" ;;
      help.uninstall.desc)    printf '%s' "Désinstalle opencode-hub et nettoie les artefacts" ;;
      help.init.cmd)          printf '%s' "init [PROJECT_ID] [path]" ;;
      help.init.desc)         printf '%s' "Initialise ou adopte un projet" ;;
      help.version.cmd)       printf '%s' "version" ;;
      help.version.desc)      printf '%s' "Affiche la version du hub" ;;
      # Projects
      help.status.cmd)        printf '%s' "status" ;;
      help.status.desc)       printf '%s' "Statut détaillé de tous les projets (Beads, API, agents déployés)" ;;
      help.status_short.cmd)  printf '%s' "status --short" ;;
      help.status_short.desc) printf '%s' "Liste compacte des projets (id, chemin, statut)" ;;
      help.remove.cmd)        printf '%s' "remove <PROJECT_ID>" ;;
      help.remove.desc)       printf '%s' "Supprime un projet du registre" ;;
      help.remove_clean.cmd)  printf '%s' "remove <PROJECT_ID> --clean" ;;
      help.remove_clean.desc) printf '%s' "Supprime + nettoie les fichiers déployés dans le projet" ;;
      # Launch
      help.start.cmd)         printf '%s' "start [PROJECT_ID] [prompt]" ;;
      help.start.desc)        printf '%s' "Lance l'outil IA" ;;
      help.start_dev.cmd)     printf '%s' "start [PROJECT_ID] --dev" ;;
      help.start_dev.desc)    printf '%s' "Bootstrap tickets ai-delegated (label par défaut)" ;;
      help.start_dev_label.cmd)    printf '%s' "start [PROJECT_ID] --dev --label <l>" ;;
      help.start_dev_label.desc)   printf '%s' "Bootstrap tickets filtrés par label" ;;
      help.start_dev_assignee.cmd) printf '%s' "start [PROJECT_ID] --dev --assignee <u>" ;;
      help.start_dev_assignee.desc) printf '%s' "Bootstrap tickets filtrés par assignee" ;;
      help.start_onboard.cmd) printf '%s' "start [PROJECT_ID] --onboard" ;;
      help.start_onboard.desc) printf '%s' "Déclenche l'agent onboarder (découverte projet)" ;;
      # Analysis
      help.audit.cmd)         printf '%s' "audit [PROJECT_ID]" ;;
      help.audit.desc)        printf '%s' "Lance un audit global du projet" ;;
      help.audit_type.cmd)    printf '%s' "audit [PROJECT_ID] --type <type>" ;;
      help.audit_type.desc)   printf '%s' "Audit ciblé : security | accessibility | architecture | ecodesign | observability | performance | privacy" ;;
      help.conventions.cmd)   printf '%s' "conventions [PROJECT_ID]" ;;
      help.conventions.desc)  printf '%s' "Détecte et documente les conventions du projet" ;;
      help.conventions_force.cmd)  printf '%s' "conventions [PROJECT_ID] --force" ;;
      help.conventions_force.desc) printf '%s' "Regénère CONVENTIONS.md sans confirmation" ;;
      help.review.cmd)        printf '%s' "review [PROJECT_ID]" ;;
      help.review.desc)       printf '%s' "Lance une code review via l'agent reviewer" ;;
      help.review_branch.cmd) printf '%s' "review [PROJECT_ID] --branch <branche>" ;;
      help.review_branch.desc) printf '%s' "Review d'une branche spécifique (défaut : branche courante)" ;;
      # Maintenance
      help.deploy.cmd)        printf '%s' "deploy [target] [PROJECT_ID]" ;;
      help.deploy.desc)       printf '%s' "Déploie les agents (opencode/claude-code/all)" ;;
      help.deploy_check.cmd)  printf '%s' "deploy --check [target] [PROJECT_ID]" ;;
      help.deploy_check.desc) printf '%s' "Vérifie si les agents déployés sont à jour" ;;
      help.deploy_diff.cmd)   printf '%s' "deploy --diff [target] [PROJECT_ID]" ;;
      help.deploy_diff.desc)  printf '%s' "Diff sources → déployés (propose le deploy si écart)" ;;
      help.sync.cmd)          printf '%s' "sync" ;;
      help.sync.desc)         printf '%s' "Redéploie les agents sur tous les projets enregistrés" ;;
      help.sync_dryrun.cmd)   printf '%s' "sync --dry-run" ;;
      help.sync_dryrun.desc)  printf '%s' "Vérifie la fraîcheur sur tous les projets (sans déployer)" ;;
      help.update.cmd)        printf '%s' "update" ;;
      help.update.desc)       printf '%s' "Met à jour les outils installés (opencode, bd, skills externes)" ;;
      help.upgrade.cmd)       printf '%s' "upgrade [vX.Y.Z]" ;;
      help.upgrade.desc)      printf '%s' "Met à jour les sources du hub via git (git pull ou checkout tag)" ;;
      # Config
      help.config_set.cmd)    printf '%s' "config set [PROJECT_ID] [--model m] [--provider p] [--api-key k] [--family-model f=m] [--agent-model a=m]" ;;
      help.config_set.desc)   printf '%s' "Configure le modèle, la clé API et les modèles par agent/famille d'un projet (ou du hub si pas de PROJECT_ID)" ;;
      help.config_get.cmd)    printf '%s' "config get <PROJECT_ID>" ;;
      help.config_get.desc)   printf '%s' "Affiche la configuration d'un projet" ;;
      help.config_list.cmd)   printf '%s' "config list" ;;
      help.config_list.desc)  printf '%s' "Liste toutes les configurations enregistrées" ;;
      help.config_unset.cmd)  printf '%s' "config unset <PROJECT_ID>" ;;
      help.config_unset.desc) printf '%s' "Supprime la configuration d'un projet" ;;
      help.config_language.cmd)  printf '%s' "config set language <en|fr>" ;;
      help.config_language.desc) printf '%s' "Définit la langue d'affichage du CLI (global)" ;;
      help.provider_list.cmd) printf '%s' "provider list" ;;
      help.provider_list.desc) printf '%s' "Liste les providers disponibles" ;;
      help.provider_set_default.cmd)  printf '%s' "provider set-default" ;;
      help.provider_set_default.desc) printf '%s' "Configure le provider par défaut du hub (interactif)" ;;
      help.target_info.cmd)   printf '%s' "target info <PROJECT_ID>" ;;
      help.target_info.desc)  printf '%s' "Affiche les cibles de déploiement d'un projet" ;;
      help.target_select.cmd) printf '%s' "target select <PROJECT_ID>" ;;
      help.target_select.desc) printf '%s' "Choisit les cibles de déploiement (interactif)" ;;
      # Deploy targets
      help.deploy_target.opencode) printf '%s' "opencode     → .opencode/agents/ + opencode.json" ;;
      help.deploy_target.claude)   printf '%s' "claude-code  → .claude/agents/" ;;
      help.deploy_target.all)      printf '%s' "all          → toutes les cibles actives" ;;
      # Agents
      help.agent_list.cmd)    printf '%s' "agent list" ;;
      help.agent_list.desc)   printf '%s' "Lister les agents canoniques" ;;
      help.agent_create.cmd)  printf '%s' "agent create" ;;
      help.agent_create.desc) printf '%s' "Créer un agent (interactif)" ;;
      help.agent_edit.cmd)    printf '%s' "agent edit <agent-id>" ;;
      help.agent_edit.desc)   printf '%s' "Modifier les skills et métadonnées" ;;
      help.agent_info.cmd)    printf '%s' "agent info <agent-id>" ;;
      help.agent_info.desc)   printf '%s' "Afficher le détail d'un agent" ;;
      help.agent_select.cmd)  printf '%s' "agent select <PROJECT_ID>" ;;
      help.agent_select.desc) printf '%s' "Choisir les agents à déployer pour un projet (interactif)" ;;
      help.agent_mode.cmd)    printf '%s' "agent mode <PROJECT_ID>" ;;
      help.agent_mode.desc)   printf '%s' "Afficher / overrider les modes primary/subagent par projet" ;;
      help.agent_validate.cmd)  printf '%s' "agent validate [agent-id]" ;;
      help.agent_validate.desc) printf '%s' "Valider la cohérence des agents (ou d'un seul)" ;;
      help.agent_deploy.cmd)  printf '%s' "agent deploy <agent-id> [PROJECT_ID]" ;;
      help.agent_deploy.desc) printf '%s' "Déploie un seul agent sur les cibles actives (ou celles du projet)" ;;
      # Skills
      help.skills_search.cmd) printf '%s' "skills search <query>" ;;
      help.skills_search.desc) printf '%s' "Rechercher des skills sur context7" ;;
      help.skills_info.cmd)   printf '%s' "skills info /owner/repo" ;;
      help.skills_info.desc)  printf '%s' "Prévisualiser les skills d'un dépôt" ;;
      help.skills_add.cmd)    printf '%s' "skills add /owner/repo [name]" ;;
      help.skills_add.desc)   printf '%s' "Télécharger et ajouter un skill externe" ;;
      help.skills_list.cmd)   printf '%s' "skills list" ;;
      help.skills_list.desc)  printf '%s' "Lister tous les skills (locaux + externes)" ;;
      help.skills_update.cmd) printf '%s' "skills update [name]" ;;
      help.skills_update.desc) printf '%s' "Mettre à jour un skill externe (ou tous)" ;;
      help.skills_used_by.cmd)  printf '%s' "skills used-by <skill>" ;;
      help.skills_used_by.desc) printf '%s' "Lister les agents qui utilisent ce skill" ;;
      help.skills_sync.cmd)   printf '%s' "skills sync" ;;
      help.skills_sync.desc)  printf '%s' "Re-télécharger tous les skills (après clone)" ;;
      help.skills_remove.cmd) printf '%s' "skills remove <name>" ;;
      help.skills_remove.desc) printf '%s' "Supprimer un skill externe" ;;
      help.skills_validate.cmd)  printf '%s' "skills validate [name]" ;;
      help.skills_validate.desc) printf '%s' "Valider la cohérence des skills (frontmatter, sources)" ;;
      # Projects
      help.project_rename.cmd)  printf '%s' "project rename <OLD_ID> <NEW_ID>" ;;
      help.project_rename.desc) printf '%s' "Renomme un projet dans tous les registres" ;;
      help.project_move.cmd)    printf '%s' "project move <PROJECT_ID> <path>" ;;
      help.project_move.desc)   printf '%s' "Change le chemin local d'un projet" ;;
      # Beads
      help.beads_status.cmd)  printf '%s' "beads status [PROJECT_ID]" ;;
      help.beads_status.desc) printf '%s' "Vérifie si Beads est initialisé (tous si sans ID)" ;;
      help.beads_init.cmd)    printf '%s' "beads init <PROJECT_ID>" ;;
      help.beads_init.desc)   printf '%s' "Initialise .beads/ dans le projet" ;;
      help.beads_list.cmd)    printf '%s' "beads list <PROJECT_ID>" ;;
      help.beads_list.desc)   printf '%s' "Liste les tickets ouverts du projet" ;;
      help.beads_show.cmd)    printf '%s' "beads show <PROJECT_ID> <TICKET_ID>" ;;
      help.beads_show.desc)   printf '%s' "Affiche le détail d'un ticket" ;;
      help.beads_create.cmd)  printf '%s' "beads create <PROJECT_ID> [titre] [--label l] [--type t] [--desc d]" ;;
      help.beads_create.desc) printf '%s' "Crée un ticket (non-interactif si titre fourni)" ;;
      help.beads_open.cmd)    printf '%s' "beads open <PROJECT_ID>" ;;
      help.beads_open.desc)   printf '%s' "Affiche le chemin pour utiliser bd manuellement" ;;
      help.beads_sync.cmd)    printf '%s' "beads sync <PROJECT_ID> [--pull-only|--push-only|--dry-run]" ;;
      help.beads_sync.desc)   printf '%s' "Synchronise avec le tracker externe (Jira/GitLab)" ;;
      help.beads_tracker_status.cmd)  printf '%s' "beads tracker status <PROJECT_ID>" ;;
      help.beads_tracker_status.desc) printf '%s' "Affiche le statut de connexion au tracker" ;;
      help.beads_tracker_setup.cmd)   printf '%s' "beads tracker setup <PROJECT_ID>" ;;
      help.beads_tracker_setup.desc)  printf '%s' "Configure le tracker (interactif)" ;;
      help.beads_tracker_switch.cmd)  printf '%s' "beads tracker switch <PROJECT_ID>" ;;
      help.beads_tracker_switch.desc) printf '%s' "Change de provider de tracker" ;;
      help.beads_tracker_set_sync_mode.cmd)  printf '%s' "beads tracker set-sync-mode <PROJECT_ID> [mode]" ;;
      help.beads_tracker_set_sync_mode.desc) printf '%s' "Définit le mode de sync (bidirectional|pull-only|push-only)" ;;
      help.beads_board.cmd)       printf '%s' "beads board [PROJECT_ID] [--watch]" ;;
      help.beads_board.desc)      printf '%s' "Affiche le board kanban dans le terminal" ;;
      help.beads_board_watch.cmd) printf '%s' "beads board [PROJECT_ID] --watch [--interval N]" ;;
      help.beads_board_watch.desc) printf '%s' "Rafraîchissement automatique du board (défaut : 5s)" ;;

      # ── cmd-agent.sh ───────────────────────────────────────────────────────
      agent.title)            printf '%s' "oc agent — Gestion des agents canoniques" ;;
      agent.list)             printf '%s' "list                  Lister les agents disponibles" ;;
      agent.info_cmd)         printf '%s' "info <agent-id>       Afficher le détail d'un agent" ;;
      agent.create_cmd)       printf '%s' "create                Créer un nouvel agent (interactif)" ;;
      agent.edit_cmd)         printf '%s' "edit <agent-id>       Modifier les skills et métadonnées d'un agent" ;;
      agent.select_cmd)       printf '%s' "select <PROJECT_ID>   Choisir les agents à déployer pour un projet" ;;
      agent.mode_cmd)         printf '%s' "mode <PROJECT_ID>     Afficher / overrider les modes primary/subagent" ;;
      agent.validate_cmd)     printf '%s' "validate [agent-id]   Valider la cohérence de tous les agents (ou d'un seul)" ;;
      agent.deploy_cmd)       printf '%s' "deploy <agent-id> [PROJECT_ID]  Déployer un seul agent sur les cibles actives" ;;
      agent.keytest_cmd)      printf '%s' "keytest               Diagnostic clavier — affiche les octets reçus" ;;
      agent.usage.info)       printf '%s' "Usage : oc agent info <agent-id>" ;;
      agent.usage.edit)       printf '%s' "Usage : oc agent edit <agent-id>" ;;
      agent.usage.select)     printf '%s' "Usage : oc agent select <PROJECT_ID>" ;;
      agent.usage.mode)       printf '%s' "Usage : oc agent mode <PROJECT_ID>" ;;
      agent.not_found)        printf '%s' "Agent introuvable dans agents/" ;;
      agent.id_required)      printf '%s' "Identifiant requis." ;;
      agent.already_exists)   printf '%s' "existe déjà. Utilisez 'oc agent edit'" ;;
      agent.no_skill)         printf '%s' "(pas de description)" ;;
      agent.no_agents)        printf '%s' "(aucun agent dans agents/)" ;;
      agent.skills_assigned)  printf '%s' "Skills assignés :" ;;
      agent.skill_missing)    printf '%s' "(fichier absent)" ;;
      agent.create.title)     printf '%s' "Créer un nouvel agent" ;;
      agent.edit.title)       printf '%s' "Modifier l'agent" ;;
      agent.list.title)       printf '%s' "Agents canoniques" ;;
      agent.select.title)     printf '%s' "Sélection des agents —" ;;
      agent.mode.title)       printf '%s' "Modes des agents —" ;;
      agent.validate.summary) printf '%s' "Résumé :" ;;
      agent.validate.ok)      printf '%s' "ok" ;;
      agent.validate.errors)  printf '%s' "erreur(s)" ;;
      agent.validate.warnings) printf '%s' "avertissement(s)" ;;
      agent.examples)         printf '%s' "Exemples :" ;;

      # ── cmd-config.sh ──────────────────────────────────────────────────────
      config.title)           printf '%s' "Usage : ./oc.sh config <sous-commande> [options]" ;;
      config.no_entry)        printf '%s' "Aucune configuration pour" ;;
      config.no_file)         printf '%s' "Aucune configuration enregistrée (api-keys.local.md absent)" ;;
      config.no_entries)      printf '%s' "Aucune entrée dans api-keys.local.md" ;;
      config.saved)           printf '%s' "Configurations API enregistrées :" ;;
      config.delete_confirm)  printf '%s' "Supprimer la configuration de" ;;
      config.deleted)         printf '%s' "Configuration supprimée pour" ;;
      config.written)         printf '%s' "Configuration enregistrée pour" ;;
      config.apply_now)       printf '%s' "Appliquer maintenant au projet (re-déployer opencode.json) ? [Y/n] : " ;;
      config.apply_later)     printf '%s' "Appliquer plus tard : ./oc.sh deploy all" ;;
      config.no_path)         printf '%s' "Chemin non enregistré pour" ;;
      config.apply_via)       printf '%s' "— appliquer via : ./oc.sh deploy all" ;;
      config.api_key.required) printf '%s' "Clé API requise" ;;
      config.api_key.unchanged) printf '%s' "Clé API inchangée" ;;
      config.language.invalid) printf '%s' "Valeur de langue invalide (accepté : en, fr)" ;;
      config.language.saved)  printf '%s' "Langue CLI définie à" ;;
      config.language.current) printf '%s' "Langue CLI actuelle :" ;;

      # ── cmd-beads.sh ───────────────────────────────────────────────────────
      beads.title)            printf '%s' "Gestion Beads :" ;;
      beads.tracker.title)    printf '%s' "Synchronisation tracker (Jira / GitLab) :" ;;
      beads.not_installed)    printf '%s' "bd (Beads) n'est pas installé" ;;
      beads.install_hint)     printf '%s' "Installation : brew install bd" ;;
      beads.not_initialized)  printf '%s' "Beads non initialisé dans" ;;
      beads.already_initialized) printf '%s' "Beads déjà initialisé dans" ;;
      beads.initialized)      printf '%s' "Beads initialisé dans" ;;
      beads.init_failed)      printf '%s' "Échec de bd init" ;;
      beads.no_tracker)       printf '%s' "Aucun tracker configuré pour" ;;
      beads.tracker.configure) printf '%s' "Configurer : ./oc.sh beads tracker setup" ;;
      beads.tracker.invalid)  printf '%s' "Choix invalide" ;;
      beads.tracker.unknown_subcmd) printf '%s' "Sous-commande tracker inconnue :" ;;
      beads.tracker.usage)    printf '%s' "Usage : ./oc.sh beads tracker <status|setup|switch> <PROJECT_ID>" ;;
      beads.status.all)       printf '%s' "Statut Beads — tous les projets" ;;
      beads.open_hint)        printf '%s' "Vous pouvez maintenant utiliser bd directement dans ce répertoire" ;;
      beads.labels.registered)       printf '%s' "Labels enregistrés :" ;;
      beads.labels.failed)           printf '%s' "Échec enregistrement labels dans Beads" ;;
      beads.labels.fetching_gitlab)  printf '%s' "Import des labels depuis GitLab…" ;;
      beads.labels.fetching_jira)    printf '%s' "Import des labels depuis Jira…" ;;
      beads.labels.imported_gitlab)  printf '%s' "Labels GitLab importés :" ;;
      beads.labels.imported_jira)    printf '%s' "Labels Jira importés :" ;;
      beads.sync.failed)      printf '%s' "Échec du sync" ;;
      beads.sync.done)        printf '%s' "Sync terminé pour" ;;
      beads.sync_mode.applied)    printf '%s' "Mode de sync appliqué :" ;;
      beads.sync_mode.current)    printf '%s' "Mode de sync actuel :" ;;
      beads.sync_mode.choose)     printf '%s' "Choisir le mode de sync pour" ;;
      beads.sync_mode.desc_bidi)  printf '%s' "Import et export (défaut)" ;;
      beads.sync_mode.desc_pull)  printf '%s' "Import seulement depuis le tracker" ;;
      beads.sync_mode.desc_push)  printf '%s' "Export seulement vers le tracker" ;;
      beads.sync_mode.invalid)    printf '%s' "Mode de sync invalide (bidirectional | pull-only | push-only)" ;;
      beads.sync_mode.save_failed) printf '%s' "Échec de la sauvegarde du Sync mode pour" ;;
      beads.sync_mode.saved)      printf '%s' "Sync mode enregistré :" ;;
      beads.status.open_tickets) printf '%s' "Tickets ouverts —" ;;
      beads.create.title)     printf '%s' "Créer un ticket —" ;;
      beads.create.title_required) printf '%s' "Titre requis" ;;
      beads.create.creating)  printf '%s' "Création du ticket dans" ;;
      beads.create.failed)    printf '%s' "Échec de bd create" ;;
      beads.gitignore_added)  printf '%s' ".beads/ ajouté au .git/info/exclude du projet (exclusion locale, invisible aux autres développeurs)" ;;
      beads.gitignore_exists) printf '%s' ".beads/ déjà présent dans .git/info/exclude du projet" ;;

      beads.board.title)      printf '%s' "Board kanban (terminal) :" ;;
      board.no_project)       printf '%s' "Aucun projet Beads trouvé dans le répertoire courant" ;;
      board.usage_hint)       printf '%s' "Usage : ./oc.sh beads board <PROJECT_ID>" ;;
      board.watch_mode)       printf '%s' "Mode watch activé — rafraîchissement toutes les 5s" ;;
      board.watch_quit)       printf '%s' "pour quitter" ;;
      board.empty_column)     printf '%s' "— aucun ticket —" ;;

      # ── cmd-target.sh ──────────────────────────────────────────────────────
      target.title)           printf '%s' "oc target — Gestion des cibles de déploiement par projet" ;;
      target.usage.info)      printf '%s' "Usage : oc target info <PROJECT_ID>" ;;
      target.usage.select)    printf '%s' "Usage : oc target select <PROJECT_ID>" ;;
      target.info_cmd)        printf '%s' "info <PROJECT_ID>     Afficher les cibles configurées pour un projet" ;;
      target.select_cmd)      printf '%s' "select <PROJECT_ID>   Choisir les cibles de déploiement pour un projet" ;;
      target.select.title)    printf '%s' "Sélection des cibles —" ;;
      target.current_all)     printf '%s' "Sélection actuelle : toutes les cibles actives (hub.json)" ;;
      target.reset_done)      printf '%s' "Cibles réinitialisées pour" ;;
      target.reset_suffix)    printf '%s' "→ toutes les cibles actives seront utilisées" ;;
      target.selected)        printf '%s' "cible(s) sélectionnée(s) pour" ;;
      target.targets_label)   printf '%s' "Cibles pour" ;;
      target.all_active)      printf '%s' "(toutes les cibles actives de hub.json)" ;;
      target.available)       printf '%s' "Cibles disponibles :" ;;
      target.default_hint)    printf '%s' "Par défaut (si non configuré), les cibles actives de hub.json sont utilisées." ;;
      target.examples)        printf '%s' "Exemples :" ;;

      # ── cmd-provider.sh ────────────────────────────────────────────────────
      provider.title)         printf '%s' "Fournisseurs LLM disponibles" ;;
      provider.default_title) printf '%s' "Fournisseur LLM par défaut" ;;
      provider.project_title) printf '%s' "Fournisseur LLM —" ;;
      provider.choose_default) printf '%s' "Choisir le fournisseur par défaut pour tous les projets :" ;;
      provider.choose_project) printf '%s' "Choisir le fournisseur pour ce projet :" ;;
      provider.current)       printf '%s' "Fournisseur actuel :" ;;
      provider.current_project) printf '%s' "Fournisseur actuel du projet :" ;;
      provider.hub_default)   printf '%s' "Fournisseur du hub (par défaut) :" ;;
      provider.selected)      printf '%s' "Fournisseur sélectionné :" ;;
      provider.saved)         printf '%s' "Fournisseur par défaut enregistré :" ;;
      provider.set_done)      printf '%s' "Fournisseur configuré pour" ;;
      provider.no_provider)   printf '%s' "Aucun fournisseur configuré" ;;
      provider.no_catalog)    printf '%s' "Catalogue providers.json introuvable" ;;
      provider.hub_json_missing) printf '%s' "hub.json introuvable — lancez d'abord : ./oc.sh install" ;;
      provider.hub_json_added_gitignore) printf '%s' "hub.json ajouté au .gitignore (contient une clé API)" ;;
      provider.api_key_required) printf '%s' "Clé API requise pour ce fournisseur" ;;
      provider.api_key_empty_warn) printf '%s' "Clé API vide — le fournisseur sera enregistré sans clé" ;;
      provider.source_project) printf '%s' "Source : configuration du projet" ;;
      provider.source_hub)    printf '%s' "Source : fournisseur par défaut du hub" ;;
      provider.apply_hint)    printf '%s' "Appliquer aux projets : ./oc.sh deploy all <PROJECT_ID>" ;;
      provider.usage)         printf '%s' "Usage : ./oc.sh provider <sous-commande>" ;;
      provider.list_cmd)      printf '%s' "list                           Liste les fournisseurs disponibles" ;;
      provider.set_default_cmd) printf '%s' "set-default                    Configure le fournisseur par défaut (hub)" ;;

      # ── cmd-skills.sh ──────────────────────────────────────────────────────
      skills.title)           printf '%s' "oc skills — Gestion des skills externes" ;;
      skills.available)       printf '%s' "Skills disponibles" ;;
      skills.local)           printf '%s' "Skills locaux :" ;;
      skills.external)        printf '%s' "Skills externes (context7) :" ;;
      skills.none_local)      printf '%s' "(aucun)" ;;
      skills.none_external)   printf '%s' "(aucun — utilisez : oc skills add /owner/repo [name])" ;;
      skills.search.title)    printf '%s' "Recherche de skills :" ;;
      skills.search.usage)    printf '%s' "Usage : oc skills search <query>" ;;
      skills.info.usage)      printf '%s' "Usage : oc skills info /owner/repo" ;;
      skills.add.title)       printf '%s' "Ajout d'un skill externe" ;;
      skills.add.usage)       printf '%s' "Usage : oc skills add /owner/repo [skill-name]" ;;
      skills.add.overwrite)   printf '%s' "Écraser ? (y/N) : " ;;
      skills.add.already_exists) printf '%s' "Le skill 'external/..." ;;
      skills.add.added)       printf '%s' "Skill ajouté →" ;;
      skills.add.use_hint)    printf '%s' "Pour l'utiliser dans un agent, ajoutez 'external/<name>' à la liste skills: de son fichier agents/<famille>/<id>.md, puis relancez : ./oc.sh deploy all" ;;
      skills.remove.usage)    printf '%s' "Usage : oc skills remove <skill-name>" ;;
      skills.remove.not_found) printf '%s' "Skill externe introuvable." ;;
      skills.remove.confirm)  printf '%s' "Supprimer le skill externe" ;;
      skills.remove.done)     printf '%s' "Skill supprimé." ;;
      skills.remove.agent_hint) printf '%s' "N'oubliez pas de le retirer de la liste skills: de vos agents si nécessaire." ;;
      skills.update.title)    printf '%s' "Mise à jour des skills externes" ;;
      skills.update.no_skills) printf '%s' "Aucun skill externe enregistré. Rien à mettre à jour." ;;
      skills.update.already_up_to_date) printf '%s' "est déjà à jour." ;;
      skills.update.apply)    printf '%s' "Appliquer la mise à jour pour" ;;
      skills.update.skipped)  printf '%s' "Mise à jour ignorée pour" ;;
      skills.update.updated)  printf '%s' "skill(s) mis à jour." ;;
      skills.update.unchanged) printf '%s' "skill(s) inchangé(s) ou ignoré(s)." ;;
      skills.used_by.title)   printf '%s' "Agents utilisant le skill :" ;;
      skills.used_by.usage)   printf '%s' "Usage : oc skills used-by <skill>" ;;
      skills.used_by.none)    printf '%s' "(aucun agent n'utilise ce skill)" ;;
      skills.sync.title)      printf '%s' "Synchronisation des skills externes" ;;
      skills.sync.none)       printf '%s' "Aucun skill externe enregistré. Rien à synchroniser." ;;
      skills.sync.done)       printf '%s' "skill(s) synchronisé(s)." ;;
      skills.examples)        printf '%s' "Exemples :" ;;

      # ── cmd-remove.sh ──────────────────────────────────────────────────────
      remove.not_found)       printf '%s' "Projet introuvable dans le registre" ;;
      remove.no_path)         printf '%s' "Chemin local introuvable pour" ;;
      remove.clean_ignored)   printf '%s' "— --clean ignoré" ;;
      remove.confirm_clean)   printf '%s' "Supprimer PROJECT du registre ET nettoyer les fichiers déployés dans PATH ? [y/N] " ;;
      remove.confirm)         printf '%s' "Supprimer PROJECT ? [y/N] " ;;
      remove.done)            printf '%s' "retiré du registre" ;;
      remove.path_removed)    printf '%s' "Chemin supprimé de paths.local.md" ;;
      remove.api_key_removed) printf '%s' "Clé API supprimée de api-keys.local.md" ;;
      remove.projects_removed) printf '%s' "supprimé de projects.md" ;;

      # ── cmd-start.sh ───────────────────────────────────────────────────────
      start.no_projects)      printf '%s' "Aucun projet enregistré → ./oc.sh init" ;;
      start.choose_project)   printf '%s' "Choisir un projet :" ;;
      start.target_unavailable) printf '%s' "Cible non disponible → oc install" ;;
      start.agents_not_deployed) printf '%s' "Agents non déployés pour" ;;
      start.deploy_now)       printf '%s' "Déployer maintenant ? [Y/n] : " ;;
      start.beads_not_init)   printf '%s' "Beads non initialisé dans ce projet (aucun .beads/ trouvé)" ;;
      start.init_beads_now)   printf '%s' "Initialiser Beads maintenant ? [Y/n] : " ;;
      start.setup_upstream)   printf '%s' "Configurer l'upstream Git (git remote add upstream) ? [Y/n] : " ;;
      start.upstream_url)     printf '%s' "URL du remote upstream : " ;;
      start.upstream_empty)   printf '%s' "URL vide — configurer plus tard : git remote add upstream <url>" ;;
      start.upstream_ok)      printf '%s' "Remote upstream configuré :" ;;
      start.upstream_failed)  printf '%s' "Échec de la configuration upstream — configurer manuellement" ;;
      start.labels_registered) printf '%s' "Labels enregistrés :" ;;
      start.labels_failed)    printf '%s' "Échec enregistrement labels dans Beads" ;;
      start.beads_init_failed) printf '%s' "Échec de bd init — initialiser plus tard : ./oc.sh beads init" ;;
      start.beads_later)      printf '%s' "Initialiser plus tard : ./oc.sh beads init" ;;
      start.dev_requires_beads) printf '%s' "--dev requiert Beads initialisé dans ce projet" ;;
      start.dev_beads_hint)   printf '%s' "Lancez d'abord : ./oc.sh beads init" ;;
      start.dev_requires_bd)  printf '%s' "--dev requiert bd (Beads) : brew install bd" ;;
      start.press_enter)      printf '%s' "Appuyer sur Entrée pour lancer" ;;
      start.dev_label_exclusive) printf '%s' "--label et --assignee sont mutuellement exclusifs" ;;
      start.dev_needs_dev_flag) printf '%s' "--label et --assignee nécessitent --dev" ;;
      start.dev_onboard_exclusive) printf '%s' "--dev et --onboard sont mutuellement exclusifs" ;;

      # ── install/uninstall ──────────────────────────────────────────────────
      install.title)          printf '%s' "Installation opencode-hub" ;;
      install.already_done)   printf '%s' "Déjà installé." ;;
      install.done)           printf '%s' "Installation terminée." ;;
      install.deps_required)  printf '%s' "Dépendances requises :" ;;
      install.os_detected)    printf '%s' "OS détecté :" ;;
      install.jq_missing)     printf '%s' "jq non détecté — dépendance critique pour opencode-hub" ;;
      install.jq_install_brew) printf '%s' "Installer jq via Homebrew ? [Y/n] : " ;;
      install.jq_installed)   printf '%s' "jq installé" ;;
      install.jq_failed)      printf '%s' "Échec installation jq — à installer manuellement" ;;
      install.jq_degraded)    printf '%s' "Certaines fonctionnalités (deploy, skills, beads) seront dégradées sans jq" ;;
      install.jq_manual)      printf '%s' "Installer jq manuellement :" ;;
      install.targets_title)  printf '%s' "Cibles à configurer" ;;
      install.target_opencode) printf '%s' "OpenCode (recommandé)" ;;
      install.target_claude)  printf '%s' "Claude Code" ;;
      install.target_all)     printf '%s' "Tout" ;;
      install.choose_prompt)  printf '%s' "Choisir (1-3, défaut: 1) : " ;;
      install.hub_json_exists) printf '%s' "config/hub.json existe déjà." ;;
      install.hub_json_overwrite) printf '%s' "Écraser avec les nouvelles cibles ? [y/N] : " ;;
      install.hub_json_kept)  printf '%s' "config/hub.json conservé tel quel." ;;
      install.hub_json_created) printf '%s' "config/hub.json créé (cibles :" ;;
      install.provider_title) printf '%s' "Fournisseur LLM" ;;
      install.provider_choose) printf '%s' "Quel fournisseur d'IA utiliser pour tous vos projets ?" ;;
      install.provider_recommended) printf '%s' "(recommandé)" ;;
      install.provider_skip)  printf '%s' "Ignorer (configurer plus tard via ./oc.sh provider set-default)" ;;
      install.provider_api_key) printf '%s' "(laisser vide pour ignorer) :" ;;
      install.provider_base_url) printf '%s' "URL de base" ;;
      install.provider_configured) printf '%s' "Fournisseur configuré :" ;;
      install.provider_skipped) printf '%s' "Fournisseur non configuré — utiliser : ./oc.sh provider set-default" ;;
      install.projects_ready) printf '%s' "projects.md prêt" ;;
      install.paths_created)  printf '%s' "paths.local.md créé" ;;
      install.skills_tip)     printf '%s' "Tip : Enrichissez vos agents avec des skills tiers via context7 :" ;;
      install.beads_title)    printf '%s' "Installation de Beads (bd)" ;;
      install.beads_already)  printf '%s' "Beads déjà installé" ;;
      install.beads_missing)  printf '%s' "Beads (bd) non détecté — requis pour la gestion des tickets" ;;
      install.beads_install_prompt) printf '%s' "Installer Beads ? [Y/n] : " ;;
      install.beads_via_brew) printf '%s' "Installation de Beads via Homebrew..." ;;
      install.beads_installed) printf '%s' "Beads installé" ;;
      install.beads_brew_failed) printf '%s' "Échec via Homebrew — tentative via curl..." ;;
      install.beads_via_curl) printf '%s' "Installation de Beads via curl..." ;;
      install.beads_curl_installed) printf '%s' "Beads installé via curl" ;;
      install.beads_failed)   printf '%s' "Échec installation Beads — installer manuellement : brew install beads" ;;
      install.beads_no_tools) printf '%s' "Homebrew et curl introuvables — installer Beads manuellement :" ;;
      install.beads_later)    printf '%s' "Beads non installé — à installer plus tard : brew install beads" ;;
      install.ready)          printf '%s' "opencode-hub prêt !" ;;
      uninstall.title)        printf '%s' "Désinstallation opencode-hub" ;;
      uninstall.confirm)      printf '%s' "Désinstaller opencode-hub ? [y/N] " ;;
      uninstall.done)         printf '%s' "Désinstallation terminée." ;;
      uninstall.cancelled)    printf '%s' "Désinstallation annulée." ;;

      # ── cmd-init.sh ────────────────────────────────────────────────────────
      init.gitignore_opencode_prompt) printf '%s' "Ajouter opencode.json et .opencode/ au .git/info/exclude du projet ? [y/N] : " ;;
      init.gitignore_opencode_added)  printf '%s' "opencode.json et .opencode/ ajoutés au .git/info/exclude du projet (exclusion locale, invisible pour les autres devs)" ;;
      init.gitignore_opencode_exists) printf '%s' "opencode.json et .opencode/ déjà présents dans le .git/info/exclude du projet" ;;

      # ── cmd-list.sh ────────────────────────────────────────────────────────
      list.title)             printf '%s' "Projets enregistrés" ;;
      list.no_projects)       printf '%s' "Aucun projet enregistré" ;;
      list.col_id)            printf '%s' "ID" ;;
      list.col_path)          printf '%s' "Chemin local" ;;
      list.col_status)        printf '%s' "Statut" ;;
      list.status_no_path)    printf '%s' "⚠ sans chemin" ;;
      list.path_undefined)    printf '%s' "non défini" ;;
      list.status_ok)         printf '%s' "✔ accessible" ;;
      list.status_missing)    printf '%s' "✘ introuvable" ;;

      # ── cmd-status.sh ──────────────────────────────────────────────────────
      status.no_path)         printf '%s' "Chemin local non configuré (paths.local.md)" ;;
      status.dir_missing)     printf '%s' "Dossier introuvable : " ;;
      status.path_label)      printf '%s' "Chemin : " ;;
      status.beads_ok)        printf '%s' "Beads initialisé" ;;
      status.beads_not_init)  printf '%s' "Beads non initialisé" ;;
      status.api_not_set)     printf '%s' "Clé API non configurée" ;;
      status.tracker_none)    printf '%s' "Tracker : aucun" ;;
      status.agents_deployed) printf '%s' "Agents déployés" ;;
      status.agents_missing)  printf '%s' "Agents non déployés pour" ;;
      status.title)           printf '%s' "Statut des projets" ;;
      status.no_projects)     printf '%s' "Aucun projet enregistré — démarrer avec : ./oc.sh init" ;;

      # ── cmd-sync.sh ────────────────────────────────────────────────────────
      sync.title_dryrun)      printf '%s' "Vérification globale des agents déployés (dry-run)" ;;
      sync.title)             printf '%s' "Synchronisation des agents sur tous les projets" ;;
      sync.no_projects)       printf '%s' "Aucun projet enregistré — lancez : ./oc.sh init" ;;
      sync.no_targets)        printf '%s' "Aucune cible configurée — vérifier active_targets dans config/hub.json" ;;
      sync.project_label)     printf '%s' "── Projet : " ;;
      sync.path_undefined)    printf '%s' "path non défini localement, ignoré" ;;
      sync.dir_missing)       printf '%s' "dossier introuvable : " ;;
      sync.missing)           printf '%s' "✗ MANQUANT" ;;
      sync.stale)             printf '%s' "⚠ OBSOLÈTE" ;;
      sync.ok)                printf '%s' "✓ À JOUR" ;;
      sync.deployed)          printf '%s' "déployé" ;;
      sync.deploy_failed)     printf '%s' "échec déploiement" ;;
      sync.target_unavailable) printf '%s' "non disponible — ignoré" ;;
      sync.result_ok)         printf '%s' "à jour" ;;
      sync.result_stale)      printf '%s' "obsolète(s)/manquant(s)" ;;
      sync.result_skipped)    printf '%s' "ignoré(s)" ;;
      sync.deploy_hint)       printf '%s' "Pour déployer : ./oc.sh sync" ;;
      sync.result_deployed)   printf '%s' "déployé(s)" ;;

      # ── cmd-update.sh ──────────────────────────────────────────────────────
      update.title)           printf '%s' "Mise à jour des outils" ;;
      update.beads_updating)  printf '%s' "Mise à jour Beads (bd)..." ;;
      update.beads_done)      printf '%s' "Beads mis à jour via Homebrew" ;;
      update.beads_failed)    printf '%s' "Échec mise à jour Beads — déjà à jour ou erreur Homebrew" ;;
      update.beads_not_brew)  printf '%s' "bd installé mais pas via Homebrew — mise à jour manuelle requise" ;;
      update.beads_manual_hint) printf '%s' "  → https://beads.sh ou via votre gestionnaire de paquets" ;;
      update.bd_missing)      printf '%s' "bd non installé — lancez : oc install" ;;
      update.skills_updating) printf '%s' "Mise à jour des skills externes..." ;;
      update.skills_none)     printf '%s' "Aucun skill externe enregistré — étape ignorée." ;;
      update.done)            printf '%s' "Mise à jour terminée" ;;
      update.skills_stale_warn) printf '%s' "Des skills ont été mis à jour — les agents déployés dans vos projets peuvent être obsolètes." ;;
      update.sync_now)        printf '%s' "  Lancer oc sync pour redéployer sur tous les projets ? [Y/n] : " ;;
      update.sync_later)      printf '%s' "Redéployer manuellement : ./oc.sh sync" ;;

      # ── cmd-upgrade.sh ─────────────────────────────────────────────────────
      upgrade.title)              printf '%s' "Mise à jour du hub" ;;
      upgrade.invalid_ref)        printf '%s' "Référence invalide — format attendu vX.Y.Z ou X.Y.Z" ;;
      upgrade.ref_hint)           printf '%s' "Exemple : oc upgrade v1.1.0" ;;
      upgrade.fetching_tag)       printf '%s' "Récupération du tag" ;;
      upgrade.tag_not_found)      printf '%s' "Tag introuvable — vérifier que la release existe" ;;
      upgrade.fetch_failed)       printf '%s' "Échec du fetch — vérifier la connexion réseau" ;;
      upgrade.pulling)            printf '%s' "Récupération des dernières sources (git pull)" ;;
      upgrade.already_uptodate)   printf '%s' "Le hub est déjà à jour." ;;
      upgrade.pull_failed)        printf '%s' "Échec du git pull — vérifier la connexion réseau" ;;
      upgrade.updated)            printf '%s' "Hub mis à jour" ;;
      upgrade.done)               printf '%s' "Hub à jour" ;;
      upgrade.sync_stale_warn)    printf '%s' "Les sources ont été mises à jour — les agents déployés dans vos projets peuvent être obsolètes." ;;
      upgrade.sync_now)           printf '%s' "  Lancer oc sync pour redéployer sur tous les projets ? [Y/n] : " ;;
      upgrade.sync_later)         printf '%s' "Redéployer manuellement : ./oc.sh sync" ;;

      # ── cmd-review.sh ──────────────────────────────────────────────────────
      review.no_projects)         printf '%s' "Aucun projet enregistré → ./oc.sh init" ;;
      review.choose_project)      printf '%s' "Choisir un projet :" ;;
      review.choose_number)       printf '%s' "  Numéro : " ;;
      review.invalid_choice)      printf '%s' "Choix invalide : " ;;
      review.target_unavailable)  printf '%s' "Cible '" ;;
      review.target_unavailable_suffix) printf '%s' "non disponible → oc install" ;;
      review.no_branch)           printf '%s' "Impossible de détecter la branche courante. Utilisez --branch <branche>" ;;
      review.label_path)          printf '%s' "Chemin" ;;
      review.label_target)        printf '%s' "Cible" ;;
      review.label_branch)        printf '%s' "Branche" ;;
      review.label_agent)         printf '%s' "Agent" ;;
      review.agent_missing_config) printf '%s' "Agent absent de la sélection projet : " ;;
      review.add_agent_prompt)    printf '%s' "Ajouter reviewer à la sélection du projet ? [Y/n] : " ;;
      review.agent_updated)       printf '%s' "Agents mis à jour dans projects.md : " ;;
      review.redeploy_prompt)     printf '%s' "Redéployer les agents maintenant ? [Y/n] : " ;;
      review.redeploy_later)      printf '%s' "Redéployer plus tard : ./oc.sh deploy " ;;
      review.agents_not_deployed) printf '%s' "Agents non déployés pour " ;;
      review.agent_not_deployed)  printf '%s' "Agent non déployé : " ;;
      review.deploy_now_prompt)   printf '%s' "Déployer maintenant ? [Y/n] : " ;;
      review.deploy_skipped)      printf '%s' "Déploiement ignoré — l'agent risque d'être introuvable" ;;
      review.deploy_later)        printf '%s' "Déployer plus tard : ./oc.sh deploy " ;;
      review.main_agent)          printf '%s' "Agent : " ;;
      review.launching)           printf '%s' "Lancement de la review " ;;

      # ── cmd-audit.sh ───────────────────────────────────────────────────────
      audit.invalid_type)     printf '%s' "Type d'audit invalide : '" ;;
      audit.valid_types)      printf '%s' "Types valides : " ;;
      audit.no_projects)      printf '%s' "Aucun projet enregistré → ./oc.sh init" ;;
      audit.choose_project)   printf '%s' "Choisir un projet :" ;;
      audit.choose_number)    printf '%s' "  Numéro : " ;;
      audit.invalid_choice)   printf '%s' "Choix invalide : " ;;
      audit.target_unavailable) printf '%s' "Cible '" ;;
      audit.label_path)       printf '%s' "Chemin" ;;
      audit.label_target)     printf '%s' "Cible" ;;
      audit.label_agents)     printf '%s' "Agents" ;;
      audit.label_type)       printf '%s' "Type" ;;
      audit.agents_missing_config) printf '%s' "Agent(s) absent(s) de la sélection projet : " ;;
      audit.add_agents_prompt) printf '%s' "Ajouter ces agents à la sélection du projet ? [Y/n] : " ;;
      audit.agents_updated)   printf '%s' "Agents mis à jour dans projects.md : " ;;
      audit.redeploy_prompt)  printf '%s' "Redéployer les agents maintenant ? [Y/n] : " ;;
      audit.redeploy_later)   printf '%s' "Redéployer plus tard : ./oc.sh deploy " ;;
      audit.searching_agents) printf '%s' "Recherche des agents audit disponibles dans " ;;
      audit.no_agents_deployed) printf '%s' "Aucun agent audit déployé dans " ;;
      audit.deploy_hint)      printf '%s' "Déployer les agents : ./oc.sh deploy " ;;
      audit.add_agents_hint)  printf '%s' "Puis ajouter les agents au projet : ./oc.sh agent edit " ;;
      audit.available_agents) printf '%s' "Agents audit disponibles :" ;;
      audit.choose_agent)     printf '%s' "  Choisir un agent (numéro) : " ;;
      audit.agent_selected)   printf '%s' "Agent sélectionné : " ;;
      audit.agents_not_deployed) printf '%s' "Agents non déployés pour " ;;
      audit.deploy_now_prompt) printf '%s' "Déployer maintenant ? [Y/n] : " ;;
      audit.deploy_skipped)   printf '%s' "Déploiement ignoré — l'agent risque d'être introuvable" ;;
      audit.deploy_later)     printf '%s' "Déployer plus tard : ./oc.sh deploy " ;;
      audit.agents_not_deployed_list) printf '%s' "Agent(s) non déployé(s) : " ;;
      audit.main_agent)       printf '%s' "Agent principal : " ;;
      audit.launching)        printf '%s' "Lancement de l'audit " ;;

      # ── cmd-version.sh ─────────────────────────────────────────────────────
      version.unknown)        printf '%s' "inconnu" ;;

      *) t_en "$key" ;;
    esac
  else
    t_en "$key"
  fi
}

# EN strings (used as fallback from FR and directly for lang=en)
t_en() {
  local key="$1"
  case "$key" in
    # ── Core / oc.sh ─────────────────────────────────────────────────────────
    cmd.unknown)            printf '%s' "Unknown command" ;;
    subcmd.unknown)         printf '%s' "Unknown subcommand" ;;
    project_id.required)    printf '%s' "PROJECT_ID required" ;;
    cancelled)              printf '%s' "Cancelled" ;;
    no_modification)        printf '%s' "No changes." ;;
    invalid_choice)         printf '%s' "Invalid choice" ;;
    deploy_later)           printf '%s' "Deploy later: ./oc.sh deploy all" ;;
    deploy_apply_prompt)    printf '%s' "Apply deployment? [Y/n]: " ;;
    deploy_cancelled)       printf '%s' "Deployment cancelled." ;;

    # ── cmd-help.sh ──────────────────────────────────────────────────────────
    help.title)             printf '%s' "opencode-hub — AI agent workspace manager" ;;
    help.usage)             printf '%s' "Usage:" ;;
    help.section.setup)     printf '%s' "Setup" ;;
    help.section.projects)  printf '%s' "Projects" ;;
    help.section.launch)    printf '%s' "Launch" ;;
    help.section.analysis)  printf '%s' "Analysis" ;;
    help.section.maintenance) printf '%s' "Maintenance" ;;
    help.section.config)    printf '%s' "Configuration" ;;
    help.section.deploy_targets) printf '%s' "Deploy targets" ;;
    help.section.agents)    printf '%s' "Agents" ;;
    help.section.skills)    printf '%s' "External skills (context7)" ;;
    help.section.beads)     printf '%s' "Beads (bd)" ;;
    help.section.examples)  printf '%s' "Skills examples" ;;
    # Setup
    help.install.cmd)       printf '%s' "install" ;;
    help.install.desc)      printf '%s' "Install AI tools and Beads (bd)" ;;
    help.uninstall.cmd)     printf '%s' "uninstall" ;;
    help.uninstall.desc)    printf '%s' "Uninstall opencode-hub and clean up artifacts" ;;
    help.init.cmd)          printf '%s' "init [PROJECT_ID] [path]" ;;
    help.init.desc)         printf '%s' "Initialize or adopt a project" ;;
    help.version.cmd)       printf '%s' "version" ;;
    help.version.desc)      printf '%s' "Show hub version" ;;
    # Projects
    help.status.cmd)        printf '%s' "status" ;;
    help.status.desc)       printf '%s' "Detailed status of all projects (Beads, API, deployed agents)" ;;
    help.status_short.cmd)  printf '%s' "status --short" ;;
    help.status_short.desc) printf '%s' "Compact project list (id, path, status)" ;;
    help.remove.cmd)        printf '%s' "remove <PROJECT_ID>" ;;
    help.remove.desc)       printf '%s' "Remove a project from the registry" ;;
    help.remove_clean.cmd)  printf '%s' "remove <PROJECT_ID> --clean" ;;
    help.remove_clean.desc) printf '%s' "Remove + clean deployed files in the project" ;;
    # Launch
    help.start.cmd)         printf '%s' "start [PROJECT_ID] [prompt]" ;;
    help.start.desc)        printf '%s' "Launch the AI tool" ;;
    help.start_dev.cmd)     printf '%s' "start [PROJECT_ID] --dev" ;;
    help.start_dev.desc)    printf '%s' "Bootstrap ai-delegated tickets (default label)" ;;
    help.start_dev_label.cmd)    printf '%s' "start [PROJECT_ID] --dev --label <l>" ;;
    help.start_dev_label.desc)   printf '%s' "Bootstrap tickets filtered by label" ;;
    help.start_dev_assignee.cmd) printf '%s' "start [PROJECT_ID] --dev --assignee <u>" ;;
    help.start_dev_assignee.desc) printf '%s' "Bootstrap tickets filtered by assignee" ;;
    help.start_onboard.cmd) printf '%s' "start [PROJECT_ID] --onboard" ;;
    help.start_onboard.desc) printf '%s' "Trigger the onboarder agent (project discovery)" ;;
    # Analysis
    help.audit.cmd)         printf '%s' "audit [PROJECT_ID]" ;;
    help.audit.desc)        printf '%s' "Run a global project audit" ;;
    help.audit_type.cmd)    printf '%s' "audit [PROJECT_ID] --type <type>" ;;
    help.audit_type.desc)   printf '%s' "Targeted audit: security | accessibility | architecture | ecodesign | observability | performance | privacy" ;;
    help.conventions.cmd)   printf '%s' "conventions [PROJECT_ID]" ;;
    help.conventions.desc)  printf '%s' "Detect and document project conventions" ;;
    help.conventions_force.cmd)  printf '%s' "conventions [PROJECT_ID] --force" ;;
    help.conventions_force.desc) printf '%s' "Regenerate CONVENTIONS.md without confirmation" ;;
    help.review.cmd)        printf '%s' "review [PROJECT_ID]" ;;
    help.review.desc)       printf '%s' "Run a code review via the reviewer agent" ;;
    help.review_branch.cmd) printf '%s' "review [PROJECT_ID] --branch <branch>" ;;
    help.review_branch.desc) printf '%s' "Review a specific branch (default: current branch)" ;;
    # Maintenance
    help.deploy.cmd)        printf '%s' "deploy [target] [PROJECT_ID]" ;;
    help.deploy.desc)       printf '%s' "Deploy agents (opencode/claude-code/all)" ;;
    help.deploy_check.cmd)  printf '%s' "deploy --check [target] [PROJECT_ID]" ;;
    help.deploy_check.desc) printf '%s' "Check if deployed agents are up to date" ;;
    help.deploy_diff.cmd)   printf '%s' "deploy --diff [target] [PROJECT_ID]" ;;
    help.deploy_diff.desc)  printf '%s' "Diff sources → deployed (proposes deploy if gap)" ;;
    help.sync.cmd)          printf '%s' "sync" ;;
    help.sync.desc)         printf '%s' "Redeploy agents on all registered projects" ;;
    help.sync_dryrun.cmd)   printf '%s' "sync --dry-run" ;;
    help.sync_dryrun.desc)  printf '%s' "Check freshness on all projects (without deploying)" ;;
    help.update.cmd)        printf '%s' "update" ;;
    help.update.desc)       printf '%s' "Update installed tools (opencode, bd, external skills)" ;;
    help.upgrade.cmd)       printf '%s' "upgrade [vX.Y.Z]" ;;
    help.upgrade.desc)      printf '%s' "Update hub sources via git (git pull or checkout tag)" ;;
    # Config
    help.config_set.cmd)    printf '%s' "config set [PROJECT_ID] [--model m] [--provider p] [--api-key k] [--family-model f=m] [--agent-model a=m]" ;;
    help.config_set.desc)   printf '%s' "Configure model, API key and per-agent/family models for a project (or hub if no PROJECT_ID)" ;;
    help.config_get.cmd)    printf '%s' "config get <PROJECT_ID>" ;;
    help.config_get.desc)   printf '%s' "Show project configuration" ;;
    help.config_list.cmd)   printf '%s' "config list" ;;
    help.config_list.desc)  printf '%s' "List all saved configurations" ;;
    help.config_unset.cmd)  printf '%s' "config unset <PROJECT_ID>" ;;
    help.config_unset.desc) printf '%s' "Delete project configuration" ;;
    help.config_language.cmd)  printf '%s' "config set language <en|fr>" ;;
    help.config_language.desc) printf '%s' "Set CLI display language (global)" ;;
    help.provider_list.cmd) printf '%s' "provider list" ;;
    help.provider_list.desc) printf '%s' "List available providers" ;;
    help.provider_set_default.cmd)  printf '%s' "provider set-default" ;;
    help.provider_set_default.desc) printf '%s' "Configure the hub default provider (interactive)" ;;
    help.target_info.cmd)   printf '%s' "target info <PROJECT_ID>" ;;
    help.target_info.desc)  printf '%s' "Show deploy targets for a project" ;;
    help.target_select.cmd) printf '%s' "target select <PROJECT_ID>" ;;
    help.target_select.desc) printf '%s' "Choose deploy targets (interactive)" ;;
    # Deploy targets
    help.deploy_target.opencode) printf '%s' "opencode     → .opencode/agents/ + opencode.json" ;;
    help.deploy_target.claude)   printf '%s' "claude-code  → .claude/agents/" ;;
    help.deploy_target.all)      printf '%s' "all          → all active targets" ;;
    # Agents
    help.agent_list.cmd)    printf '%s' "agent list" ;;
    help.agent_list.desc)   printf '%s' "List canonical agents" ;;
    help.agent_create.cmd)  printf '%s' "agent create" ;;
    help.agent_create.desc) printf '%s' "Create an agent (interactive)" ;;
    help.agent_edit.cmd)    printf '%s' "agent edit <agent-id>" ;;
    help.agent_edit.desc)   printf '%s' "Edit skills and metadata" ;;
    help.agent_info.cmd)    printf '%s' "agent info <agent-id>" ;;
    help.agent_info.desc)   printf '%s' "Show agent details" ;;
    help.agent_select.cmd)  printf '%s' "agent select <PROJECT_ID>" ;;
    help.agent_select.desc) printf '%s' "Choose agents to deploy for a project (interactive)" ;;
    help.agent_mode.cmd)    printf '%s' "agent mode <PROJECT_ID>" ;;
    help.agent_mode.desc)   printf '%s' "Show / override primary/subagent modes per project" ;;
    help.agent_validate.cmd)  printf '%s' "agent validate [agent-id]" ;;
    help.agent_validate.desc) printf '%s' "Validate agent consistency (or a single one)" ;;
    help.agent_deploy.cmd)  printf '%s' "agent deploy <agent-id> [PROJECT_ID]" ;;
    help.agent_deploy.desc) printf '%s' "Deploy a single agent to active targets (or project targets)" ;;
    # Skills
    help.skills_search.cmd) printf '%s' "skills search <query>" ;;
    help.skills_search.desc) printf '%s' "Search for skills on context7" ;;
    help.skills_info.cmd)   printf '%s' "skills info /owner/repo" ;;
    help.skills_info.desc)  printf '%s' "Preview skills in a repository" ;;
    help.skills_add.cmd)    printf '%s' "skills add /owner/repo [name]" ;;
    help.skills_add.desc)   printf '%s' "Download and add an external skill" ;;
    help.skills_list.cmd)   printf '%s' "skills list" ;;
    help.skills_list.desc)  printf '%s' "List all skills (local + external)" ;;
    help.skills_update.cmd) printf '%s' "skills update [name]" ;;
    help.skills_update.desc) printf '%s' "Update an external skill (or all)" ;;
    help.skills_used_by.cmd)  printf '%s' "skills used-by <skill>" ;;
    help.skills_used_by.desc) printf '%s' "List agents that use this skill" ;;
    help.skills_sync.cmd)   printf '%s' "skills sync" ;;
    help.skills_sync.desc)  printf '%s' "Re-download all skills (after clone)" ;;
    help.skills_remove.cmd) printf '%s' "skills remove <name>" ;;
    help.skills_remove.desc) printf '%s' "Remove an external skill" ;;
    help.skills_validate.cmd)  printf '%s' "skills validate [name]" ;;
    help.skills_validate.desc) printf '%s' "Validate skill consistency (frontmatter, sources)" ;;
    # Projects
    help.project_rename.cmd)  printf '%s' "project rename <OLD_ID> <NEW_ID>" ;;
    help.project_rename.desc) printf '%s' "Rename a project across all registry files" ;;
    help.project_move.cmd)    printf '%s' "project move <PROJECT_ID> <path>" ;;
    help.project_move.desc)   printf '%s' "Change the local path of a project" ;;
    # Beads
    help.beads_status.cmd)  printf '%s' "beads status [PROJECT_ID]" ;;
    help.beads_status.desc) printf '%s' "Check if Beads is initialized (all if no ID)" ;;
    help.beads_init.cmd)    printf '%s' "beads init <PROJECT_ID>" ;;
    help.beads_init.desc)   printf '%s' "Initialize .beads/ in the project" ;;
    help.beads_list.cmd)    printf '%s' "beads list <PROJECT_ID>" ;;
    help.beads_list.desc)   printf '%s' "List open tickets in the project" ;;
    help.beads_show.cmd)    printf '%s' "beads show <PROJECT_ID> <TICKET_ID>" ;;
    help.beads_show.desc)   printf '%s' "Show ticket details" ;;
    help.beads_create.cmd)  printf '%s' "beads create <PROJECT_ID> [title] [--label l] [--type t] [--desc d]" ;;
    help.beads_create.desc) printf '%s' "Create a ticket (non-interactive if title provided)" ;;
    help.beads_open.cmd)    printf '%s' "beads open <PROJECT_ID>" ;;
    help.beads_open.desc)   printf '%s' "Show path to use bd manually" ;;
    help.beads_sync.cmd)    printf '%s' "beads sync <PROJECT_ID> [--pull-only|--push-only|--dry-run]" ;;
    help.beads_sync.desc)   printf '%s' "Sync with external tracker (Jira/GitLab)" ;;
    help.beads_tracker_status.cmd)  printf '%s' "beads tracker status <PROJECT_ID>" ;;
    help.beads_tracker_status.desc) printf '%s' "Show tracker connection status" ;;
    help.beads_tracker_setup.cmd)   printf '%s' "beads tracker setup <PROJECT_ID>" ;;
    help.beads_tracker_setup.desc)  printf '%s' "Configure the tracker (interactive)" ;;
    help.beads_tracker_switch.cmd)  printf '%s' "beads tracker switch <PROJECT_ID>" ;;
    help.beads_tracker_switch.desc) printf '%s' "Switch tracker provider" ;;
    help.beads_tracker_set_sync_mode.cmd)  printf '%s' "beads tracker set-sync-mode <PROJECT_ID> [mode]" ;;
    help.beads_tracker_set_sync_mode.desc) printf '%s' "Set sync mode (bidirectional|pull-only|push-only)" ;;
    help.beads_ui_start.cmd)  printf '%s' "beads ui start [PROJECT_ID] [--port N]" ;;
    help.beads_ui_start.desc) printf '%s' "Start beads-ui (local graphical interface)" ;;
    help.beads_ui_stop.cmd)   printf '%s' "beads ui stop" ;;
    help.beads_ui_stop.desc)  printf '%s' "Stop the beads-ui server" ;;
    help.beads_ui_status.cmd)  printf '%s' "beads ui status" ;;
    help.beads_ui_status.desc) printf '%s' "Show beads-ui server status" ;;
    help.beads_ui_install.cmd)  printf '%s' "beads ui install" ;;
    help.beads_ui_install.desc) printf '%s' "Install beads-ui (npm install -g beads-ui)" ;;

    # ── cmd-agent.sh ─────────────────────────────────────────────────────────
    agent.title)            printf '%s' "oc agent — Canonical agent management" ;;
    agent.list)             printf '%s' "list                  List available agents" ;;
    agent.info_cmd)         printf '%s' "info <agent-id>       Show agent details" ;;
    agent.create_cmd)       printf '%s' "create                Create a new agent (interactive)" ;;
    agent.edit_cmd)         printf '%s' "edit <agent-id>       Edit agent skills and metadata" ;;
    agent.select_cmd)       printf '%s' "select <PROJECT_ID>   Choose agents to deploy for a project" ;;
    agent.mode_cmd)         printf '%s' "mode <PROJECT_ID>     Show / override primary/subagent modes" ;;
    agent.validate_cmd)     printf '%s' "validate [agent-id]   Validate consistency of all agents (or one)" ;;
    agent.deploy_cmd)       printf '%s' "deploy <agent-id> [PROJECT_ID]  Deploy a single agent to active targets" ;;
    agent.keytest_cmd)      printf '%s' "keytest               Keyboard diagnostic — show bytes received" ;;
    agent.usage.info)       printf '%s' "Usage: oc agent info <agent-id>" ;;
    agent.usage.edit)       printf '%s' "Usage: oc agent edit <agent-id>" ;;
    agent.usage.select)     printf '%s' "Usage: oc agent select <PROJECT_ID>" ;;
    agent.usage.mode)       printf '%s' "Usage: oc agent mode <PROJECT_ID>" ;;
    agent.not_found)        printf '%s' "Agent not found in agents/" ;;
    agent.id_required)      printf '%s' "ID required." ;;
    agent.already_exists)   printf '%s' "already exists. Use 'oc agent edit'" ;;
    agent.no_skill)         printf '%s' "(no description)" ;;
    agent.no_agents)        printf '%s' "(no agents in agents/)" ;;
    agent.skills_assigned)  printf '%s' "Assigned skills:" ;;
    agent.skill_missing)    printf '%s' "(file missing)" ;;
    agent.create.title)     printf '%s' "Create a new agent" ;;
    agent.edit.title)       printf '%s' "Edit agent" ;;
    agent.list.title)       printf '%s' "Canonical agents" ;;
    agent.select.title)     printf '%s' "Agent selection —" ;;
    agent.mode.title)       printf '%s' "Agent modes —" ;;
    agent.validate.summary) printf '%s' "Summary:" ;;
    agent.validate.ok)      printf '%s' "ok" ;;
    agent.validate.errors)  printf '%s' "error(s)" ;;
    agent.validate.warnings) printf '%s' "warning(s)" ;;
    agent.examples)         printf '%s' "Examples:" ;;

    # ── cmd-config.sh ────────────────────────────────────────────────────────
    config.title)           printf '%s' "Usage: ./oc.sh config <subcommand> [options]" ;;
    config.no_entry)        printf '%s' "No configuration for" ;;
    config.no_file)         printf '%s' "No configuration saved (api-keys.local.md missing)" ;;
    config.no_entries)      printf '%s' "No entries in api-keys.local.md" ;;
    config.saved)           printf '%s' "API configurations saved:" ;;
    config.delete_confirm)  printf '%s' "Delete configuration for" ;;
    config.deleted)         printf '%s' "Configuration deleted for" ;;
    config.written)         printf '%s' "Configuration saved for" ;;
    config.apply_now)       printf '%s' "Apply now to project (redeploy opencode.json)? [Y/n]: " ;;
    config.apply_later)     printf '%s' "Apply later: ./oc.sh deploy all" ;;
    config.no_path)         printf '%s' "Path not registered for" ;;
    config.apply_via)       printf '%s' "— apply via: ./oc.sh deploy all" ;;
    config.api_key.required) printf '%s' "API key required" ;;
    config.api_key.unchanged) printf '%s' "API key unchanged" ;;
    config.language.invalid) printf '%s' "Invalid language value (accepted: en, fr)" ;;
    config.language.saved)  printf '%s' "CLI language set to" ;;
    config.language.current) printf '%s' "Current CLI language:" ;;

    # ── cmd-beads.sh ─────────────────────────────────────────────────────────
    beads.title)            printf '%s' "Beads management:" ;;
    beads.tracker.title)    printf '%s' "Tracker sync (Jira / GitLab):" ;;
    beads.not_installed)    printf '%s' "bd (Beads) is not installed" ;;
    beads.install_hint)     printf '%s' "Install: brew install bd" ;;
    beads.not_initialized)  printf '%s' "Beads not initialized in" ;;
    beads.already_initialized) printf '%s' "Beads already initialized in" ;;
    beads.initialized)      printf '%s' "Beads initialized in" ;;
    beads.init_failed)      printf '%s' "bd init failed" ;;
    beads.no_tracker)       printf '%s' "No tracker configured for" ;;
    beads.tracker.configure) printf '%s' "Configure: ./oc.sh beads tracker setup" ;;
    beads.tracker.invalid)  printf '%s' "Invalid tracker choice" ;;
    beads.tracker.unknown_subcmd) printf '%s' "Unknown tracker subcommand:" ;;
    beads.tracker.usage)    printf '%s' "Usage: ./oc.sh beads tracker <status|setup|switch> <PROJECT_ID>" ;;
    beads.status.all)       printf '%s' "Beads status — all projects" ;;
    beads.open_hint)        printf '%s' "You can now use bd directly in this directory" ;;
    beads.labels.registered)       printf '%s' "Labels registered:" ;;
    beads.labels.failed)           printf '%s' "Failed to register labels in Beads" ;;
    beads.labels.fetching_gitlab)  printf '%s' "Importing labels from GitLab…" ;;
    beads.labels.fetching_jira)    printf '%s' "Importing labels from Jira…" ;;
    beads.labels.imported_gitlab)  printf '%s' "GitLab labels imported:" ;;
    beads.labels.imported_jira)    printf '%s' "Jira labels imported:" ;;
    beads.sync.failed)      printf '%s' "Sync failed for" ;;
    beads.sync.done)        printf '%s' "Sync done for" ;;
    beads.sync_mode.applied)    printf '%s' "Sync mode applied:" ;;
    beads.sync_mode.current)    printf '%s' "Current sync mode:" ;;
    beads.sync_mode.choose)     printf '%s' "Choose sync mode for" ;;
    beads.sync_mode.desc_bidi)  printf '%s' "Import and export (default)" ;;
    beads.sync_mode.desc_pull)  printf '%s' "Import only from the tracker" ;;
    beads.sync_mode.desc_push)  printf '%s' "Export only to the tracker" ;;
    beads.sync_mode.invalid)    printf '%s' "Invalid sync mode (bidirectional | pull-only | push-only)" ;;
    beads.sync_mode.save_failed) printf '%s' "Failed to save Sync mode for" ;;
    beads.sync_mode.saved)      printf '%s' "Sync mode saved:" ;;
    beads.status.open_tickets) printf '%s' "Open tickets —" ;;
    beads.create.title)     printf '%s' "Create ticket —" ;;
    beads.create.title_required) printf '%s' "Title required" ;;
    beads.create.creating)  printf '%s' "Creating ticket in" ;;
    beads.create.failed)    printf '%s' "bd create failed" ;;
    beads.gitignore_added)  printf '%s' ".beads/ added to project .git/info/exclude (local exclusion, invisible to other developers)" ;;
    beads.gitignore_exists) printf '%s' ".beads/ already present in project .git/info/exclude" ;;

    # ── beads board (terminal kanban) ────────────────────────────────────────
    beads.board.title)      printf '%s' "Kanban board (terminal):" ;;
    board.no_project)       printf '%s' "No Beads project found in the current directory" ;;
    board.usage_hint)       printf '%s' "Usage: ./oc.sh beads board <PROJECT_ID>" ;;
    board.watch_mode)       printf '%s' "Watch mode enabled — refreshing every 5s" ;;
    board.watch_quit)       printf '%s' "to quit" ;;
    board.empty_column)     printf '%s' "— no tickets —" ;;
    help.beads_board.cmd)       printf '%s' "beads board [PROJECT_ID] [--watch]" ;;
    help.beads_board.desc)      printf '%s' "Display the kanban board in the terminal" ;;
    help.beads_board_watch.cmd) printf '%s' "beads board [PROJECT_ID] --watch [--interval N]" ;;
    help.beads_board_watch.desc) printf '%s' "Auto-refresh board (default: 5s)" ;;

    # ── cmd-target.sh ────────────────────────────────────────────────────────
    target.title)           printf '%s' "oc target — Deploy target management by project" ;;
    target.usage.info)      printf '%s' "Usage: oc target info <PROJECT_ID>" ;;
    target.usage.select)    printf '%s' "Usage: oc target select <PROJECT_ID>" ;;
    target.info_cmd)        printf '%s' "info <PROJECT_ID>     Show configured targets for a project" ;;
    target.select_cmd)      printf '%s' "select <PROJECT_ID>   Choose deploy targets for a project" ;;
    target.select.title)    printf '%s' "Target selection —" ;;
    target.current_all)     printf '%s' "Current selection: all active targets (hub.json)" ;;
    target.reset_done)      printf '%s' "Targets reset for" ;;
    target.reset_suffix)    printf '%s' "→ all active targets will be used" ;;
    target.selected)        printf '%s' "target(s) selected for" ;;
    target.targets_label)   printf '%s' "Targets:" ;;
    target.all_active)      printf '%s' "(all active targets from hub.json)" ;;
    target.available)       printf '%s' "Available targets:" ;;
    target.default_hint)    printf '%s' "By default (if not configured), active targets from hub.json are used." ;;
    target.examples)        printf '%s' "Examples:" ;;

    # ── cmd-provider.sh ──────────────────────────────────────────────────────
    provider.title)         printf '%s' "Available LLM providers" ;;
    provider.default_title) printf '%s' "Default LLM provider" ;;
    provider.project_title) printf '%s' "LLM provider —" ;;
    provider.choose_default) printf '%s' "Choose the default provider for all projects:" ;;
    provider.choose_project) printf '%s' "Choose the provider for this project:" ;;
    provider.current)       printf '%s' "Current provider:" ;;
    provider.current_project) printf '%s' "Current project provider:" ;;
    provider.hub_default)   printf '%s' "Hub default provider:" ;;
    provider.selected)      printf '%s' "Provider selected:" ;;
    provider.saved)         printf '%s' "Default provider saved:" ;;
    provider.set_done)      printf '%s' "Provider configured for" ;;
    provider.no_provider)   printf '%s' "No provider configured" ;;
    provider.no_catalog)    printf '%s' "providers.json catalog not found" ;;
    provider.hub_json_missing) printf '%s' "hub.json not found — run first: ./oc.sh install" ;;
    provider.hub_json_added_gitignore) printf '%s' "hub.json added to .gitignore (contains API key)" ;;
    provider.api_key_required) printf '%s' "API key required for this provider" ;;
    provider.api_key_empty_warn) printf '%s' "Empty API key — provider will be saved without key" ;;
    provider.source_project) printf '%s' "Source: project configuration" ;;
    provider.source_hub)    printf '%s' "Source: hub default provider" ;;
    provider.apply_hint)    printf '%s' "Apply to projects: ./oc.sh deploy all <PROJECT_ID>" ;;
    provider.usage)         printf '%s' "Usage: ./oc.sh provider <subcommand>" ;;
    provider.list_cmd)      printf '%s' "list                           List available providers" ;;
    provider.set_default_cmd) printf '%s' "set-default                    Configure the default provider (hub)" ;;

    # ── cmd-skills.sh ────────────────────────────────────────────────────────
    skills.title)           printf '%s' "oc skills — External skill management" ;;
    skills.available)       printf '%s' "Available skills" ;;
    skills.local)           printf '%s' "Local skills:" ;;
    skills.external)        printf '%s' "External skills (context7):" ;;
    skills.none_local)      printf '%s' "(none)" ;;
    skills.none_external)   printf '%s' "(none — use: oc skills add /owner/repo [name])" ;;
    skills.search.title)    printf '%s' "Searching for skills:" ;;
    skills.search.usage)    printf '%s' "Usage: oc skills search <query>" ;;
    skills.info.usage)      printf '%s' "Usage: oc skills info /owner/repo" ;;
    skills.add.title)       printf '%s' "Adding an external skill" ;;
    skills.add.usage)       printf '%s' "Usage: oc skills add /owner/repo [skill-name]" ;;
    skills.add.overwrite)   printf '%s' "Overwrite? (y/N): " ;;
    skills.add.already_exists) printf '%s' "External skill already exists." ;;
    skills.add.added)       printf '%s' "Skill added →" ;;
    skills.add.use_hint)    printf '%s' "To use in an agent, add 'external/<name>' to the skills: list of its agents/<family>/<id>.md, then run: ./oc.sh deploy all" ;;
    skills.remove.usage)    printf '%s' "Usage: oc skills remove <skill-name>" ;;
    skills.remove.not_found) printf '%s' "External skill not found." ;;
    skills.remove.confirm)  printf '%s' "Remove external skill" ;;
    skills.remove.done)     printf '%s' "Skill removed." ;;
    skills.remove.agent_hint) printf '%s' "Don't forget to remove it from the skills: list of your agents if needed." ;;
    skills.update.title)    printf '%s' "Updating external skills" ;;
    skills.update.no_skills) printf '%s' "No external skills registered. Nothing to update." ;;
    skills.update.already_up_to_date) printf '%s' "is already up to date." ;;
    skills.update.apply)    printf '%s' "Apply update for" ;;
    skills.update.skipped)  printf '%s' "Update skipped for" ;;
    skills.update.updated)  printf '%s' "skill(s) updated." ;;
    skills.update.unchanged) printf '%s' "skill(s) unchanged or skipped." ;;
    skills.used_by.title)   printf '%s' "Agents using skill:" ;;
    skills.used_by.usage)   printf '%s' "Usage: oc skills used-by <skill>" ;;
    skills.used_by.none)    printf '%s' "(no agent uses this skill)" ;;
    skills.sync.title)      printf '%s' "Syncing external skills" ;;
    skills.sync.none)       printf '%s' "No external skills registered. Nothing to sync." ;;
    skills.sync.done)       printf '%s' "skill(s) synced." ;;
    skills.examples)        printf '%s' "Examples:" ;;

    # ── cmd-remove.sh ────────────────────────────────────────────────────────
    remove.not_found)       printf '%s' "Project not found in registry" ;;
    remove.no_path)         printf '%s' "Local path not found for" ;;
    remove.clean_ignored)   printf '%s' "— --clean ignored" ;;
    remove.confirm_clean)   printf '%s' "Remove PROJECT from registry AND clean deployed files in PATH? [y/N] " ;;
    remove.confirm)         printf '%s' "Remove PROJECT? [y/N] " ;;
    remove.done)            printf '%s' "removed from registry" ;;
    remove.path_removed)    printf '%s' "Path removed from paths.local.md" ;;
    remove.api_key_removed) printf '%s' "API key removed from api-keys.local.md" ;;
    remove.projects_removed) printf '%s' "removed from projects.md" ;;

    # ── cmd-start.sh ─────────────────────────────────────────────────────────
    start.no_projects)      printf '%s' "No registered projects → ./oc.sh init" ;;
    start.choose_project)   printf '%s' "Choose a project:" ;;
    start.target_unavailable) printf '%s' "Target not available → oc install" ;;
    start.agents_not_deployed) printf '%s' "Agents not deployed for" ;;
    start.deploy_now)       printf '%s' "Deploy now? [Y/n]: " ;;
    start.beads_not_init)   printf '%s' "Beads not initialized in this project (no .beads/ found)" ;;
    start.init_beads_now)   printf '%s' "Initialize Beads now? [Y/n]: " ;;
    start.setup_upstream)   printf '%s' "Configure Git upstream (git remote add upstream)? [Y/n]: " ;;
    start.upstream_url)     printf '%s' "Remote upstream URL: " ;;
    start.upstream_empty)   printf '%s' "Empty URL — configure later: git remote add upstream <url>" ;;
    start.upstream_ok)      printf '%s' "Remote upstream configured:" ;;
    start.upstream_failed)  printf '%s' "Failed to configure upstream — configure manually" ;;
    start.labels_registered) printf '%s' "Labels registered:" ;;
    start.labels_failed)    printf '%s' "Failed to register labels in Beads" ;;
    start.beads_init_failed) printf '%s' "bd init failed — initialize later: ./oc.sh beads init" ;;
    start.beads_later)      printf '%s' "Initialize later: ./oc.sh beads init" ;;
    start.dev_requires_beads) printf '%s' "--dev requires Beads initialized in this project" ;;
    start.dev_beads_hint)   printf '%s' "Run first: ./oc.sh beads init" ;;
    start.dev_requires_bd)  printf '%s' "--dev requires bd (Beads): brew install bd" ;;
    start.press_enter)      printf '%s' "Press Enter to launch" ;;
    start.dev_label_exclusive) printf '%s' "--label and --assignee are mutually exclusive" ;;
    start.dev_needs_dev_flag) printf '%s' "--label and --assignee require --dev" ;;
    start.dev_onboard_exclusive) printf '%s' "--dev and --onboard are mutually exclusive" ;;

    # ── install/uninstall ────────────────────────────────────────────────────
    install.title)          printf '%s' "opencode-hub installation" ;;
    install.already_done)   printf '%s' "Already installed." ;;
    install.done)           printf '%s' "Installation complete." ;;
    install.deps_required)  printf '%s' "Required dependencies:" ;;
    install.os_detected)    printf '%s' "OS detected:" ;;
    install.jq_missing)     printf '%s' "jq not detected — critical dependency for opencode-hub" ;;
    install.jq_install_brew) printf '%s' "Install jq via Homebrew? [Y/n]: " ;;
    install.jq_installed)   printf '%s' "jq installed" ;;
    install.jq_failed)      printf '%s' "jq installation failed — install manually" ;;
    install.jq_degraded)    printf '%s' "Some features (deploy, skills, beads) will be degraded without jq" ;;
    install.jq_manual)      printf '%s' "Install jq manually:" ;;
    install.targets_title)  printf '%s' "Targets to configure" ;;
    install.target_opencode) printf '%s' "OpenCode (recommended)" ;;
    install.target_claude)  printf '%s' "Claude Code" ;;
    install.target_all)     printf '%s' "All" ;;
    install.choose_prompt)  printf '%s' "Choose (1-3, default: 1): " ;;
    install.hub_json_exists) printf '%s' "config/hub.json already exists." ;;
    install.hub_json_overwrite) printf '%s' "Overwrite with new targets? [y/N]: " ;;
    install.hub_json_kept)  printf '%s' "config/hub.json kept as-is." ;;
    install.hub_json_created) printf '%s' "config/hub.json created (targets:" ;;
    install.provider_title) printf '%s' "LLM Provider" ;;
    install.provider_choose) printf '%s' "Which AI provider to use for all your projects?" ;;
    install.provider_recommended) printf '%s' "(recommended)" ;;
    install.provider_skip)  printf '%s' "Skip (configure later via ./oc.sh provider set-default)" ;;
    install.provider_api_key) printf '%s' "(leave empty to skip):" ;;
    install.provider_base_url) printf '%s' "Base URL" ;;
    install.provider_configured) printf '%s' "Provider configured:" ;;
    install.provider_skipped) printf '%s' "Provider not configured — use: ./oc.sh provider set-default" ;;
    install.projects_ready) printf '%s' "projects.md ready" ;;
    install.paths_created)  printf '%s' "paths.local.md created" ;;
    install.skills_tip)     printf '%s' "Tip: Enrich your agents with third-party skills via context7:" ;;
    install.beads_title)    printf '%s' "Beads (bd) installation" ;;
    install.beads_already)  printf '%s' "Beads already installed" ;;
    install.beads_missing)  printf '%s' "Beads (bd) not detected — required for ticket management" ;;
    install.beads_install_prompt) printf '%s' "Install Beads? [Y/n]: " ;;
    install.beads_via_brew) printf '%s' "Installing Beads via Homebrew..." ;;
    install.beads_installed) printf '%s' "Beads installed" ;;
    install.beads_brew_failed) printf '%s' "Homebrew failed — trying via curl..." ;;
    install.beads_via_curl) printf '%s' "Installing Beads via curl..." ;;
    install.beads_curl_installed) printf '%s' "Beads installed via curl" ;;
    install.beads_failed)   printf '%s' "Beads installation failed — install manually: brew install beads" ;;
    install.beads_no_tools) printf '%s' "Homebrew and curl not found — install Beads manually:" ;;
    install.beads_later)    printf '%s' "Beads not installed — install later: brew install beads" ;;
    install.ready)          printf '%s' "opencode-hub ready!" ;;
    uninstall.title)        printf '%s' "opencode-hub uninstall" ;;
    uninstall.confirm)      printf '%s' "Uninstall opencode-hub? [y/N] " ;;
    uninstall.done)         printf '%s' "Uninstall complete." ;;
    uninstall.cancelled)    printf '%s' "Uninstall cancelled." ;;

    # ── cmd-init.sh ──────────────────────────────────────────────────────────
    init.gitignore_opencode_prompt) printf '%s' "Add opencode.json and .opencode/ to the project .git/info/exclude? [y/N]: " ;;
    init.gitignore_opencode_added)  printf '%s' "opencode.json and .opencode/ added to project .git/info/exclude (local exclusion, invisible to other devs)" ;;
    init.gitignore_opencode_exists) printf '%s' "opencode.json and .opencode/ already present in project .git/info/exclude" ;;

    # ── cmd-list.sh ──────────────────────────────────────────────────────────
    list.title)             printf '%s' "Registered projects" ;;
    list.no_projects)       printf '%s' "No registered projects" ;;
    list.col_id)            printf '%s' "ID" ;;
    list.col_path)          printf '%s' "Local path" ;;
    list.col_status)        printf '%s' "Status" ;;
    list.status_no_path)    printf '%s' "⚠ no path" ;;
    list.path_undefined)    printf '%s' "undefined" ;;
    list.status_ok)         printf '%s' "✔ accessible" ;;
    list.status_missing)    printf '%s' "✘ not found" ;;

    # ── cmd-status.sh ────────────────────────────────────────────────────────
    status.no_path)         printf '%s' "Local path not configured (paths.local.md)" ;;
    status.dir_missing)     printf '%s' "Directory not found: " ;;
    status.path_label)      printf '%s' "Path: " ;;
    status.beads_ok)        printf '%s' "Beads initialized" ;;
    status.beads_not_init)  printf '%s' "Beads not initialized" ;;
    status.api_not_set)     printf '%s' "API key not configured" ;;
    status.tracker_none)    printf '%s' "Tracker: none" ;;
    status.agents_deployed) printf '%s' "Deployed agents" ;;
    status.agents_missing)  printf '%s' "Agents not deployed for" ;;
    status.title)           printf '%s' "Project status" ;;
    status.no_projects)     printf '%s' "No registered projects — start with: ./oc.sh init" ;;

    # ── cmd-sync.sh ──────────────────────────────────────────────────────────
    sync.title_dryrun)      printf '%s' "Global check of deployed agents (dry-run)" ;;
    sync.title)             printf '%s' "Sync agents on all registered projects" ;;
    sync.no_projects)       printf '%s' "No registered projects — run: ./oc.sh init" ;;
    sync.no_targets)        printf '%s' "No target configured — check active_targets in config/hub.json" ;;
    sync.project_label)     printf '%s' "── Project: " ;;
    sync.path_undefined)    printf '%s' "path not defined locally, skipped" ;;
    sync.dir_missing)       printf '%s' "directory not found: " ;;
    sync.missing)           printf '%s' "✗ MISSING" ;;
    sync.stale)             printf '%s' "⚠ STALE" ;;
    sync.ok)                printf '%s' "✓ UP TO DATE" ;;
    sync.deployed)          printf '%s' "deployed" ;;
    sync.deploy_failed)     printf '%s' "deploy failed" ;;
    sync.target_unavailable) printf '%s' "not available — skipped" ;;
    sync.result_ok)         printf '%s' "up to date" ;;
    sync.result_stale)      printf '%s' "stale/missing" ;;
    sync.result_skipped)    printf '%s' "skipped" ;;
    sync.deploy_hint)       printf '%s' "To deploy: ./oc.sh sync" ;;
    sync.result_deployed)   printf '%s' "deployed" ;;

    # ── cmd-update.sh ────────────────────────────────────────────────────────
    update.title)           printf '%s' "Updating tools" ;;
    update.beads_updating)  printf '%s' "Updating Beads (bd)..." ;;
    update.beads_done)      printf '%s' "Beads updated via Homebrew" ;;
    update.beads_failed)    printf '%s' "Beads update failed — already up to date or Homebrew error" ;;
    update.beads_not_brew)  printf '%s' "bd installed but not via Homebrew — manual update required" ;;
    update.beads_manual_hint) printf '%s' "  → https://beads.sh or via your package manager" ;;
    update.bd_missing)      printf '%s' "bd not installed — run: oc install" ;;
    update.skills_updating) printf '%s' "Updating external skills..." ;;
    update.skills_none)     printf '%s' "No external skills registered — step skipped." ;;
    update.done)            printf '%s' "Update complete" ;;
    update.skills_stale_warn) printf '%s' "Some skills were updated — deployed agents in your projects may be outdated." ;;
    update.sync_now)        printf '%s' "  Run oc sync to redeploy on all projects? [Y/n]: " ;;
    update.sync_later)      printf '%s' "Redeploy manually: ./oc.sh sync" ;;

    # ── cmd-upgrade.sh ───────────────────────────────────────────────────────
    upgrade.title)              printf '%s' "Upgrading the hub" ;;
    upgrade.invalid_ref)        printf '%s' "Invalid reference — expected format vX.Y.Z or X.Y.Z" ;;
    upgrade.ref_hint)           printf '%s' "Example: oc upgrade v1.1.0" ;;
    upgrade.fetching_tag)       printf '%s' "Fetching tag" ;;
    upgrade.tag_not_found)      printf '%s' "Tag not found — check the release exists" ;;
    upgrade.fetch_failed)       printf '%s' "Fetch failed — check network connection" ;;
    upgrade.pulling)            printf '%s' "Fetching latest sources (git pull)" ;;
    upgrade.already_uptodate)   printf '%s' "Hub is already up to date." ;;
    upgrade.pull_failed)        printf '%s' "git pull failed — check network connection" ;;
    upgrade.updated)            printf '%s' "Hub updated" ;;
    upgrade.done)               printf '%s' "Hub up to date" ;;
    upgrade.sync_stale_warn)    printf '%s' "Sources were updated — deployed agents in your projects may be outdated." ;;
    upgrade.sync_now)           printf '%s' "  Run oc sync to redeploy on all projects? [Y/n]: " ;;
    upgrade.sync_later)         printf '%s' "Redeploy manually: ./oc.sh sync" ;;

    # ── cmd-review.sh ────────────────────────────────────────────────────────
    review.no_projects)         printf '%s' "No registered projects → ./oc.sh init" ;;
    review.choose_project)      printf '%s' "Choose a project:" ;;
    review.choose_number)       printf '%s' "  Number: " ;;
    review.invalid_choice)      printf '%s' "Invalid choice: " ;;
    review.target_unavailable)  printf '%s' "Target '" ;;
    review.target_unavailable_suffix) printf '%s' "unavailable → oc install" ;;
    review.no_branch)           printf '%s' "Cannot detect current branch. Use --branch <branch>" ;;
    review.label_path)          printf '%s' "Path" ;;
    review.label_target)        printf '%s' "Target" ;;
    review.label_branch)        printf '%s' "Branch" ;;
    review.label_agent)         printf '%s' "Agent" ;;
    review.agent_missing_config) printf '%s' "Agent missing from project selection: " ;;
    review.add_agent_prompt)    printf '%s' "Add reviewer to project selection? [Y/n]: " ;;
    review.agent_updated)       printf '%s' "Agents updated in projects.md: " ;;
    review.redeploy_prompt)     printf '%s' "Redeploy agents now? [Y/n]: " ;;
    review.redeploy_later)      printf '%s' "Redeploy later: ./oc.sh deploy " ;;
    review.agents_not_deployed) printf '%s' "Agents not deployed for " ;;
    review.agent_not_deployed)  printf '%s' "Agent not deployed: " ;;
    review.deploy_now_prompt)   printf '%s' "Deploy now? [Y/n]: " ;;
    review.deploy_skipped)      printf '%s' "Deployment skipped — agent may not be found" ;;
    review.deploy_later)        printf '%s' "Deploy later: ./oc.sh deploy " ;;
    review.main_agent)          printf '%s' "Agent: " ;;
    review.launching)           printf '%s' "Launching review on " ;;

    # ── cmd-audit.sh ─────────────────────────────────────────────────────────
    audit.invalid_type)     printf '%s' "Invalid audit type: '" ;;
    audit.valid_types)      printf '%s' "Valid types: " ;;
    audit.no_projects)      printf '%s' "No registered projects → ./oc.sh init" ;;
    audit.choose_project)   printf '%s' "Choose a project:" ;;
    audit.choose_number)    printf '%s' "  Number: " ;;
    audit.invalid_choice)   printf '%s' "Invalid choice: " ;;
    audit.target_unavailable) printf '%s' "Target '" ;;
    audit.label_path)       printf '%s' "Path" ;;
    audit.label_target)     printf '%s' "Target" ;;
    audit.label_agents)     printf '%s' "Agents" ;;
    audit.label_type)       printf '%s' "Type" ;;
    audit.agents_missing_config) printf '%s' "Agent(s) missing from project selection: " ;;
    audit.add_agents_prompt) printf '%s' "Add these agents to project selection? [Y/n]: " ;;
    audit.agents_updated)   printf '%s' "Agents updated in projects.md: " ;;
    audit.redeploy_prompt)  printf '%s' "Redeploy agents now? [Y/n]: " ;;
    audit.redeploy_later)   printf '%s' "Redeploy later: ./oc.sh deploy " ;;
    audit.searching_agents) printf '%s' "Searching for available audit agents in " ;;
    audit.no_agents_deployed) printf '%s' "No audit agent deployed in " ;;
    audit.deploy_hint)      printf '%s' "Deploy agents: ./oc.sh deploy " ;;
    audit.add_agents_hint)  printf '%s' "Then add agents to project: ./oc.sh agent edit " ;;
    audit.available_agents) printf '%s' "Available audit agents:" ;;
    audit.choose_agent)     printf '%s' "  Choose an agent (number): " ;;
    audit.agent_selected)   printf '%s' "Selected agent: " ;;
    audit.agents_not_deployed) printf '%s' "Agents not deployed for " ;;
    audit.deploy_now_prompt) printf '%s' "Deploy now? [Y/n]: " ;;
    audit.deploy_skipped)   printf '%s' "Deploy skipped — the agent may not be found" ;;
    audit.deploy_later)     printf '%s' "Deploy later: ./oc.sh deploy " ;;
    audit.agents_not_deployed_list) printf '%s' "Agent(s) not deployed: " ;;
    audit.main_agent)       printf '%s' "Main agent: " ;;
    audit.launching)        printf '%s' "Launching audit " ;;

    # ── cmd-version.sh ───────────────────────────────────────────────────────
    version.unknown)        printf '%s' "unknown" ;;

    # Fallback: return the key itself
    *) printf '%s' "$key" ;;
  esac
}
