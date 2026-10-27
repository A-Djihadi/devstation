package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// pythonServer sert un contenu fixe et compte les requêtes reçues.
func pythonServer(t *testing.T, body []byte) (*httptest.Server, *int) {
	t.Helper()
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// pythonDeps construit des dépendances de test avec un cache temporaire.
func pythonDeps(t *testing.T, run *execx.Fake) Deps {
	t.Helper()
	root := t.TempDir()
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("download.New : %v", err)
	}
	return Deps{
		DL:    dl,
		Env:   winenv.NewFake(),
		Links: winenv.NewFake(),
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  root,
	}
}

// pythonRelease construit une release d'installeur .exe servie par srv.
func pythonRelease(srv *httptest.Server) sources.Release {
	return sources.Release{
		Version: "3.15.2",
		URL:     srv.URL + "/python-3.15.2-amd64.exe",
		Archive: "exe",
	}
}

// TestPythonInstallerArguments vérifie les arguments exacts de l'installeur.
func TestPythonInstallerArguments(t *testing.T) {
	srv, _ := pythonServer(t, []byte("MZ fake installer"))
	run := &execx.Fake{}
	d := pythonDeps(t, run)
	r := pythonRelease(srv)

	if err := NewPython().Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	call := run.Calls[0]
	if !strings.HasSuffix(strings.ToLower(call.Name), ".exe") {
		t.Fatalf("l'exécutable doit se terminer par .exe : %q", call.Name)
	}
	dir := ToolDir(d, "python", r.Version)
	want := []string{
		"/quiet",
		"InstallAllUsers=0",
		"PrependPath=0",
		"Include_launcher=0",
		"Include_test=0",
		"Shortcuts=0",
		"TargetDir=" + dir,
	}
	if len(call.Args) != len(want) {
		t.Fatalf("arguments = %v, attendu %v", call.Args, want)
	}
	for i := range want {
		if call.Args[i] != want[i] {
			t.Fatalf("argument %d = %q, attendu %q", i, call.Args[i], want[i])
		}
	}

	if _, ok := d.State.Installed("python", r.Version); !ok {
		t.Fatal("installation non enregistrée dans l'état")
	}
}

// TestPythonInstallIdempotent vérifie qu'une seconde installation ne fait rien.
func TestPythonInstallIdempotent(t *testing.T) {
	srv, count := pythonServer(t, []byte("MZ fake installer"))
	run := &execx.Fake{}
	d := pythonDeps(t, run)
	r := pythonRelease(srv)
	p := NewPython()

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("premier Install : %v", err)
	}
	first := *count
	calls := len(run.Calls)

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("second Install : %v", err)
	}
	if *count != first {
		t.Fatalf("requêtes réseau lors du second appel : %d -> %d", first, *count)
	}
	if len(run.Calls) != calls {
		t.Fatalf("commandes lors du second appel : %d -> %d", calls, len(run.Calls))
	}
}

// TestPythonActivateAddsScripts vérifie l'activation et l'ajout au Path.
func TestPythonActivateAddsScripts(t *testing.T) {
	run := &execx.Fake{}
	d := pythonDeps(t, run)
	env := d.Env.(*winenv.Fake)

	if err := NewPython().Activate(d, "3.15.2"); err != nil {
		t.Fatalf("Activate : %v", err)
	}

	pathVar, _, err := env.UserVar("Path")
	if err != nil {
		t.Fatalf("UserVar : %v", err)
	}
	current := CurrentDir(d, "python")
	if !strings.Contains(pathVar, current) {
		t.Fatalf("Path %q ne contient pas %q", pathVar, current)
	}
	scripts := filepath.Join(current, "Scripts")
	if !strings.Contains(pathVar, scripts) {
		t.Fatalf("Path %q ne contient pas %q", pathVar, scripts)
	}
	if d.State.Active["python"] != "3.15.2" {
		t.Fatalf("version active = %q", d.State.Active["python"])
	}
}

// TestPythonVerify vérifie l'extraction de la version.
func TestPythonVerify(t *testing.T) {
	run := &execx.Fake{Responses: map[string]execx.Result{}}
	d := pythonDeps(t, run)
	exe := filepath.Join(CurrentDir(d, "python"), "python.exe")
	run.Responses[exe+" --version"] = execx.Result{Stdout: "Python 3.15.2\n"}

	got, err := NewPython().Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if got != "3.15.2" {
		t.Fatalf("version = %q, attendu %q", got, "3.15.2")
	}
	if len(run.Calls) != 1 || run.Calls[0].Name != exe {
		t.Fatalf("commande inattendue : %+v", run.Calls)
	}
}
