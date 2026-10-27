// Package state gère l'état local persistant de DevStation (state.json).
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Install décrit une installation d'outil enregistrée dans l'état.
type Install struct {
	Tool    string    `json:"tool"`
	Version string    `json:"version"`
	Path    string    `json:"path"`
	At      time.Time `json:"at"`
}

// ManagedKey décrit une clé de configuration gérée par DevStation.
// Original vaut nil lorsque la clé était absente à l'origine.
type ManagedKey struct {
	File     string  `json:"file"`
	Key      string  `json:"key"`
	Original *string `json:"original"`
}

// State est l'état local persistant de DevStation.
type State struct {
	Installs []Install         `json:"installs"`
	Keys     []ManagedKey      `json:"keys"`
	Backups  []string          `json:"backups"`
	Active   map[string]string `json:"active"`
}

// ensureActive initialise la map Active si elle est nil.
func (s *State) ensureActive() {
	if s.Active == nil {
		s.Active = map[string]string{}
	}
}

// Load lit l'état depuis path. Un fichier absent renvoie un état vide sans erreur.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{Active: map[string]string{}}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Active == nil {
		s.Active = map[string]string{}
	}
	return &s, nil
}

// Save écrit l'état dans path de façon atomique (fichier temporaire puis rename).
func (s *State) Save(path string) error {
	s.ensureActive()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// RecordInstall enregistre une installation, en remplaçant l'entrée (Tool, Version) existante.
func (s *State) RecordInstall(i Install) {
	s.ensureActive()
	for idx, existing := range s.Installs {
		if existing.Tool == i.Tool && existing.Version == i.Version {
			s.Installs[idx] = i
			return
		}
	}
	s.Installs = append(s.Installs, i)
}

// Installed indique si l'outil et la version sont déjà installés.
func (s *State) Installed(tool, version string) (Install, bool) {
	for _, existing := range s.Installs {
		if existing.Tool == tool && existing.Version == version {
			return existing, true
		}
	}
	return Install{}, false
}

// RecordKey enregistre une clé gérée. La première valeur enregistrée pour
// (file, key) est conservée : les appels suivants sont ignorés.
func (s *State) RecordKey(file, key string, original *string) {
	s.ensureActive()
	for _, existing := range s.Keys {
		if existing.File == file && existing.Key == key {
			return
		}
	}
	s.Keys = append(s.Keys, ManagedKey{File: file, Key: key, Original: original})
}

// KeysFor renvoie les clés gérées pour le fichier donné.
func (s *State) KeysFor(file string) []ManagedKey {
	var out []ManagedKey
	for _, k := range s.Keys {
		if k.File == file {
			out = append(out, k)
		}
	}
	return out
}
