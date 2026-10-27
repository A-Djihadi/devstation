package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
)

func TestStepsTitles(t *testing.T) {
	steps := Steps()
	want := []string{
		"Profil", "Rôle", "Éditeur", "Versions", "Réseau",
		"Git", "Dépôts", "Conteneurs", "Récapitulatif",
	}
	if len(steps) != len(want) {
		t.Fatalf("Steps() = %d titres, attendu %d", len(steps), len(want))
	}
	for i, w := range want {
		if steps[i] != w {
			t.Errorf("Steps()[%d] = %q, attendu %q", i, steps[i], w)
		}
	}
}

func TestVersionOptionsLabels(t *testing.T) {
	rs := []sources.Release{
		{Version: "21.0.12", LTS: true},
		{Version: "22.0.1"},
	}
	opts := wizVersionOptions(rs)
	if len(opts) != 2 {
		t.Fatalf("wizVersionOptions = %d options, attendu 2", len(opts))
	}
	if opts[0].Key != "21.0.12 (LTS)" {
		t.Errorf("libellé LTS = %q, attendu %q", opts[0].Key, "21.0.12 (LTS)")
	}
	if opts[0].Value != "21.0.12" {
		t.Errorf("valeur LTS = %q, attendu %q", opts[0].Value, "21.0.12")
	}
	if opts[1].Key != "22.0.1" {
		t.Errorf("libellé non LTS = %q, attendu %q", opts[1].Key, "22.0.1")
	}
}

func TestSummaryMentionsChoices(t *testing.T) {
	a := &profile.Answers{
		Profile:   "demo",
		Roles:     []string{"back", "front"},
		Editor:    "vscode",
		Versions:  map[string]string{"java": "21.0.12", "node": "22.0.1"},
		GitName:   "Alice",
		GitEmail:  "alice@example.com",
		Repos:     []string{"api"},
		Workspace: `C:\Users\alice\workspace\demo`,
		Podman:    true,
	}
	s := wizSummary(a)
	for _, want := range []string{"demo", "back", "front", "vscode", "21.0.12", "22.0.1", "Alice", "api", "Podman"} {
		if !strings.Contains(s, want) {
			t.Errorf("wizSummary ne mentionne pas %q :\n%s", want, s)
		}
	}
}

func TestToolsForRoles(t *testing.T) {
	cases := []struct {
		roles []string
		want  []string
	}{
		{[]string{"back"}, []string{"java", "maven"}},
		{[]string{"front"}, []string{"node", "angular_cli"}},
		{[]string{"python"}, []string{"python"}},
		{[]string{"fullstack"}, []string{"java", "maven", "node", "angular_cli"}},
	}
	for _, c := range cases {
		got := wizToolsFor(c.roles)
		if len(got) != len(c.want) {
			t.Fatalf("wizToolsFor(%v) = %v, attendu %v", c.roles, got, c.want)
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("wizToolsFor(%v)[%d] = %q, attendu %q", c.roles, i, got[i], c.want[i])
			}
		}
	}
}

func TestNotConfirmedIsCancelled(t *testing.T) {
	if err := wizConfirmed(false); !errors.Is(err, ErrCancelled) {
		t.Errorf("wizConfirmed(false) = %v, attendu ErrCancelled", err)
	}
	if err := wizConfirmed(true); err != nil {
		t.Errorf("wizConfirmed(true) = %v, attendu nil", err)
	}
}

func TestAskTokenSignature(t *testing.T) {
	var _ func(WizardIO) (string, error) = AskToken
}
