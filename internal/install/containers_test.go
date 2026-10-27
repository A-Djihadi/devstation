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
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// containersZip construit une archive zip en mémoire à partir de fichiers
// nommés relativement.
func containersZip(t *testing.T, files map[string]string) []byte {
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

// containersServer sert data sur toutes les requêtes.
func containersServer(t *testing.T, data []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// containersDeps prépare des dépendances de test avec un exécuteur factice.
func containersDeps(t *testing.T) (Deps, *execx.Fake, *winenv.Fake) {
	t.Helper()
	run := &execx.Fake{Responses: map[string]execx.Result{}}
	env := winenv.NewFake()
	d := Deps{
		Env:   env,
		Links: env,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, run, env
}

// containersPodmanRelease construit une release Podman servie par srv.
func containersPodmanRelease(t *testing.T, srv *httptest.Server) sources.Release {
	t.Helper()
	return sources.Release{Version: "6.1.4", URL: srv.URL + "/podman.zip", Archive: "zip"}
}

// containersComposeRelease construit une release Compose servie par srv.
func containersComposeRelease(t *testing.T, srv *httptest.Server) sources.Release {
	t.Helper()
	return sources.Release{Version: "2.30.0", URL: srv.URL + "/compose.zip", Archive: "zip"}
}

// containersSetup prépare un installeur Podman complet pour les tests.
func containersSetup(t *testing.T) (*Podman, Deps, *execx.Fake, sources.Release) {
	t.Helper()
	srv := containersServer(t, containersZip(t, map[string]string{
		"podman/usr/bin/podman.exe": "binaire",
	}))
	composeSrv := containersServer(t, containersZip(t, map[string]string{"docker-compose.exe": "compose"}))
	d, run, _ := containersDeps(t)
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("client de téléchargement : %v", err)
	}
	d.DL = dl
	p := NewPodman(containersComposeRelease(t, composeSrv), nil, "")
	return p, d, run, containersPodmanRelease(t, srv)
}

// TestPodmanInitWhenNoMachine vérifie que machine init --now est lancé quand
// aucune machine n'existe.
func TestPodmanInitWhenNoMachine(t *testing.T) {
	p, d, run, r := containersSetup(t)
	run.Responses["wsl.exe --status"] = execx.Result{Code: 0}
	run.Responses[filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "podman.exe")+" machine list --format json"] = execx.Result{Stdout: "[]"}

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	var found bool
	for _, c := range run.Calls {
		if c.Name == filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "podman.exe") &&
			strings.Join(c.Args, " ") == "machine init --now" {
			found = true
		}
	}
	if !found {
		t.Fatalf("machine init --now non lancé : %+v", run.Calls)
	}
}

// TestPodmanSkipsInitWhenMachineExists vérifie que machine init n'est pas lancé
// quand une machine existe déjà.
func TestPodmanSkipsInitWhenMachineExists(t *testing.T) {
	p, d, run, r := containersSetup(t)
	run.Responses["wsl.exe --status"] = execx.Result{Code: 0}
	run.Responses[filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "podman.exe")+" machine list --format json"] = execx.Result{Stdout: `[{"Name":"podman-machine-default"}]`}

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	for _, c := range run.Calls {
		if strings.Join(c.Args, " ") == "machine init --now" {
			t.Fatalf("machine init --now lancé à tort")
		}
	}
}

// TestPodmanCopiesCACerts vérifie que les certificats sont écrits via tee avec
// leur contenu sur l'entrée standard.
func TestPodmanCopiesCACerts(t *testing.T) {
	p, d, run, r := containersSetup(t)
	run.Responses["wsl.exe --status"] = execx.Result{Code: 0}
	run.Responses[filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "podman.exe")+" machine list --format json"] = execx.Result{Stdout: `[{"Name":"m"}]`}

	certPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(certPath, []byte("PEMDATA"), 0o644); err != nil {
		t.Fatalf("écriture du certificat : %v", err)
	}
	p.CACerts = []string{certPath}

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	var found bool
	for _, c := range run.Calls {
		if strings.Contains(strings.Join(c.Args, " "), "devstation-0.pem") {
			if c.Stdin != "PEMDATA" {
				t.Fatalf("Stdin = %q, attendu PEMDATA", c.Stdin)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("écriture du certificat non lancée : %+v", run.Calls)
	}
}

// TestPodmanComposeCopied vérifie que docker-compose.exe est copié à côté de
// podman.exe.
func TestPodmanComposeCopied(t *testing.T) {
	p, d, run, r := containersSetup(t)
	run.Responses["wsl.exe --status"] = execx.Result{Code: 0}
	run.Responses[filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "podman.exe")+" machine list --format json"] = execx.Result{Stdout: `[{"Name":"m"}]`}

	if err := p.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	composePath := filepath.Join(d.Root, "podman", "6.1.4", "usr", "bin", "docker-compose.exe")
	if _, err := os.Stat(composePath); err != nil {
		t.Fatalf("docker-compose.exe absent : %v", err)
	}
}

// TestPodmanRequiresWSL vérifie que l'absence de WSL2 produit une erreur.
func TestPodmanRequiresWSL(t *testing.T) {
	p, d, run, r := containersSetup(t)
	run.Responses["wsl.exe --status"] = execx.Result{Code: 1, Stderr: "absent"}

	err := p.Install(context.Background(), d, r, nil)
	if err == nil {
		t.Fatal("erreur attendue sans WSL2")
	}
	if !strings.Contains(err.Error(), "WSL2 est requis") {
		t.Fatalf("message inattendu : %v", err)
	}
}
