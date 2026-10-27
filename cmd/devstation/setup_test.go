package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/ui"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// testSource est une source de versions en mémoire.
type testSource struct {
	name     string
	releases []sources.Release
	err      error
}

// Name retourne le nom de la source.
func (s *testSource) Name() string { return s.name }

// Releases retourne les versions en mémoire ou l'erreur configurée.
func (s *testSource) Releases(ctx context.Context) ([]sources.Release, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.releases, nil
}

// Resolve retourne la release inchangée.
func (s *testSource) Resolve(ctx context.Context, r sources.Release) (sources.Release, error) {
	return r, nil
}

// testSources construit des sources factices pour tous les outils.
func testSources() map[string]sources.Source {
	mk := func(name, version string) sources.Source {
		return &testSource{name: name, releases: []sources.Release{{Version: version, LTS: true}}}
	}
	return map[string]sources.Source{
		"java":        mk("java", "21.0.4"),
		"node":        mk("node", "22.11.0"),
		"angular_cli": mk("angular_cli", "18.2.0"),
		"python":      mk("python", "3.12.7"),
		"maven":       mk("maven", "3.9.9"),
		"gradle":      mk("gradle", "8.10.2"),
		"vscode":      mk("vscode", "1.95.0"),
		"intellij":    mk("intellij", "2024.2"),
		"podman":      mk("podman", "5.2.0"),
		"compose":     mk("compose", "2.30.0"),
		"px":          mk("px", "0.9.0"),
		"vault":       mk("vault", "1.18.0"),
	}
}

// testApp construit une App minimale dans un dossier temporaire.
func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	return &App{
		Home:         dir,
		AppData:      filepath.Join(dir, "AppData", "Roaming"),
		LocalAppData: filepath.Join(dir, "AppData", "Local"),
		Root:         filepath.Join(dir, "DevStation"),
		StatePath:    filepath.Join(dir, ".devstation", "state.json"),
		AnswersPath:  filepath.Join(dir, ".devstation", "answers.yaml"),
		LogPath:      filepath.Join(dir, "devstation.log"),
		State:        &state.State{Active: map[string]string{}},
		Theme:        ui.NewTheme(false),
	}
}

// testDeps construit des dépendances factices.
func testDeps(t *testing.T, app *App) (install.Deps, *execx.Fake) {
	t.Helper()
	fake := winenv.NewFake()
	run := &execx.Fake{}
	return install.Deps{
		Env:   fake,
		Links: fake,
		Certs: fake,
		Creds: fake,
		Run:   run,
		State: app.State,
		Root:  filepath.Join(app.Root, "tools"),
	}, run
}

// TestDryRunListsStepsInOrder vérifie l'ordre des étapes en simulation.
func TestDryRunListsStepsInOrder(t *testing.T) {
	app := testApp(t)
	deps, run := testDeps(t, app)
	var out bytes.Buffer

	h := SetupHooks{
		Sources: func(f sources.Fetcher, p *profile.Profile) map[string]sources.Source {
			return testSources()
		},
		Out: &out,
	}
	o := SetupOptions{ProfilePath: filepath.Join("..", "..", "profiles", "demo.yaml"), Yes: true, DryRun: true}
	if err := runSetup(context.Background(), app, deps, o, h); err != nil {
		t.Fatalf("runSetup : %v", err)
	}

	got := out.String()
	want := []string{"Réseau d'entreprise", "Java", "Maven", "Node.js", "VS Code", "Git", "Dépôts du projet"}
	pos := -1
	for _, w := range want {
		i := strings.Index(got, w)
		if i < 0 {
			t.Fatalf("sortie sans %q :\n%s", w, got)
		}
		if i <= pos {
			t.Fatalf("%q hors ordre :\n%s", w, got)
		}
		pos = i
	}
	if !strings.Contains(got, "simulation") {
		t.Fatalf("ligne de simulation absente :\n%s", got)
	}
	if len(run.Calls) != 0 {
		t.Fatalf("le runner a été appelé : %v", run.Calls)
	}
}

