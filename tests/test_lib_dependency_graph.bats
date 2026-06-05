#!/usr/bin/env bats
# Tests unitaires pour scripts/lib/dependency-graph.sh
# Fonctions testées :
#   depgraph_file_path, generate_dependency_graph,
#   depgraph_exists, depgraph_get_imported_by, depgraph_get_imports,
#   depgraph_are_linked, depgraph_stats

load helpers

setup() {
  common_setup

  export SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  source "$SCRIPT_DIR/common.sh"
  source "$LIB_DIR/dependency-graph.sh"

  # Projet fictif utilisé par les tests
  export PROJECT_PATH="$TEST_DIR/fake-project"
  mkdir -p "$PROJECT_PATH"
  mkdir -p "$PROJECT_PATH/.opencode"
}

teardown() {
  common_teardown
}

# ── Helpers internes ──────────────────────────────────────────────────────────

# Crée un fichier TS minimal dans le projet de test
# Usage : make_ts_file "src/a.ts" ["import from './b'"]
make_ts_file() {
  local rel_path="${1:?rel_path requis}"
  local content="${2:-// empty}"
  local full_path="$PROJECT_PATH/$rel_path"
  mkdir -p "$(dirname "$full_path")"
  printf '%s\n' "$content" > "$full_path"
}

# ── depgraph_file_path ────────────────────────────────────────────────────────

@test "depgraph_file_path : retourne le bon chemin absolu" {
  run depgraph_file_path "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  [ "$output" = "$PROJECT_PATH/.opencode/dependency-graph.json" ]
}

@test "depgraph_file_path : échoue sans argument" {
  run depgraph_file_path
  [ "$status" -ne 0 ]
}

# ── depgraph_exists ───────────────────────────────────────────────────────────

@test "depgraph_exists : retourne 1 si le graphe n'existe pas" {
  run depgraph_exists "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "depgraph_exists : retourne 0 si le graphe existe" {
  echo '{}' > "$PROJECT_PATH/.opencode/dependency-graph.json"
  run depgraph_exists "$PROJECT_PATH"
  [ "$status" -eq 0 ]
}

# ── generate_dependency_graph ─────────────────────────────────────────────────

