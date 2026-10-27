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

// vaultcliZip construit une archive zip contenant vault.exe à la racine.
func vaultcliZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("vault.exe")
	if err != nil {
		t.Fatalf("création de l'entrée zip : %v", err)
	}
	if _, err := w.Write([]byte("binaire")); err != nil {
		t.Fatalf("écriture de l'entrée zip : %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture du zip : %v", err)
	}
	return buf.Bytes()
}

// vaultcliServer sert l'archive zip de test.
func vaultcliServer(t *testing.T) *httptest.Server {
	t.Helper()
	data := vaultcliZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// vaultcliDeps prépare des dépendances de test.
func vaultcliDeps(t *testing.T) (Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	env := winenv.NewFake()
	run := &execx.Fake{}
	d := Deps{
		Env:   env,
		Links: env,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, env, run
}

// TestVaultHelperCmdContent vérifie le contenu exact du script de token helper.
func TestVaultHelperCmdContent(t *testing.T) {
	srv := vaultcliServer(t)
	d, _, _ := vaultcliDeps(t)
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("download.New : %v", err)
	}
	d.DL = dl

	binDir := t.TempDir()
	homeDir := t.TempDir()
	v := NewVaultCLI("https://vault.example.com:8200", "", binDir, homeDir)
	r := sources.Release{Version: "1.20.4", URL: srv.URL + "/vault.zip", Archive: "zip"}
	if err := v.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	data, err := os.ReadFile(filepath.Join(binDir, "vault-token-helper.cmd"))
	if err != nil {
		t.Fatalf("lecture du script : %v", err)
	}
	want := "@\"%~dp0devstation.exe\" vault-token-helper %*\r\n"
	if string(data) != want {
		t.Fatalf("contenu du script = %q, attendu %q", string(data), want)
	}
}

// TestVaultConfigEscapesBackslashes vérifie le doublement des antislashs.
func TestVaultConfigEscapesBackslashes(t *testing.T) {
	srv := vaultcliServer(t)
	d, _, _ := vaultcliDeps(t)
	dl, err := download.New(download.Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("download.New : %v", err)
	}
	d.DL = dl

	binDir := t.TempDir()
	homeDir := t.TempDir()
	v := NewVaultCLI("", "", binDir, homeDir)
	r := sources.Release{Version: "1.20.4", URL: srv.URL + "/vault.zip", Archive: "zip"}
	if err := v.Install(context.Background(), d, r, nil); err != nil {
		t.Fatalf("Install : %v", err)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".vault"))
	if err != nil {
		t.Fatalf("lecture de .vault : %v", err)
	}
	helper := filepath.Join(binDir, "vault-token-helper.cmd")
	want := "token_helper = \"" + strings.ReplaceAll(helper, `\`, `\\`) + "\"\n"
	if string(data) != want {
		t.Fatalf("contenu de .vault = %q, attendu %q", string(data), want)
	}
	if strings.Contains(string(data), `\`) && !strings.Contains(string(data), `\\`) {
		t.Fatalf("les antislashs ne sont pas doublés : %q", string(data))
	}
}

// TestVaultActivateSetsEnv vérifie l'activation et les variables d'environnement.
func TestVaultActivateSetsEnv(t *testing.T) {
	d, env, _ := vaultcliDeps(t)
	v := NewVaultCLI("https://vault.example.com:8200", `C:\certs\ca.pem`, t.TempDir(), t.TempDir())
	if err := v.Activate(d, "1.20.4"); err != nil {
		t.Fatalf("Activate : %v", err)
	}
	if got := env.Vars["VAULT_ADDR"]; got != "https://vault.example.com:8200" {
		t.Fatalf("VAULT_ADDR = %q", got)
	}
	if got := env.Vars["VAULT_CACERT"]; got != `C:\certs\ca.pem` {
		t.Fatalf("VAULT_CACERT = %q", got)
	}
	if got := d.State.Active["vault"]; got != "1.20.4" {
		t.Fatalf("version active = %q", got)
	}
	if env.Broadcasts == 0 {
		t.Fatal("aucune notification d'environnement")
	}
}

// TestVaultVerify vérifie l'extraction de la version depuis "vault version".
func TestVaultVerify(t *testing.T) {
	d, _, run := vaultcliDeps(t)
	v := NewVaultCLI("", "", t.TempDir(), t.TempDir())
	exe := filepath.Join(CurrentDir(d, "vault"), "vault.exe")
	run.Responses = map[string]execx.Result{
		exe + " version": {Stdout: "Vault v1.20.4 (abcdef), built 2024-01-01T00:00:00Z\n"},
	}
	got, err := v.Verify(context.Background(), d)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}
	if got != "1.20.4" {
		t.Fatalf("version = %q, attendu 1.20.4", got)
	}
	if len(run.Calls) != 1 {
		t.Fatalf("appels = %d, attendu 1", len(run.Calls))
	}
	if run.Calls[0].Name != exe || len(run.Calls[0].Args) != 1 || run.Calls[0].Args[0] != "version" {
		t.Fatalf("commande inattendue : %v", run.Calls[0])
	}
}
