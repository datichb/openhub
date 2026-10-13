# Onboarding Atlas

Atlas est une plateforme de vente B2B. Ce guide donne le minimum pour contribuer dès le premier jour.

## Démarrer en local

- Installer Go 1.24, Node 22 et Docker ; lancer make bootstrap à la racine.
- make dev démarre la base, la file d'événements, l'API sur le port 8080 et le frontend sur le port 5173.
- Les données de démonstration se chargent avec make seed ; le compte admin est admin@atlas.test.
- La variable ATLAS_ENV vaut local, staging ou production ; elle choisit le fichier de configuration.

## Carte du dépôt

- api/ : le backend Go (cmd/, internal/, db/migrations).
- web/ : le frontend React (src/pages, src/components, src/hooks, src/api).
- worker/ : les consommateurs d'événements (facturation, notifications, exports).
- infra/ : Terraform des environnements, charts Helm, tableaux de bord.
- docs/ : ADR, guides, wiki documentaire vivant.

## Domaines métier

- Comptes : inscription, authentification, rôles, équipes.
- Catalogue : produits, variantes, tarifs, disponibilité.
- Commandes : panier, paiement, remboursement, factures.
- Exports : rapports comptables mensuels, exports CSV à la demande.

## Points d'attention

- Le module billing/ calcule la TVA par pays : toute modification passe par l'équipe finance.
- Le worker d'exports est sensible à la mémoire : les exports sont traités par lots de 1 000 lignes.
- La file d'événements garantit une livraison au moins une fois : les consommateurs sont idempotents.
- Le frontend cible les deux dernières versions des navigateurs majeurs et Safari iOS 17.