@test "generate_dependency_graph : retourne 1 si aucun fichier TS/JS" {
  require_command jq
  run generate_dependency_graph "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

@test "generate_dependency_graph : génère un JSON valide pour un projet simple" {
  require_command jq
  make_ts_file "src/index.ts" "// entry point"
  make_ts_file "src/utils.ts" "// utils"

  run generate_dependency_graph "$PROJECT_PATH"
  [ "$status" -eq 0 ]

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  [ -f "$graph_file" ]
  assert_json_valid "$graph_file"
}

@test "generate_dependency_graph : contient les champs version, generated_at, root, stats, nodes" {
  require_command jq
  make_ts_file "src/a.ts" "// a"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  assert_json_field "$graph_file" '.version' "1.0"
  [ "$(jq -r '.root' "$graph_file")" = "$PROJECT_PATH" ]
  [ "$(jq -r '.stats.files_scanned' "$graph_file")" -ge 1 ]
  [ "$(jq -r '.nodes | type' "$graph_file")" = "object" ]
}

@test "generate_dependency_graph : scanne les fichiers .ts, .tsx, .js" {
  require_command jq
  make_ts_file "src/a.ts"  "// ts"
  make_ts_file "src/b.tsx" "// tsx"
  make_ts_file "src/c.js"  "// js"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  assert_json_valid "$graph_file"
  local count
  count=$(jq '.stats.files_scanned' "$graph_file")
  [ "$count" -ge 3 ]
}

@test "generate_dependency_graph : exclut node_modules" {
  require_command jq
  make_ts_file "src/a.ts" "// main"
  make_ts_file "node_modules/lib/index.ts" "// should be excluded"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  local has_node_modules
  has_node_modules=$(jq 'any(.nodes | keys[]; startswith("node_modules"))' "$graph_file")
  [ "$has_node_modules" = "false" ]
}

@test "generate_dependency_graph : exclut dist/" {
  require_command jq
  make_ts_file "src/a.ts" "// main"
  make_ts_file "dist/a.js" "// compiled"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  local has_dist
  has_dist=$(jq 'any(.nodes | keys[]; startswith("dist"))' "$graph_file")
  [ "$has_dist" = "false" ]
}

@test "generate_dependency_graph : résout les imports relatifs" {
  require_command jq
  make_ts_file "src/index.ts" "import { foo } from './utils'"
  make_ts_file "src/utils.ts" "export const foo = 1"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  # src/index.ts doit avoir src/utils.ts dans ses imports
  local imports
  imports=$(jq -r '.nodes["src/index.ts"].imports[]?' "$graph_file" 2>/dev/null)
  [[ "$imports" == *"utils"* ]]
}

@test "generate_dependency_graph : calcule correctement imported_by (inversion du graphe)" {
  require_command jq
  make_ts_file "src/service.ts" "export const svc = 1"
  make_ts_file "src/controller.ts" "import { svc } from './service'"
  make_ts_file "src/app.ts" "import { svc } from './service'"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  # src/service.ts doit être importé par controller et app
  local importers_count
  importers_count=$(jq '.nodes["src/service.ts"].imported_by | length' "$graph_file" 2>/dev/null)
  [ "$importers_count" -eq 2 ]
}

@test "generate_dependency_graph : ignore les imports non-relatifs (node_modules)" {
  require_command jq
  make_ts_file "src/a.ts" "import axios from 'axios'; import { foo } from './b'"
  make_ts_file "src/b.ts" "export const foo = 1"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  # axios ne doit pas apparaître dans les imports de src/a.ts
  local imports
  imports=$(jq -r '.nodes["src/a.ts"].imports[]?' "$graph_file" 2>/dev/null)
  [[ "$imports" != *"axios"* ]]
}

@test "generate_dependency_graph : écrit atomiquement (pas de fichier .tmp en cas de succès)" {
  require_command jq
  make_ts_file "src/a.ts" "// file"

  generate_dependency_graph "$PROJECT_PATH"

  # Aucun fichier tmp ne doit rester
  local tmp_count
  tmp_count=$(find "$PROJECT_PATH/.opencode" -name "*.tmp.*" 2>/dev/null | wc -l | tr -d ' ')
  [ "$tmp_count" -eq 0 ]
}

# ── depgraph_get_imports ──────────────────────────────────────────────────────

@test "depgraph_get_imports : retourne les imports d'un fichier" {
  require_command jq
  make_ts_file "src/a.ts" "import { b } from './b'"
  make_ts_file "src/b.ts" "export const b = 1"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_get_imports "$PROJECT_PATH" "src/a.ts"
  [ "$status" -eq 0 ]
  [[ "$output" == *"src/b.ts"* ]]
}

@test "depgraph_get_imports : retourne vide pour un fichier sans imports" {
  require_command jq
  make_ts_file "src/leaf.ts" "export const x = 1"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_get_imports "$PROJECT_PATH" "src/leaf.ts"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "depgraph_get_imports : retourne 1 si le graphe n'existe pas" {
  run depgraph_get_imports "$PROJECT_PATH" "src/a.ts"
  [ "$status" -ne 0 ]
}

# ── depgraph_get_imported_by ──────────────────────────────────────────────────

@test "depgraph_get_imported_by : retourne les importeurs d'un fichier" {
  require_command jq
  make_ts_file "src/shared.ts" "export const x = 1"
  make_ts_file "src/a.ts" "import { x } from './shared'"
  make_ts_file "src/b.ts" "import { x } from './shared'"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_get_imported_by "$PROJECT_PATH" "src/shared.ts"
  [ "$status" -eq 0 ]
  [[ "$output" == *"src/a.ts"* ]]
  [[ "$output" == *"src/b.ts"* ]]
}

@test "depgraph_get_imported_by : retourne vide pour un fichier non importé" {
  require_command jq
  make_ts_file "src/orphan.ts" "export const y = 2"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_get_imported_by "$PROJECT_PATH" "src/orphan.ts"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "depgraph_get_imported_by : retourne 1 si le graphe n'existe pas" {
  run depgraph_get_imported_by "$PROJECT_PATH" "src/x.ts"
  [ "$status" -ne 0 ]
}

# ── depgraph_are_linked ───────────────────────────────────────────────────────

@test "depgraph_are_linked : détecte lien A→B (A importe B)" {
  require_command jq
  make_ts_file "src/a.ts" "import { b } from './b'"
  make_ts_file "src/b.ts" "export const b = 1"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"
  [ "$status" -eq 0 ]
}

@test "depgraph_are_linked : détecte lien B→A (B importe A)" {
  require_command jq
  make_ts_file "src/a.ts" "export const a = 1"
  make_ts_file "src/b.ts" "import { a } from './a'"

  generate_dependency_graph "$PROJECT_PATH"

  # Tester dans le sens inverse (B importe A, on demande A,B)
  run depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"
  [ "$status" -eq 0 ]
}

@test "depgraph_are_linked : retourne 1 si aucun lien direct" {
  require_command jq
  make_ts_file "src/a.ts" "export const a = 1"
  make_ts_file "src/b.ts" "export const b = 2"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"
  [ "$status" -ne 0 ]
}

@test "depgraph_are_linked : retourne 1 si graphe absent" {
  run depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"
  [ "$status" -ne 0 ]
}

# ── depgraph_stats ────────────────────────────────────────────────────────────

@test "depgraph_stats : affiche le nombre de fichiers et imports" {
  require_command jq
  make_ts_file "src/a.ts" "import { b } from './b'"
  make_ts_file "src/b.ts" "export const b = 1"

  generate_dependency_graph "$PROJECT_PATH"

  run depgraph_stats "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fichiers scannés"* ]]
  [[ "$output" == *"Imports trouvés"* ]]
}

@test "depgraph_stats : retourne 1 si graphe absent" {
  run depgraph_stats "$PROJECT_PATH"
  [ "$status" -ne 0 ]
}

# ── Cas limites ───────────────────────────────────────────────────────────────

@test "generate_dependency_graph : gère les noms de fichiers avec espaces" {
  require_command jq
  # bash 3.2 : créer manuellement le fichier avec espace dans le nom
  local dir="$PROJECT_PATH/src"
  mkdir -p "$dir"
  printf '// file with space\n' > "$dir/my component.ts"

  # Générer le graphe — ne doit pas planter
  run generate_dependency_graph "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  assert_json_valid "$PROJECT_PATH/.opencode/dependency-graph.json"
}

@test "generate_dependency_graph : graphe JSON invalide préexistant est remplacé" {
  require_command jq
  make_ts_file "src/a.ts" "// a"

  # Écrire un JSON invalide dans le fichier graphe
  echo "INVALID JSON" > "$PROJECT_PATH/.opencode/dependency-graph.json"

  run generate_dependency_graph "$PROJECT_PATH"
  [ "$status" -eq 0 ]
  assert_json_valid "$PROJECT_PATH/.opencode/dependency-graph.json"
}

@test "generate_dependency_graph : compte correct des stats" {
  require_command jq
  make_ts_file "src/a.ts" "import { b } from './b'; import { c } from './c'"
  make_ts_file "src/b.ts" "export const b = 1"
  make_ts_file "src/c.ts" "export const c = 2"

  generate_dependency_graph "$PROJECT_PATH"

  local graph_file="$PROJECT_PATH/.opencode/dependency-graph.json"
  local files_scanned
  files_scanned=$(jq '.stats.files_scanned' "$graph_file")
  [ "$files_scanned" -eq 3 ]

  local total_imports
  total_imports=$(jq '.stats.total_imports' "$graph_file")
  [ "$total_imports" -eq 2 ]
}
