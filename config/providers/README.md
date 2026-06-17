# Providers opencode

| Provider | Auth | Statut |
|----------|------|--------|
| mammouth | Clé API (litellm) | ✅ Prêt |
| copilot | OAuth GitHub (built-in) | ✅ Prêt |
| openrouter | Clé API | ⚠️ Clé à configurer |
| ollama | Aucune (local) | ⚠️ Modèle à configurer |
| bedrock | AWS credentials | ⚠️ ~/.aws/credentials requis |

## Usage

```
ocp mammouth openhub        # lancer avec mammouth
ocp bedrock openhub --dev   # mode dev avec bedrock
ocp --list                       # lister les providers
ocp                              # picker interactif
```

## Actions manuelles requises

- **openrouter** : éditer `config/providers/openrouter.json` et remplacer `sk-or-v1-REPLACE_ME` par ta vraie clé API
- **ollama** : éditer `config/providers/ollama.json` et remplacer `REPLACE_ME` par le nom du modèle (ex: `qwen2.5-coder:7b`)
- **bedrock** : configurer `~/.aws/credentials` ou `export AWS_PROFILE=<profil>`

## Notes

**Deep-merge** : chaque provider dispose de son propre `opencode.json` — la config globale `~/.config/opencode/opencode.jsonc` (skills, agents) est conservée car opencode applique un deep-merge.

**Redémarrage** : opencode ne hot-reload pas la config — redémarrer après chaque switch.
