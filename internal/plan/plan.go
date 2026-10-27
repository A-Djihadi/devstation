// Package plan fournit un exécuteur d'étapes ordonnancées par dépendances.
package plan

import (
	"context"
	"fmt"
)

// Status représente l'état d'une étape pendant ou après l'exécution.
type Status int

const (
	// Pending indique une étape pas encore traitée.
	Pending Status = iota
	// Running indique une étape en cours d'exécution.
	Running
	// Done indique une étape exécutée avec succès.
	Done
	// Skipped indique une étape déjà satisfaite (Check vrai), Run non appelé.
	Skipped
	// Failed indique une étape dont l'exécution a échoué.
	Failed
	// Blocked indique une étape non exécutée car une dépendance a échoué.
	Blocked
)

// String renvoie le nom lisible du statut.
func (s Status) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Running:
		return "Running"
	case Done:
		return "Done"
	case Skipped:
		return "Skipped"
	case Failed:
		return "Failed"
	case Blocked:
		return "Blocked"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// Step décrit une étape du plan.
type Step struct {
	// ID identifie l'étape de façon unique.
	ID string
	// Title est le libellé affiché à l'utilisateur.
	Title string
	// DependsOn liste les IDs des étapes préalables.
	DependsOn []string
	// Check indique si l'étape est déjà satisfaite (vrai ⇒ Skipped).
	Check func(ctx context.Context) (bool, error)
	// Run exécute l'étape et peut rapporter une progression.
	Run func(ctx context.Context, report func(msg string, done, total int64)) error
}

// Event décrit un changement d'état d'une étape.
type Event struct {
	// StepID identifie l'étape concernée.
	StepID string
	// Status est le nouvel état de l'étape.
	Status Status
	// Msg est un message optionnel de progression.
	Msg string
	// Done est la quantité réalisée (progression).
	Done int64
	// Total est la quantité totale (progression).
	Total int64
	// Err est l'erreur éventuelle associée à l'événement.
	Err error
}

// Summary résume le résultat d'une exécution.
type Summary struct {
	// Done liste les étapes exécutées avec succès.
	Done []string
	// Skipped liste les étapes déjà satisfaites.
	Skipped []string
	// Failed liste les étapes en échec.
	Failed []string
	// Blocked liste les étapes bloquées par une dépendance en échec.
	Blocked []string
	// Errors associe à chaque étape en échec son erreur.
	Errors map[string]error
}

// Execute exécute les étapes dans un ordre topologique stable (ordre de
// déclaration). Une erreur n'est renvoyée que si le graphe est invalide
// (dépendance inconnue ou cycle). Un échec d'étape bloque uniquement ses
// descendants ; les autres étapes continuent.
func Execute(ctx context.Context, steps []Step, onEvent func(Event)) (Summary, error) {
	summary := Summary{Errors: map[string]error{}}
	emit := func(ev Event) {
		if onEvent != nil {
			onEvent(ev)
		}
	}

	order, err := topoSort(steps)
	if err != nil {
		return summary, err
	}

	byID := make(map[string]Step, len(steps))
	for _, s := range steps {
		byID[s.ID] = s
	}

	// failedOrBlocked mémorise les étapes dont les descendants sont bloqués.
	blocked := make(map[string]bool, len(steps))

	for _, id := range order {
		step := byID[id]

		if ctx.Err() != nil {
			blocked[id] = true
			summary.Blocked = append(summary.Blocked, id)
			emit(Event{StepID: id, Status: Blocked, Err: ctx.Err()})
			continue
		}

		depFailed := false
		for _, dep := range step.DependsOn {
			if blocked[dep] {
				depFailed = true
				break
			}
		}
		if depFailed {
			blocked[id] = true
			summary.Blocked = append(summary.Blocked, id)
			emit(Event{StepID: id, Status: Blocked})
			continue
		}

		if step.Check != nil {
			ok, cerr := step.Check(ctx)
			if cerr != nil {
				blocked[id] = true
				summary.Failed = append(summary.Failed, id)
				summary.Errors[id] = cerr
				emit(Event{StepID: id, Status: Failed, Err: cerr})
				continue
			}
			if ok {
				summary.Skipped = append(summary.Skipped, id)
				emit(Event{StepID: id, Status: Skipped})
				continue
			}
		}

		emit(Event{StepID: id, Status: Running})
		var runErr error
		if step.Run != nil {
			report := func(msg string, done, total int64) {
				emit(Event{StepID: id, Status: Running, Msg: msg, Done: done, Total: total})
			}
			runErr = step.Run(ctx, report)
		}
		if runErr != nil {
			blocked[id] = true
			summary.Failed = append(summary.Failed, id)
			summary.Errors[id] = runErr
			emit(Event{StepID: id, Status: Failed, Err: runErr})
			continue
		}
		summary.Done = append(summary.Done, id)
		emit(Event{StepID: id, Status: Done})
	}

	return summary, nil
}

// topoSort renvoie les IDs dans un ordre topologique stable (ordre de
// déclaration) et détecte les dépendances inconnues et les cycles.
func topoSort(steps []Step) ([]string, error) {
	index := make(map[string]int, len(steps))
	for i, s := range steps {
		if _, dup := index[s.ID]; dup {
			return nil, fmt.Errorf("étape dupliquée : %q", s.ID)
		}
		index[s.ID] = i
	}

	// Vérifie les dépendances inconnues.
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := index[dep]; !ok {
				return nil, fmt.Errorf("étape %q : dépendance inconnue %q", s.ID, dep)
			}
		}
	}

	// Tri topologique stable : à chaque tour, on émet l'étape de plus petit
	// indice de déclaration dont toutes les dépendances sont déjà émises.
	emitted := make(map[string]bool, len(steps))
	order := make([]string, 0, len(steps))
	for len(order) < len(steps) {
		progressed := false
		for _, s := range steps {
			if emitted[s.ID] {
				continue
			}
			ready := true
			for _, dep := range s.DependsOn {
				if !emitted[dep] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			emitted[s.ID] = true
			order = append(order, s.ID)
			progressed = true
			break
		}
		if !progressed {
			return nil, fmt.Errorf("cycle détecté dans les dépendances des étapes")
		}
	}
	return order, nil
}