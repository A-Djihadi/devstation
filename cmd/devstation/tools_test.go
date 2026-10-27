package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/doctor"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// toolsTestState construit un état avec deux outils installés.
func toolsTestState() *state.State {
	st := &state.State{Active: map[string]string{}}
	st.RecordInstall(state.Install{Tool: "java", Version: "21.0.4"})
	st.RecordInstall(state.Install{Tool: "java", Version: "17.0.12"})
	st.RecordInstall(state.Install{Tool: "node", Version: "22.11.0"})
	st.Active["java"] = "21.0.4"
	return st
}

// toolsTestDeps construit des dépendances factices.
func toolsTestDeps(st *state.State, run execx.Runner) install.Deps {
	fake := winenv.NewFake()
	return install.Deps{
		Env:   fake,
		Links: fake,
		Run:   run,
		State: st,
		Root:  "/tmp/devstation-tools",
	}
}

// toolsFakeSource est une source de versions factice.
type toolsFakeSource struct {
	releases []sources.Release
	calls    int
}

// Name retourne le nom de la source.
func (s *toolsFakeSource) Name() string { return "fake" }

// Releases retourne les versions publiées.
func (s *toolsFakeSource) Releases(ctx context.Context) ([]sources.Release, error) {
	s.calls++
	return s.releases, nil
}

// Resolve complète une release choisie.
func (s *toolsFakeSource) Resolve(ctx context.Context, r sources.Release) (sources.Release, error) {
	return r, nil
}

// toolsFakeInstaller est un installeur factice.
type toolsFakeInstaller struct {
	tool      string
	installed []string
	activated []string
}

// Tool retourne le nom de l'outil géré.
func (i *toolsFakeInstaller) Tool() string { return i.tool }

// Install enregistre l'installation.
func (i *toolsFakeInstaller) Install(ctx context.Context, d install.Deps, r sources.Release, report install.Report) error {
	i.installed = append(i.installed, r.Version)
	if d.State != nil {
		d.State.RecordInstall(state.Install{Tool: i.tool, Version: r.Version})
	}
	return nil
}

// Activate enregistre l'activation.
func (i *toolsFakeInstaller) Activate(d install.Deps, version string) error {
	i.activated = append(i.activated, version)
	if d.State != nil {
		d.State.Active[i.tool] = version
	}
	return nil
}

// Verify retourne une version factice.
func (i *toolsFakeInstaller) Verify(ctx context.Context, d install.Deps) (string, error) {
	return "", nil
}

// toolsFailInstaller est un installeur factice dont Verify échoue.
type toolsFailInstaller struct {
	tool string
}

// Tool retourne le nom de l'outil géré.
func (i *toolsFailInstaller) Tool() string { return i.tool }

// Install n'installe rien.
func (i *toolsFailInstaller) Install(ctx context.Context, d install.Deps, r sources.Release, report install.Report) error {
	return nil
}

// Activate n'active rien.
func (i *toolsFailInstaller) Activate(d install.Deps, version string) error { return nil }

// Verify échoue toujours.
func (i *toolsFailInstaller) Verify(ctx context.Context, d install.Deps) (string, error) {
	return "", errors.New("outil absent")
}

// TestListMarksActive vérifie le marquage de la version active.
func TestListMarksActive(t *testing.T) {
	var buf bytes.Buffer
	toolsList(&buf, toolsTestState(), "")
	out := buf.String()
	if !strings.Contains(out, "java 21.0.4 (active)") {
		t.Fatalf("version active absente : %q", out)
	}
	if !strings.Contains(out, "java 17.0.12\n") {
		t.Fatalf("version inactive absente : %q", out)
	}
	if strings.Index(out, "java 21.0.4") > strings.Index(out, "java 17.0.12") {
		t.Fatalf("tri décroissant attendu : %q", out)
	}
}

// TestListFiltersTool vérifie le filtrage par outil.
func TestListFiltersTool(t *testing.T) {
	var buf bytes.Buffer
	toolsList(&buf, toolsTestState(), "node")
	out := buf.String()
	if !strings.Contains(out, "node 22.11.0") {
		t.Fatalf("node absent : %q", out)
	}
	if strings.Contains(out, "java") {
		t.Fatalf("java ne devait pas apparaître : %q", out)
	}
}

// TestUseActivatesInstalledWithoutDownload vérifie qu'une version installée
// est activée sans interroger la source.
func TestUseActivatesInstalledWithoutDownload(t *testing.T) {
	st := toolsTestState()
	inst := &toolsFakeInstaller{tool: "java"}
	src := &toolsFakeSource{}
	var buf bytes.Buffer
	if err := toolsUse(context.Background(), &buf, toolsTestDeps(st, &execx.Fake{}), inst, src, "17.0.12"); err != nil {
		t.Fatalf("toolsUse : %v", err)
	}
	if src.calls != 0 {
		t.Fatalf("la source ne devait pas être interrogée (%d appels)", src.calls)
	}
	if len(inst.installed) != 0 {
		t.Fatalf("aucune installation attendue : %v", inst.installed)
	}
	if len(inst.activated) != 1 || inst.activated[0] != "17.0.12" {
		t.Fatalf("activation inattendue : %v", inst.activated)
	}
	if !strings.Contains(buf.String(), "java 17.0.12 est maintenant actif") {
		t.Fatalf("message inattendu : %q", buf.String())
	}
}

// TestUseInstallsMissingVersion vérifie l'installation d'une version absente.
func TestUseInstallsMissingVersion(t *testing.T) {
	st := &state.State{Active: map[string]string{}}
	inst := &toolsFakeInstaller{tool: "node"}
	src := &toolsFakeSource{releases: []sources.Release{
		{Version: "22.11.0"},
		{Version: "20.18.0"},
	}}
	var buf bytes.Buffer
	if err := toolsUse(context.Background(), &buf, toolsTestDeps(st, &execx.Fake{}), inst, src, "20"); err != nil {
		t.Fatalf("toolsUse : %v", err)
	}
	if len(inst.installed) != 1 || inst.installed[0] != "20.18.0" {
		t.Fatalf("installation inattendue : %v", inst.installed)
	}
	if len(inst.activated) != 1 || inst.activated[0] != "20.18.0" {
		t.Fatalf("activation inattendue : %v", inst.activated)
	}
}

// TestDoctorExitCode vérifie le code de sortie et le rendu du diagnostic.
func TestDoctorExitCode(t *testing.T) {
	inst := &toolsFailInstaller{tool: "java"}
	o := doctor.Options{
		Deps:       toolsTestDeps(&state.State{Active: map[string]string{}}, &execx.Fake{}),
		Installers: map[string]install.Installer{"java": inst},
	}
	var buf bytes.Buffer
	code := toolsRunDoctor(context.Background(), &buf, o)
	if code != 1 {
		t.Fatalf("code de sortie attendu 1, obtenu %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "✗") {
		t.Fatalf("symbole d'échec absent : %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Fatalf("action corrective absente : %q", out)
	}
	if !strings.Contains(out, "problème(s) bloquant(s)") {
		t.Fatalf("récapitulatif absent : %q", out)
	}
}

// TestVersionCommand vérifie l'affichage de la version.
func TestVersionCommand(t *testing.T) {
	cmd := newToolsVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exécution : %v", err)
	}
	if !strings.Contains(buf.String(), "devstation ") {
		t.Fatalf("sortie inattendue : %q", buf.String())
	}
}
