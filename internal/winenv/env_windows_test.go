//go:build windows

package winenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
)

// TestUserVarRoundTrip vérifie la pose, la lecture et la suppression d'une variable utilisateur.
func TestUserVarRoundTrip(t *testing.T) {
	env := NewEnv()
	const name = "DEVSTATION_TEST_VAR"
	const value = `C:\Users\Jean Émile\x`

	if err := env.SetUserVar(name, value); err != nil {
		t.Fatalf("SetUserVar : %v", err)
	}
	t.Cleanup(func() { _ = env.DeleteUserVar(name) })

	got, ok, err := env.UserVar(name)
	if err != nil {
		t.Fatalf("UserVar : %v", err)
	}
	if !ok {
		t.Fatalf("UserVar : variable absente après écriture")
	}
	if got != value {
		t.Fatalf("UserVar = %q, attendu %q", got, value)
	}

	if err := env.DeleteUserVar(name); err != nil {
		t.Fatalf("DeleteUserVar : %v", err)
	}
	if _, ok, err := env.UserVar(name); err != nil {
		t.Fatalf("UserVar après suppression : %v", err)
	} else if ok {
		t.Fatalf("UserVar : variable toujours présente après suppression")
	}
}

// TestMachinePathReadable vérifie que le PATH machine est lisible et non vide.
func TestMachinePathReadable(t *testing.T) {
	env := NewEnv()
	p, err := env.MachinePath()
	if err != nil {
		t.Fatalf("MachinePath : %v", err)
	}
	if p == "" {
		t.Fatalf("MachinePath : valeur vide")
	}
}

// TestJunctionRoundTrip vérifie la création et la lecture d'une jonction NTFS.
func TestJunctionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll : %v", err)
	}
	link := filepath.Join(dir, "link")

	links := NewLinks(execx.OS())
	if err := links.SetJunction(link, target); err != nil {
		t.Fatalf("SetJunction : %v", err)
	}

	got, err := links.ReadJunction(link)
	if err != nil {
		t.Fatalf("ReadJunction : %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(target) {
		t.Fatalf("ReadJunction = %q, attendu %q", got, target)
	}
}
