package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// polishHelpOutput exécute --help et retourne la sortie standard.
func polishHelpOutput(t *testing.T) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("exécution de --help : %v", err)
	}
	return buf.String()
}

// TestHelpIsFrench vérifie que l'aide est traduite en français.
func TestHelpIsFrench(t *testing.T) {
	out := polishHelpOutput(t)
	if !strings.Contains(out, "Commandes disponibles") {
		t.Errorf("aide sans « Commandes disponibles » :\n%s", out)
	}
	if !strings.Contains(out, "affiche l'aide") {
		t.Errorf("aide sans « affiche l'aide » :\n%s", out)
	}
	if strings.Contains(out, "Available Commands") {
		t.Errorf("aide encore en anglais :\n%s", out)
	}
}

// TestNoCompletionCommand vérifie l'absence de la commande completion.
func TestNoCompletionCommand(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == "completion" {
			t.Fatalf("la commande completion est encore présente")
		}
	}
}

// TestListEmptyMessage vérifie le message de list sans installation.
func TestListEmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	toolsList(&buf, nil, "")
	if buf.Len() != 0 {
		t.Fatalf("toolsList(nil) a écrit %q", buf.String())
	}
}

// TestExitCodeErrorIsSilent vérifie que errorMessage masque les exitCode.
func TestExitCodeErrorIsSilent(t *testing.T) {
	msg, code := errorMessage(exitCode(1))
	if msg != "" || code != 1 {
		t.Fatalf("errorMessage(exitCode(1)) = (%q, %d), attendu (\"\", 1)", msg, code)
	}
	msg, code = errorMessage(errors.New("boum"))
	if msg != "erreur : boum" || code != 1 {
		t.Fatalf("errorMessage(err) = (%q, %d), attendu (\"erreur : boum\", 1)", msg, code)
	}
}
