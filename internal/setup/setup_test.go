package setup

import (
	"context"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/network"
	"github.com/A-Djihadi/devstation/internal/plan"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// testOptions construit des options de test avec un état vide et des releases
// factices.
func testOptions(a *profile.Answers, p *profile.Profile) Options {
	fake := winenv.NewFake()
	deps := install.Deps{
		Env:   fake,
		Links: fake,
		Certs: fake,
		Creds: fake,
		Run:   &execx.Fake{},
		State: &state.State{Active: map[string]string{}},
		Root:  "/tools",
	}
	releases := map[string]sources.Release{}
	for _, name := range []string{"java", "node", "python", "maven", "gradle", "vscode", "intellij", "podman", "compose", "vault", "px"} {
		releases[name] = sources.Release{Version: "1.0.0", Archive: "zip"}
	}
	return Options{
		Deps:     deps,
		Answers:  a,
		Profile:  p,
		Releases: releases,
		Paths:    network.Paths{},
		Dirs:     Dirs{Bin: "/bin", Home: "/home", StartMenu: "/menu", Workspace: "/ws", PxDir: "/px"},
	}
}

// stepIDs retourne les IDs des étapes dans l'ordre.
func stepIDs(steps []plan.Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.ID
	}
	return out
}

// TestToolsFullstack vérifie la sélection des outils pour un profil fullstack.
func TestToolsFullstack(t *testing.T) {
	a := &profile.Answers{Roles: []string{"fullstack"}, Editor: "vscode"}
	p := &profile.Profile{}
	got := Tools(a, p)
	want := []string{"java", "maven", "node", "vscode"}
	if len(got) != len(want) {
		t.Fatalf("Tools = %v, attendu %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tools = %v, attendu %v", got, want)
		}
	}
}

// TestToolsFrontOnly vérifie qu'un profil front n'installe que Node.js.
func TestToolsFrontOnly(t *testing.T) {
	a := &profile.Answers{Roles: []string{"front"}, Editor: "vscode"}
	p := &profile.Profile{}
	got := Tools(a, p)
	want := []string{"node", "vscode"}
	if len(got) != len(want) {
		t.Fatalf("Tools = %v, attendu %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tools = %v, attendu %v", got, want)
		}
	}
}

// TestBuildOrderAndDependencies vérifie l'ordre des étapes et les dépendances.
func TestBuildOrderAndDependencies(t *testing.T) {
	a := &profile.Answers{Roles: []string{"fullstack"}, Editor: "vscode", Repos: []string{"app"}}
	p := &profile.Profile{Repos: []profile.Repo{{Name: "app", URL: "https://git.example.com/app.git"}}}
	steps, err := Build(testOptions(a, p))
	if err != nil {
		t.Fatalf("Build : %v", err)
	}
	ids := stepIDs(steps)
	want := []string{"network", "java", "maven", "node", "vscode", "git", "repos"}
	if len(ids) != len(want) {
		t.Fatalf("IDs = %v, attendu %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("IDs = %v, attendu %v", ids, want)
		}
	}
	for _, s := range steps {
		switch s.ID {
		case "java", "maven", "node", "vscode":
			if len(s.DependsOn) != 1 || s.DependsOn[0] != "network" {
				t.Fatalf("étape %s : dépendances = %v, attendu [network]", s.ID, s.DependsOn)
			}
		case "repos":
			found := false
			for _, d := range s.DependsOn {
				if d == "git" {
					found = true
				}
			}
			if !found {
				t.Fatalf("étape repos : dépendances = %v, attendu git", s.DependsOn)
			}
		}
	}
}

// TestBuildKerberosAddsPxFirst vérifie que Kerberos ajoute l'étape px en tête.
func TestBuildKerberosAddsPxFirst(t *testing.T) {
	a := &profile.Answers{Roles: []string{"front"}, Editor: "vscode", ProxyAuth: "kerberos", Proxy: "http://proxy.example.com:8080"}
	p := &profile.Profile{}
	steps, err := Build(testOptions(a, p))
	if err != nil {
		t.Fatalf("Build : %v", err)
	}
	ids := stepIDs(steps)
	if len(ids) == 0 || ids[0] != "px" {
		t.Fatalf("IDs = %v, attendu px en tête", ids)
	}
	if ids[1] != "network" {
		t.Fatalf("IDs = %v, attendu network après px", ids)
	}
}

// TestBuildMissingReleaseFails vérifie qu'une release absente fait échouer Build.
func TestBuildMissingReleaseFails(t *testing.T) {
	a := &profile.Answers{Roles: []string{"front"}, Editor: "vscode"}
	p := &profile.Profile{}
	o := testOptions(a, p)
	delete(o.Releases, "node")
	if _, err := Build(o); err == nil {
		t.Fatal("Build aurait dû échouer sans release node")
	}
}

// TestCheckSkipsInstalledActive vérifie que Check est vrai quand l'outil est
// installé et actif.
func TestCheckSkipsInstalledActive(t *testing.T) {
	a := &profile.Answers{Roles: []string{"front"}, Editor: "vscode"}
	p := &profile.Profile{}
	o := testOptions(a, p)
	o.Deps.State.RecordInstall(state.Install{Tool: "node", Version: "1.0.0"})
	o.Deps.State.Active["node"] = "1.0.0"
	steps, err := Build(o)
	if err != nil {
		t.Fatalf("Build : %v", err)
	}
	for _, s := range steps {
		if s.ID != "node" {
			continue
		}
		ok, err := s.Check(context.Background())
		if err != nil {
			t.Fatalf("Check : %v", err)
		}
		if !ok {
			t.Fatal("Check devrait être vrai pour node installé et actif")
		}
		return
	}
	t.Fatal("étape node introuvable")
}

// TestTokenNeverInSteps vérifie que le jeton GitHub n'apparaît dans aucun ID
// ni titre d'étape.
func TestTokenNeverInSteps(t *testing.T) {
	const token = "ghp_secret_token_value"
	a := &profile.Answers{Roles: []string{"front"}, Editor: "vscode"}
	p := &profile.Profile{}
	o := testOptions(a, p)
	o.GitHubToken = token
	steps, err := Build(o)
	if err != nil {
		t.Fatalf("Build : %v", err)
	}
	for _, s := range steps {
		if containsToken(s.ID, token) || containsToken(s.Title, token) {
			t.Fatalf("le jeton apparaît dans l'étape %q", s.ID)
		}
	}
}
