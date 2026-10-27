package winenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrependPathDedupCaseInsensitive(t *testing.T) {
	got := PrependPath(`C:\A;c:\a\;C:\B`, `C:\A`)
	want := `C:\A;C:\B`
	if got != want {
		t.Fatalf("PrependPath = %q, attendu %q", got, want)
	}
}

func TestPrependPathWithSpaces(t *testing.T) {
	dir := `C:\Users\Jean Émile\x`
	got := PrependPath(`C:\B`, dir)
	want := dir + `;C:\B`
	if got != want {
		t.Fatalf("PrependPath = %q, attendu %q", got, want)
	}
}

func TestRemovePath(t *testing.T) {
	got := RemovePath(`C:\A;c:\a\;C:\B`, `C:\A`)
	want := `C:\B`
	if got != want {
		t.Fatalf("RemovePath = %q, attendu %q", got, want)
	}
}

func TestShadowing(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir1, "java.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "java.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	machinePath := dir1 + ";" + dir2 + ";" + dir3
	got := Shadowing(machinePath, "java.exe", []string{dir2})
	if len(got) != 1 || got[0] != dir1 {
		t.Fatalf("Shadowing = %v, attendu [%s]", got, dir1)
	}
}

func TestFakeCredsRoundTrip(t *testing.T) {
	f := NewFake()
	if err := f.Set("github", "user", "secret"); err != nil {
		t.Fatal(err)
	}
	user, secret, err := f.Get("github")
	if err != nil {
		t.Fatal(err)
	}
	if user != "user" || secret != "secret" {
		t.Fatalf("Get = (%q, %q)", user, secret)
	}
	if err := f.Delete("github"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Get("github"); err != ErrNotFound {
		t.Fatalf("Get après Delete err = %v, attendu ErrNotFound", err)
	}
}
