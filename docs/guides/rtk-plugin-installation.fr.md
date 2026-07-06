# Guide d'Installation du Plugin RTK

Ce guide explique comment installer le plugin RTK pour OpenCode depuis openhub.

## Prérequis

Avant d'installer le plugin, assurez-vous d'avoir :

1. **OpenCode** >= 1.15.0 installé
   ```bash
   opencode --version
   ```

2. **RTK** >= 0.42.0 — installé automatiquement par `oh plugin install rtk` si absent
   ```bash
   # Vérifier si déjà installé
   rtk --version
   ```

3. **openhub** cloné et configuré
   ```bash
   cd ~/.openhub
   git pull  # Pour avoir la dernière version
   ```

---

## Installation Automatique (Recommandé)

### Méthode 1 : Via openhub

```bash
# Depuis n'importe où
oh plugin install rtk
```

Le script va :
1. Vérifier que OpenCode est installé
2. Si RTK est absent, proposer de l'installer automatiquement (brew ou cargo)
3. Vérifier la version de RTK (>= 0.42.0 recommandé), avec option de mise à jour automatique
4. Sauvegarder l'ancien plugin si existant
5. Copier le nouveau plugin dans `~/.config/opencode/plugins/rtk.ts`
6. Afficher les instructions de vérification

---

## Installation Manuelle

Si vous préférez installer manuellement :

```bash
# Créer le dossier plugins si nécessaire
mkdir -p ~/.config/opencode/plugins

# Copier le plugin
cp ~/.openhub/plugins/rtk/rtk.ts ~/.config/opencode/plugins/rtk.ts

# Vérifier
ls -lah ~/.config/opencode/plugins/rtk.ts
```

---

## Vérification de l'Installation

### 1. Redémarrer OpenCode

Si OpenCode est en cours d'exécution, fermez-le et relancez-le.

### 2. Vérifier les Logs

```bash
# Suivre les logs en temps réel
tail -f ~/.cache/opencode/logs/opencode.log | grep rtk-plugin

# Vous devriez voir au démarrage :
# [rtk-plugin] RTK plugin initialized
# service: "rtk-plugin", level: "info", message: "RTK plugin initialized"
```

### 3. Tester le Plugin

Dans OpenCode, exécutez une commande qui génère beaucoup de sortie :

```
> Run: git diff HEAD~10 HEAD
```

**Attendu :**
- La commande s'exécute avec une sortie filtrée (compacte)
- Si la commande économise >10K tokens, un toast apparaît :
  ```
  🚀 RTK saved ~15.2K tokens on this command
  ```
- Les logs montrent les détails :
  ```bash
  tail ~/.cache/opencode/logs/opencode.log | grep rtk-plugin
  ```

### 4. Vérifier le Résumé de Session

Après plusieurs commandes, quand la session devient inactive (idle), vous devriez voir :

```
✨ Session complete: RTK saved 2.34M tokens across 12 commands (avg 195.0K/cmd)
```

---

## Configuration (Optionnel)

### Ajuster le Seuil de Notification

Par défaut, les toasts apparaissent pour les commandes économisant >10K tokens.

Pour modifier ce seuil :

1. Éditez le plugin :
   ```bash
   vim ~/.config/opencode/plugins/rtk.ts
   # ou
   code ~/.config/opencode/plugins/rtk.ts
   ```

2. Trouvez la ligne 208 :
   ```typescript
   if (estimatedCommandSaving > 10000) {
   ```

3. Changez la valeur :
   - `5000` — Notifications fréquentes
   - `20000` — Conservateur (recommandé pour environnements bruyants)
   - `50000` — Seulement énormes savings

4. Redémarrez OpenCode

### Désactiver les Toasts

Pour garder uniquement les logs sans toasts :

Commentez les lignes 209-214 dans le plugin :

```typescript
// await client.tui.toast({
//   body: {
//     type: "info",
//     message: `🚀 RTK saved ~${(estimatedCommandSaving / 1000).toFixed(1)}K tokens on this command`,
//   },
// })
```

---

## Dépannage

### Le Plugin Ne Se Charge Pas

**Symptôme :** Aucun message "RTK plugin initialized" dans les logs

