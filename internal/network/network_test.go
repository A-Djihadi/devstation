package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// netTestDeps construit des dépendances de test avec un Fake et un Runner factice.
func netTestDeps(t *testing.T) (install.Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	fake := winenv.NewFake()
	run := &execx.Fake{}
	d := install.Deps{
		Env:   fake,
		Links: fake,
		Certs: fake,
		Creds: fake,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, fake, run
}

// netTestPaths construit des chemins de configuration dans un dossier temporaire.
func netTestPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		Npmrc:         filepath.Join(dir, ".npmrc"),
		PipIni:        filepath.Join(dir, "pip.ini"),
		MavenSettings: filepath.Join(dir, "settings.xml"),
		GradleProps:   filepath.Join(dir, "gradle.properties"),
		CABundle:      filepath.Join(dir, "ca-bundle.pem"),
	}
}

// netReadFile lit un fichier de test et échoue en cas d'erreur.
func netReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de %q : %v", path, err)
	}
	return string(data)
}

// netGitCalled indique si une commande git config avec la clé donnée a été appelée.
func netGitCalled(run *execx.Fake, key string) bool {
	for _, c := range run.Calls {
		if c.Name != "git" {
			continue
		}
		for i, a := range c.Args {
			if a == key && i > 0 {
				return true
			}
		}
	}
	return false
}

// TestEffectiveKerberosUsesPx vérifie que Kerberos impose le proxy local.
func TestEffectiveKerberosUsesPx(t *testing.T) {
	proxyURL, usePx := Effective(profile.Network{ProxyAuth: "kerberos"})
	if proxyURL != "http://127.0.0.1:3128" || !usePx {
		t.Fatalf("Effective = (%q, %v), attendu (http://127.0.0.1:3128, true)", proxyURL, usePx)
	}
	proxyURL, usePx = Effective(profile.Network{Proxy: "http://proxy.example.com:8080"})
	if proxyURL != "http://proxy.example.com:8080" || usePx {
		t.Fatalf("Effective = (%q, %v), attendu (http://proxy.example.com:8080, false)", proxyURL, usePx)
	}
}

// TestApplyProxyNone vérifie qu'aucune configuration n'est écrite sans proxy.
func TestApplyProxyNone(t *testing.T) {
	d, fake, run := netTestDeps(t)
	p := netTestPaths(t)
	if err := Apply(context.Background(), d, profile.Network{}, p); err != nil {
		t.Fatalf("Apply : %v", err)
	}
	for _, path := range []string{p.Npmrc, p.PipIni, p.MavenSettings, p.GradleProps, p.CABundle} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("fichier %q créé alors qu'il ne devrait pas l'être", path)
		}
	}
	if len(fake.Vars) != 0 {
		t.Fatalf("variables posées : %v", fake.Vars)
	}
	if len(run.Calls) != 0 {
		t.Fatalf("commandes exécutées : %v", run.Calls)
	}
}

// TestApplyKerberosWritesLocalProxy vérifie l'écriture du proxy local.
func TestApplyKerberosWritesLocalProxy(t *testing.T) {
	d, fake, run := netTestDeps(t)
	p := netTestPaths(t)
	n := profile.Network{ProxyAuth: "kerberos"}
	if err := Apply(context.Background(), d, n, p); err != nil {
		t.Fatalf("Apply : %v", err)
	}
	npmrc := netReadFile(t, p.Npmrc)
	if !strings.Contains(npmrc, "proxy=http://127.0.0.1:3128") {
		t.Fatalf(".npmrc = %q", npmrc)
	}
	if fake.Vars["HTTPS_PROXY"] != "http://127.0.0.1:3128" {
		t.Fatalf("HTTPS_PROXY = %q", fake.Vars["HTTPS_PROXY"])
	}
	if !netGitCalled(run, "http.proxy") {
		t.Fatalf("git config http.proxy non appelé : %v", run.Calls)
	}
}

// TestApplyNexusMirrors vérifie l'écriture des dépôts Nexus.
func TestApplyNexusMirrors(t *testing.T) {
	d, _, _ := netTestDeps(t)
	p := netTestPaths(t)
	n := profile.Network{Nexus: profile.Nexus{
		Npm:   "https://nexus.example.com/repository/npm/",
		Pypi:  "https://nexus.example.com/repository/pypi/simple",
		Maven: "https://nexus.example.com/repository/maven/",
	}}
	if err := Apply(context.Background(), d, n, p); err != nil {
		t.Fatalf("Apply : %v", err)
	}
	if npmrc := netReadFile(t, p.Npmrc); !strings.Contains(npmrc, "registry=https://nexus.example.com/repository/npm/") {
		t.Fatalf(".npmrc = %q", npmrc)
	}
	if pip := netReadFile(t, p.PipIni); !strings.Contains(pip, "index-url=https://nexus.example.com/repository/pypi/simple") {
		t.Fatalf("pip.ini = %q", pip)
	}
	if maven := netReadFile(t, p.MavenSettings); !strings.Contains(maven, "https://nexus.example.com/repository/maven/") {
		t.Fatalf("settings.xml = %q", maven)
	}
}

// TestApplyCACertsBundleAndStore vérifie le magasin et le bundle de certificats.
func TestApplyCACertsBundleAndStore(t *testing.T) {
	d, fake, run := netTestDeps(t)
	p := netTestPaths(t)
	dir := t.TempDir()
	cert1 := filepath.Join(dir, "ca1.pem")
	cert2 := filepath.Join(dir, "ca2.pem")
	if err := os.WriteFile(cert1, []byte("PEM-UN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cert2, []byte("PEM-DEUX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := profile.Network{CACerts: []string{cert1, cert2}}
	if err := Apply(context.Background(), d, n, p); err != nil {
		t.Fatalf("Apply : %v", err)
	}
	if !fake.Roots[cert1] || !fake.Roots[cert2] {
		t.Fatalf("certificats absents du magasin : %v", fake.Roots)
	}
	bundle := netReadFile(t, p.CABundle)
	if bundle != "PEM-UN\nPEM-DEUX\n" {
		t.Fatalf("bundle = %q", bundle)
	}
	if npmrc := netReadFile(t, p.Npmrc); !strings.Contains(npmrc, "cafile="+p.CABundle) {
		t.Fatalf(".npmrc = %q", npmrc)
	}
	if pip := netReadFile(t, p.PipIni); !strings.Contains(pip, "cert="+p.CABundle) {
		t.Fatalf("pip.ini = %q", pip)
	}
	if !netGitCalled(run, "http.sslBackend") {
		t.Fatalf("git config http.sslBackend non appelé : %v", run.Calls)
	}
}
