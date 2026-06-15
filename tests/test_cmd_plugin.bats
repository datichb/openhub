#!/usr/bin/env bats
# Tests pour scripts/cmd-plugin.sh
# Vérifie : plugin manquant, installation, sauvegarde, vérification post-install

load helpers

setup() {
  common_setup

  SCRIPT_DIR="$BATS_TEST_DIRNAME/../scripts"
  export LIB_DIR="$SCRIPT_DIR/lib"
  # HUB_DIR pointe vers le vrai repo pour que common.sh trouve i18n.sh, etc.
  # On surcharge uniquement les chemins de données sensibles
  export HUB_DIR="$BATS_TEST_DIRNAME/.."

  CMD_PLUGIN="$SCRIPT_DIR/cmd-plugin.sh"

  # Répertoire plugins de test isolé dans TEST_DIR
  PLUGIN_HUB_DIR="$TEST_DIR/hub"
  mkdir -p "$PLUGIN_HUB_DIR/plugins/rtk"
  cat > "$PLUGIN_HUB_DIR/plugins/rtk/rtk.ts" <<'EOF'
// Fake RTK plugin for tests
export default {}
EOF
  cat > "$PLUGIN_HUB_DIR/plugins/rtk/README.md" <<'EOF'
# RTK Plugin
Documentation du plugin RTK.
EOF

  mkdir -p "$PLUGIN_HUB_DIR/plugins/myplugin"
  cat > "$PLUGIN_HUB_DIR/plugins/myplugin/myplugin.ts" <<'EOF'
// Fake plugin
EOF

  # HOME isolé pour ne pas toucher la config réelle
  export HOME="$TEST_DIR/home"
  mkdir -p "$HOME/.config/opencode/plugins"

  # Mock opencode et rtk dans le PATH
  mkdir -p "$TEST_DIR/bin"
  cat > "$TEST_DIR/bin/opencode" <<'OCEOF'
#!/bin/bash
echo "opencode 1.0.0"
exit 0
OCEOF
  chmod +x "$TEST_DIR/bin/opencode"

  cat > "$TEST_DIR/bin/rtk" <<'RTKEOF'
#!/bin/bash
echo "rtk 0.50.0"
exit 0
RTKEOF
  chmod +x "$TEST_DIR/bin/rtk"

  export PATH="$TEST_DIR/bin:$PATH"
}

teardown() {
  common_teardown
}

# ── Plugin inexistant ─────────────────────────────────────────────────────────

@test "plugin : plugin inexistant → erreur et code non-zéro" {
  run bash "$CMD_PLUGIN" "inexistant"
  [ "$status" -ne 0 ]
}

@test "plugin : plugin inexistant → liste les plugins disponibles" {
  # Utilise HUB_DIR réel qui a un dossier plugins/
  run bash "$CMD_PLUGIN" "inexistant_xyz_$$"
  [ "$status" -ne 0 ]
  # Le script doit afficher les plugins disponibles via ls HUB_DIR/plugins
  [ -n "$output" ]
}

# ── Installation ──────────────────────────────────────────────────────────────

@test "plugin rtk : installe depuis le vrai hub" {
  # On teste avec le vrai plugin rtk si présent, sinon on skip
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  [ -f "$HOME/.config/opencode/plugins/rtk.ts" ]
}

@test "plugin rtk : contenu installé correspond à la source" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  diff "$real_plugin" "$HOME/.config/opencode/plugins/rtk.ts"
}

@test "plugin rtk : crée le dossier destination s'il n'existe pas" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  rm -rf "$HOME/.config/opencode/plugins"
  OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  [ -d "$HOME/.config/opencode/plugins" ]
}

@test "plugin rtk : sauvegarde l'ancien plugin si déjà présent" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  echo "old version" > "$HOME/.config/opencode/plugins/rtk.ts"
  run bash -c 'OC_NON_INTERACTIVE=1 bash "$1" install rtk' _ "$CMD_PLUGIN"
  local backups
  backups=$(ls "$HOME/.config/opencode/plugins/.backup/rtk.ts."* 2>/dev/null | wc -l | tr -d ' ')
  [ "$backups" -ge 1 ]
}

@test "plugin rtk : code de sortie 0 après installation réussie" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  run bash -c 'OC_NON_INTERACTIVE=1 bash "$1" install rtk' _ "$CMD_PLUGIN"
  [ "$status" -eq 0 ]
}

# ── Post-installation ─────────────────────────────────────────────────────────

@test "plugin rtk : affiche les étapes suivantes après installation" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  run bash -c 'OC_NON_INTERACTIVE=1 bash "$1" install rtk' _ "$CMD_PLUGIN"
  [[ "$output" =~ "opencode" ]] || [[ "$output" =~ "tail" ]] || [[ "$output" =~ "log" ]]
}

