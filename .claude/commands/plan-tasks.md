---
description: Planifie une issue en déléguant aux sous-agents spécialisés, puis produit .agent/tasks.json pour les agents DeepSeek
argument-hint: <description de l'issue ou numéro>
---

Issue à planifier : $ARGUMENTS

## 0. Préparation
- Lis `CLAUDE.md`. S'il contient encore `<à compléter>`, complète-le d'après le repo (stack, commandes build/test/lint, conventions, architecture) avant de continuer.
- Crée le dossier `.agent/` et ajoute `.agent/` au `.gitignore` s'il n'y est pas déjà.
- Repère les sous-agents installés : fichiers de `.claude/agents/`.

## 1. Analyse avec les sous-agents
Délègue l'analyse aux sous-agents installés dont la spécialité correspond à l'issue. N'appelle pas tout le monde : seulement ceux qui apportent quelque chose.

| Sujet de l'issue | Sous-agent |
|---|---|
| API, base de données, architecture serveur | backend-architect |
| Interface, composants, Angular / React | frontend-developer (+ ui-designer pour l'UX) |
| CI/CD, Docker, déploiement, IaC | devops-automator |
| IA / LLM / ML | ai-engineer |
| Application mobile | mobile-app-builder |
| MVP, prototype rapide | rapid-prototyper |
| Tests d'API | api-tester |
| Performance, charge | performance-benchmarker |
| Lecture de résultats de tests | test-results-analyzer |
| Conformité, RGPD, licences | legal-compliance-checker |
| Priorisation, découpage en sprints | sprint-prioritizer |

Les agents marketing, finance, support et analytics ne servent pas au workflow de code : ignore-les sauf demande explicite.

Demande à chaque sous-agent un avis court (décisions, risques, pièges). Synthétise le tout dans `.agent/plan.md`.

## 2. Produis `.agent/tasks.json`
Format exact :

{
  "tasks": [
    {
      "id": 1,
      "title": "titre court à l'impératif",
      "role": "backend-architect",
      "files": ["chemins des fichiers à lire/modifier (1 à 3 max)"],
      "acceptance": "critère vérifiable par un test précis",
      "depends_on": []
    }
  ]
}

Règles :
- `role` = nom (slug) d'un sous-agent installé dont l'expertise correspond à CETTE tâche. DeepSeek recevra ce profil comme guide pour coder la tâche. Omets `role` si aucun ne convient.
- Chaque tâche touche 1 à 3 fichiers et est vérifiable par un test automatisé
- Inclure les tests dans la tâche (ou une tâche dédiée juste après)
- Les tâches doivent être indépendantes les unes des autres (les branches ne sont pas chaînées)
- Pas de tâche floue du type « améliorer X »
- Ne PAS écrire le code d'implémentation : seuls `.agent/plan.md` et `.agent/tasks.json` sont produits
