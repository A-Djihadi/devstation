// Package ui fournit le thème et le rendu texte de la sortie DevStation.
package ui

// Theme regroupe les symboles et styles d'affichage. Lorsque la couleur est
// désactivée, les styles sont vides et le rendu reste du texte simple.
type Theme struct {
	// Color indique si les couleurs et styles sont activés.
	Color bool
	// SymbolDone est le symbole d'une étape réussie.
	SymbolDone string
	// SymbolSkipped est le symbole d'une étape déjà satisfaite.
	SymbolSkipped string
	// SymbolFailed est le symbole d'une étape en échec.
	SymbolFailed string
	// SymbolBlocked est le symbole d'une étape bloquée.
	SymbolBlocked string
	// SymbolPending est le symbole d'une étape en attente.
	SymbolPending string
}

// NewTheme construit un thème. Si color est faux, le rendu est du texte simple
// sans séquence d'échappement.
func NewTheme(color bool) Theme {
	return Theme{
		Color:         color,
		SymbolDone:    "✓",
		SymbolSkipped: "↷",
		SymbolFailed:  "✗",
		SymbolBlocked: "⊘",
		SymbolPending: "·",
	}
}

// ColorEnabled indique si la couleur doit être utilisée : faux si la variable
// d'environnement NO_COLOR est définie ou si la sortie n'est pas un terminal.
func ColorEnabled(isTTY bool, getenv func(string) string) bool {
	if !isTTY {
		return false
	}
	if getenv != nil && getenv("NO_COLOR") != "" {
		return false
	}
	return true
}
