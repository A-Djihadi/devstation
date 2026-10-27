package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s == nil {
		t.Fatal("état nil")
	}
	if len(s.Installs) != 0 || len(s.Keys) != 0 || len(s.Backups) != 0 {
		t.Fatalf("état non vide: %+v", s)
	}
	if s.Active == nil {
		t.Fatal("Active doit être initialisé")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	orig := "valeur-originale"
	s := &State{}
	s.RecordInstall(Install{Tool: "git", Version: "2.45.0", Path: `C:\tools\git`, At: now})
	s.RecordKey(`C:\cfg\gitconfig`, "user.name", &orig)
	s.Backups = append(s.Backups, `C:\cfg\gitconfig.devstation.bak`)
	s.Active["git"] = "2.45.0"
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Installs) != 1 || got.Installs[0].Tool != "git" || got.Installs[0].Version != "2.45.0" {
		t.Fatalf("installs: %+v", got.Installs)
	}
	if !got.Installs[0].At.Equal(now) {
		t.Fatalf("At: %v", got.Installs[0].At)
	}
	keys := got.KeysFor(`C:\cfg\gitconfig`)
	if len(keys) != 1 || keys[0].Original == nil || *keys[0].Original != orig {
		t.Fatalf("keys: %+v", keys)
	}
	if len(got.Backups) != 1 || got.Backups[0] != `C:\cfg\gitconfig.devstation.bak` {
		t.Fatalf("backups: %+v", got.Backups)
	}
	if got.Active["git"] != "2.45.0" {
		t.Fatalf("active: %+v", got.Active)
	}
}

func TestRecordKeyFirstWins(t *testing.T) {
	s := &State{}
	first := "premier"
	second := "second"
	s.RecordKey("f", "k", &first)
	s.RecordKey("f", "k", &second)
	keys := s.KeysFor("f")
	if len(keys) != 1 {
		t.Fatalf("attendu 1 clé, obtenu %d", len(keys))
	}
	if keys[0].Original == nil || *keys[0].Original != first {
		t.Fatalf("Original du premier appel attendu, obtenu %+v", keys[0].Original)
	}
}

func TestRecordInstallReplaces(t *testing.T) {
	s := &State{}
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	s.RecordInstall(Install{Tool: "git", Version: "2.45.0", Path: "a", At: t1})
	s.RecordInstall(Install{Tool: "git", Version: "2.45.0", Path: "b", At: t2})
	if len(s.Installs) != 1 {
		t.Fatalf("attendu 1 install, obtenu %d", len(s.Installs))
	}
	got, ok := s.Installed("git", "2.45.0")
	if !ok {
		t.Fatal("install non trouvée")
	}
	if got.Path != "b" || !got.At.Equal(t2) {
		t.Fatalf("remplacement non effectué: %+v", got)
	}
}