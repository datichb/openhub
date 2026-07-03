---
marp: true
theme: gaia
class: invert
paginate: true
size: 16:9
style: |
  section {
    font-size: 1.05rem;
    padding: 48px 64px;
  }
  section.lead {
    text-align: center;
  }
  section.lead h1 {
    font-size: 3rem;
    font-weight: 900;
    letter-spacing: -1px;
  }
  section.lead p {
    font-size: 1.2rem;
    opacity: 0.85;
  }
  h2 {
    font-size: 1.7rem;
    font-weight: 800;
    border-bottom: 3px solid #7c3aed;
    padding-bottom: 8px;
    margin-bottom: 24px;
  }
  h3 {
    font-size: 1.1rem;
    font-weight: 700;
    color: #a78bfa;
    margin-top: 20px;
    margin-bottom: 6px;
  }
  table {
    font-size: 0.88rem;
    width: 100%;
  }
  th {
    background: #3b0764;
    color: #e9d5ff;
  }
  code {
    background: #2d2b55;
    color: #e2d9f3;
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 0.92em;
  }
  pre {
    background: #2d2b55 !important;
    border-left: 4px solid #7c3aed;
    padding: 20px 24px !important;
    font-size: 0.9rem !important;
    border-radius: 8px;
    line-height: 1.5;
  }
  pre code {
    background: transparent;
    color: #e2d9f3;
    font-size: 0.9rem;
  }
  blockquote {
    border-left: 4px solid #7c3aed;
    background: rgba(124, 58, 237, 0.12);
    padding: 12px 20px;
    border-radius: 0 8px 8px 0;
    font-style: normal;
    color: #e9d5ff;
  }
  .highlight {
    color: #a78bfa;
    font-weight: 700;
  }
  ul li::marker {
    color: #7c3aed;
  }
  .demo-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    margin-top: 12px;
  }
  .video-placeholder {
    width: 100%;
    max-width: 780px;
    height: 340px;
    background: #0f0e17;
    border: 2px dashed #7c3aed;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: #a78bfa;
    font-size: 1rem;
    gap: 10px;
  }
  .video-placeholder .play-icon {
    font-size: 3rem;
    opacity: 0.6;
  }
  .video-placeholder .video-label {
    font-size: 0.85rem;
    color: #6d28d9;
    font-family: monospace;
  }
  .demo-meta {
    display: flex;
    gap: 24px;
    font-size: 0.82rem;
    color: #7c3aed;
  }
  section.part-title {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
  }
  section.part-title h1 {
    font-size: 2.4rem;
    font-weight: 900;
    color: #a78bfa;
  }
  section.part-title p {
    font-size: 1.1rem;
    opacity: 0.7;
  }
---

<!-- _class: lead invert -->
<!-- _paginate: false -->

# openhub

### Hub central pour piloter vos assistants IA sur tous vos projets

**27 agents spécialisés · Skills injectables · Workflow Beads intégré**
**OpenCode**

---

## Sommaire

### Partie 1 — Créer un projet avec l'IA
Comment j'ai utilisé OpenCode pour concevoir et construire openhub de A à Z

### Partie 2 — Utiliser openhub
Ce que le projet fait, comment l'installer et comment l'utiliser au quotidien

---

<!-- _class: part-title -->
<!-- _paginate: false -->

# Partie 1
Créer un projet avec l'IA et OpenCode

---

## Le point de départ

> *"J'ai une idée de hub d'agents IA, mais aucune ligne de code."*

### Les objectifs
- **Centraliser** la gestion des agents IA en un seul endroit
- **Multi-projets** — un hub, N projets, zéro duplication
- **Facilement déployable** — une commande et c'est fait
- **Travailler AVEC l'IA** — un workflow de collaboration, pas de délégation aveugle

### L'outil choisi
**OpenCode** (terminal) — un assistant IA qui lit, écrit et raisonne dans le contexte du projet entier

---

## Méthodologie — travailler AVEC l'IA

> **Philosophie : l'IA est un partenaire de travail, pas un exécutant autonome.**

### Le rôle de l'humain (moi)
- Définir la **vision** et les **objectifs**
- Valider ou rejeter chaque décision structurante
- Reformuler quand l'IA part dans la mauvaise direction
- Prioriser : "pas ça maintenant, d'abord ceci"

### Le rôle de l'IA (OpenCode)
- Proposer des architectures et des alternatives
- Rédiger le code, la documentation, les scripts
- Détecter les incohérences dans mes demandes
- Challenger mes choix : "as-tu pensé à…"

---

## Les étapes de construction

