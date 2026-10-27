package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// fakeInstaller est un installeur factice pour les tests.
type fakeInstaller struct {
	tool    string
	version string
	err     error
}

func (f fakeInstaller) Tool() string { return f.tool }

func (f fakeInstaller) Install(ctx context.Context, d install.Deps, r sources.Release, report install.Report) error {
	return nil
}

func (f fakeInstaller) Activate(d install.Deps, version string) error { return nil }

func (f fakeInstaller) Verify(ctx context.Context, d install.Deps) (string, error) {
	return f.version, f.err
}

// baseOptions construit des options minimales avec un Fake et un dossier racine.
func baseOptions(t *testing.T) Options {
	t.Helper()
	fake := winenv.NewFake()
	return Options{
		Deps: install.Deps{
			Env:   fake,
			Run:   &execx.Fake{},
			Root:  t.TempDir(),
			State: nil,
		},
		Answers: &profile.Answers{Versions: map[string]string{}},
		Profile: &profile.Profile{},
	}
}

// findCheck retourne la vérification dont le nom contient substr.
func findCheck(checks []Check, substr string) (Check, bool) {
	for _, c := range checks {
		if contains(c.Name, substr) {
			return c, true
		}
	}
	return Check{}, false
}

// contains indique si s contient substr.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

// indexOf retourne l'index de substr dans s, ou -1.
func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestAllGreen(t *testing.T) {
	o := baseOptions(t)
	o.Answers.Versions["java"] = "21"
	o.Installers = map[string]install.Installer{
		"java": fakeInstaller{tool: "java", version: "21.0.12"},
	}
	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "java")
	if !ok {
		t.Fatalf("vérification java absente : %+v", checks)
	}
	if c.Status != OK {
		t.Fatalf("statut = %s, attendu ok (%+v)", c.Status, c)
	}
	if c.Detail != "21.0.12" {
		t.Fatalf("détail = %q", c.Detail)
	}
}

func TestVersionMismatchWarns(t *testing.T) {
	o := baseOptions(t)
	o.Answers.Versions["java"] = "21"
	o.Installers = map[string]install.Installer{
		"java": fakeInstaller{tool: "java", version: "17.0.9"},
	}
	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "java")
	if !ok {
		t.Fatalf("vérification java absente")
	}
	if c.Status != Warn {
		t.Fatalf("statut = %s, attendu warn", c.Status)
	}
	if c.Fix != "devstation use java 21" {
		t.Fatalf("fix = %q", c.Fix)
	}
}

func TestShadowingWarns(t *testing.T) {
	o := baseOptions(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "java.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := o.Deps.Env.(*winenv.Fake)
	fake.Machine = dir

	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "java.exe")
	if !ok {
		t.Fatalf("vérification de masquage java.exe absente : %+v", checks)
	}
	if c.Status != Warn {
		t.Fatalf("statut = %s, attendu warn", c.Status)
	}
	if c.Detail != dir {
		t.Fatalf("détail = %q, attendu %q", c.Detail, dir)
	}
}

func TestPxDownFails(t *testing.T) {
	o := baseOptions(t)
	o.Profile.Network.ProxyAuth = "kerberos"
	o.Profile.Network.Proxy = "http://proxy.example.com:8080"
	o.PxAddr = "127.0.0.1:1"

	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "px")
	if !ok {
		t.Fatalf("vérification px absente : %+v", checks)
	}
	if c.Status != Fail {
		t.Fatalf("statut = %s, attendu fail", c.Status)
	}
}

func TestVaultExpiredWarns(t *testing.T) {
	o := baseOptions(t)
	o.Answers.Vault = true
	run := o.Deps.Run.(*execx.Fake)
	run.Responses = map[string]execx.Result{
		filepath.Join(install.CurrentDir(o.Deps, "vault"), "vault.exe") + " token lookup -format=json": {
			Stdout: `{"data":{"ttl":0}}`,
		},
	}

	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "Vault")
	if !ok {
		t.Fatalf("vérification Vault absente : %+v", checks)
	}
	if c.Status != Warn {
		t.Fatalf("statut = %s, attendu warn", c.Status)
	}
}

func TestPodmanStoppedWarns(t *testing.T) {
	o := baseOptions(t)
	o.Answers.Podman = true

	current := install.CurrentDir(o.Deps, "podman")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(current, "podman.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := o.Deps.Run.(*execx.Fake)
	run.Responses = map[string]execx.Result{
		exe + " machine list --format json": {Stdout: `[{"Name":"default","Running":false}]`},
	}

	checks := Run(context.Background(), o)
	c, ok := findCheck(checks, "Podman")
	if !ok {
		t.Fatalf("vérification Podman absente : %+v", checks)
	}
	if c.Status != Warn {
		t.Fatalf("statut = %s, attendu warn", c.Status)
	}
	if c.Fix != "podman machine start" {
		t.Fatalf("fix = %q", c.Fix)
	}
}
