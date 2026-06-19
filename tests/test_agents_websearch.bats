#!/usr/bin/env bats
# Tests de validation des agents avec WebSearch
# Vérifie : permissions websearch/webfetch, agents exclus, skills référencées

setup() {
  AGENTS_DIR="$BATS_TEST_DIRNAME/../agents"
}

# ── Agents avec websearch : 13 agents attendus ───────────────────────

@test "7 agents ont la permission websearch allow" {
  local expected_agents=(
    "auditor/auditor-subagent"
    "planning/scout"
    "planning/onboarder"
    "planning/planner"
    "design/ux-designer"
    "design/ui-designer"
    "documentation/documentarian"
  )
  
  for agent in "${expected_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    
    # Vérifier que le fichier existe
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    
    # Extraire le frontmatter (contenu entre les deux premiers ---) et chercher websearch: allow
    if ! awk 'BEGIN{found=0} /^---$/{found++; next} found==1{print} found==2{exit}' "$file" | grep -E '^\s*websearch:\s*allow' > /dev/null; then
      echo "ERROR: Agent $agent missing 'websearch: allow' in frontmatter" >&2
      echo "Frontmatter content:" >&2
      awk 'BEGIN{found=0} /^---$/{found++; next} found==1{print} found==2{exit}' "$file" | head -20 >&2
      return 1
    fi
  done
}

@test "les agents websearch ont la permission webfetch" {
  local expected_agents=(
    "auditor/auditor-subagent"
    "planning/scout"
    "design/ux-designer"
    "documentation/documentarian"
  )
  
  for agent in "${expected_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    
    # Extraire le frontmatter et chercher webfetch: allow
    if ! awk 'BEGIN{found=0} /^---$/{found++; next} found==1{print} found==2{exit}' "$file" | grep -E '^\s*webfetch:\s*allow' > /dev/null; then
      echo "ERROR: Agent $agent missing 'webfetch: allow' in frontmatter" >&2
      return 1
    fi
  done
}

@test "websearch skills existent et sont référencées" {
  local skills=(
    "skills/shared/websearch-usage.md"
    "skills/auditor/websearch-cve-lookup.md"
    "skills/auditor/websearch-performance-research.md"
    "skills/planning/websearch-stack-research.md"
    "skills/design/websearch-design-patterns.md"
  )
  
  for skill in "${skills[@]}"; do
    local skill_file="$BATS_TEST_DIRNAME/../${skill}"
    
    # Vérifier que le fichier skill existe
    if [ ! -f "$skill_file" ]; then
      echo "ERROR: Skill file not found: $skill_file" >&2
      return 1
    fi
  done
  
  # Vérifier que websearch-usage.md est référencée dans au moins un agent
  local usage_referenced=false
  for agent_file in "$AGENTS_DIR"/*/*websearch*.md "$AGENTS_DIR"/auditor/*.md "$AGENTS_DIR"/planning/*.md "$AGENTS_DIR"/design/*.md; do
    if [ -f "$agent_file" ] && grep -q "websearch-usage" "$agent_file"; then
      usage_referenced=true
      break
    fi
  done
  
  if ! $usage_referenced; then
    echo "ERROR: shared/websearch-usage.md not referenced in any agent" >&2
    return 1
  fi
}

@test "agents exclus n'ont PAS websearch permission" {
  local excluded_agents=(
    "developer/developer-backend"
    "developer/developer-frontend"
    "developer/developer-fullstack"
    "qa/qa-engineer"
    "review/reviewer"
  )
  
  for agent in "${excluded_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    
    # Skip si le fichier n'existe pas (certains agents peuvent être absents)
    if [ ! -f "$file" ]; then
      continue
    fi
    
    # Extraire le frontmatter et vérifier que websearch: allow n'est PAS présent
    if awk 'BEGIN{found=0} /^---$/{found++; next} found==1{print} found==2{exit}' "$file" | grep -E '^\s*websearch:\s*allow' > /dev/null; then
      echo "ERROR: Excluded agent $agent should NOT have 'websearch: allow'" >&2
      return 1
    fi
  done
}

@test "shared/websearch-usage skill est référencée dans tous les agents websearch" {
  local websearch_agents=(
    "auditor/auditor-subagent"
    "planning/scout"
    "planning/onboarder"
    "planning/planner"
    "design/ux-designer"
    "design/ui-designer"
    "documentation/documentarian"
  )
  
  local missing_reference=()
  
  for agent in "${websearch_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    
    # Vérifier que websearch-usage est mentionné dans le fichier
    if ! grep -q "websearch-usage" "$file"; then
      missing_reference+=("$agent")
    fi
  done
  
  if [ ${#missing_reference[@]} -gt 0 ]; then
    echo "ERROR: The following agents don't reference websearch-usage.md:" >&2
    for agent in "${missing_reference[@]}"; do
      echo "  - $agent" >&2
    done
    return 1
  fi
}
