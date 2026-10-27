//go:build windows

package winenv

import (
	"errors"
	"fmt"

	"github.com/danieljoos/wincred"
)

// credPrefix est le préfixe appliqué aux cibles du Gestionnaire d'identifiants.
const credPrefix = "DevStation/"

// creds implémente Creds via le Gestionnaire d'identifiants Windows.
type creds struct{}

// NewCreds retourne un Creds basé sur le Gestionnaire d'identifiants Windows.
func NewCreds() Creds {
	return &creds{}
}

// credTarget retourne la cible effective, préfixée une seule fois.
func credTarget(target string) string {
	if len(target) >= len(credPrefix) && target[:len(credPrefix)] == credPrefix {
		return target
	}
	return credPrefix + target
}

// Get retourne les identifiants associés à une cible.
func (c *creds) Get(target string) (string, string, error) {
	cred, err := wincred.GetGenericCredential(credTarget(target))
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return "", "", fmt.Errorf("identifiants %q : %w", target, ErrNotFound)
		}
		return "", "", fmt.Errorf("lecture des identifiants %q : %w", target, err)
	}
	return cred.UserName, string(cred.CredentialBlob), nil
}

// Set enregistre les identifiants associés à une cible.
func (c *creds) Set(target, user, secret string) error {
	cred := wincred.NewGenericCredential(credTarget(target))
	cred.UserName = user
	cred.CredentialBlob = []byte(secret)
	if err := cred.Write(); err != nil {
		return fmt.Errorf("écriture des identifiants %q : %w", target, err)
	}
	return nil
}

// Delete supprime les identifiants associés à une cible ; une cible absente est ignorée.
func (c *creds) Delete(target string) error {
	cred, err := wincred.GetGenericCredential(credTarget(target))
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return nil
		}
		return fmt.Errorf("lecture des identifiants %q : %w", target, err)
	}
	if err := cred.Delete(); err != nil {
		return fmt.Errorf("suppression des identifiants %q : %w", target, err)
	}
	return nil
}
