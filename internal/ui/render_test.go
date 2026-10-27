package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/plan"
)

func TestNoColorEnv(t *testing.T) {
	getenv := func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	}
	if ColorEnabled(true, getenv) {
		t.Fatal("NO_COLOR défini : couleur attendue désactivée")
	}
	if ColorEnabled(false, func(string) string { return "" }) {
		t.Fatal("pas de TTY : couleur attendue désactivée")
	}
	if !ColorEnabled(true, func(string) string { return "" }) {
		t.Fatal("TTY sans NO_COLOR : couleur attendue activée")
	}
}

func TestStepLineSymbolsPlain(t *testing.T) {
	th := NewTheme(false)
	got := th.StepLine(plan.Done, "Java 21", "")
	if got != "✓ Java 21" {
		t.Fatalf("StepLine = %q, attendu %q", got, "✓ Java 21")
	}
	if s := th.StepLine(plan.Skipped, "Git", ""); !strings.HasPrefix(s, "↷ ") {
		t.Fatalf("Skipped = %q", s)
	}
	if s := th.StepLine(plan.Failed, "Maven", ""); !strings.HasPrefix(s, "✗ ") {
		t.Fatalf("Failed = %q", s)
	}
	if s := th.StepLine(plan.Blocked, "Node", ""); !strings.HasPrefix(s, "⊘ ") {
		t.Fatalf("Blocked = %q", s)
	}
}

func TestSummaryListsFailuresAndLog(t *testing.T) {
	th := NewTheme(false)
	sum := plan.Summary{
		Done:    []string{"java"},
		Failed:  []string{"maven"},
		Errors:  map[string]error{"maven": errors.New("téléchargement échoué")},
		Blocked: []string{"node"},
	}
	out := th.Summary(sum, `C:\logs\20261027-120000.log`)
	if !strings.Contains(out, "maven") {
		t.Fatalf("résumé sans l'échec : %q", out)
	}
	if !strings.Contains(out, "téléchargement échoué") {
		t.Fatalf("résumé sans le message d'erreur : %q", out)
	}
	if !strings.Contains(out, `C:\logs\20261027-120000.log`) {
		t.Fatalf("résumé sans le chemin du journal : %q", out)
	}
}
