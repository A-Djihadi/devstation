package winenv

import "os"

// Fake est une implémentation en mémoire de Env, Links, CertStore et Creds.
type Fake struct {
	// Vars contient les variables d'environnement utilisateur.
	Vars map[string]string
	// Machine contient la valeur du PATH machine.
	Machine string
	// Junctions associe un lien à sa cible.
	Junctions map[string]string
	// Roots indique les certificats racine présents.
	Roots map[string]bool
	// Secrets associe une cible à un couple (utilisateur, secret).
	Secrets map[string][2]string
	// Broadcasts compte les notifications d'environnement émises.
	Broadcasts int
}

// NewFake crée un Fake vide prêt à l'emploi.
func NewFake() *Fake {
	return &Fake{
		Vars:      map[string]string{},
		Junctions: map[string]string{},
		Roots:     map[string]bool{},
		Secrets:   map[string][2]string{},
	}
}

// UserVar retourne la valeur d'une variable utilisateur.
func (f *Fake) UserVar(name string) (string, bool, error) {
	v, ok := f.Vars[name]
	return v, ok, nil
}

// SetUserVar définit une variable utilisateur.
func (f *Fake) SetUserVar(name, value string) error {
	f.Vars[name] = value
	return nil
}

// DeleteUserVar supprime une variable utilisateur.
func (f *Fake) DeleteUserVar(name string) error {
	delete(f.Vars, name)
	return nil
}

// MachinePath retourne le PATH machine factice.
func (f *Fake) MachinePath() (string, error) {
	return f.Machine, nil
}

// Broadcast incrémente le compteur de notifications.
func (f *Fake) Broadcast() error {
	f.Broadcasts++
	return nil
}

// SetJunction enregistre une jonction.
func (f *Fake) SetJunction(link, target string) error {
	f.Junctions[link] = target
	return nil
}

// ReadJunction retourne la cible d'une jonction.
func (f *Fake) ReadJunction(link string) (string, error) {
	t, ok := f.Junctions[link]
	if !ok {
		return "", ErrNotFound
	}
	return t, nil
}

// AddUserRoot ajoute un certificat racine et indique s'il était absent.
func (f *Fake) AddUserRoot(pemPath string) (bool, error) {
	if f.Roots[pemPath] {
		return false, nil
	}
	f.Roots[pemPath] = true
	return true, nil
}

// HasUserRoot indique si un certificat racine est présent.
func (f *Fake) HasUserRoot(pemPath string) (bool, error) {
	return f.Roots[pemPath], nil
}

// Get retourne les identifiants associés à une cible.
func (f *Fake) Get(target string) (string, string, error) {
	c, ok := f.Secrets[target]
	if !ok {
		return "", "", ErrNotFound
	}
	return c[0], c[1], nil
}

// Set enregistre les identifiants associés à une cible.
func (f *Fake) Set(target, user, secret string) error {
	f.Secrets[target] = [2]string{user, secret}
	return nil
}

// Delete supprime les identifiants associés à une cible.
func (f *Fake) Delete(target string) error {
	delete(f.Secrets, target)
	return nil
}

// fileExists indique si le chemin correspond à un fichier existant.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
