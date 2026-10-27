package install

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// buildZip construit une archive zip en mémoire à partir des entrées fournies.
func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("création de l'entrée %q : %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("écriture de l'entrée %q : %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture du zip : %v", err)
	}
	return buf.Bytes()
}

// sha256Hex retourne l'empreinte sha256 hexadécimale de data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newDeps construit des dépendances de test avec un serveur httptest.
func newDeps(t *testing.T, handler http.HandlerFunc) (Deps, *winenv.Fake, *execx.Fake, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	dl, err := download.New(download.Options{
		CacheDir:   t.TempDir(),
		HTTPClient: srv.Client(),
		Retries:    1,
		Backoff:    func(int) time.Duration { return 0 },
	})
	if err != nil {
		t.Fatalf("création du client : %v", err)
	}

	fake := winenv.NewFake()
	run := &execx.Fake{}
	d := Deps{
		DL:    dl,
		Env:   fake,
		Links: fake,
		Certs: fake,
		Creds: fake,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, fake, run, srv
}

// TestInstallZipIdempotent vérifie qu'une seconde installation ne télécharge rien.
func TestInstallZipIdempotent(t *testing.T) {
	data := buildZip(t, map[string]string{"bin/java.exe": "java"})
	var hits int32
	d, _, _, srv := newDeps(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write(data)
	})

	r := sources.Release{
		Version:  "21.0.12",
		URL:      srv.URL + "/jdk.zip",
		Checksum: download.Checksum{Algo: "sha256", Hex: sha256Hex(data)},
		Archive:  "zip",
	}

	dir1, err := InstallZip(context.Background(), d, "java", r, false, nil)
	if err != nil {
		t.Fatalf("première installation : %v", err)
	}
	first := atomic.LoadInt32(&hits)

	dir2, err := InstallZip(context.Background(), d, "java", r, false, nil)
	if err != nil {
		t.Fatalf("seconde installation : %v", err)
	}
	if dir1 != dir2 {
		t.Fatalf("dossiers différents : %q et %q", dir1, dir2)
	}
	if got := atomic.LoadInt32(&hits); got != first {
		t.Fatalf("requêtes supplémentaires : %d puis %d", first, got)
	}
}

// TestInstallZipStripTop vérifie le retrait du dossier racine unique.
func TestInstallZipStripTop(t *testing.T) {
	data := buildZip(t, map[string]string{
		"jdk-21/bin/java.exe": "java",
		"jdk-21/README.txt":   "readme",
	})
	d, _, _, srv := newDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})

	r := sources.Release{
		Version:  "21.0.12",
		URL:      srv.URL + "/jdk.zip",
		Checksum: download.Checksum{Algo: "sha256", Hex: sha256Hex(data)},
		Archive:  "zip",
	}

	dir, err := InstallZip(context.Background(), d, "java", r, true, nil)
	if err != nil {
		t.Fatalf("installation : %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "bin", "java.exe")); err != nil {
		t.Fatalf("bin/java.exe absent : %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jdk-21")); err == nil {
		t.Fatalf("le dossier racine jdk-21 aurait dû être retiré")
	}
}

// TestInstallZipChecksumMismatch vérifie le rejet d'une empreinte invalide.
func TestInstallZipChecksumMismatch(t *testing.T) {
	data := buildZip(t, map[string]string{"bin/java.exe": "java"})
	d, _, _, srv := newDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})

	r := sources.Release{
		Version:  "21.0.12",
		URL:      srv.URL + "/jdk.zip",
		Checksum: download.Checksum{Algo: "sha256", Hex: "0000000000000000000000000000000000000000000000000000000000000000"},
		Archive:  "zip",
	}

	dir, err := InstallZip(context.Background(), d, "java", r, false, nil)
	if err == nil {
		t.Fatalf("une erreur était attendue")
	}
	if dir != "" {
		t.Fatalf("dossier retourné non vide : %q", dir)
	}

	expected := ToolDir(d, "java", "21.0.12")
	if _, statErr := os.Stat(expected); statErr == nil {
		t.Fatalf("le dossier %q ne devrait pas exister", expected)
	}
	if _, statErr := os.Stat(expected + ".partial"); statErr == nil {
		t.Fatalf("le dossier %q ne devrait pas exister", expected+".partial")
	}
	if len(d.State.Installs) != 0 {
		t.Fatalf("installation enregistrée malgré l'échec : %+v", d.State.Installs)
	}
}

// TestSetCurrentPathAndJunction vérifie la jonction, le Path et l'état actif.
func TestSetCurrentPathAndJunction(t *testing.T) {
	d, fake, _, _ := newDeps(t, func(w http.ResponseWriter, r *http.Request) {})

	if err := SetCurrent(d, "java", "21.0.12", []string{"bin"}); err != nil {
		t.Fatalf("activation : %v", err)
	}

	current := CurrentDir(d, "java")
	target := ToolDir(d, "java", "21.0.12")
	if got := fake.Junctions[current]; got != target {
		t.Fatalf("jonction %q -> %q, attendu %q", current, got, target)
	}

	pathVar, _, _ := fake.UserVar("Path")
	wantPrefix := filepath.Join(current, "bin")
	if len(pathVar) < len(wantPrefix) || pathVar[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("Path %q ne commence pas par %q", pathVar, wantPrefix)
	}

	if err := SetCurrent(d, "java", "21.0.12", []string{"bin"}); err != nil {
		t.Fatalf("seconde activation : %v", err)
	}
	pathVar2, _, _ := fake.UserVar("Path")
	if pathVar2 != pathVar {
		t.Fatalf("Path modifié au second appel : %q puis %q", pathVar, pathVar2)
	}

	if fake.Broadcasts != 2 {
		t.Fatalf("Broadcasts = %d, attendu 2", fake.Broadcasts)
	}
	if d.State.Active["java"] != "21.0.12" {
		t.Fatalf("version active = %q", d.State.Active["java"])
	}
}
