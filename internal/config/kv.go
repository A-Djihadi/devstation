// Package config gère l'édition des fichiers de configuration clé=valeur
// (par exemple .npmrc ou gradle.properties) avec sauvegarde et restauration.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/A-Djihadi/devstation/internal/state"
)

// backupSuffix est le suffixe du fichier de sauvegarde unique par fichier.
const backupSuffix = ".devstation.bak"

// BackupOnce copie path vers path+".devstation.bak" si ce .bak n'existe pas
// encore et si path existe ; ajoute le .bak au champ st.Backups sans doublon.
func BackupOnce(path string, st *state.State) error {
	bak := path + backupSuffix
	if _, err := os.Stat(bak); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(bak), 0o755); err != nil {
		return err
	}
	if err := writeAtomic(bak, data); err != nil {
		return err
	}
	for _, existing := range st.Backups {
		if existing == bak {
			return nil
		}
	}
	st.Backups = append(st.Backups, bak)
	return nil
}

// SetKeys applique les paires clé=valeur de kv au fichier path. Le fichier est
// créé si absent ; une sauvegarde unique est réalisée avant toute écriture.
// Les lignes non gérées sont conservées à l'octet près.
func SetKeys(path string, kv map[string]string, st *state.State) error {
	if err := BackupOnce(path, st); err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content := string(original)
	eol := detectEOL(content)
	lines := splitLines(content)

	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := kv[key]
		idx, old, found := findKeyLine(lines, key)
		if found {
			prev := old
			st.RecordKey(path, key, &prev)
			lines[idx] = key + "=" + value
		} else {
			st.RecordKey(path, key, nil)
			lines = append(lines, key+"="+value)
		}
	}

	out := strings.Join(lines, eol)
	if len(lines) > 0 && !strings.HasSuffix(out, eol) {
		out += eol
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, []byte(out))
}

// RestoreKeys restaure les clés gérées du fichier path à partir de st.
// Une clé dont la valeur d'origine est nil est supprimée ; les autres lignes
// sont conservées. Un fichier absent ne provoque pas d'erreur.
func RestoreKeys(path string, st *state.State) error {
	managed := st.KeysFor(path)
	if len(managed) == 0 {
		return nil
	}
	original, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	content := string(original)
	eol := detectEOL(content)
	lines := splitLines(content)

	for _, mk := range managed {
		idx, _, found := findKeyLine(lines, mk.Key)
		if mk.Original == nil {
			if found {
				lines = append(lines[:idx], lines[idx+1:]...)
			}
			continue
		}
		if found {
			lines[idx] = mk.Key + "=" + *mk.Original
		} else {
			lines = append(lines, mk.Key+"="+*mk.Original)
		}
	}

	out := strings.Join(lines, eol)
	if len(lines) > 0 && !strings.HasSuffix(out, eol) {
		out += eol
	}
	return writeAtomic(path, []byte(out))
}

// detectEOL renvoie la fin de ligne dominante du contenu ("\r\n" ou "\n").
func detectEOL(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// splitLines découpe le contenu en lignes sans leur fin de ligne.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// findKeyLine cherche une ligne non commentée "clé=valeur" dont la clé
// correspond exactement. Renvoie l'index, l'ancienne valeur et un booléen.
func findKeyLine(lines []string, key string) (int, string, bool) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			continue
		}
		if strings.TrimSpace(trimmed[:eq]) == key {
			return i, strings.TrimSpace(trimmed[eq+1:]), true
		}
	}
	return -1, "", false
}

// writeAtomic écrit data dans path via un fichier temporaire puis rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
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
