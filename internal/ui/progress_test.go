package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/plan"
)

// progTitles retourne une table de titres pour les tests.
func progTitles() map[string]string {
	return map[string]string{
		"java": "Java",
		"node": "Node.js",
	}
}

// TestTextModeLines vérifie qu'un passage Running puis Done produit les
// lignes attendues avec le symbole de réussite.
func TestTextModeLines(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, NewTheme(false), progTitles(), false)
	p.Handle(plan.Event{StepID: "java", Status: plan.Running})
	p.Handle(plan.Event{StepID: "java", Status: plan.Done})
	out := buf.String()
	if !strings.Contains(out, "Java") {
		t.Fatalf("sortie sans titre d'étape : %q", out)
	}
	if !strings.Contains(out, "✓ Java") {
		t.Fatalf("sortie sans ligne de réussite : %q", out)
	}
}

// TestTextModeThrottlesBytes vérifie que 100 événements de progression
// produisent au plus 5 lignes de progression.
func TestTextModeThrottlesBytes(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, NewTheme(false), progTitles(), false)
	for i := 0; i <= 100; i++ {
		p.Handle(plan.Event{StepID: "node", Status: plan.Running, Done: int64(i), Total: 100})
	}
	lines := 0
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "%") {
			lines++
		}
	}
	if lines > 5 {
		t.Fatalf("trop de lignes de progression : %d", lines)
	}
}

// TestTextModeNoANSI vérifie qu'aucune séquence d'échappement ANSI n'est
// émise en mode texte.
func TestTextModeNoANSI(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, NewTheme(false), progTitles(), false)
	p.Handle(plan.Event{StepID: "java", Status: plan.Running})
	p.Handle(plan.Event{StepID: "java", Status: plan.Done})
	p.Handle(plan.Event{StepID: "node", Status: plan.Running, Done: 50, Total: 100})
	p.Close(plan.Summary{}, "")
	if strings.Contains(buf.String(), "\x1b") {
		t.Fatalf("séquence ANSI détectée : %q", buf.String())
	}
}

// TestCloseWritesSummary vérifie que Close écrit le récapitulatif.
func TestCloseWritesSummary(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, NewTheme(false), progTitles(), false)
	s := plan.Summary{Done: []string{"java"}}
	p.Close(s, "journal.log")
	out := buf.String()
	if !strings.Contains(out, "Terminé") {
		t.Fatalf("récapitulatif absent : %q", out)
	}
	if !strings.Contains(out, "journal.log") {
		t.Fatalf("chemin du journal absent : %q", out)
	}
}
