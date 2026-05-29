#!/usr/bin/env bats
# Tests de validation des skills WebSearch
# Vérifie : existence, sections requises, query examples, données sensibles, liens internes

setup() {
  SKILLS_DIR="$BATS_TEST_DIRNAME/../skills"
}

# ── Skills existence ─────────────────────────────────────────────────

@test "all 5 websearch skills exist" {
  local skills=(
    "shared/websearch-usage.md"
    "auditor/websearch-cve-lookup.md"
    "auditor/websearch-performance-research.md"
    "planning/websearch-stack-research.md"
    "design/websearch-design-patterns.md"
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$SKILLS_DIR/$skill"
    if [ ! -f "$skill_file" ]; then
      echo "ERROR: Skill file not found: $skill_file" >&2
      return 1
    fi
  done
}

# ── Sections requises ────────────────────────────────────────────────

@test "websearch skills contain required sections" {
  local skills=(
    "shared/websearch-usage.md"
    "auditor/websearch-cve-lookup.md"
    "auditor/websearch-performance-research.md"
  )
  
  local required_sections=(
    "Purpose"
    "When to Use"
    "Best Practices"
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$SKILLS_DIR/$skill"
    
    for section in "${required_sections[@]}"; do
      # Chercher les headers markdown (## Section ou ### Section)
      if ! grep -qE "^##+ .*${section}" "$skill_file"; then
        echo "ERROR: Skill $skill missing required section: $section" >&2
        return 1
      fi
    done
  done
}

# ── Query examples ──────────────────────────────────────────────────

@test "websearch skills contain at least 5 query examples" {
  local skills=(
    "shared/websearch-usage.md"
    "auditor/websearch-cve-lookup.md"
    "auditor/websearch-performance-research.md"
    "planning/websearch-stack-research.md"
    "design/websearch-design-patterns.md"
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$SKILLS_DIR/$skill"
    
    # Compter les exemples de queries (format: ✅ "query" ou ✓ "query")
    local count
    count=$(grep -cE '(✅|✓|✔) "' "$skill_file" 2>/dev/null || echo 0)
    
    if [ "$count" -lt 5 ]; then
      echo "ERROR: Skill $skill has only $count query examples (expected at least 5)" >&2
      echo "Query examples found:" >&2
      grep -E '(✅|✓|✔) "' "$skill_file" >&2 || echo "  (none)" >&2
      return 1
    fi
  done
}

# ── Données sensibles ───────────────────────────────────────────────

@test "websearch skills contain no sensitive data" {
  local skills=(
    "shared/websearch-usage.md"
    "auditor/websearch-cve-lookup.md"
    "auditor/websearch-performance-research.md"
    "planning/websearch-stack-research.md"
    "design/websearch-design-patterns.md"
  )
  
  # Patterns qui indiqueraient des vraies données sensibles (pas juste mentionner le mot)
  # Format: sk-ant-xxx, token=xxx, api_key="xxx", etc.
  local sensitive_patterns=(
    "api_key[ ]*=[ ]*['\"]?[a-zA-Z0-9]+"
    "api-key[ ]*=[ ]*['\"]?[a-zA-Z0-9]+"
    "token[ ]*=[ ]*['\"]?[a-zA-Z0-9]+"
    "password[ ]*=[ ]*['\"]?[a-zA-Z0-9]+"
    "sk-ant-[a-zA-Z0-9]{20,}"
    "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"  # Email réel
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$SKILLS_DIR/$skill"
    
    for pattern in "${sensitive_patterns[@]}"; do
      # Ignorer les commentaires, exemples génériques et placeholders
      if grep -E "$pattern" "$skill_file" | grep -viE "(example|placeholder|xxx|\\*\\*\\*|sk-\\.\\.\\.|\\.\\.\\.)" | grep -q .; then
        echo "ERROR: Skill $skill contains potential sensitive data matching pattern: $pattern" >&2
        grep -E "$pattern" "$skill_file" | grep -viE "(example|placeholder|xxx|\\*\\*\\*|sk-\\.\\.\\.|\\.\\.\\.)" >&2
        return 1
      fi
    done
  done
}

# ── Liens internes valides ──────────────────────────────────────────

@test "websearch skills have valid internal links" {
  local skills=(
    "shared/websearch-usage.md"
    "auditor/websearch-cve-lookup.md"
    "auditor/websearch-performance-research.md"
    "planning/websearch-stack-research.md"
    "design/websearch-design-patterns.md"
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$SKILLS_DIR/$skill"
    local skill_dir="$(dirname "$skill_file")"
    
    # Extraire les liens markdown relatifs [text](path.md)
    # Ignorer les URLs absolutes (http://, https://)
    local links
    links=$(grep -oE '\[([^\]]+)\]\(([^)]+)\)' "$skill_file" | grep -oE '\(([^)]+)\)' | tr -d '()' | grep -vE '^https?://' || true)
    
    if [ -z "$links" ]; then
      # Pas de liens internes, c'est OK
      continue
    fi
    
    while IFS= read -r link; do
      [ -z "$link" ] && continue
      
      # Résoudre le chemin relatif
      local target_file
      if [[ "$link" = /* ]]; then
        # Lien absolu depuis la racine du repo
        target_file="$BATS_TEST_DIRNAME/..$link"
      elif [[ "$link" = ../* ]]; then
        # Lien relatif vers le parent
        target_file="$skill_dir/$link"
      else
        # Lien relatif dans le même dossier
        target_file="$skill_dir/$link"
      fi
      
      # Supprimer les ancres (#section)
      target_file="${target_file%%#*}"
      
      if [ ! -f "$target_file" ] && [ ! -d "$target_file" ]; then
        echo "ERROR: Skill $skill has broken internal link: $link" >&2
        echo "  Target not found: $target_file" >&2
        return 1
      fi
    done <<< "$links"
  done
}
