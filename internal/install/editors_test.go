package install

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// editorsZip construit une archive zip en mémoire contenant les fichiers donnés.
func editorsZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("création de %q : %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("écriture de %q : %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture du zip : %v", err)
	}
	return buf.Bytes()
}

// editorsServer sert l'archive donnée et retourne le serveur et la release.
func editorsServer(t *testing.T, files map[string]string) (*httptest.Server, sources.Release) {
	t.Helper()
	data := editorsZip(t, files)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, sources.Release{Version: "1.0.0", URL: srv.URL + "/tool.zip", Archive: "zip"}
}

// editorsDeps construit des dépendances de test avec un serveur d'archive.
func editorsDeps(t *testing.T, srv *httptest.Server, run execx.Runner) Deps {
	t.Helper()
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("download.New : %v", err)
	}
	return Deps{
		DL:    dl,
		Env:   winenv.NewFake(),
		Links: winenv.NewFake(),
		Run:   run,
		Root:  t.TempDir(),
	}
}

// TestExtensionsFullstack vérifie la concaténation common + back + front sans doublon.
func TestExtensionsFullstack(t *testing.T) {
	p := &profile.Profile{
		EditorExtensions: map[string]map[string][]string{
			"vscode": {
				"common": {"a", "b"},
				"back":   {"b", "c"},
				"front":  {"d", "a"},
			},
		},
	}
	got := Extensions(p, "vscode", []string{"fullstack"})
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("Extensions = %v, attendu %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Extensions = %v, attendu %v", got, want)
		}
	}
}

// TestVSCodeNoDataDir vérifie qu'aucun dossier "data" n'est créé.
func TestVSCodeNoDataDir(t *testing.T) {
	srv, rel := editorsServer(t, map[string]string{
		"bin/code.cmd": "echo",
	})
	run := &execx.Fake{}
	d := editorsDeps(t, srv, run)

	v := NewVSCode(nil)
	if err := v.Install(context.Background(), d, rel, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	dir := ToolDir(d, "vscode", rel.Version)
	if _, err := os.Stat(filepath.Join(dir, "data")); !os.IsNotExist(err) {
		t.Fatalf("le dossier data ne doit pas exister (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "code.cmd")); err != nil {
		t.Fatalf("code.cmd absent : %v", err)
	}
}

// TestVSCodeExtensionFailureContinues vérifie qu'un échec d'extension n'arrête
// pas les suivantes et que l'erreur cite l'extension en échec.
func TestVSCodeExtensionFailureContinues(t *testing.T) {
	srv, rel := editorsServer(t, map[string]string{
		"bin/code.cmd": "echo",
	})
	dir := filepath.Join(t.TempDir(), "vscode", rel.Version)
	code := filepath.Join(dir, "bin", "code.cmd")
	run := &execx.Fake{
		Responses: map[string]execx.Result{
			code + " --install-extension bad --force":  {Code: 1, Stderr: "boom"},
			code + " --install-extension good --force": {Code: 0},
		},
	}
	d := editorsDeps(t, srv, run)
	d.Root = filepath.Dir(filepath.Dir(dir))

	v := NewVSCode([]string{"bad", "good"})
	err := v.Install(context.Background(), d, rel, nil)
	if err == nil {
		t.Fatal("Install devait retourner une erreur")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("erreur %q ne cite pas l'extension en échec", err)
	}

	var installed []string
	for _, c := range run.Calls {
		if len(c.Args) >= 2 && c.Args[0] == "--install-extension" {
			installed = append(installed, c.Args[1])
		}
	}
	if len(installed) != 2 || installed[0] != "bad" || installed[1] != "good" {
		t.Fatalf("extensions installées = %v, attendu [bad good]", installed)
	}
}

// TestIntelliJShortcutCommand vérifie la commande PowerShell de création du raccourci.
func TestIntelliJShortcutCommand(t *testing.T) {
	srv, rel := editorsServer(t, map[string]string{
		"bin/idea64.exe": "bin",
	})
	run := &execx.Fake{}
	d := editorsDeps(t, srv, run)

	startMenu := filepath.Join(t.TempDir(), "Start Menu")
	ij := NewIntelliJ(nil, startMenu)
	if err := ij.Install(context.Background(), d, rel, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	c := run.Calls[0]
	if c.Name != "powershell.exe" {
		t.Fatalf("Name = %q, attendu powershell.exe", c.Name)
	}
	if len(c.Args) != 4 || c.Args[0] != "-NoProfile" || c.Args[1] != "-NonInteractive" || c.Args[2] != "-Command" {
		t.Fatalf("Args = %v", c.Args)
	}
	script := c.Args[3]
	lnk := filepath.Join(startMenu, "IntelliJ IDEA.lnk")
	exe := filepath.Join(ToolDir(d, "intellij", rel.Version), "bin", "idea64.exe")
	if !strings.Contains(script, lnk) {
		t.Fatalf("script %q ne cite pas %q", script, lnk)
	}
	if !strings.Contains(script, exe) {
		t.Fatalf("script %q ne cite pas %q", script, exe)
	}
}

// TestIntelliJPluginsBestEffort vérifie que l'échec des plugins n'empêche pas
// l'installation de réussir.
func TestIntelliJPluginsBestEffort(t *testing.T) {
	srv, rel := editorsServer(t, map[string]string{
		"bin/idea64.exe": "bin",
	})
	exe := filepath.Join(ToolDir(Deps{Root: ""}, "intellij", rel.Version), "bin", "idea64.exe")
	_ = exe
	run := &execx.Fake{}
	d := editorsDeps(t, srv, run)
	exe = filepath.Join(ToolDir(d, "intellij", rel.Version), "bin", "idea64.exe")
	run.Responses = map[string]execx.Result{
		exe + " installPlugins p1 p2": {Code: 1, Stderr: "échec"},
	}

	ij := NewIntelliJ([]string{"p1", "p2"}, "")
	if err := ij.Install(context.Background(), d, rel, nil); err != nil {
		t.Fatalf("Install ne devait pas échouer : %v", err)
	}

	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	c := run.Calls[0]
	if c.Name != exe {
		t.Fatalf("Name = %q, attendu %q", c.Name, exe)
	}
	want := []string{"installPlugins", "p1", "p2"}
	if len(c.Args) != len(want) {
		t.Fatalf("Args = %v, attendu %v", c.Args, want)
	}
	for i := range want {
		if c.Args[i] != want[i] {
			t.Fatalf("Args = %v, attendu %v", c.Args, want)
		}
	}
}