// TestYesReadsTokenFromEnvAndNeverPrintsIt vérifie que le token reste masqué.
func TestYesReadsTokenFromEnvAndNeverPrintsIt(t *testing.T) {
	app := testApp(t)
	deps, _ := testDeps(t, app)
	var out bytes.Buffer
	const token = "ghp_secretfactice123"

	h := SetupHooks{
		Sources: func(f sources.Fetcher, p *profile.Profile) map[string]sources.Source {
			return testSources()
		},
		Getenv: func(name string) string {
			if name == "DEVSTATION_GITHUB_TOKEN" {
				return token
			}
			return ""
		},
		Out: &out,
	}
	o := SetupOptions{ProfilePath: filepath.Join("..", "..", "profiles", "demo.yaml"), Yes: true, DryRun: true}
	if err := runSetup(context.Background(), app, deps, o, h); err != nil {
		t.Fatalf("runSetup : %v", err)
	}
	if strings.Contains(out.String(), token) {
		t.Fatalf("le token apparaît dans la sortie :\n%s", out.String())
	}
}

// TestSourceErrorNamesTool vérifie qu'une erreur de source cite l'outil.
func TestSourceErrorNamesTool(t *testing.T) {
	app := testApp(t)
	deps, _ := testDeps(t, app)
	var out bytes.Buffer

	srcs := testSources()
	srcs["java"] = &testSource{name: "java", err: errors.New("panne réseau")}

	h := SetupHooks{
		Sources: func(f sources.Fetcher, p *profile.Profile) map[string]sources.Source {
			return srcs
		},
		Out: &out,
	}
	o := SetupOptions{ProfilePath: filepath.Join("..", "..", "profiles", "demo.yaml"), Yes: true, DryRun: true}
	err := runSetup(context.Background(), app, deps, o, h)
	if err == nil {
		t.Fatal("erreur attendue")
	}
	if !strings.Contains(err.Error(), "java") {
		t.Fatalf("erreur sans le nom de l'outil : %v", err)
	}
}

// TestWizardAddedRoleFetchesReleases vérifie que les versions d'un rôle ajouté
// par l'assistant sont recherchées après l'assistant.
func TestWizardAddedRoleFetchesReleases(t *testing.T) {
	app := testApp(t)
	deps, _ := testDeps(t, app)
	var out bytes.Buffer

	pythonCalls := 0
	srcs := testSources()
	srcs["python"] = &countingSource{
		testSource: testSource{name: "python", releases: []sources.Release{{Version: "3.12.7"}}},
		calls:      &pythonCalls,
	}

	h := SetupHooks{
		Sources: func(f sources.Fetcher, p *profile.Profile) map[string]sources.Source {
			return srcs
		},
		Wizard: func(p *profile.Profile, a *profile.Answers, v map[string][]sources.Release, io ui.WizardIO) (*profile.Answers, error) {
			out := *a
			out.Roles = append(append([]string(nil), a.Roles...), "python")
			return &out, nil
		},
		Token: func(io ui.WizardIO) (string, error) { return "", nil },
		Out:   &out,
	}
	o := SetupOptions{ProfilePath: filepath.Join("..", "..", "profiles", "demo.yaml"), DryRun: true}
	if err := runSetup(context.Background(), app, deps, o, h); err != nil {
		t.Fatalf("runSetup : %v", err)
	}
	if pythonCalls == 0 {
		t.Fatal("la source python n'a pas été interrogée")
	}
	if !strings.Contains(out.String(), "Python") {
		t.Fatalf("étape Python absente :\n%s", out.String())
	}
}

// countingSource compte les appels à Releases.
type countingSource struct {
	testSource
	calls *int
}

// Releases compte l'appel puis délègue à la source en mémoire.
func (s *countingSource) Releases(ctx context.Context) ([]sources.Release, error) {
	*s.calls++
	return s.testSource.Releases(ctx)
}

// TestRootCmdHasSetup vérifie la présence de setup et de --version.
func TestRootCmdHasSetup(t *testing.T) {
	root := newRootCmd()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "setup" {
			found = true
		}
	}
	if !found {
		t.Fatal("sous-commande setup absente")
	}
	if root.Version != version {
		t.Fatalf("version = %q, attendu %q", root.Version, version)
	}
}
