package plan

import (
	"context"
	"errors"
	"testing"
)

func TestSkipWhenCheckTrue(t *testing.T) {
	ran := false
	steps := []Step{{
		ID:    "a",
		Title: "A",
		Check: func(ctx context.Context) (bool, error) { return true, nil },
		Run: func(ctx context.Context, report func(string, int64, int64)) error {
			ran = true
			return nil
		},
	}}
	var events []Event
	sum, err := Execute(context.Background(), steps, func(ev Event) { events = append(events, ev) })
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if ran {
		t.Fatal("Run ne doit pas être appelé quand Check est vrai")
	}
	if len(sum.Skipped) != 1 || sum.Skipped[0] != "a" {
		t.Fatalf("Skipped attendu [a], obtenu %v", sum.Skipped)
	}
	if len(sum.Done) != 0 {
		t.Fatalf("Done attendu vide, obtenu %v", sum.Done)
	}
	if len(events) != 1 || events[0].Status != Skipped {
		t.Fatalf("événement Skipped attendu, obtenu %v", events)
	}
}

func TestFailureBlocksDependentsOnly(t *testing.T) {
	steps := []Step{
		{ID: "a", Run: func(ctx context.Context, report func(string, int64, int64)) error {
			return errors.New("boom")
		}},
		{ID: "b", DependsOn: []string{"a"}, Run: func(ctx context.Context, report func(string, int64, int64)) error {
			t.Fatal("b ne doit pas s'exécuter")
			return nil
		}},
		{ID: "c", DependsOn: []string{"b"}, Run: func(ctx context.Context, report func(string, int64, int64)) error {
			t.Fatal("c ne doit pas s'exécuter")
			return nil
		}},
		{ID: "d", Run: func(ctx context.Context, report func(string, int64, int64)) error {
			return nil
		}},
	}
	sum, err := Execute(context.Background(), steps, nil)
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if len(sum.Failed) != 1 || sum.Failed[0] != "a" {
		t.Fatalf("Failed attendu [a], obtenu %v", sum.Failed)
	}
	if len(sum.Blocked) != 2 || sum.Blocked[0] != "b" || sum.Blocked[1] != "c" {
		t.Fatalf("Blocked attendu [b c], obtenu %v", sum.Blocked)
	}
	if len(sum.Done) != 1 || sum.Done[0] != "d" {
		t.Fatalf("Done attendu [d], obtenu %v", sum.Done)
	}
	if sum.Errors["a"] == nil {
		t.Fatal("erreur de a attendue dans Summary.Errors")
	}
}

func TestCycleRejected(t *testing.T) {
	steps := []Step{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	if _, err := Execute(context.Background(), steps, nil); err == nil {
		t.Fatal("un cycle doit être rejeté")
	}
}

func TestUnknownDependencyRejected(t *testing.T) {
	steps := []Step{
		{ID: "a", DependsOn: []string{"inconnu"}},
	}
	if _, err := Execute(context.Background(), steps, nil); err == nil {
		t.Fatal("une dépendance inconnue doit être rejetée")
	}
}

func TestEventsOrder(t *testing.T) {
	steps := []Step{
		{ID: "a", Run: func(ctx context.Context, report func(string, int64, int64)) error {
			report("progression", 1, 2)
			return nil
		}},
		{ID: "b", DependsOn: []string{"a"}, Run: func(ctx context.Context, report func(string, int64, int64)) error {
			return nil
		}},
	}
	var events []Event
	if _, err := Execute(context.Background(), steps, func(ev Event) { events = append(events, ev) }); err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	want := []struct {
		id     string
		status Status
	}{
		{"a", Running},
		{"a", Running},
		{"a", Done},
		{"b", Running},
		{"b", Done},
	}
	if len(events) != len(want) {
		t.Fatalf("nombre d'événements attendu %d, obtenu %d : %v", len(want), len(events), events)
	}
	for i, w := range want {
		if events[i].StepID != w.id || events[i].Status != w.status {
			t.Fatalf("événement %d attendu {%s %s}, obtenu {%s %s}", i, w.id, w.status, events[i].StepID, events[i].Status)
		}
	}
	if events[1].Msg != "progression" || events[1].Done != 1 || events[1].Total != 2 {
		t.Fatalf("progression attendue (progression,1,2), obtenu (%s,%d,%d)", events[1].Msg, events[1].Done, events[1].Total)
	}
}