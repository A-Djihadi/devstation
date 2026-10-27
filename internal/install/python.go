package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
)

// Python installe Python par utilisateur à partir de l'installeur officiel.
type Python struct{}

var _ Installer = (*Python)(nil)

// NewPython crée un installeur Python.
func NewPython() *Python {
	return &Python{}
}

// Tool retourne le nom de l'outil géré.
func (p *Python) Tool() string {
	return "python"
}

// Install télécharge l'installeur Python et l'exécute en mode silencieux pour
// une installation par utilisateur (sans droits administrateur).
func (p *Python) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir := ToolDir(d, "python", r.Version)

	if d.State != nil {
		if _, ok := d.State.Installed("python", r.Version); ok {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return nil
			}
		}
	}

	if d.DL == nil {
		return errors.New("client de téléchargement manquant")
	}
	if d.Run == nil {
		return errors.New("exécuteur de commandes manquant")
	}

	var progress download.Progress
	if report != nil {
		progress = func(done, total int64) {
			report("téléchargement", done, total)
		}
	}

	path, verified, err := d.DL.Fetch(ctx, r.URL, r.Checksum, progress)
	if err != nil {
		return err
	}
	if !verified && d.Log != nil {
		d.Log.Printf("empreinte non vérifiée : %s", r.URL)
	}

	// Windows refuse d'exécuter un installeur dont le nom ne se termine pas
	// par ".exe" : le fichier de cache est donc copié sous un nom explicite.
	exe := filepath.Join(os.TempDir(), fmt.Sprintf("python-%s-amd64.exe", r.Version))
	if err := pythonCopyFile(path, exe); err != nil {
		return err
	}
	defer os.Remove(exe)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("création de %q : %w", dir, err)
	}

	args := []string{
		"/quiet",
		"InstallAllUsers=0",
		"PrependPath=0",
		"Include_launcher=0",
		"Include_test=0",
		"Shortcuts=0",
		"TargetDir=" + dir,
	}
	res, err := d.Run.Run(ctx, pythonCmd(exe, args))
	if err != nil {
		return fmt.Errorf("exécution de l'installeur Python : %w (%s)", err, strings.TrimSpace(res.Stderr))
	}
	if res.Code != 0 {
		return fmt.Errorf("l'installeur Python a échoué (code %d) : %s", res.Code, strings.TrimSpace(res.Stderr))
	}

	if d.State != nil {
		d.State.RecordInstall(state.Install{
			Tool:    "python",
			Version: r.Version,
			Path:    dir,
			At:      now(d),
		})
	}

	return nil
}

// Activate active la version donnée de Python.
func (p *Python) Activate(d Deps, version string) error {
	return SetCurrent(d, "python", version, []string{"", "Scripts"})
}

// Verify retourne la version de Python effectivement disponible.
func (p *Python) Verify(ctx context.Context, d Deps) (string, error) {
	if d.Run == nil {
		return "", errors.New("exécuteur de commandes manquant")
	}
	exe := filepath.Join(CurrentDir(d, "python"), "python.exe")
	res, err := d.Run.Run(ctx, pythonCmd(exe, []string{"--version"}))
	if err != nil {
		return "", fmt.Errorf("exécution de python --version : %w (%s)", err, strings.TrimSpace(res.Stderr))
	}
	if res.Code != 0 {
		return "", fmt.Errorf("python --version a échoué (code %d) : %s", res.Code, strings.TrimSpace(res.Stderr))
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		out = strings.TrimSpace(res.Stderr)
	}
	out = strings.TrimPrefix(out, "Python ")
	return strings.TrimSpace(out), nil
}

// pythonCmd construit une commande non secrète.
func pythonCmd(name string, args []string) execx.Cmd {
	return execx.Cmd{Name: name, Args: args}
}

// pythonCopyFile copie src vers dst en écrasant dst.
func pythonCopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("lecture de %q : %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return fmt.Errorf("écriture de %q : %w", dst, err)
	}
	return nil
}