| Étape | Ce que j'ai demandé | Ce que l'IA a produit |
|-------|---------------------|----------------------|
| 1 | Structure de base | Arborescence agents/skills/scripts |
| 2 | 5 premiers agents | Markdown structuré avec workflow |
| 3 | Système de skills injectables | Mécanisme de référence + injection |
| 4 | CLI `oh` | Script shell avec sous-commandes |
| 5 | Adapters opencode + opencode | Traduction hub → format cible |
| 6 | 22 agents supplémentaires | Spécialisation par domaine |
| 7 | Beads (tickets IA) | Intégration workflow complet |
| 8 | Documentation + ADR | 6 ADR, guides, référence CLI |

---

## Retour d'expérience

### ✅ Ce qui fonctionne
- Prompts **contextuels**, **contraints**, **itératifs**
- L'IA maintient la cohérence de format et détecte les incohérences

### ❌ Ce qui ne fonctionne pas
- Prompts vagues ou massifs, sans relecture
- L'IA sur-ingénierie et ajoute du non-demandé

### En chiffres

| Code produit par l'IA | Décisions prises par l'humain | Durée totale |
|:---:|:---:|:---:|
| ~90% | ~90% | ~3 semaines |

> On travaille ensemble, on ne délègue pas

---

## Conseils clés

1. **Vision d'abord** — expliquer le "pourquoi" avant le "comment"
2. **Itérer petit** — un fichier à la fois, relire, versionner
3. **Rester dans la boucle** — travailler avec l'IA, pas la faire travailler
4. **Challenger** — "Quelles sont les faiblesses de cette approche ?"

---

<!-- _class: part-title -->
<!-- _paginate: false -->

# Partie 2
Utiliser openhub

---

## En bref — ce que fait le hub

```
openhub/                  ← source de vérité unique
├── agents/    ← 27 rôles IA (Markdown, ~50 lignes chacun)
├── skills/    ← ~40 protocoles injectables (partagés entre agents)
└── scripts/   ← CLI `oh` + adapters par outil cible
```

> **1 modification dans le hub → tous les projets à jour** au prochain `oh deploy`

### Deux façons de l'utiliser

| | **CLI `oh`** | **OpenCode** |
|---|---|---|
| **Quoi** | Commandes ciblées depuis le terminal | Session interactive avec un agent |
| **Quand** | Gérer, déployer, lancer une action ponctuelle | Travailler sur une feature, un bug, un audit |
| **Exemple** | `oh review` · `oh audit` · `oh deploy` | Dialoguer avec l'agent orchestrator |

---

## Le CLI `oh` — gérer et agir

### Gestion des projets
```bash
oh init <ID> <path>           # Enregistrer un projet dans le hub
oh deploy <target> <ID>       # Déployer les agents (opencode | opencode)
oh status                     # État de tous les projets enregistrés
oh upgrade                    # Mettre à jour le hub (git pull + rebuild)
```

### Actions directes — lancer un agent en une commande
```bash
oh review [ID]                # Review de code : analyse le diff, produit un rapport
oh audit [ID]                 # Audit multi-domaine : sécu, perf, accessibilité, écodesign
oh debug [ID]                 # (prochainement) Diagnostic de bug
oh plan [ID]                  # (prochainement) Découpe un besoin en tickets Beads
oh doc [ID]                   # (prochainement) Détecte les lacunes documentaires
```

---

## OpenCode — travailler avec les agents

```bash
oh start MON-APP              # Ouvre OpenCode dans le contexte du projet
```

> Une fois dans OpenCode, vous choisissez un agent et vous collaborez en temps réel.

### L'agent principal — votre point d'entrée
| Agent | Ce qu'il fait |
|-------|--------------|
| `orchestrator` | Pilote une feature de A à Z (plan → spec → code → tests → PR) |

> 90% du temps, vous travaillez avec l'agent orchestrator. Il délègue aux bons agents pour vous.

### Les agents spécialisés — pour des besoins ciblés

| Agent | Quand l'utiliser directement |
|-------|------------------------------|
| `debugger` | Diagnostiquer un bug précis |
| `auditor` | Lancer un audit complet |
| `documentarian` | Rédiger ou mettre à jour la documentation |
| `reviewer` | Relire du code manuellement |
| `planner` | Découper un besoin en tickets sans implémenter |
| `onboarder` | Guider un nouveau dev dans le projet |

---

## Exemple concret — demander une feature dans OpenCode

```
Vous :  "Ajoute un export CSV des factures"

orchestrator :
  → planner              crée 3 tickets Beads              (sous-agent délégué)
  ✋ CP-0                 "Voici le plan. On continue ?"          ← VOUS
  → auditor-*            vérifie sécurité + perf           (sous-agent délégué)
  ✋ CP-audit             "2 warnings sécu. On corrige ?"         ← VOUS
  → orchestrator-dev :                                     (sous-agent délégué)
      → developer-*      implémente le code                (sous-agent délégué)
      → qa-engineer      écrit les tests                   (sous-agent délégué)
      ✋ CP-2             "PR prête. On merge ?"                  ← VOUS
      → reviewer         valide                            (sous-agent délégué)
      ✋ CP-3             "Ticket suivant ?"                      ← VOUS

Résultat : 3 commits propres, tests verts, PR prête à merger
```

