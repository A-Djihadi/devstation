// Package gitx configure Git et prépare les identifiants de développement
// (nom, courriel, clé SSH, jeton GitHub) via un execx.Runner injectable.
package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/execx"
)

// Configure définit l'identité Git globale et, si autocrlf n'est pas vide,
// la politique de fin de ligne. Une valeur vide de name ou email laisse la
// clé correspondante inchangée.
func Configure(ctx context.Context, r execx.Runner, name, email, autocrlf string) error {
	if name != "" {
		if err := run(ctx, r, execx.Cmd{Name: "git", Args: []string{"config", "--global", "user.name", name}}); err != nil {
			return err
		}
	}
	if email != "" {
		if err := run(ctx, r, execx.Cmd{Name: "git", Args: []string{"config", "--global", "user.email", email}}); err != nil {
			return err
		}
	}
	if autocrlf != "" {
		if err := run(ctx, r, execx.Cmd{Name: "git", Args: []string{"config", "--global", "core.autocrlf", autocrlf}}); err != nil {
			return err
		}
	}
	return nil
}

// EnsureSSHKey garantit la présence d'une clé SSH ed25519 dans home/.ssh.
// Si la clé privée existe déjà, elle n'est jamais écrasée et created vaut
// false. Sinon la clé est générée et created vaut true. pubPath pointe
// toujours vers le fichier de clé publique associé.
func EnsureSSHKey(ctx context.Context, r execx.Runner, home, email string) (pubPath string, created bool, err error) {
	key := filepath.Join(home, ".ssh", "id_ed25519")
	pubPath = key + ".pub"
	if _, statErr := os.Stat(key); statErr == nil {
		return pubPath, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(key), 0o700); err != nil {
		return "", false, fmt.Errorf("création du dossier .ssh : %w", err)
	}
	cmd := execx.Cmd{
		Name: "ssh-keygen",
		Args: []string{"-t", "ed25519", "-C", email, "-f", key, "-N", ""},
	}
	if err := run(ctx, r, cmd); err != nil {
		return "", false, err
	}
	return pubPath, true, nil
}

// StoreToken enregistre le jeton GitHub dans Git Credential Manager via
// `git credential approve`. Le jeton est transmis sur l'entrée standard et
// marqué secret : il n'apparaît ni dans les arguments ni dans les erreurs.
// Un jeton vide ne déclenche aucune action.
func StoreToken(ctx context.Context, r execx.Runner, token string) error {
	if token == "" {
		return nil
	}
	stdin := "protocol=https\nhost=github.com\nusername=x-access-token\npassword=" + token + "\n\n"
	cmd := execx.Cmd{
		Name:   "git",
		Args:   []string{"credential", "approve"},
		Stdin:  stdin,
		Secret: true,
	}
	return run(ctx, r, cmd)
}

// Clone clone url dans dir. Si dir existe déjà et n'est pas vide, aucun
// clonage n'est lancé et cloned vaut false. En cas d'échec, le message
// d'erreur reprend Stderr sans exposer d'identifiants présents dans l'URL.
func Clone(ctx context.Context, r execx.Runner, url, dir string) (cloned bool, err error) {
	if entries, readErr := os.ReadDir(dir); readErr == nil && len(entries) > 0 {
		return false, nil
	}
	cmd := execx.Cmd{Name: "git", Args: []string{"clone", url, dir}}
	res, runErr := r.Run(ctx, cmd)
	if runErr != nil || res.Code != 0 {
		return false, fmt.Errorf("git clone %s : %s", safeURL(url), strings.TrimSpace(res.Stderr))
	}
	return true, nil
}

// run exécute c et transforme un code de sortie non nul en erreur reprenant
// Stderr.
func run(ctx context.Context, r execx.Runner, c execx.Cmd) error {
	res, err := r.Run(ctx, c)
	if err != nil {
		return fmt.Errorf("%s : %w", c.String(), err)
	}
	if res.Code != 0 {
		return fmt.Errorf("%s : %s", c.String(), strings.TrimSpace(res.Stderr))
	}
	return nil
}

// safeURL masque les identifiants éventuels (user:pass@) d'une URL.
func safeURL(url string) string {
	at := strings.LastIndex(url, "@")
	if at < 0 {
		return url
	}
	scheme := strings.Index(url, "://")
	if scheme < 0 || scheme+3 > at {
		return url
	}
	return url[:scheme+3] + "***@" + url[at+1:]
}
