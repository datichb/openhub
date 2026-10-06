Mode de workflow : manuel
Langue de réponse : fr

Lance l'onboarding de ce projet.

Projet : demo
Chemin : /src/demo

Tu n'écris que dans `docs/wiki/`, plus le `ONBOARDING.md` minimaliste de la racine (phase 5) : aucun autre fichier du projet n'est modifié.

Crée le wiki documentaire vivant du projet dans `docs/wiki/` (s'il existe déjà, enrichis-le selon les règles du mode refresh : ne supprime aucune page).

Le wiki doit couvrir :
- Architecture globale (modules, couches, patterns)
- Stack technique (langages, frameworks, dépendances clés)
- Conventions de code (nommage, structure fichiers, patterns récurrents)
- Points d'entrée (comment build, test, run)
- Décisions d'architecture (ADR si présents)
- Dépendances externes et intégrations

Format : protocole `doc-wiki-protocol` (frontmatter YAML, tags de confiance, structure par heading).
Emplacement : `docs/wiki/` (un fichier `.md` par sujet majeur + un `index.md`).
