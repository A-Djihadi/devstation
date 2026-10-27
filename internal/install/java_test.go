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
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// javaTestArchive construit une archive zip contenant bin/java.exe et
// bin/keytool.exe sous un dossier racine, et retourne son contenu.
func javaTestArchive(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "java.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("création de l'archive : %v", err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"jdk-21/bin/java.exe", "jdk-21/bin/keytool.exe"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("ajout de %q : %v", name, err)
		}
		if _, err := w.Write([]byte("fake")); err != nil {
			t.Fatalf("écriture de %q : %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture du zip : %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("fermeture du fichier : %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de l'archive : %v", err)
	}
	return data
}

// javaTestDeps prépare des dépendances de test avec un serveur servant
// l'archive fournie.
func javaTestDeps(t *testing.T, data []byte) (Deps, *execx.Fake, *winenv.Fake) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("création du client de téléchargement : %v", err)
	}
	run := &execx.Fake{Responses: map[string]execx.Result{}}
	env := winenv.NewFake()
	d := Deps{
		DL:    dl,
		Env:   env,
		Links: env,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, run, env
}

// javaTestRelease construit la release de test pointant vers srv.
func javaTestRelease(srvURL string) sources.Release {
	return sources.Release{Version: "21.0.12", URL: srvURL, Archive: "zip"}
}

// TestJavaInstallImportsCAs vérifie que chaque certificat est importé via
// keytool avec les arguments attendus.
func TestJavaInstallImportsCAs(t *testing.T) {
	data := javaTestArchive(t)
	d, run, _ := javaTestDeps(t, data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("création du client de téléchargement : %v", err)
	}
	d.DL = dl

	certs := []string{`C:\certs\a.pem`, `C:\certs\b.pem`}
	j := NewJava(certs)
	if err := j.Install(context.Background(), d, javaTestRelease(srv.URL), nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	dir := ToolDir(d, "java", "21.0.12")
	keytool := filepath.Join(dir, "bin", "keytool.exe")
	if len(run.Calls) != 2 {
		t.Fatalf("appels keytool = %d, attendu 2", len(run.Calls))
	}
	for i, cert := range certs {
		call := run.Calls[i]
		if call.Name != keytool {
			t.Errorf("appel %d : Name = %q, attendu %q", i, call.Name, keytool)
		}
		want := []string{
			"-importcert", "-noprompt", "-cacerts",
			"-storepass", "changeit",
			"-alias", "devstation-" + string(rune('1'+i)),
			"-file", cert,
		}
		if len(call.Args) != len(want) {
			t.Fatalf("appel %d : Args = %v, attendu %v", i, call.Args, want)
		}
		for k := range want {
			if call.Args[k] != want[k] {
				t.Errorf("appel %d : Args[%d] = %q, attendu %q", i, k, call.Args[k], want[k])
			}
		}
	}
}

// TestJavaImportAlreadyExistsOK vérifie qu'un certificat déjà présent n'est
// pas une erreur.
func TestJavaImportAlreadyExistsOK(t *testing.T) {
	data := javaTestArchive(t)
	d, run, _ := javaTestDeps(t, data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer srv.Close()
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("création du client de téléchargement : %v", err)
	}
	d.DL = dl

	dir := ToolDir(d, "java", "21.0.12")
	keytool := filepath.Join(dir, "bin", "keytool.exe")
	key := keytool + " -importcert -noprompt -cacerts -storepass changeit -alias devstation-1 -file C:\\certs\\a.pem"
	run.Responses[key] = execx.Result{
		Stderr: "Certificate already exists in keystore under alias <devstation-1>",
		Code:   1,
	}

	j := NewJava([]string{`C:\certs\a.pem`})
	if err := j.Install(context.Background(), d, javaTestRelease(srv.URL), nil); err != nil {
		t.Fatalf("Install : %v", err)
	}
}

// TestJavaActivateSetsJavaHome vérifie l'activation et l'écriture de JAVA_HOME.
func TestJavaActivateSetsJavaHome(t *testing.T) {
	d, _, env := javaTestDeps(t, nil)
	j := NewJava(nil)
	if err := j.Activate(d, "21.0.12"); err != nil {
		t.Fatalf("Activate : %v", err)
	}

	current := CurrentDir(d, "java")
	if got := env.Junctions[current]; got != ToolDir(d, "java", "21.0.12") {
		t.Errorf("jonction current = %q", got)
	}
	if got := env.Vars["JAVA_HOME"]; got != current {
		t.Errorf("JAVA_HOME = %q, attendu %q", got, current)
	}
	if d.State.Active["java"] != "21.0.12" {
		t.Errorf("version active = %q", d.State.Active["java"])
	}
}

// TestJavaVerifyParsesStderr vérifie l'extraction de la version depuis Stderr.
func TestJavaVerifyParsesStderr(t *testing.T) {
	d, run, _ := javaTestDeps(t, nil)
	java := filepath.Join(CurrentDir(d, "java"), "bin", "java.exe")
	run.Responses[java+" -version"] = execx.Result{
		Stderr: "openjdk version \"21.0.12\" 2025-10-21 LTS\nOpenJDK Runtime Environment",
	}

	j := NewJava(nil)
	got, err := j.Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if got != "21.0.12" {
		t.Errorf("version = %q, attendu %q", got, "21.0.12")
	}
}
