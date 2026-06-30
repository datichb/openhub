#!/usr/bin/env bats
# Tests de validation des permissions ctx pour les agents
# Vérifie : ctx_search, ctx_batch_execute, ctx_execute, ctx_index selon le type d'agent

setup() {
  AGENTS_DIR="$BATS_TEST_DIRNAME/../agents"
}

# ── Helpers ──────────────────────────────────────────────────────────

# Extrait le frontmatter d'un fichier agent
frontmatter() {
  awk 'BEGIN{found=0} /^---$/{found++; next} found==1{print} found==2{exit}' "$1"
}

has_permission() {
  local file="$1"
  local perm="$2"
  frontmatter "$file" | grep -E "^\s*${perm}:\s*allow" > /dev/null
}

# ── Orchestrateurs : ctx_search + ctx_stats + ctx_batch_execute ──────

@test "orchestrateurs ont ctx_search allow" {
  local agents=("planning/orchestrator" "planning/orchestrator-dev")
  for agent in "${agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    if ! has_permission "$file" "ctx_search"; then
      echo "ERROR: $agent missing 'ctx_search: allow'" >&2
      return 1
    fi
  done
}

@test "orchestrateurs ont ctx_stats allow" {
  local agents=("planning/orchestrator" "planning/orchestrator-dev")
  for agent in "${agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    if ! has_permission "$file" "ctx_stats"; then
      echo "ERROR: $agent missing 'ctx_stats: allow'" >&2
      return 1
    fi
  done
}

@test "orchestrateurs ont ctx_batch_execute allow" {
  local agents=("planning/orchestrator" "planning/orchestrator-dev")
  for agent in "${agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if ! has_permission "$file" "ctx_batch_execute"; then
      echo "ERROR: $agent missing 'ctx_batch_execute: allow'" >&2
      return 1
    fi
  done
}

# ── Développeurs : jeu complet ctx ───────────────────────────────────

@test "agents développeurs ont le jeu complet de permissions ctx" {
  local dev_agents=(
    "developer/developer"
    "developer/developer-refactor"
    "developer/developer-migrator"
  )
  local required_perms=(
    "ctx_search"
    "ctx_execute"
    "ctx_execute_file"
    "ctx_batch_execute"
    "ctx_fetch_and_index"
    "ctx_index"
  )

  for agent in "${dev_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    for perm in "${required_perms[@]}"; do
      if ! has_permission "$file" "$perm"; then
        echo "ERROR: $agent missing '$perm: allow'" >&2
        return 1
      fi
    done
  done
}

# ── Qualité : ctx sans ctx_index ─────────────────────────────────────

@test "agents qualité ont ctx_search, ctx_execute, ctx_execute_file, ctx_batch_execute" {
  local quality_agents=(
    "quality/qa-engineer"
    "quality/debugger"
    "quality/reviewer"
  )
  local required_perms=(
    "ctx_search"
    "ctx_execute"
    "ctx_execute_file"
    "ctx_batch_execute"
  )

  for agent in "${quality_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    for perm in "${required_perms[@]}"; do
      if ! has_permission "$file" "$perm"; then
        echo "ERROR: $agent missing '$perm: allow'" >&2
        return 1
      fi
    done
  done
}

@test "agents qualité n'ont PAS ctx_index (lecture seule)" {
  local quality_agents=(
    "quality/reviewer"
    "auditor/auditor-subagent"
  )

  for agent in "${quality_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      continue
    fi
    if has_permission "$file" "ctx_index"; then
      echo "ERROR: Read-only agent $agent should NOT have 'ctx_index: allow'" >&2
      return 1
    fi
  done
}

# ── Planning : ctx_search + ctx_stats + ctx_batch_execute ────────────

@test "agents planning ont ctx_search, ctx_stats, ctx_batch_execute" {
  local planning_agents=(
    "planning/planner"
    "planning/pathfinder"
    "planning/onboarder"
  )
  local required_perms=(
    "ctx_search"
    "ctx_stats"
    "ctx_batch_execute"
  )

  for agent in "${planning_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    for perm in "${required_perms[@]}"; do
      if ! has_permission "$file" "$perm"; then
        echo "ERROR: $agent missing '$perm: allow'" >&2
        return 1
      fi
    done
  done
}

# ── Design : ctx_search + ctx_batch_execute ──────────────────────────

@test "agents design ont ctx_search et ctx_batch_execute" {
  local design_agents=(
    "design/designer"
  )
  local required_perms=("ctx_search" "ctx_batch_execute")

  for agent in "${design_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    for perm in "${required_perms[@]}"; do
      if ! has_permission "$file" "$perm"; then
        echo "ERROR: $agent missing '$perm: allow'" >&2
        return 1
      fi
    done
  done
}

# ── Documentation : ctx_search + ctx_batch_execute + ctx_index ───────

@test "documentarian a ctx_search, ctx_batch_execute, ctx_index" {
  local file="$AGENTS_DIR/documentation/documentarian.md"
  if [ ! -f "$file" ]; then
    echo "ERROR: Agent file not found: $file" >&2
    return 1
  fi
  for perm in "ctx_search" "ctx_batch_execute" "ctx_index"; do
    if ! has_permission "$file" "$perm"; then
      echo "ERROR: documentarian missing '$perm: allow'" >&2
      return 1
    fi
  done
}

# ── Audit : ctx_search + ctx_batch_execute (lecture seule) ───────────

@test "agents audit ont ctx_search et ctx_batch_execute" {
  local audit_agents=(
    "auditor/auditor"
    "auditor/auditor-subagent"
  )
  local required_perms=("ctx_search" "ctx_batch_execute")

  for agent in "${audit_agents[@]}"; do
    file="$AGENTS_DIR/${agent}.md"
    if [ ! -f "$file" ]; then
      echo "ERROR: Agent file not found: $file" >&2
      return 1
    fi
    for perm in "${required_perms[@]}"; do
      if ! has_permission "$file" "$perm"; then
        echo "ERROR: $agent missing '$perm: allow'" >&2
        return 1
      fi
    done
  done
}

# ── Cohérence : ctx_execute implique ctx_search ──────────────────────

@test "tout agent avec ctx_execute a aussi ctx_search" {
  for file in "$AGENTS_DIR"/*/*.md; do
    if has_permission "$file" "ctx_execute"; then
      if ! has_permission "$file" "ctx_search"; then
        agent=$(basename "$(dirname "$file")")/$(basename "$file" .md)
        echo "ERROR: $agent has ctx_execute but missing ctx_search" >&2
        return 1
      fi
    fi
  done
}

@test "tout agent avec ctx_index a aussi ctx_search" {
  for file in "$AGENTS_DIR"/*/*.md; do
    if has_permission "$file" "ctx_index"; then
      if ! has_permission "$file" "ctx_search"; then
        agent=$(basename "$(dirname "$file")")/$(basename "$file" .md)
        echo "ERROR: $agent has ctx_index but missing ctx_search" >&2
        return 1
      fi
    fi
  done
}
