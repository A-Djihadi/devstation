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
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// nodeZip construit une archive zip contenant un dossier racine "node-v22.22.3-win-x64"
// avec les fichiers fournis.
func nodeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create("node-v22.22.3-win-x64/" + name)
		if err != nil {
			t.Fatalf("création de l'entrée zip : %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("écriture de l'entrée zip : %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture du zip : %v", err)
	}
	return buf.Bytes()
}

// nodeTestServer sert l'archive fournie et retourne le serveur ainsi que la release associée.
func nodeTestServer(t *testing.T, data []byte) (*httptest.Server, sources.Release) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, sources.Release{
		Version: "22.22.3",
		URL:     srv.URL + "/node.zip",
		Archive: "zip",
	}
}

// nodeDeps construit des dépendances de test avec un serveur d'archive.
func nodeDeps(t *testing.T, srv *httptest.Server) (Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	env := winenv.NewFake()
	run := &execx.Fake{Responses: map[string]execx.Result{}}
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("création du client de téléchargement : %v", err)
	}
	d := Deps{
		DL:    dl,
		Env:   env,
		Links: env,
		Run:   run,
		Root:  t.TempDir(),
	}
	return d, env, run
}

// nodeInstall installe Node.js avec l'installeur fourni et retourne les dépendances.
func nodeInstall(t *testing.T, n *Node) (Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	data := nodeZip(t, map[string]string{
		"node.exe": "binaire",
		"npm.cmd":  "script",
	})
	srv, rel := nodeTestServer(t, data)
	d, env, run := nodeDeps(t, srv)
	if err := n.Install(context.Background(), d, rel, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}
	return d, env, run
}

// TestNodeInstallCommands vérifie l'ordre et les arguments exacts des commandes npm.
func TestNodeInstallCommands(t *testing.T) {
	n := NewNode("10.9.0", "17.3.0", "")
	d, _, run := nodeInstall(t, n)

	dir := ToolDir(d, "node", "22.22.3")
	prefix := filepath.Join(d.Root, "node", "global")
	npm := filepath.Join(dir, "npm.cmd")

	want := []execx.Cmd{
		{Name: npm, Args: []string{"config", "set", "prefix", prefix, "--location=user"}},
		{Name: npm, Args: []string{"install", "-g", "npm@10.9.0"}},
		{Name: npm, Args: []string{"install", "-g", "@angular/cli@17.3.0"}},
	}
	if len(run.Calls) != len(want) {
		t.Fatalf("nombre d'appels = %d, attendu %d : %+v", len(run.Calls), len(want), run.Calls)
	}
	for i, w := range want {
		got := run.Calls[i]
		if got.Name != w.Name {
			t.Errorf("appel %d : Name = %q, attendu %q", i, got.Name, w.Name)
		}
		if strings.Join(got.Args, " ") != strings.Join(w.Args, " ") {
			t.Errorf("appel %d : Args = %v, attendu %v", i, got.Args, w.Args)
		}
	}
}

// TestNodeBundledNpmSkipsUpgrade vérifie que npm fourni avec Node.js n'est pas mis à jour.
func TestNodeBundledNpmSkipsUpgrade(t *testing.T) {
	n := NewNode("bundled", "", "")
	d, _, run := nodeInstall(t, n)

	dir := ToolDir(d, "node", "22.22.3")
	prefix := filepath.Join(d.Root, "node", "global")
	npm := filepath.Join(dir, "npm.cmd")

	if len(run.Calls) != 1 {
		t.Fatalf("nombre d'appels = %d, attendu 1 : %+v", len(run.Calls), run.Calls)
	}
	got := run.Calls[0]
	if got.Name != npm {
		t.Errorf("Name = %q, attendu %q", got.Name, npm)
	}
	wantArgs := []string{"config", "set", "prefix", prefix, "--location=user"}
	if strings.Join(got.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("Args = %v, attendu %v", got.Args, wantArgs)
	}
}

// TestNodeActivateAddsGlobalPrefix vérifie l'activation : jonction current,
// dossier global dans le Path et NODE_EXTRA_CA_CERTS.
func TestNodeActivateAddsGlobalPrefix(t *testing.T) {
	env := winenv.NewFake()
	d := Deps{Env: env, Links: env, Root: t.TempDir()}
	n := NewNode("", "", `C:\certs\bundle.pem`)

	if err := n.Activate(d, "22.22.3"); err != nil {
		t.Fatalf("Activate : %v", err)
	}

	current := CurrentDir(d, "node")
	if target, ok := env.Junctions[current]; !ok || target != ToolDir(d, "node", "22.22.3") {
		t.Errorf("jonction current = %q (présente=%v)", target, ok)
	}

	pathVar := env.Vars["Path"]
	global := filepath.Join(d.Root, "node", "global")
	if !strings.Contains(pathVar, current) {
		t.Errorf("Path %q ne contient pas %q", pathVar, current)
	}
	if !strings.Contains(pathVar, global) {
		t.Errorf("Path %q ne contient pas %q", pathVar, global)
	}
	if env.Vars["NODE_EXTRA_CA_CERTS"] != `C:\certs\bundle.pem` {
		t.Errorf("NODE_EXTRA_CA_CERTS = %q", env.Vars["NODE_EXTRA_CA_CERTS"])
	}
}

// TestNodeVerify vérifie la lecture de la version de node.exe.
func TestNodeVerify(t *testing.T) {
	env := winenv.NewFake()
	d := Deps{Env: env, Links: env, Root: t.TempDir()}
	n := NewNode("", "", "")

	exe := filepath.Join(CurrentDir(d, "node"), "node.exe")
	run := &execx.Fake{Responses: map[string]execx.Result{
		exe + " --version": {Stdout: "v22.22.3\n"},
	}}
	d.Run = run

	got, err := n.Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if got != "22.22.3" {
		t.Errorf("version = %q, attendu %q", got, "22.22.3")
	}
	if len(run.Calls) != 1 || run.Calls[0].Name != exe {
		t.Fatalf("appels = %+v", run.Calls)
	}
	if len(run.Calls[0].Args) != 1 || run.Calls[0].Args[0] != "--version" {
		t.Errorf("Args = %v", run.Calls[0].Args)
	}
}

// nodeEnsureUnused évite un import inutilisé de os si le fichier évolue.
var _ = os.Getenv
