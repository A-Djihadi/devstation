package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/config"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// manageTestDeps construit des dépendances de test avec un Fake et un état vide.
func manageTestDeps(t *testing.T) (install.Deps, *winenv.Fake, *execx.Fake) {
	t.Helper()
	fake := winenv.NewFake()
	run := &execx.Fake{}
	d := install.Deps{
		Env:   fake,
		Links: fake,
		Certs: fake,
		Creds: fake,
		Run:   run,
		State: &state.State{Active: map[string]string{}},
		Root:  t.TempDir(),
	}
	return d, fake, run
}

// TestReposSyncSkipsExisting vérifie qu'un dépôt déjà présent n'est pas recloné.
func TestReposSyncSkipsExisting(t *testing.T) {
	run := &execx.Fake{}
	workspace := t.TempDir()
	existing := filepath.Join(workspace, "api")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &profile.Profile{Repos: []profile.Repo{
		{Name: "api", URL: "https://git.example.com/api.git"},
		{Name: "web", URL: "https://git.example.com/web.git"},
	}}
	a := &profile.Answers{Repos: []string{"api", "web"}, Workspace: workspace}

	var out bytes.Buffer
	if err := manageReposSync(context.Background(), &out, run, p, a); err != nil {
		t.Fatalf("manageReposSync : %v", err)
	}
	if !strings.Contains(out.String(), "↷ api déjà présent") {
		t.Errorf("sortie attendue pour api : %q", out.String())
	}
	if !strings.Contains(out.String(), "✓ web cloné") {
		t.Errorf("sortie attendue pour web : %q", out.String())
	}
	if len(run.Calls) != 1 {
		t.Fatalf("appels attendus : 1, obtenus : %d", len(run.Calls))
	}
	if run.Calls[0].Name != "git" || run.Calls[0].Args[0] != "clone" {
		t.Errorf("commande inattendue : %s", run.Calls[0].String())
	}
}

// TestUninstallToolCleansPathEnvAndState vérifie le retrait d'un outil.
func TestUninstallToolCleansPathEnvAndState(t *testing.T) {
	d, fake, _ := manageTestDeps(t)
	javaDir := filepath.Join(d.Root, "java")
	nodeDir := filepath.Join(d.Root, "node")
	if err := os.MkdirAll(javaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake.Vars["Path"] = filepath.Join(javaDir, "current", "bin") + ";" + filepath.Join(nodeDir, "current") + ";C:\\Windows"
	fake.Vars["JAVA_HOME"] = javaDir
	d.State.Installs = []state.Install{
		{Tool: "java", Version: "21", Path: javaDir},
		{Tool: "node", Version: "20", Path: nodeDir},
	}
	d.State.Active["java"] = "21"
	d.State.Active["node"] = "20"

	var out bytes.Buffer
	if err := manageUninstall(context.Background(), &out, d, "java", false); err != nil {
		t.Fatalf("manageUninstall : %v", err)
	}

	pathVar := fake.Vars["Path"]
	if strings.Contains(strings.ToLower(pathVar), strings.ToLower(javaDir)) {
		t.Errorf("Path contient encore java : %q", pathVar)
	}
	if !strings.Contains(pathVar, nodeDir) {
		t.Errorf("Path a perdu node : %q", pathVar)
	}
	if _, ok := fake.Vars["JAVA_HOME"]; ok {
		t.Error("JAVA_HOME devrait être supprimé")
	}
	if _, err := os.Stat(javaDir); !os.IsNotExist(err) {
		t.Errorf("dossier java toujours présent : %v", err)
	}
	if _, ok := d.State.Installed("java", "21"); ok {
		t.Error("état contient encore java")
	}
	if _, ok := d.State.Installed("node", "20"); !ok {
		t.Error("état a perdu node")
	}
	if fake.Broadcasts != 1 {
		t.Errorf("notifications attendues : 1, obtenues : %d", fake.Broadcasts)
	}
}

// TestUninstallAllRestoresConfigs vérifie la restauration des fichiers gérés.
func TestUninstallAllRestoresConfigs(t *testing.T) {
	d, fake, _ := manageTestDeps(t)
	home := t.TempDir()
	npmrc := filepath.Join(home, ".npmrc")
	original := "registry=https://registry.example.com/\n"
	if err := os.WriteFile(npmrc, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.SetKeys(npmrc, map[string]string{"proxy": "http://proxy.example.com:3128"}, d.State); err != nil {
		t.Fatalf("SetKeys : %v", err)
	}
	fake.Vars["HTTP_PROXY"] = "http://proxy.example.com:3128"

	var out bytes.Buffer
	if err := manageUninstall(context.Background(), &out, d, "", true); err != nil {
		t.Fatalf("manageUninstall : %v", err)
	}

	data, err := os.ReadFile(npmrc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("contenu restauré : %q, attendu : %q", string(data), original)
	}
	if _, ok := fake.Vars["HTTP_PROXY"]; ok {
		t.Error("HTTP_PROXY devrait être supprimé")
	}
	if len(d.State.Keys) != 0 {
		t.Errorf("clés gérées restantes : %d", len(d.State.Keys))
	}
}

// TestSecretsGetCopyNeverPrintsValue vérifie que --copy n'affiche pas la valeur.
func TestSecretsGetCopyNeverPrintsValue(t *testing.T) {
	run := &execx.Fake{Responses: map[string]execx.Result{
		"vault.exe kv get -field=password secret/team": {Stdout: "s3cr3t-value\n"},
	}}
	var out bytes.Buffer
	if err := manageSecretsGet(context.Background(), &out, run, "vault.exe", "secret/team", "password", true); err != nil {
		t.Fatalf("manageSecretsGet : %v", err)
	}
	if strings.Contains(out.String(), "s3cr3t-value") {
		t.Errorf("la valeur apparaît dans la sortie : %q", out.String())
	}
	if !strings.Contains(out.String(), "Copié dans le presse-papiers") {
		t.Errorf("message de copie absent : %q", out.String())
	}
	found := false
	for _, c := range run.Calls {
		if c.Name == "clip.exe" {
			found = true
			if !c.Secret {
				t.Error("clip.exe devrait être marqué secret")
			}
			if c.Stdin != "s3cr3t-value" {
				t.Errorf("entrée de clip.exe : %q", c.Stdin)
			}
		}
	}
	if !found {
		t.Error("clip.exe n'a pas été appelé")
	}
}

// TestVaultTokenHelperIsHidden vérifie que la commande est cachée.
func TestVaultTokenHelperIsHidden(t *testing.T) {
	cmd := newVaultTokenHelperCmd()
	if !cmd.Hidden {
		t.Error("vault-token-helper devrait être cachée")
	}
}
