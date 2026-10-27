package install

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// buildtoolsWriteZip crée une archive zip contenant les fichiers fournis.
func buildtoolsWriteZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("création de l'archive : %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("ajout de %q : %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("écriture de %q : %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("clôture de l'archive : %v", err)
	}
}

// buildtoolsServer démarre un serveur HTTP de test qui sert le fichier path.
func buildtoolsServer(t *testing.T, path string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}))
}

// buildtoolsDeps construit des dépendances de test avec un Fake d'environnement.
func buildtoolsDeps(t *testing.T) (Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	env := winenv.NewFake()
	run := &execx.Fake{Responses: map[string]execx.Result{}}
	d := Deps{
		Env:   env,
		Links: env,
		Run:   run,
		Root:  t.TempDir(),
	}
	return d, env, run
}

// TestMavenActivateSetsHome vérifie l'activation de Maven et MAVEN_HOME.
func TestMavenActivateSetsHome(t *testing.T) {
	d, env, _ := buildtoolsDeps(t)
	m := NewMaven()
	if m.Tool() != "maven" {
		t.Fatalf("Tool() = %q, attendu %q", m.Tool(), "maven")
	}
	if err := m.Activate(d, "3.9.16"); err != nil {
		t.Fatalf("Activate : %v", err)
	}
	current := CurrentDir(d, "maven")
	if got := env.Junctions[current]; got != ToolDir(d, "maven", "3.9.16") {
		t.Fatalf("jonction = %q, attendu %q", got, ToolDir(d, "maven", "3.9.16"))
	}
	if got := env.Vars["MAVEN_HOME"]; got != current {
		t.Fatalf("MAVEN_HOME = %q, attendu %q", got, current)
	}
	if env.Broadcasts != 1 {
		t.Fatalf("Broadcasts = %d, attendu 1", env.Broadcasts)
	}
}

// TestMavenVerify vérifie la lecture de la version de Maven.
func TestMavenVerify(t *testing.T) {
	d, _, run := buildtoolsDeps(t)
	exe := filepath.Join(CurrentDir(d, "maven"), "bin", "mvn.cmd")
	run.Responses[exe+" -v"] = execx.Result{
		Stdout: "Apache Maven 3.9.16 (2e9d1a1b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f)\nMaven home: C:\\tools\n",
	}
	v, err := NewMaven().Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if v != "3.9.16" {
		t.Fatalf("version = %q, attendu %q", v, "3.9.16")
	}
	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	if run.Calls[0].Name != exe {
		t.Fatalf("Name = %q, attendu %q", run.Calls[0].Name, exe)
	}
	if len(run.Calls[0].Args) != 1 || run.Calls[0].Args[0] != "-v" {
		t.Fatalf("Args = %v, attendu [-v]", run.Calls[0].Args)
	}
}

// TestGradleActivateSetsHome vérifie l'activation de Gradle et GRADLE_HOME.
func TestGradleActivateSetsHome(t *testing.T) {
	d, env, _ := buildtoolsDeps(t)
	g := NewGradle()
	if g.Tool() != "gradle" {
		t.Fatalf("Tool() = %q, attendu %q", g.Tool(), "gradle")
	}
	if err := g.Activate(d, "9.2.1"); err != nil {
		t.Fatalf("Activate : %v", err)
	}
	current := CurrentDir(d, "gradle")
	if got := env.Junctions[current]; got != ToolDir(d, "gradle", "9.2.1") {
		t.Fatalf("jonction = %q, attendu %q", got, ToolDir(d, "gradle", "9.2.1"))
	}
	if got := env.Vars["GRADLE_HOME"]; got != current {
		t.Fatalf("GRADLE_HOME = %q, attendu %q", got, current)
	}
	if env.Broadcasts != 1 {
		t.Fatalf("Broadcasts = %d, attendu 1", env.Broadcasts)
	}
}

// TestGradleVerify vérifie la lecture de la version de Gradle.
func TestGradleVerify(t *testing.T) {
	d, _, run := buildtoolsDeps(t)
	exe := filepath.Join(CurrentDir(d, "gradle"), "bin", "gradle.bat")
	run.Responses[exe+" --version"] = execx.Result{
		Stdout: "\n------------------------------------------------------------\nGradle 9.2.1\n------------------------------------------------------------\n",
	}
	v, err := NewGradle().Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if v != "9.2.1" {
		t.Fatalf("version = %q, attendu %q", v, "9.2.1")
	}
	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	if run.Calls[0].Name != exe {
		t.Fatalf("Name = %q, attendu %q", run.Calls[0].Name, exe)
	}
	if len(run.Calls[0].Args) != 1 || run.Calls[0].Args[0] != "--version" {
		t.Fatalf("Args = %v, attendu [--version]", run.Calls[0].Args)
	}
}

// TestMavenInstallStripsTopDir vérifie l'installation de Maven sans dossier racine.
func TestMavenInstallStripsTopDir(t *testing.T) {
	d, _, _ := buildtoolsDeps(t)
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("création du client de téléchargement : %v", err)
	}
	d.DL = dl

	archivePath := filepath.Join(t.TempDir(), "maven.zip")
	buildtoolsWriteZip(t, archivePath, map[string]string{
		"apache-maven-3.9.16/bin/mvn.cmd": "@echo off\r\n",
		"apache-maven-3.9.16/README.txt":  "maven",
	})

	srv := buildtoolsServer(t, archivePath)
	defer srv.Close()

	r := sources.Release{
		Version: "3.9.16",
		URL:     srv.URL + "/maven.zip",
		Archive: "zip",
	}
	if err := NewMaven().Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	dir := ToolDir(d, "maven", "3.9.16")
	if _, err := os.Stat(filepath.Join(dir, "bin", "mvn.cmd")); err != nil {
		t.Fatalf("mvn.cmd absent après installation : %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "apache-maven-3.9.16")); !os.IsNotExist(err) {
		t.Fatalf("le dossier racine de l'archive n'a pas été retiré (err = %v)", err)
	}
}
