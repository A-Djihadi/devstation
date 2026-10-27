package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/state"
)

// newState crée un état vide pour les tests.
func newState() *state.State {
	return &state.State{Active: map[string]string{}}
}

// writeFile écrit le contenu donné dans un fichier temporaire et renvoie son chemin.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".npmrc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("écriture: %v", err)
	}
	return path
}

// readFile lit le contenu d'un fichier de test.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture: %v", err)
	}
	return string(data)
}

func TestPreservesUnmanagedBytes(t *testing.T) {
	orig := "# commentaire\r\n\r\n//registry.npmjs.org/:_authToken=secret\r\nregistry=https://old.example.com\r\n"
	path := writeFile(t, orig)
	st := newState()
	if err := SetKeys(path, map[string]string{"registry": "https://new.example.com"}, st); err != nil {
		t.Fatalf("SetKeys: %v", err)
	}
	got := readFile(t, path)
	want := "# commentaire\r\n\r\n//registry.npmjs.org/:_authToken=secret\r\nregistry=https://new.example.com\r\n"
	if got != want {
		t.Fatalf("contenu inattendu:\n%q\nattendu:\n%q", got, want)
	}
}

func TestBackupOnlyOnce(t *testing.T) {
	orig := "registry=https://first.example.com\n"
	path := writeFile(t, orig)
	st := newState()
	if err := SetKeys(path, map[string]string{"registry": "https://second.example.com"}, st); err != nil {
		t.Fatalf("SetKeys 1: %v", err)
	}
	if err := SetKeys(path, map[string]string{"registry": "https://third.example.com"}, st); err != nil {
		t.Fatalf("SetKeys 2: %v", err)
	}
	bak := readFile(t, path+backupSuffix)
	if bak != orig {
		t.Fatalf("sauvegarde inattendue: %q", bak)
	}
	if len(st.Backups) != 1 {
		t.Fatalf("Backups attendu 1, obtenu %d", len(st.Backups))
	}
}

func TestRestoreRemovesAddedKeys(t *testing.T) {
	path := writeFile(t, "registry=https://old.example.com\n")
	st := newState()
	if err := SetKeys(path, map[string]string{"always-auth": "true"}, st); err != nil {
		t.Fatalf("SetKeys: %v", err)
	}
	if err := RestoreKeys(path, st); err != nil {
		t.Fatalf("RestoreKeys: %v", err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "always-auth") {
		t.Fatalf("clé ajoutée non supprimée: %q", got)
	}
}

func TestRestoreRestoresOriginalValue(t *testing.T) {
	path := writeFile(t, "registry=https://old.example.com\n")
	st := newState()
	if err := SetKeys(path, map[string]string{"registry": "https://new.example.com"}, st); err != nil {
		t.Fatalf("SetKeys: %v", err)
	}
	if err := RestoreKeys(path, st); err != nil {
		t.Fatalf("RestoreKeys: %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "registry=https://old.example.com") {
		t.Fatalf("valeur d'origine non restaurée: %q", got)
	}
}

func TestRestoreKeepsLaterUserEdits(t *testing.T) {
	path := writeFile(t, "registry=https://old.example.com\n")
	st := newState()
	if err := SetKeys(path, map[string]string{"registry": "https://new.example.com"}, st); err != nil {
		t.Fatalf("SetKeys: %v", err)
	}
	content := readFile(t, path)
	content += "user-key=user-value\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("écriture: %v", err)
	}
	if err := RestoreKeys(path, st); err != nil {
		t.Fatalf("RestoreKeys: %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "user-key=user-value") {
		t.Fatalf("édition utilisateur perdue: %q", got)
	}
	if !strings.Contains(got, "registry=https://old.example.com") {
		t.Fatalf("valeur d'origine non restaurée: %q", got)
	}
}

func TestCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "gradle.properties")
	st := newState()
	if err := SetKeys(path, map[string]string{"org.gradle.jvmargs": "-Xmx2g"}, st); err != nil {
		t.Fatalf("SetKeys: %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "org.gradle.jvmargs=-Xmx2g") {
		t.Fatalf("fichier non créé correctement: %q", got)
	}
}
