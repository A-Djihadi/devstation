package execx

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

// TestOSRunnerEcho vérifie que le Runner réel exécute une commande simple.
func TestOSRunnerEcho(t *testing.T) {
	ctx := context.Background()
	var c Cmd
	if runtime.GOOS == "windows" {
		c = Cmd{Name: "cmd", Args: []string{"/c", "echo ok"}}
	} else {
		c = Cmd{Name: "sh", Args: []string{"-c", "echo ok"}}
	}
	res, err := OS().Run(ctx, c)
	if err != nil {
		t.Fatalf("Run a échoué : %v", err)
	}
	if res.Code != 0 {
		t.Fatalf("code de sortie attendu 0, obtenu %d", res.Code)
	}
	if !strings.Contains(res.Stdout, "ok") {
		t.Fatalf("stdout attendu contenant %q, obtenu %q", "ok", res.Stdout)
	}
}

// TestStringQuotesSpaces vérifie que les arguments contenant des espaces sont entre guillemets.
func TestStringQuotesSpaces(t *testing.T) {
	c := Cmd{Name: "git", Args: []string{"commit", "-m", "message avec espaces"}}
	got := c.String()
	want := `git commit -m "message avec espaces"`
	if got != want {
		t.Fatalf("String() = %q, attendu %q", got, want)
	}
}

// TestStringMasksSecret vérifie que les arguments sont masqués pour une commande secrète.
func TestStringMasksSecret(t *testing.T) {
	c := Cmd{Name: "vault", Args: []string{"login", "-token=abc"}, Secret: true}
	got := c.String()
	if strings.Contains(got, "abc") {
		t.Fatalf("String() ne doit pas exposer le secret : %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("String() doit contenir *** : %q", got)
	}
}

// TestFakeRecordsCalls vérifie que Fake enregistre les appels et renvoie les réponses.
func TestFakeRecordsCalls(t *testing.T) {
	f := &Fake{Responses: map[string]Result{
		"git status": {Stdout: "clean", Code: 0},
	}}
	ctx := context.Background()
	res, err := f.Run(ctx, Cmd{Name: "git", Args: []string{"status"}})
	if err != nil {
		t.Fatalf("Run a échoué : %v", err)
	}
	if res.Stdout != "clean" {
		t.Fatalf("stdout attendu %q, obtenu %q", "clean", res.Stdout)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("1 appel attendu, obtenu %d", len(f.Calls))
	}
	if f.Calls[0].Name != "git" || len(f.Calls[0].Args) != 1 || f.Calls[0].Args[0] != "status" {
		t.Fatalf("appel enregistré incorrect : %+v", f.Calls[0])
	}
}