> **Vous décidez à chaque ✋** — l'IA ne franchit aucune étape sans votre accord

---

## Checkpoints — le cœur du projet

> **L'IA ne fait RIEN d'irréversible sans votre accord.**

| Checkpoint | Quand | Vous décidez |
|---|---|---|
| **CP-0** | Après planification | Valider le plan · choisir le mode |
| **CP-spec** | Après spec UX/UI | Approuver · corriger · rejeter |
| **CP-audit** | Après audit | Corriger · accepter le risque · ignorer |
| **CP-2** | Après implémentation | Merger · corriger · rejeter |
| **CP-3** | Entre chaque ticket | Continuer · stopper |

Stopper à tout moment · Corriger le cap · Changer de mode en cours de route

> **Travailler avec l'IA** = garder la main sur chaque décision qui compte

---

## Beads — la mémoire du workflow

> **Sans tickets, pas de traçabilité. Sans traçabilité, pas de collaboration.**

### Ce que Beads apporte au workflow

- **Découpage structuré** — le planner crée des tickets, pas du texte dans le vide
- **Progression visible** — chaque ticket a un statut (`open` → `in_progress` → `review` → `closed`)
- **Dépendances explicites** — l'IA sait quel ticket débloquer en premier
- **Délégation contrôlée** — seuls les tickets `ai-delegated` sont traités par l'IA
- **Historique** — chaque décision, correction, blocage est tracé dans les commentaires

### Sans Beads
L'IA fait "quelque chose" → pas de trace → impossible de reprendre ou de comprendre après coup

### Avec Beads
Chaque action a un ticket, un statut, un historique → **on sait où on en est, toujours**

---

## Configuration — globale et par projet

### Configuration globale (dans le hub)
Les agents, skills et workflows sont définis **une seule fois** dans le hub.
Tous les projets héritent de cette base commune par défaut.

### Configuration par projet (overrides)
Chaque projet peut **surcharger** la configuration globale selon ses besoins :

| Ce qu'on peut personnaliser | Exemple |
|---|---|
| Provider LLM | Projet A sur Anthropic, Projet B sur Bedrock |
| Agents activés | Désactiver `ui-designer` sur un projet backend pur |
| Skills injectés | Ajouter un skill métier spécifique (RGPD, fintech…) |
| Conventions | Langue, format de commit, structure de dossiers |

> **Global par défaut, local si besoin** — pas de copier/coller entre projets

---

## Démarrage — copier/coller et c'est parti

```bash
# Installation complète (clone, dépendances, alias oc, config LLM)
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
source ~/.zshrc

# Enregistrer un projet et déployer
oh init MON-APP ~/workspace/mon-app
oh deploy opencode MON-APP

# Lancer l'IA dans le contexte du projet
oh start MON-APP
```

```bash
# Maintenance quotidienne
oh upgrade    # git pull du hub
oh status     # vue d'ensemble de tous les projets enregistrés
```

---

## Démo live

<div class="demo-container">
  <div class="video-placeholder">
    <span class="play-icon">▶</span>
    <strong>[ Démo : install → deploy → feature request → PR ]</strong>
    <span class="video-label">⟵ remplacer par vidéo ou démo terminale live</span>
  </div>
  <div class="demo-meta">
    <span>⏱ ~5 min</span>
    <span>📌 oh init · oh deploy · orchestrator · developer-* · reviewer</span>
  </div>
</div>

---

## Next steps

### 🔧 En cours — un modèle par agent
Chaque agent pourra être configuré avec son propre modèle LLM.
Ex : un modèle léger pour les `developer-*` (pré-cadrés par le workflow et les tickets Beads), un modèle puissant pour le `reviewer` ou l'`orchestrator` qui doivent raisonner et décider.

### 🌿 À venir — git worktree pour le travail en parallèle
Permettre à plusieurs agents de travailler **simultanément** sur des branches isolées
grâce à `git worktree` — sans conflits, sans attente.

### 🚀 À venir — nouvelles commandes directes
`oh debug` · `oh plan` · `oh doc` — lancer un agent spécialisé en une commande, sans ouvrir OpenCode.

---

<!-- _class: lead invert -->
<!-- _paginate: false -->

## Ce qu'il faut retenir

### 🧠 Travailler AVEC l'IA — pas la faire travailler à notre place
### 🎛️ Checkpoints — l'humain décide, l'IA exécute
### 🏗️ Un hub, N projets — zéro duplication, un `oh deploy` et c'est à jour

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

`README.fr.md` · `docs/guides/workflows.fr.md` · `docs/reference/cli.fr.md`

---

<!-- _class: lead invert -->
<!-- _paginate: false -->

# Des questions ?
