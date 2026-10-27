//go:build !windows

package winenv

import (
	"github.com/A-Djihadi/devstation/internal/execx"
)

// errWindowsOnly est déclarée dans secrets_other.go.

// NewEnv retourne une implémentation de Env non fonctionnelle hors Windows.
func NewEnv() Env {
	return stubEnv{}
}

// NewLinks retourne une implémentation de Links non fonctionnelle hors Windows.
func NewLinks(r execx.Runner) Links {
	return stubLinks{}
}

// stubEnv implémente Env hors Windows en renvoyant errWindowsOnly.
type stubEnv struct{}

// UserVar renvoie errWindowsOnly.
func (stubEnv) UserVar(name string) (string, bool, error) {
	return "", false, errWindowsOnly
}

// SetUserVar renvoie errWindowsOnly.
func (stubEnv) SetUserVar(name, value string) error {
	return errWindowsOnly
}

// DeleteUserVar renvoie errWindowsOnly.
func (stubEnv) DeleteUserVar(name string) error {
	return errWindowsOnly
}

// MachinePath renvoie errWindowsOnly.
func (stubEnv) MachinePath() (string, error) {
	return "", errWindowsOnly
}

// Broadcast renvoie errWindowsOnly.
func (stubEnv) Broadcast() error {
	return errWindowsOnly
}

// stubLinks implémente Links hors Windows en renvoyant errWindowsOnly.
type stubLinks struct{}

// SetJunction renvoie errWindowsOnly.
func (stubLinks) SetJunction(link, target string) error {
	return errWindowsOnly
}

// ReadJunction renvoie errWindowsOnly.
func (stubLinks) ReadJunction(link string) (string, error) {
	return "", errWindowsOnly
}
