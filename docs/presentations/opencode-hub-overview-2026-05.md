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
    background: #1e1b4b;
    color: #c4b5fd;
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 0.85em;
  }
  pre {
    background: #0f0e17 !important;
    border-left: 4px solid #7c3aed;
    padding: 16px 20px !important;
    font-size: 0.78rem !important;
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
---

<!-- _class: lead invert -->
<!-- _paginate: false -->

# opencode-hub

### Hub central pour piloter vos assistants IA sur tous vos projets

**27 agents spécialisés · Skills injectables · Workflow Beads intégré**
**OpenCode & Claude Code**

---
<!-- _paginate: false -->

## Le problème à résoudre

> *Comment maintenir des agents IA cohérents et à jour sur plusieurs projets sans tout dupliquer ?*

### Aujourd'hui, sans hub

- 🔁 Chaque projet possède **ses propres copies** d'agents — elles divergent au fil du temps
- 🛠️ Un changement de protocole = **mise à jour manuelle** dans chaque dépôt
- 🏝️ Aucun standard partagé — chaque équipe réinvente sa configuration
- 📉 La qualité du workflow IA **se dégrade** à mesure que les projets prolifèrent

### Ce que ça coûte

Temps perdu en maintenance · Incohérences entre équipes · Agents "fantômes" obsolètes

---

## La solution — une source de vérité unique

```
opencode-hub/                  ← source de vérité (éditer ICI, jamais dans les projets)
├── agents/    ← identité des rôles IA (~40-80 lignes par agent)
├── skills/    ← protocoles détaillés injectables
└── scripts/   ← assemblage + déploiement automatisé
```

```
         oc deploy opencode MON-APP
opencode-hub  ──────────────────────►  mon-app/.opencode/agents/*.md
                                   └►  mon-app/opencode.json

         oc deploy claude-code MON-APP
opencode-hub  ──────────────────────►  mon-app/.claude/agents/*.md
```

> Mettre à jour **un seul fichier dans le hub** = tous les projets récupèrent le changement au prochain `oc deploy`

---

## 3 concepts fondamentaux

### Agent
Fichier Markdown de ~40-80 lignes. Définit **qui** est le rôle IA : périmètre, ce qu'il fait, ce qu'il ne fait pas, son workflow condensé. Court, lisible, maintenable.

### Skill
Bloc de protocole injectable. Définit **comment** l'agent travaille : standards de code, checklists, formats de rapport. Déclaré **une seule fois**, partagé entre plusieurs agents.

### Adapter
Script shell qui traduit le format hub → format attendu par l'outil cible (`opencode`, `claude-code`). Ajouter un nouvel outil = écrire un seul adapter.

---

## Chiffres clés

| | |
|---|---|
| 🤖 **27 agents** spécialisés | couvrant tout le cycle de dev |
| 🧩 **~40 skills** injectables | protocoles réutilisables entre agents |
| 🎯 **2 modes d'invocation** | `primary` (picker IA) · `subagent` (délégation) |
| ⚙️ **2 outils cibles** | OpenCode · Claude Code |
| 🔌 **5 providers LLM** | Anthropic · MammouthAI · GitHub Models · Bedrock · Ollama |
| 📋 **6 ADR** | décisions d'architecture documentées |
| 🛡️ **5 checkpoints** | l'humain garde le contrôle à chaque étape critique |

---

## 27 agents — la carte complète

| Famille | Agents | Mode |
|---------|--------|------|
| **Coordinateurs** | `orchestrator` · `orchestrator-dev` · `auditor` · `onboarder` | primary |
| **Planification** | `planner` | primary |
| **Design** | `ux-designer` · `ui-designer` | primary |
| **Qualité** | `reviewer` · `qa-engineer` · `debugger` · `documentarian` | primary |
| **Developer** | `frontend` · `backend` · `fullstack` · `data` · `devops` · `mobile` · `api` · `platform` · `security` | subagent |
| **Auditor** | `security` · `performance` · `accessibility` · `ecodesign` · `architecture` · `privacy` · `observability` | subagent |

> Les subagents sont aussi invocables **directement** si besoin — ex : `auditor-security` seul

---

## Workflows — 3 scénarios types

### Feature de A à Z → `orchestrator`
```
orchestrator → planner → ux/ui-designer → auditor-* → orchestrator-dev
                                                              ↓
                                              developer-* + qa-engineer + reviewer
```

### Bug en production → `debugger`
```
debugger → reproduction → isolation → hypothèse → rapport de cause racine → ticket Beads
```

### Audit complet → `auditor`
```
auditor → security · performance · accessibility
       → ecodesign · architecture · privacy · observability
       → rapport consolidé multi-domaine avec sévérités
```

---

## Démo 1 — Installation & premier déploiement

> **Scénario :** installer opencode-hub, enregistrer un projet, déployer les 27 agents en moins de 3 minutes

<div class="demo-container">
  <div class="video-placeholder">
    <span class="play-icon">▶</span>
    <strong>[ Insérer vidéo — Démo installation ]</strong>
    <span class="video-label">⟵ remplacer par : &lt;video src="demo-install.mp4"&gt; ou lien YouTube embed</span>
  </div>
  <div class="demo-meta">
    <span>⏱ Durée suggérée : ~2 min</span>
    <span>📌 Commandes : oc init · oc deploy · oc start</span>
  </div>
</div>

---

## Démo 2 — Feature de A à Z avec l'orchestrateur

> **Scénario :** demander une feature à `orchestrator`, observer la délégation automatique vers planner → designer → developer → qa-engineer → reviewer

<div class="demo-container">
  <div class="video-placeholder">
    <span class="play-icon">▶</span>
    <strong>[ Insérer vidéo — Démo orchestrateur ]</strong>
    <span class="video-label">⟵ remplacer par : &lt;video src="demo-orchestrator.mp4"&gt; ou lien YouTube embed</span>
  </div>
  <div class="demo-meta">
    <span>⏱ Durée suggérée : ~4 min</span>
    <span>📌 Agents : orchestrator · planner · developer-* · qa-engineer · reviewer</span>
  </div>
</div>

---

## Démo 3 — Diagnostic de bug avec le debugger

> **Scénario :** soumettre une stacktrace à `debugger`, obtenir le rapport de cause racine et la création automatique du ticket Beads

<div class="demo-container">
  <div class="video-placeholder">
    <span class="play-icon">▶</span>
    <strong>[ Insérer vidéo — Démo debugger ]</strong>
    <span class="video-label">⟵ remplacer par : &lt;video src="demo-debugger.mp4"&gt; ou lien YouTube embed</span>
  </div>
  <div class="demo-meta">
    <span>⏱ Durée suggérée : ~2 min</span>
    <span>📌 Agents : debugger · Beads CLI</span>
  </div>
</div>

---

## Checkpoints — l'humain garde le contrôle

> L'orchestrateur **ne progresse jamais sans confirmation explicite**

| Checkpoint | Déclencheur | Ce que vous décidez |
|-----------|-------------|---------------------|
| **CP-0** | Après planification | Valider le plan · choisir le mode (manuel / semi-auto / auto) |
| **CP-spec** | Après spec UX/UI | Approuver · corriger · rejeter la spec |
| **CP-audit** | Après audit | Corriger immédiatement · accepter le risque · ignorer |
| **CP-2** | Après chaque ticket | Merger · demander une correction |
| **CP-3** | Entre chaque ticket | Continuer · stopper la session |

### Pourquoi c'est important
Aucune action irréversible ne se produit sans votre accord. Vous pouvez **stopper à tout moment**, corriger le cap et reprendre.

---

## Installation — 3 minutes chrono

```bash
# 1. Installation one-liner (clone, dépendances, alias oc, configuration LLM)
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
source ~/.zshrc
```

```bash
# 2. Enregistrer votre premier projet
oc init MON-APP ~/workspace/mon-app

# 3. Déployer les 27 agents dans le projet
oc deploy opencode MON-APP       # pour OpenCode
oc deploy claude-code MON-APP    # pour Claude Code

# 4. Lancer l'outil IA dans le projet
oc start MON-APP
```

```bash
# Maintenir le hub à jour
oc upgrade          # met à jour les sources du hub (git pull)
oc update           # met à jour opencode, Beads et les skills externes
```

---

## CLI — référence rapide

| Commande | Description |
|----------|-------------|
| `oc init <ID> <path>` | Enregistrer un nouveau projet |
| `oc deploy <target> <ID>` | Déployer tous les agents (`opencode` ou `claude-code`) |
| `oc agent deploy <agent-id> [ID]` | Redéployer un seul agent |
| `oc status` | Vue d'ensemble de tous les projets |
| `oc config set <ID>` | Configurer le provider LLM (menu interactif) |
| `oc upgrade [version]` | Mettre à jour les sources du hub |
| `oc update` | Mettre à jour les outils installés |
| `oc skills validate [name]` | Vérifier la cohérence des skills |
| `oc uninstall` | Désinstallation guidée étape par étape |

> Référence complète : `docs/reference/cli.fr.md`

---

## Principes d'architecture

### Séparation identité / protocole
Agent = *qui il est* · Skill = *comment il travaille*. Modifier un skill = mise à jour immédiate sur tous les agents qui l'utilisent, sans toucher aux agents eux-mêmes.

### Spécialisation plutôt que généralisme
9 agents `developer-*` segmentés par domaine. Chaque agent reçoit **uniquement le contexte pertinent** à sa spécialité — pas de bruit, pas de confusion.

### Lecture seule pour les agents d'analyse
`auditor-*`, `reviewer`, `debugger` **n'écrivent jamais** dans le projet cible. Seuls `developer-*` et `qa-engineer` modifient des fichiers.

### Checkpoints non négociables
Aucune étape structurante ne peut être franchie sans confirmation explicite. Documenté dans [ADR-003](../architecture/adr/).

---

<!-- _class: lead invert -->
<!-- _paginate: false -->

## Prêt à démarrer ?

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
```

### Documentation
`README.fr.md` · `docs/architecture/overview.fr.md`
`docs/guides/workflows.fr.md` · `docs/reference/cli.fr.md`

---

**Questions ?**
