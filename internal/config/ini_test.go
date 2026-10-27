package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/A-Djihadi/devstation/internal/state"
)

// iniRead lit le contenu du fichier de test.
func iniRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture %s : %v", path, err)
	}
	return string(data)
}

// iniWrite écrit le contenu initial du fichier de test.
func iniWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir : %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("écriture %s : %v", path, err)
	}
}

func TestINIAddsSectionWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pip.ini")
	iniWrite(t, path, "[global]\nindex-url = https://pypi.example.com/simple\n")
	st := &state.State{}

	if err := SetINI(path, "proxy", map[string]string{"http": "http://127.0.0.1:3128"}, st); err != nil {
		t.Fatalf("SetINI : %v", err)
	}
	got := iniRead(t, path)
	want := "[global]\nindex-url = https://pypi.example.com/simple\n[proxy]\nhttp=http://127.0.0.1:3128\n"
	if got != want {
		t.Fatalf("contenu = %q, attendu %q", got, want)
	}
}

func TestINIOnlyTouchesItsSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "px.ini")
	iniWrite(t, path, "[global]\nhttp = ancien\n[proxy]\nhttp = autre\n")
	st := &state.State{}

	if err := SetINI(path, "global", map[string]string{"http": "http://127.0.0.1:3128"}, st); err != nil {
		t.Fatalf("SetINI : %v", err)
	}
	got := iniRead(t, path)
	want := "[global]\nhttp=http://127.0.0.1:3128\n[proxy]\nhttp = autre\n"
	if got != want {
		t.Fatalf("contenu = %q, attendu %q", got, want)
	}
}

func TestINIPreservesCommentsAndCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pip.ini")
	iniWrite(t, path, "; commentaire\r\n[global]\r\n# autre\r\nindex-url: https://pypi.example.com/simple\r\n")
	st := &state.State{}

	if err := SetINI(path, "global", map[string]string{"index-url": "https://mirror.example.com/simple"}, st); err != nil {
		t.Fatalf("SetINI : %v", err)
	}
	got := iniRead(t, path)
	want := "; commentaire\r\n[global]\r\n# autre\r\nindex-url=https://mirror.example.com/simple\r\n"
	if got != want {
		t.Fatalf("contenu = %q, attendu %q", got, want)
	}
}

func TestINIRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pip.ini")
	iniWrite(t, path, "[global]\nindex-url = https://pypi.example.com/simple\n")
	st := &state.State{}

	if err := SetINI(path, "global", map[string]string{
		"index-url": "https://mirror.example.com/simple",
		"timeout":   "30",
	}, st); err != nil {
		t.Fatalf("SetINI : %v", err)
	}
	if err := RestoreINI(path, "global", st); err != nil {
		t.Fatalf("RestoreINI : %v", err)
	}
	got := iniRead(t, path)
	want := "[global]\nindex-url=https://pypi.example.com/simple\n"
	if got != want {
		t.Fatalf("contenu = %q, attendu %q", got, want)
	}
}

func TestINIBackupOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pip.ini")
	iniWrite(t, path, "[global]\nindex-url = https://pypi.example.com/simple\n")
	st := &state.State{}

	if err := SetINI(path, "global", map[string]string{"index-url": "https://a.example.com/simple"}, st); err != nil {
		t.Fatalf("SetINI 1 : %v", err)
	}
	if err := SetINI(path, "global", map[string]string{"index-url": "https://b.example.com/simple"}, st); err != nil {
		t.Fatalf("SetINI 2 : %v", err)
	}
	bak := path + backupSuffix
	got := iniRead(t, bak)
	want := "[global]\nindex-url = https://pypi.example.com/simple\n"
	if got != want {
		t.Fatalf("sauvegarde = %q, attendu %q", got, want)
	}
	count := 0
	for _, b := range st.Backups {
		if b == bak {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("sauvegardes enregistrées = %d, attendu 1", count)
	}
}
