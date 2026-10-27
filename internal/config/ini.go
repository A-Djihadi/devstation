package config

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/A-Djihadi/devstation/internal/state"
)

// SetINI applique les paires clé=valeur de kv à la section [section] du
// fichier INI path. La section est ajoutée en fin de fichier si absente ; une
// sauvegarde unique est réalisée avant toute écriture. Les commentaires, les
// autres sections et les fins de ligne sont conservés à l'octet près.
func SetINI(path, section string, kv map[string]string, st *state.State) error {
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
		start, end, found := iniFindSection(lines, section)
		if !found {
			lines = append(lines, "["+section+"]")
			start, end = len(lines)-1, len(lines)
		}
		idx, old, keyFound := iniFindKeyLine(lines, start, end, key)
		if keyFound {
			prev := old
			st.RecordKey(path, section+"."+key, &prev)
			lines[idx] = key + "=" + value
		} else {
			st.RecordKey(path, section+"."+key, nil)
			lines = iniInsertLine(lines, end, key+"="+value)
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

// RestoreINI restaure les clés gérées de la section [section] du fichier path
// à partir de st. Une clé dont la valeur d'origine est nil est supprimée ; une
// section devenue vide et créée par DevStation est supprimée. Un fichier absent
// ne provoque pas d'erreur.
func RestoreINI(path, section string, st *state.State) error {
	managed := iniManagedFor(st, path, section)
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
		start, end, found := iniFindSection(lines, section)
		if !found {
			if mk.Original == nil {
				continue
			}
			lines = append(lines, "["+section+"]")
			start, end = len(lines)-1, len(lines)
		}
		idx, _, keyFound := iniFindKeyLine(lines, start, end, mk.Key)
		if mk.Original == nil {
			if keyFound {
				lines = append(lines[:idx], lines[idx+1:]...)
			}
			continue
		}
		if keyFound {
			lines[idx] = mk.Key + "=" + *mk.Original
		} else {
			lines = iniInsertLine(lines, end, mk.Key+"="+*mk.Original)
		}
	}

	lines = iniDropEmptySection(lines, section, managed)

	out := strings.Join(lines, eol)
	if len(lines) > 0 && !strings.HasSuffix(out, eol) {
		out += eol
	}
	return writeAtomic(path, []byte(out))
}

// iniManagedFor renvoie les clés gérées de st pour (path, section).
func iniManagedFor(st *state.State, path, section string) []state.ManagedKey {
	prefix := section + "."
	var out []state.ManagedKey
	for _, k := range st.KeysFor(path) {
		if strings.HasPrefix(k.Key, prefix) {
			out = append(out, state.ManagedKey{
				File:     k.File,
				Key:      strings.TrimPrefix(k.Key, prefix),
				Original: k.Original,
			})
		}
	}
	return out
}

// iniFindSection localise la section [section] (comparaison insensible à la
// casse). Renvoie l'index de la ligne d'en-tête, l'index de fin exclusif
// (début de la section suivante ou fin du fichier) et un booléen.
func iniFindSection(lines []string, section string) (int, int, bool) {
	for i, line := range lines {
		name, ok := iniSectionName(line)
		if !ok {
			continue
		}
		if !strings.EqualFold(name, section) {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if _, ok := iniSectionName(lines[j]); ok {
				end = j
				break
			}
		}
		return i, end, true
	}
	return -1, -1, false
}

// iniSectionName reconnaît une ligne d'en-tête "[nom]" et renvoie le nom.
func iniSectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	close := strings.Index(trimmed, "]")
	if close < 0 {
		return "", false
	}
	return strings.TrimSpace(trimmed[1:close]), true
}

// iniFindKeyLine cherche une ligne non commentée "clé=valeur" ou "clé:valeur"
// entre start (inclus) et end (exclu). Renvoie l'index, l'ancienne valeur et
// un booléen.
func iniFindKeyLine(lines []string, start, end int, key string) (int, string, bool) {
	for i := start + 1; i < end && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		sep := strings.IndexAny(trimmed, "=:")
		if sep < 0 {
			continue
		}
		if strings.TrimSpace(trimmed[:sep]) == key {
			return i, strings.TrimSpace(trimmed[sep+1:]), true
		}
	}
	return -1, "", false
}

// iniInsertLine insère line à l'index pos et renvoie le nouveau slice.
func iniInsertLine(lines []string, pos int, line string) []string {
	if pos < 0 || pos > len(lines) {
		pos = len(lines)
	}
	lines = append(lines, "")
	copy(lines[pos+1:], lines[pos:])
	lines[pos] = line
	return lines
}

// iniDropEmptySection supprime la section [section] si elle ne contient plus
// aucune ligne utile et si toutes ses clés gérées ont été supprimées.
func iniDropEmptySection(lines []string, section string, managed []state.ManagedKey) []string {
	for _, mk := range managed {
		if mk.Original != nil {
			return lines
		}
	}
	start, end, found := iniFindSection(lines, section)
	if !found {
		return lines
	}
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		return lines
	}
	return append(lines[:start], lines[end:]...)
}
