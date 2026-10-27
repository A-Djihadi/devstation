//go:build !windows

package winenv

import "errors"

// errWindowsOnly est renvoyée par les implémentations non Windows.
var errWindowsOnly = errors.New("disponible sous Windows uniquement")

// stubCertStore est l'implémentation non Windows de CertStore.
type stubCertStore struct{}

// NewCertStore retourne un CertStore indisponible hors Windows.
func NewCertStore() CertStore {
	return &stubCertStore{}
}

// AddUserRoot échoue hors Windows.
func (s *stubCertStore) AddUserRoot(string) (bool, error) {
	return false, errWindowsOnly
}

// HasUserRoot échoue hors Windows.
func (s *stubCertStore) HasUserRoot(string) (bool, error) {
	return false, errWindowsOnly
}

// stubCreds est l'implémentation non Windows de Creds.
type stubCreds struct{}

// NewCreds retourne un Creds indisponible hors Windows.
func NewCreds() Creds {
	return &stubCreds{}
}

// Get échoue hors Windows.
func (s *stubCreds) Get(string) (string, string, error) {
	return "", "", errWindowsOnly
}

// Set échoue hors Windows.
func (s *stubCreds) Set(string, string, string) error {
	return errWindowsOnly
}

// Delete échoue hors Windows.
func (s *stubCreds) Delete(string) error {
	return errWindowsOnly
}
