package execx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// fakeRunner est un Runner minimal renvoyant un résultat fixe.
type fakeRunner struct {
	res Result
	err error
}

// Run retourne le résultat et l'erreur prédéfinis.
func (f fakeRunner) Run(ctx context.Context, c Cmd) (Result, error) {
	return f.res, f.err
}

// TestLoggedWritesCommandAndFailure vérifie la journalisation d'une commande
// et de son échec.
func TestLoggedWritesCommandAndFailure(t *testing.T) {
	var lines []string
	logf := func(format string, a ...any) {
		lines = append(lines, fmt.Sprintf(format, a...))
	}
	r := Logged(fakeRunner{
		res: Result{Code: 125, Stderr: "Error: machine did not transition into running state"},
		err: errors.New("exit status 125"),
	}, logf)

	_, _ = r.Run(context.Background(), Cmd{Name: "podman", Args: []string{"machine", "start"}})

	if len(lines) != 2 {
		t.Fatalf("lignes journalisées = %d, attendu 2 : %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "podman machine start") {
		t.Errorf("première ligne = %q, attendu la commande", lines[0])
	}
	if !strings.Contains(lines[1], "125") || !strings.Contains(lines[1], "machine did not transition") {
		t.Errorf("seconde ligne = %q, attendu le code et le message", lines[1])
	}
}

// TestLoggedNeverLogsSecretOrStdin vérifie qu'aucun secret ni Stdin n'est
// journalisé.
func TestLoggedNeverLogsSecretOrStdin(t *testing.T) {
	var lines []string
	logf := func(format string, a ...any) {
		lines = append(lines, fmt.Sprintf(format, a...))
	}
	r := Logged(fakeRunner{
		res: Result{Code: 1, Stderr: "boom"},
		err: errors.New("exit status 1"),
	}, logf)

	_, _ = r.Run(context.Background(), Cmd{
		Name:   "vault",
		Args:   []string{"login", "-method=token"},
		Stdin:  "s3cr3t-token",
		Secret: true,
	})

	for _, l := range lines {
		if strings.Contains(l, "s3cr3t-token") {
			t.Errorf("ligne journalisée contient le Stdin : %q", l)
		}
		if strings.Contains(l, "-method=token") {
			t.Errorf("ligne journalisée contient les arguments secrets : %q", l)
		}
	}
}

// TestOSRunnerErrorIncludesStderr vérifie que l'erreur d'osRunner contient un
// extrait de stderr et reste une *exec.ExitError.
func TestOSRunnerErrorIncludesStderr(t *testing.T) {
	_, err := OS().Run(context.Background(), Cmd{
		Name: "sh",
		Args: []string{"-c", "echo boom >&2; exit 3"},
	})
	if err == nil {
		t.Fatal("erreur attendue")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("erreur = %q, attendu un extrait de stderr", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("errors.As vers *exec.ExitError a échoué : %v", err)
	}
}