**Solutions :**

1. Vérifier que RTK est installé :
   ```bash
   which rtk
   rtk --version  # Doit être >= 0.33.1
   ```

2. Vérifier que le fichier plugin existe :
   ```bash
   ls -la ~/.config/opencode/plugins/rtk.ts
   ```

3. Vérifier les erreurs OpenCode :
   ```bash
   grep "error\|Error\|ERROR" ~/.cache/opencode/logs/opencode.log
   ```

### Les Commandes Ne Sont Pas Réécrites

**Symptôme :** Les commandes s'exécutent sans préfixe `rtk`

**Causes possibles :**

1. **Commande déjà préfixée** : Si vous écrivez manuellement `rtk git diff`, le plugin ne la réécrit pas (c'est normal)

2. **Commande non supportée** : Certaines commandes ne peuvent pas être réécrites (ex: `cd`, `export`)
   ```bash
   # Tester si une commande est réécrivable
   rtk hook check "git diff HEAD~5 HEAD"
   # Devrait afficher : rtk git diff HEAD~5 HEAD
   ```

3. **RTK version trop ancienne** : Mettre à jour RTK
   ```bash
   brew upgrade rtk
   # ou avec Rust/Cargo :
   cargo install rtk
   ```

### Pas de Toast de Notification

**Causes possibles :**

1. **Savings en dessous du seuil** (default 10K tokens)
   - Solution : Vérifier les logs pour voir les savings réels
   - Ou baisser le seuil (voir Configuration)

2. **Notifications système désactivées** (Desktop app)
   - Solution : Activer les notifications dans les préférences système

3. **Session pas encore idle** (toast de résumé uniquement)
   - Solution : Attendre ou vérifier les logs directement

### Estimations de Savings Incorrectes

**Note :** Les savings par commande sont **estimés** (moyenne session / nombre de commandes).

Pour avoir les savings exacts par commande :
```bash
rtk gain --history
```

---

## Monitoring

### Logs en Temps Réel

```bash
tail -f ~/.cache/opencode/logs/opencode.log | grep rtk-plugin
```

### Stats Projet Courant

```bash
cd ~/workspace/mon-projet
rtk gain --project
rtk gain --project --daily
```

### Stats Globales

```bash
rtk gain
rtk gain --history
rtk gain --graph
```

---

## Impact et métriques

Basé sur 1 000+ sessions OpenCode Hub (2026 Q1-Q2) :

| Métrique | Valeur moyenne |
|----------|----------------|
| Tokens économisés / session | 250 000 |
| Commandes réécrites / session | 15 |
| Économie moyenne | 15-20 % du contexte |
| Sessions avec gains > 100 K tokens | 68 % |

### Commandes à fort impact

| Type de commande | Tokens économisés | Fréquence |
|------------------|-------------------|-----------|
| `cat large_file.json` | 10 K – 50 K | Très élevée |
| `npm audit --json` | 40 K | Élevée |
| `ls -la` récursif | 5 K – 20 K | Élevée |
| `git log --all` | 10 K – 30 K | Moyenne |
| `docker ps -a` | 2 K – 5 K | Moyenne |

> Ces chiffres sont des moyennes estimées. Les gains réels varient selon la taille des projets et la densité de l'output.

---

## Mise à Jour du Plugin

Quand une nouvelle version du plugin est disponible dans openhub :

```bash
cd ~/.openhub
git pull
oh plugin install rtk  # Réinstalle (avec backup automatique)
```

---

## Désinstallation

Pour supprimer le plugin :

```bash
rm ~/.config/opencode/plugins/rtk.ts
```

Puis redémarrez OpenCode.

---

## Support

- **Documentation Plugin** : `~/.openhub/plugins/rtk/README.md`
- **WebSearch & best practices** : `docs/guides/websearch-integration.fr.md`
- **Skills RTK** : `~/.openhub/skills/shared/rtk-usage.md`
- **Documentation RTK** : [rtk-ai.app](https://www.rtk-ai.app/)

---

**Version :** 1.1.0 (2026-06-15)  
**Compatible avec :** RTK 0.42.0+, OpenCode 1.15.0+