@test "plugin rtk : affiche message de succès" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  run bash -c 'OC_NON_INTERACTIVE=1 bash "$1" install rtk' _ "$CMD_PLUGIN"
  [[ "$output" =~ "✓" ]] || [[ "$output" =~ "success" ]] || [[ "$output" =~ "installé" ]] || [[ "$output" =~ "installed" ]]
}

# ── Status ────────────────────────────────────────────────────────────────────

@test "plugin status : sans plugin installé → ne mentionne pas le binaire" {
  # Plugin absent → pas de vérification du binaire, pas de message "introuvable"
  rm -f "$HOME/.config/opencode/plugins/rtk.ts"
  run bash "$CMD_PLUGIN" status
  [[ ! "$output" =~ "introuvable" ]]
  [[ ! "$output" =~ "Binary not found" ]]
}

@test "plugin status : avec plugin installé et binaire absent → mentionne le binaire" {
  # Plugin présent mais binaire rtk absent du PATH → signaler le problème
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  # Installer le plugin
  OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  # Masquer le binaire rtk : créer un dossier sans rtk en tête de PATH
  # On garde le PATH système mais on retire $TEST_DIR/bin (qui contient le mock rtk)
  local path_without_rtk
  path_without_rtk=$(echo "$PATH" | tr ':' '\n' | grep -v "$TEST_DIR/bin$" | tr '\n' ':' | sed 's/:$//')
  run env PATH="$path_without_rtk" bash "$CMD_PLUGIN" status
  [[ "$output" =~ "introuvable" ]] || [[ "$output" =~ "Binary not found" ]]
}

# ── Install sans binaire ──────────────────────────────────────────────────────

@test "plugin install rtk : sans binaire + réponse n → exit 1 + instructions" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  # PATH sans le mock rtk (mais avec tous les outils système)
  local path_without_rtk
  path_without_rtk=$(echo "$PATH" | tr ':' '\n' | grep -v "$TEST_DIR/bin$" | tr '\n' ':' | sed 's/:$//')
  run env PATH="$path_without_rtk" OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  [ "$status" -ne 0 ]
  # Les instructions manuelles doivent apparaître
  [[ "$output" =~ "rtk-ai.app" ]] || [[ "$output" =~ "cargo" ]] || [[ "$output" =~ "brew" ]]
}

@test "plugin install rtk : sans binaire + OC_NON_INTERACTIVE=1 → exit 1 proprement" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  # PATH sans le mock rtk
  local path_without_rtk
  path_without_rtk=$(echo "$PATH" | tr ':' '\n' | grep -v "$TEST_DIR/bin$" | tr '\n' ':' | sed 's/:$//')
  run env PATH="$path_without_rtk" OC_NON_INTERACTIVE=1 bash "$CMD_PLUGIN" install rtk
  [ "$status" -ne 0 ]
  [[ "$output" =~ "rtk-ai.app" ]] || [[ "$output" =~ "cargo" ]] || [[ "$output" =~ "brew" ]]
}

@test "plugin install rtk : sans binaire + brew mocké → exit 0 + plugin installé" {
  local real_plugin="$HUB_DIR/plugins/rtk/rtk.ts"
  [ -f "$real_plugin" ] || skip "plugin rtk absent du hub"
  # Créer un dossier avec brew mocké mais sans rtk (en tête de PATH)
  mkdir -p "$TEST_DIR/bin_mock"
  # Mock brew qui installe rtk en le créant dans bin_mock
  cat > "$TEST_DIR/bin_mock/brew" <<BREWEOF
#!/bin/bash
if [ "\$1" = "install" ] && [ "\$2" = "rtk" ]; then
  cat > "$TEST_DIR/bin_mock/rtk" <<'RTKEOF'
#!/bin/bash
echo "rtk 0.50.0"
exit 0
RTKEOF
  chmod +x "$TEST_DIR/bin_mock/rtk"
  exit 0
fi
exit 1
BREWEOF
  chmod +x "$TEST_DIR/bin_mock/brew"
  # PATH : bin_mock en tête (brew présent, rtk absent au départ), sans bin (mock rtk)
  local path_without_rtk
  path_without_rtk=$(echo "$PATH" | tr ':' '\n' | grep -v "$TEST_DIR/bin$" | tr '\n' ':' | sed 's/:$//')
  run env PATH="$TEST_DIR/bin_mock:$path_without_rtk" OC_NON_INTERACTIVE=0 \
    bash -c "echo 'y' | bash '$CMD_PLUGIN' install rtk"
  [ "$status" -eq 0 ]
  [ -f "$HOME/.config/opencode/plugins/rtk.ts" ]
}
