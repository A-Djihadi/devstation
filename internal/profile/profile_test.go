package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDemoProfile(t *testing.T) {
	p, err := Load(filepath.Join("..", "..", "profiles", "demo.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "demo" {
		t.Errorf("Name = %q, attendu demo", p.Name)
	}
	if len(p.Repos) != 2 {
		t.Errorf("len(Repos) = %d, attendu 2", len(p.Repos))
	}
	if p.Dir == "" {
		t.Error("Dir vide")
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.yaml")
	content := "name: x\ntoolz:\n  java: {}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("erreur attendue")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("type d'erreur = %T, attendu *ValidationError", err)
	}
	if ve.Key != "toolz" {
		t.Errorf("Key = %q, attendu toolz", ve.Key)
	}
	if ve.Line <= 0 {
		t.Errorf("Line = %d, attendu > 0", ve.Line)
	}
}

func TestKerberosNeedsProxy(t *testing.T) {
	p := &Profile{}
	p.Network.ProxyAuth = "kerberos"
	err := p.Validate()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("erreur attendue *ValidationError, obtenu %v", err)
	}
	if ve.Key != "network.proxy" {
		t.Errorf("Key = %q, attendu network.proxy", ve.Key)
	}
}

func TestDefaultNotInVersions(t *testing.T) {
	p := &Profile{}
	p.Tools.Java = ToolSpec{Versions: []string{"21", "17"}, Default: "11"}
	err := p.Validate()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("erreur attendue *ValidationError, obtenu %v", err)
	}
	if ve.Key != "tools.java.default" {
		t.Errorf("Key = %q, attendu tools.java.default", ve.Key)
	}
}

func TestReposForRoles(t *testing.T) {
	p, err := Load(filepath.Join("..", "..", "profiles", "demo.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	front := p.ReposFor([]string{"front"})
	if len(front) != 1 || front[0].Name != "demo-frontend" {
		t.Errorf("ReposFor([front]) = %+v", front)
	}
	full := p.ReposFor([]string{"fullstack"})
	if len(full) != 2 {
		t.Errorf("ReposFor([fullstack]) = %d dépôts, attendu 2", len(full))
	}
}