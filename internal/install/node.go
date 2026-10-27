package install

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// Node installe Node.js, met à jour npm et installe Angular CLI.
type Node struct {
	// NpmVersion est la version de npm à installer ; "" ou "bundled" conserve
	// la version fournie avec Node.js.
	NpmVersion string
	// AngularCLI est la version de @angular/cli à installer ; vide désactive
	// l'installation.
	AngularCLI string
	// CABundle est le chemin du bundle de certificats à exposer via
	// NODE_EXTRA_CA_CERTS ; vide désactive la variable.
	CABundle string
}

var _ Installer = (*Node)(nil)

// NewNode crée un installeur Node.js.
func NewNode(npmVersion, angularCLI, caBundle string) *Node {
	return &Node{NpmVersion: npmVersion, AngularCLI: angularCLI, CABundle: caBundle}
}

// Tool retourne le nom de l'outil géré.
func (n *Node) Tool() string {
	return "node"
}

// nodeNpmCmd retourne le chemin de npm.cmd dans le dossier d'installation.
func nodeNpmCmd(dir string) string {
	return filepath.Join(dir, "npm.cmd")
}

// nodeGlobalDir retourne le dossier de préfixe global npm.
func nodeGlobalDir(d Deps) string {
	return filepath.Join(d.Root, "node", "global")
}

// nodeRun exécute une commande et transforme un code de sortie non nul en erreur.
func nodeRun(ctx context.Context, d Deps, c execx.Cmd) error {
	if d.Run == nil {
		return errors.New("exécuteur de commandes manquant")
	}
	res, err := d.Run.Run(ctx, c)
	if err != nil {
		return fmt.Errorf("exécution de %q : %w", c.String(), err)
	}
	if res.Code != 0 {
		return fmt.Errorf("échec de %q (code %d) : %s", c.String(), res.Code, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// Install installe Node.js puis configure npm et les paquets globaux.
func (n *Node) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir, err := InstallZip(ctx, d, "node", r, true, report)
	if err != nil {
		return err
	}

	prefix := nodeGlobalDir(d)
	if err := nodeRun(ctx, d, execx.Cmd{
		Name: nodeNpmCmd(dir),
		Args: []string{"config", "set", "prefix", prefix, "--location=user"},
	}); err != nil {
		return err
	}

	if n.NpmVersion != "" && n.NpmVersion != "bundled" {
		if err := nodeRun(ctx, d, execx.Cmd{
			Name: nodeNpmCmd(dir),
			Args: []string{"install", "-g", "npm@" + n.NpmVersion},
		}); err != nil {
			return err
		}
	}

	if n.AngularCLI != "" {
		if err := nodeRun(ctx, d, execx.Cmd{
			Name: nodeNpmCmd(dir),
			Args: []string{"install", "-g", "@angular/cli@" + n.AngularCLI},
		}); err != nil {
			return err
		}
	}

	return nil
}

// Activate active une version de Node.js et ajoute le dossier global au Path.
func (n *Node) Activate(d Deps, version string) error {
	if err := SetCurrent(d, "node", version, []string{""}); err != nil {
		return err
	}

	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}

	pathVar, _, err := d.Env.UserVar("Path")
	if err != nil {
		return fmt.Errorf("lecture du Path utilisateur : %w", err)
	}
	pathVar = winenv.PrependPath(pathVar, nodeGlobalDir(d))
	if err := d.Env.SetUserVar("Path", pathVar); err != nil {
		return fmt.Errorf("écriture du Path utilisateur : %w", err)
	}

	if n.CABundle != "" {
		if err := d.Env.SetUserVar("NODE_EXTRA_CA_CERTS", n.CABundle); err != nil {
			return fmt.Errorf("écriture de NODE_EXTRA_CA_CERTS : %w", err)
		}
	}

	return nil
}

// Verify retourne la version de Node.js effectivement disponible.
func (n *Node) Verify(ctx context.Context, d Deps) (string, error) {
	if d.Run == nil {
		return "", errors.New("exécuteur de commandes manquant")
	}
	exe := filepath.Join(CurrentDir(d, "node"), "node.exe")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: exe, Args: []string{"--version"}})
	if err != nil {
		return "", fmt.Errorf("exécution de %q : %w", exe, err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("échec de %q (code %d) : %s", exe, res.Code, strings.TrimSpace(res.Stderr))
	}
	return strings.TrimPrefix(strings.TrimSpace(res.Stdout), "v"), nil
}
