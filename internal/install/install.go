// Package install fournit le cœur des installeurs d'outils : dépendances
// partagées, installation d'archives zip/tar.gz et activation d'une version.
package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/A-Djihadi/devstation/internal/archive"
	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/logx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// Deps regroupe les dépendances partagées par les installeurs.
type Deps struct {
	// DL est le client de téléchargement.
	DL *download.Client
	// Env expose les variables d'environnement utilisateur.
	Env winenv.Env
	// Links expose les jonctions NTFS.
	Links winenv.Links
	// Certs expose le magasin de certificats utilisateur.
	Certs winenv.CertStore
	// Creds expose le Gestionnaire d'identifiants Windows.
	Creds winenv.Creds
	// Run exécute des commandes externes.
	Run execx.Runner
	// State est l'état local persistant.
	State *state.State
	// Log est le journal ; peut être nil (aucune journalisation).
	Log *logx.Logger
	// Root est le dossier des outils (%LOCALAPPDATA%\DevStation\tools).
	Root string
	// Now retourne l'heure courante ; nil signifie time.Now.
	Now func() time.Time
}

// Report est appelé pendant une installation avec le nombre d'octets déjà
// traités et le total annoncé (-1 si inconnu).
type Report func(msg string, done, total int64)

// Installer décrit un installeur d'outil.
type Installer interface {
	// Tool retourne le nom de l'outil géré.
	Tool() string
	// Install installe la version r.
	Install(ctx context.Context, d Deps, r sources.Release, report Report) error
	// Activate active la version donnée.
	Activate(d Deps, version string) error
	// Verify retourne la version effectivement disponible.
	Verify(ctx context.Context, d Deps) (version string, err error)
}

// now retourne l'heure courante selon Deps.
func now(d Deps) time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// ToolDir retourne le dossier d'installation d'une version d'outil.
func ToolDir(d Deps, tool, version string) string {
	return filepath.Join(d.Root, tool, version)
}

// CurrentDir retourne le dossier de la version active d'un outil.
func CurrentDir(d Deps, tool string) string {
	return filepath.Join(d.Root, tool, "current")
}

// InstallZip télécharge et extrait l'archive de r dans le dossier de version
// de l'outil, puis enregistre l'installation dans l'état. Elle retourne le
// dossier d'installation. Si la version est déjà installée et présente sur le
// disque, aucun téléchargement n'est effectué.
func InstallZip(ctx context.Context, d Deps, tool string, r sources.Release, stripTop bool, report Report) (string, error) {
	dir := ToolDir(d, tool, r.Version)

	if d.State != nil {
		if _, ok := d.State.Installed(tool, r.Version); ok {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir, nil
			}
		}
	}

	if d.DL == nil {
		return "", errors.New("client de téléchargement manquant")
	}

	var progress download.Progress
	if report != nil {
		progress = func(done, total int64) {
			report("téléchargement", done, total)
		}
	}

	path, verified, err := d.DL.Fetch(ctx, r.URL, r.Checksum, progress)
	if err != nil {
		return "", err
	}
	if !verified && d.Log != nil {
		d.Log.Printf("empreinte non vérifiée : %s", r.URL)
	}

	partial := dir + ".partial"
	if err := os.RemoveAll(partial); err != nil {
		return "", fmt.Errorf("suppression de %q : %w", partial, err)
	}
	if err := os.MkdirAll(partial, 0o755); err != nil {
		return "", fmt.Errorf("création de %q : %w", partial, err)
	}

	switch r.Archive {
	case "zip":
		err = archive.Unzip(path, partial, stripTop)
	case "tar.gz":
		err = archive.Untar(path, partial, stripTop)
	default:
		err = fmt.Errorf("format d'archive non pris en charge : %q", r.Archive)
	}
	if err != nil {
		os.RemoveAll(partial)
		return "", err
	}

	if err := os.RemoveAll(dir); err != nil {
		os.RemoveAll(partial)
		return "", fmt.Errorf("suppression de %q : %w", dir, err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		os.RemoveAll(partial)
		return "", fmt.Errorf("création du dossier parent de %q : %w", dir, err)
	}
	if err := os.Rename(partial, dir); err != nil {
		os.RemoveAll(partial)
		return "", fmt.Errorf("renommage de %q vers %q : %w", partial, dir, err)
	}

	if d.State != nil {
		d.State.RecordInstall(state.Install{
			Tool:    tool,
			Version: r.Version,
			Path:    dir,
			At:      now(d),
		})
	}

	return dir, nil
}

// SetCurrent active une version d'outil : elle crée la jonction « current »
// vers le dossier de version, ajoute les sous-dossiers d'exécutables au Path
// utilisateur, enregistre la version active et notifie Windows.
func SetCurrent(d Deps, tool, version string, binSubdirs []string) error {
	if d.Links == nil {
		return errors.New("gestion des jonctions manquante")
	}
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}

	current := CurrentDir(d, tool)
	target := ToolDir(d, tool, version)
	if err := d.Links.SetJunction(current, target); err != nil {
		return fmt.Errorf("création de la jonction %q : %w", current, err)
	}

	pathVar, _, err := d.Env.UserVar("Path")
	if err != nil {
		return fmt.Errorf("lecture du Path utilisateur : %w", err)
	}

	for i := len(binSubdirs) - 1; i >= 0; i-- {
		sub := binSubdirs[i]
		var entry string
		if sub == "" {
			entry = current
		} else {
			entry = filepath.Join(current, sub)
		}
		pathVar = winenv.PrependPath(pathVar, entry)
	}

	if err := d.Env.SetUserVar("Path", pathVar); err != nil {
		return fmt.Errorf("écriture du Path utilisateur : %w", err)
	}

	if d.State != nil {
		if d.State.Active == nil {
			d.State.Active = map[string]string{}
		}
		d.State.Active[tool] = version
	}

	if err := d.Env.Broadcast(); err != nil {
		return fmt.Errorf("notification d'environnement : %w", err)
	}

	return nil
}
