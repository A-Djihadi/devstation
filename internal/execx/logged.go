package execx

import (
	"context"
)

// Logged enveloppe r pour journaliser chaque commande avant exécution et son
// échec éventuel après. logf reçoit des lignes déjà masquées pour les
// commandes Secret ; Stdin et Stdout ne sont jamais journalisés. Si logf est
// nil, r est retourné tel quel.
func Logged(r Runner, logf func(format string, a ...any)) Runner {
	if logf == nil {
		return r
	}
	return loggedRunner{r: r, logf: logf}
}

// loggedRunner est l'implémentation de Logged.
type loggedRunner struct {
	r    Runner
	logf func(format string, a ...any)
}

// Run journalise la commande, l'exécute puis journalise son échec éventuel.
func (l loggedRunner) Run(ctx context.Context, c Cmd) (Result, error) {
	l.logf("$ %s", c.String())
	res, err := l.r.Run(ctx, c)
	if c.Interactive || c.Secret {
		return res, err
	}
	if err != nil || res.Code != 0 {
		extrait := StderrExtract(res.Stderr)
		if extrait == "" && err != nil {
			extrait = err.Error()
		}
		l.logf("  -> code %d : %s", res.Code, extrait)
	}
	return res, err
}
