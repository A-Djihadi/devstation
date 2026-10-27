# Conventions du projet DevStation (lu par Claude Code ET par les agents DeepSeek)

Spec : `docs/superpowers/specs/2026-10-27-devstation-design.md`
Plan : `docs/superpowers/plans/2026-10-27-devstation.md` (signatures et tests attendus par tâche)

## Stack
- Go 1.27 (`go.mod`), CLI cobra, interface charmbracelet (huh, lipgloss, bubbles, bubbletea), YAML `gopkg.in/yaml.v3`
- Windows : `golang.org/x/sys/windows`, `github.com/danieljoos/wincred`
- Démarrage : PowerShell 5.1 (`setup.ps1`, `bootstrap/`), tests Pester 5 dans `tests/ps/`

## Commandes
- Tests (sandbox et CI) : `sh scripts/test.sh` — go vet, go test, vet/build `GOOS=windows`, Pester
- Pas de lint séparé : `go vet` + `gofmt` (le code doit être formaté par gofmt)

## Règles
- Le sandbox de test est Linux et SANS réseau : aucun test ne fait de vraie requête (utiliser `httptest`),
  aucune nouvelle dépendance hors `go.mod` (elle ne pourrait pas être téléchargée)
- Code propre à Windows : fichiers `*_windows.go`, pendant `*_other.go` avec `//go:build !windows`
- Logique testable via les interfaces de `internal/winenv` et `internal/execx` et leurs `Fake`
- Aucune action admin : HKCU uniquement, jamais HKLM ni PATH machine
- Jamais de secret dans le code, les logs, `answers.yaml` ou `state.json`
- Exemples : domaines fictifs `*.example.com` uniquement
- Messages affichés à l'utilisateur en français ; identifiants Go en anglais
- Commentaires de doc Go sur tout identifiant exporté
- Modifications minimales : ne pas reformater du code non concerné
- Un commit = une tâche ; message au format `agent: <titre de la tâche>`
