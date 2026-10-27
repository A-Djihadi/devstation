package ui

import (
	"fmt"
	"strings"

	"github.com/A-Djihadi/devstation/internal/plan"
)

// StepLine rend la ligne d'une étape : symbole, titre et détail optionnel.
func (t Theme) StepLine(st plan.Status, title, detail string) string {
	var sym string
	switch st {
	case plan.Done:
		sym = t.SymbolDone
	case plan.Skipped:
		sym = t.SymbolSkipped
	case plan.Failed:
		sym = t.SymbolFailed
	case plan.Blocked:
		sym = t.SymbolBlocked
	default:
		sym = t.SymbolPending
	}
	line := sym + " " + title
	if detail != "" {
		line += " " + detail
	}
	return line
}

// Banner rend l'en-tête de l'application avec la version fournie.
func (t Theme) Banner(version string) string {
	return fmt.Sprintf("DevStation %s", version)
}

// Summary rend le récapitulatif d'une exécution et le chemin du journal.
func (t Theme) Summary(s plan.Summary, logPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Terminé : %d réussie(s), %d ignorée(s), %d échec(s), %d bloquée(s)",
		len(s.Done), len(s.Skipped), len(s.Failed), len(s.Blocked))
	if len(s.Failed) > 0 {
		b.WriteString("\nÉchecs :")
		for _, id := range s.Failed {
			b.WriteString("\n  " + t.SymbolFailed + " " + id)
			if err, ok := s.Errors[id]; ok && err != nil {
				b.WriteString(" : " + err.Error())
			}
		}
	}
	if logPath != "" {
		b.WriteString("\nJournal : " + logPath)
	}
	return b.String()
}
