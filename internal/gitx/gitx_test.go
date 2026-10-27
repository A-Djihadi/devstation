package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
)

func TestConfigureCommands(t *testing.T) {
	f := &execx.Fake{}
	if err := Configure(context.Background(), f, "Alice", "alice@example.com", "true"); err != nil {
		t.Fatalf("Configure : %v", err)
	}
	if len(f.Calls) != 3 {
		t.Fatalf("appels = %d, attendu 3", len(f.Calls))
	}
	want := [][]string{
		{"config", "--global", "user.name", "Alice"},
		{"config", "--global", "user.email", "alice@example.com"},
		{"config", "--global", "core.autocrlf", "true"},
	}
	for i, w := range want {
		if f.Calls[i].Name != "git" {
			t.Errorf("appel %d : Name = %q", i, f.Calls[i].Name)
		}
		if strings.Join(f.Calls[i].Args, " ") != strings.Join(w, " ") {
			t.Errorf("appel %d : Args = %v, attendu %v", i, f.Calls[i].Args, w)
		}
	}
}

func TestEnsureSSHKeyKeepsExisting(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(key, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &execx.Fake{}
	pub, created, err := EnsureSSHKey(context.Background(), f, home, "alice@example.com")
	if err != nil {
		t.Fatalf("EnsureSSHKey : %v", err)
	}
	if created {
		t.Error("created = true, attendu false")
	}
	if pub != key+".pub" {
		t.Errorf("pubPath = %q, attendu %q", pub, key+".pub")
	}
	if len(f.Calls) != 0 {
		t.Errorf("appels = %d, attendu 0", len(f.Calls))
	}
}

func TestEnsureSSHKeyGenerates(t *testing.T) {
	home := t.TempDir()
	f := &execx.Fake{}
	pub, created, err := EnsureSSHKey(context.Background(), f, home, "alice@example.com")
	if err != nil {
		t.Fatalf("EnsureSSHKey : %v", err)
	}
	if !created {
		t.Error("created = false, attendu true")
	}
	key := filepath.Join(home, ".ssh", "id_ed25519")
	if pub != key+".pub" {
		t.Errorf("pubPath = %q, attendu %q", pub, key+".pub")
	}
	if len(f.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(f.Calls))
	}
	c := f.Calls[0]
	if c.Name != "ssh-keygen" {
		t.Errorf("Name = %q", c.Name)
	}
	want := []string{"-t", "ed25519", "-C", "alice@example.com", "-f", key, "-N", ""}
	if len(c.Args) != len(want) {
		t.Fatalf("Args = %v, attendu %v", c.Args, want)
	}
	for i := range want {
		if c.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, attendu %q", i, c.Args[i], want[i])
		}
	}
}

func TestStoreTokenIsSecret(t *testing.T) {
	f := &execx.Fake{}
	const token = "ghp_secretvalue"
	if err := StoreToken(context.Background(), f, token); err != nil {
		t.Fatalf("StoreToken : %v", err)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(f.Calls))
	}
	c := f.Calls[0]
	if !c.Secret {
		t.Error("Secret = false, attendu true")
	}
	for _, a := range c.Args {
		if strings.Contains(a, token) {
			t.Errorf("le jeton apparaît dans les arguments : %q", a)
		}
	}
	wantStdin := "protocol=https\nhost=github.com\nusername=x-access-token\npassword=" + token + "\n\n"
	if c.Stdin != wantStdin {
		t.Errorf("Stdin = %q, attendu %q", c.Stdin, wantStdin)
	}
}

func TestStoreTokenEmpty(t *testing.T) {
	f := &execx.Fake{}
	if err := StoreToken(context.Background(), f, ""); err != nil {
		t.Fatalf("StoreToken : %v", err)
	}
	if len(f.Calls) != 0 {
		t.Errorf("appels = %d, attendu 0", len(f.Calls))
	}
}

func TestCloneSkipsExistingDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fichier.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &execx.Fake{}
	cloned, err := Clone(context.Background(), f, "https://example.com/repo.git", dir)
	if err != nil {
		t.Fatalf("Clone : %v", err)
	}
	if cloned {
		t.Error("cloned = true, attendu false")
	}
	if len(f.Calls) != 0 {
		t.Errorf("appels = %d, attendu 0", len(f.Calls))
	}
}

func TestCloneRuns(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	f := &execx.Fake{}
	cloned, err := Clone(context.Background(), f, "https://example.com/repo.git", dir)
	if err != nil {
		t.Fatalf("Clone : %v", err)
	}
	if !cloned {
		t.Error("cloned = false, attendu true")
	}
	if len(f.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(f.Calls))
	}
	c := f.Calls[0]
	if c.Name != "git" {
		t.Errorf("Name = %q", c.Name)
	}
	want := []string{"clone", "https://example.com/repo.git", dir}
	if strings.Join(c.Args, " ") != strings.Join(want, " ") {
		t.Errorf("Args = %v, attendu %v", c.Args, want)
	}
}
