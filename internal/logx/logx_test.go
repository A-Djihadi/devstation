package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactRegistered(t *testing.T) {
	got := Redact("token=abc123 et abc123", []string{"abc123"})
	if strings.Contains(got, "abc123") {
		t.Fatalf("secret non masqué : %q", got)
	}
}

func TestRedactGitHubPatterns(t *testing.T) {
	got := Redact("ghp_abcdef123 et github_pat_xyz_789", nil)
	if strings.Contains(got, "ghp_abcdef123") || strings.Contains(got, "github_pat_xyz_789") {
		t.Fatalf("motif GitHub non masqué : %q", got)
	}
}

func TestRedactNpmAuthToken(t *testing.T) {
	got := Redact("//registry.example.com/:_authToken=secretvalue", nil)
	if strings.Contains(got, "secretvalue") {
		t.Fatalf("_authToken non masqué : %q", got)
	}
}

func TestRedactPasswordCaseAndJSON(t *testing.T) {
	for _, in := range []string{"Password=abc", "PASSWORD: abc", `{"password":"abc"}`} {
		got := Redact(in, nil)
		if strings.Contains(got, "abc") {
			t.Fatalf("mot de passe non masqué pour %q : %q", in, got)
		}
	}
}

func TestRedactBearer(t *testing.T) {
	got := Redact("Authorization: Bearer xyz", nil)
	if strings.Contains(got, "xyz") {
		t.Fatalf("jeton Bearer non masqué : %q", got)
	}
}

func TestLoggerNeverWritesSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.AddSecret("s3cr3t")
	l.Printf("mot de passe : %s", "s3cr3t")
	l.Printf("token ghp_abcdef123")
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "s3cr3t") || strings.Contains(string(data), "ghp_abcdef123") {
		t.Fatalf("secret écrit dans le journal : %q", string(data))
	}
}