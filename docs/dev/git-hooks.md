# Git Hooks — OpenCode Hub

**Version** : 1.0.0  
**Date** : 2026-05-29

---

## Vue d'ensemble

Ce document décrit les git hooks disponibles pour OpenCode Hub, notamment pour la validation automatique des configurations WebSearch avant chaque commit.

## Pre-commit Hook : WebSearch Validation

Le pre-commit hook WebSearch valide automatiquement les configurations WebSearch (agents, skills) avant chaque commit pour garantir l'intégrité du système.

### Qu'est-ce qui est validé ?

1. **Agents WebSearch** : Vérifie que les 13 agents attendus ont la permission `websearch: allow`
2. **Skills WebSearch** : Vérifie que les 5 skills WebSearch existent et sont valides
3. **Format frontmatter** : Vérifie la syntaxe YAML des frontmatters

### Installation

#### Option 1 : Installation manuelle

```bash
# Copier le hook dans .git/hooks/
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash
# Pre-commit hook: WebSearch validation

echo "🔍 Validating WebSearch configuration..."

if ! command -v bats &> /dev/null; then
  echo "⚠️  BATS not installed, skipping WebSearch validation"
  echo "   Install with: sudo apt-get install bats (Linux) or brew install bats-core (macOS)"
  exit 0
fi

# Run WebSearch validation tests
echo "Running agent validation tests..."
if ! bats tests/test_agents_websearch.bats > /dev/null 2>&1; then
  echo "❌ Agent validation failed"
  echo ""
  echo "Run manually to see errors:"
  echo "  bats tests/test_agents_websearch.bats"
  echo ""
  echo "To commit anyway, use: git commit --no-verify"
  exit 1
fi

echo "Running skill validation tests..."
if ! bats tests/test_skills_websearch.bats > /dev/null 2>&1; then
  echo "❌ Skill validation failed"
  echo ""
  echo "Run manually to see errors:"
  echo "  bats tests/test_skills_websearch.bats"
  echo ""
  echo "To commit anyway, use: git commit --no-verify"
  exit 1
fi

echo "✅ WebSearch validation passed"
exit 0
EOF

# Rendre le hook exécutable
chmod +x .git/hooks/pre-commit

echo "✅ Pre-commit hook installed"
```

#### Option 2 : Installation via script

```bash
# Créer un script d'installation
./scripts/install-git-hooks.sh
```

### Usage

Une fois installé, le hook s'exécute automatiquement avant chaque commit :

```bash
git add agents/auditor/auditor-subagent.md
git commit -m "feat: update auditor-subagent agent"

# Output:
# 🔍 Validating WebSearch configuration...
# Running agent validation tests...
# Running skill validation tests...
# ✅ WebSearch validation passed
# [main 3391e13] feat: update auditor-subagent agent
```

### Bypass (si nécessaire)

Si vous devez commiter sans passer la validation (par exemple lors d'un travail en cours) :

```bash
git commit --no-verify -m "wip: work in progress"
```

**⚠️ Attention** : Utiliser `--no-verify` désactive TOUS les hooks. À utiliser avec précaution.

### Dépannage

#### BATS non installé

**Erreur** :
```
⚠️  BATS not installed, skipping WebSearch validation
```

**Solution** :
```bash
# Linux (Ubuntu/Debian)
sudo apt-get install bats

# macOS
brew install bats-core

# Vérifier l'installation
which bats
bats --version
```

#### Tests échouent localement

**Erreur** :
```
❌ Agent validation failed
```

**Solution** :
```bash
# Exécuter les tests manuellement pour voir les erreurs détaillées
bats tests/test_agents_websearch.bats

# Corriger les erreurs identifiées
# Puis recommiter
git commit
```

#### Hook ne s'exécute pas

**Problème** : Le hook est installé mais ne s'exécute pas.

**Vérification** :
```bash
# Vérifier que le hook existe
ls -la .git/hooks/pre-commit

# Vérifier qu'il est exécutable
chmod +x .git/hooks/pre-commit

# Tester manuellement
.git/hooks/pre-commit
```

---

## Autres hooks disponibles

### Pre-push Hook : Tests complets

Un pre-push hook peut être ajouté pour exécuter tous les tests avant un push :

```bash
cat > .git/hooks/pre-push <<'EOF'
#!/bin/bash
# Pre-push hook: Run all tests

echo "🧪 Running all tests before push..."

if ! bats tests/*.bats; then
  echo "❌ Tests failed. Push aborted."
  echo "Fix the errors and try again."
  exit 1
fi

echo "✅ All tests passed"
exit 0
EOF

chmod +x .git/hooks/pre-push
```

### Commit-msg Hook : Format de message

Un commit-msg hook peut valider le format des messages de commit :

```bash
cat > .git/hooks/commit-msg <<'EOF'
#!/bin/bash
# Commit-msg hook: Validate commit message format

commit_msg=$(cat "$1")

# Vérifier le format conventional commits
if ! echo "$commit_msg" | grep -qE "^(feat|fix|docs|style|refactor|test|chore)(\(.+\))?: .+"; then
  echo "❌ Invalid commit message format"
  echo ""
  echo "Expected format: <type>(<scope>): <message>"
  echo "Examples:"
  echo "  feat(websearch): add new query optimization"
  echo "  fix(agents): correct frontmatter validation"
  echo "  docs: update websearch usage examples"
  echo ""
  echo "Valid types: feat, fix, docs, style, refactor, test, chore"
  exit 1
fi

exit 0
EOF

chmod +x .git/hooks/commit-msg
```

---

## CI/CD Integration

Les mêmes validations sont également exécutées dans GitHub Actions via le workflow `.github/workflows/test-websearch.yml`.

Voir la documentation CI/CD pour plus de détails.

---

## Ressources

- **Tests BATS** : `tests/test_*websearch*.bats`
- **GitHub Actions** : `.github/workflows/test-websearch.yml`
- **Documentation Git Hooks** : https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks

---

**Version** : 1.0.0  
**Auteur** : OpenCode Hub  
**Dernière mise à jour** : 2026-05-29
