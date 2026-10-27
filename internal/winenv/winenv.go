// Package winenv fournit les interfaces d'accès à l'environnement Windows
// (variables utilisateur, PATH machine, jonctions NTFS, magasin de certificats,
// Gestionnaire d'identifiants) ainsi qu'une implémentation factice pour les tests.
package winenv

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrNotFound indique qu'un élément demandé est introuvable.
var ErrNotFound = errors.New("introuvable")

// Env expose les opérations sur les variables d'environnement utilisateur et le PATH machine.
type Env interface {
	// UserVar retourne la valeur d'une variable utilisateur et indique si elle existe.
	UserVar(name string) (string, bool, error)
	// SetUserVar définit une variable utilisateur.
	SetUserVar(name, value string) error
	// DeleteUserVar supprime une variable utilisateur.
	DeleteUserVar(name string) error
	// MachinePath retourne la valeur du PATH machine (lecture seule).
	MachinePath() (string, error)
	// Broadcast notifie Windows d'un changement d'environnement.
	Broadcast() error
}

// Links expose les opérations sur les jonctions NTFS.
type Links interface {
	// SetJunction crée ou remplace une jonction de link vers target.
	SetJunction(link, target string) error
	// ReadJunction retourne la cible d'une jonction.
	ReadJunction(link string) (string, error)
}

// CertStore expose les opérations sur le magasin de certificats utilisateur.
type CertStore interface {
	// AddUserRoot ajoute un certificat racine au magasin utilisateur et indique s'il a été ajouté.
	AddUserRoot(pemPath string) (added bool, err error)
	// HasUserRoot indique si un certificat racine est présent dans le magasin utilisateur.
	HasUserRoot(pemPath string) (bool, error)
}

// Creds expose les opérations sur le Gestionnaire d'identifiants Windows.
type Creds interface {
	// Get retourne les identifiants associés à une cible.
	Get(target string) (user, secret string, err error)
	// Set enregistre les identifiants associés à une cible.
	Set(target, user, secret string) error
	// Delete supprime les identifiants associés à une cible.
	Delete(target string) error
}

// PrependPath ajoute dir en tête de pathVar sans doublon (comparaison insensible
// à la casse), en ignorant un éventuel ';' final.
func PrependPath(pathVar, dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return pathVar
	}
	entries := splitPath(pathVar)
	out := make([]string, 0, len(entries)+1)
	out = append(out, dir)
	for _, e := range entries {
		if !samePath(e, dir) {
			out = append(out, e)
		}
	}
	return strings.Join(out, ";")
}

// RemovePath retire dir de pathVar (comparaison insensible à la casse).
func RemovePath(pathVar, dir string) string {
	dir = strings.TrimSpace(dir)
	entries := splitPath(pathVar)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !samePath(e, dir) {
			out = append(out, e)
		}
	}
	return strings.Join(out, ";")
}

// Shadowing retourne les dossiers de machinePath contenant l'exécutable exe,
// en ignorant les dossiers présents dans dirs.
func Shadowing(machinePath string, exe string, dirs []string) []string {
	var result []string
	for _, entry := range splitPath(machinePath) {
		if entry == "" {
			continue
		}
		if containsDir(dirs, entry) {
			continue
		}
		if fileExists(filepath.Join(entry, exe)) {
			result = append(result, entry)
		}
	}
	return result
}

// splitPath découpe une valeur de PATH en entrées non vides.
func splitPath(pathVar string) []string {
	pathVar = strings.TrimSuffix(pathVar, ";")
	if pathVar == "" {
		return nil
	}
	parts := strings.Split(pathVar, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// samePath compare deux entrées de PATH sans tenir compte de la casse, d'un ';'
// final ni d'un séparateur de fin ('\\' ou '/').
func samePath(a, b string) bool {
	return strings.EqualFold(normalizePath(a), normalizePath(b))
}

// normalizePath retire un ';' final puis un séparateur de fin pour comparer deux entrées.
func normalizePath(p string) string {
	p = strings.TrimSuffix(p, ";")
	p = strings.TrimSuffix(p, `\`)
	p = strings.TrimSuffix(p, "/")
	return p
}

// containsDir indique si dir figure dans dirs (insensible à la casse).
func containsDir(dirs []string, dir string) bool {
	for _, d := range dirs {
		if samePath(d, dir) {
			return true
		}
	}
	return false
}