// Package setup transforme les réponses de l'assistant de configuration en
// étapes plan.Step ordonnancées (réseau, outils, Git, dépôts, Vault).
package setup

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/A-Djihadi/devstation/internal/gitx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/network"
	"github.com/A-Djihadi/devstation/internal/plan"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/vaultx"
)

// Dirs regroupe les dossiers utilisés par les étapes du setup.
type Dirs struct {
	// Bin est le dossier des exécutables de DevStation
	// (%LOCALAPPDATA%\DevStation\bin).
	Bin string
	// Home est le dossier utilisateur.
	Home string
	// StartMenu est le dossier du menu Démarrer.
	StartMenu string
	// Workspace est le dossier où cloner les dépôts.
	Workspace string
	// PxDir est le dossier du fichier px.ini.
	PxDir string
}

// Options regroupe les entrées de Build.
type Options struct {
	// Deps regroupe les dépendances partagées par les installeurs.
	Deps install.Deps
	// Answers contient les réponses de l'assistant.
	Answers *profile.Answers
	// Profile est le profil d'équipe chargé.
	Profile *profile.Profile
	// Releases contient les releases déjà résolues, clés : java, node, python,
	// maven, gradle, vscode, intellij, podman, compose, vault, px.
	Releases map[string]sources.Release
	// Paths regroupe les chemins des fichiers de configuration réseau.
	Paths network.Paths
	// Dirs regroupe les dossiers utilisés par les étapes.
	Dirs Dirs
	// GitHubToken est le jeton GitHub facultatif ; il n'est jamais journalisé.
	GitHubToken string
}

// toolOrder est l'ordre d'installation des outils.
var toolOrder = []string{"java", "maven", "gradle", "node", "python", "vscode", "intellij", "podman", "vault"}

// hasRole indique si roles contient l'un des rôles voulus.
func hasRole(roles []string, want ...string) bool {
	for _, r := range roles {
		for _, w := range want {
			if r == w {
				return true
			}
		}
	}
	return false
}

// Tools retourne les outils à installer, dans l'ordre d'installation, sans
// doublon, d'après les réponses a et le profil p.
func Tools(a *profile.Answers, p *profile.Profile) []string {
	if a == nil {
		return nil
	}
	want := map[string]bool{}

	if hasRole(a.Roles, "back", "fullstack") {
		want["java"] = true
		want["maven"] = true
		if p != nil && p.Tools.Gradle.Enabled != nil && *p.Tools.Gradle.Enabled {
			want["gradle"] = true
		}
	}
	if hasRole(a.Roles, "front", "fullstack") {
		want["node"] = true
	}
	if hasRole(a.Roles, "python") {
		want["python"] = true
	}
	switch a.Editor {
	case "vscode":
		want["vscode"] = true
	case "intellij":
		want["intellij"] = true
	case "both":
		want["vscode"] = true
		want["intellij"] = true
	}
	if a.Podman {
		want["podman"] = true
	}
	if a.Vault {
		want["vault"] = true
	}

	out := make([]string, 0, len(want))
	for _, name := range toolOrder {
		if want[name] {
			out = append(out, name)
		}
	}
	return out
}

// effectiveNetwork retourne la configuration réseau du profil p surchargée par
// les réponses a lorsque celles-ci sont renseignées.
func effectiveNetwork(a *profile.Answers, p *profile.Profile) profile.Network {
	var n profile.Network
	if p != nil {
		n = p.Network
	}
	if a == nil {
		return n
	}
	if a.Proxy != "" {
		n.Proxy = a.Proxy
	}
	if a.ProxyAuth != "" {
		n.ProxyAuth = a.ProxyAuth
	}
	if len(a.NoProxy) > 0 {
		n.NoProxy = a.NoProxy
	}
	if len(a.CACerts) > 0 {
		n.CACerts = a.CACerts
	}
	return n
}

// toolTitle construit le libellé affiché d'une étape d'outil.
func toolTitle(tool, version string) string {
	names := map[string]string{
		"java":     "Java",
		"maven":    "Maven",
		"gradle":   "Gradle",
		"node":     "Node.js",
		"python":   "Python",
		"vscode":   "VS Code",
		"intellij": "IntelliJ IDEA",
		"podman":   "Podman",
		"vault":    "Client Vault",
	}
	name := names[tool]
	if name == "" {
		name = tool
	}
	if version == "" {
		return name
	}
	return name + " " + version
}

// newInstaller construit l'installeur de l'outil demandé.
func newInstaller(tool string, o Options, n profile.Network) (install.Installer, error) {
	a := o.Answers
	p := o.Profile
	switch tool {
	case "java":
		return install.NewJava(n.CACerts), nil
	case "maven":
		return install.NewMaven(), nil
	case "gradle":
		return install.NewGradle(), nil
	case "node":
		npm := ""
		angular := ""
		if a != nil {
			npm = a.Versions["npm"]
			angular = a.Versions["angular_cli"]
		}
		caBundle := ""
		if len(n.CACerts) > 0 {
			caBundle = o.Paths.CABundle
		}
		return install.NewNode(npm, angular, caBundle), nil
	case "python":
		return install.NewPython(), nil
	case "vscode":
		var roles []string
		if a != nil {
			roles = a.Roles
		}
		return install.NewVSCode(install.Extensions(p, "vscode", roles)), nil
	case "intellij":
		return install.NewIntelliJ(nil, o.Dirs.StartMenu), nil
	case "podman":
		mirror := ""
		if p != nil {
			mirror = p.Network.Nexus.Containers
		}
		return install.NewPodman(o.Releases["compose"], n.CACerts, mirror), nil
	case "vault":
		addr := ""
		if p != nil {
			addr = p.Vault.Addr
		}
		caCert := ""
		if len(n.CACerts) > 0 {
			caCert = n.CACerts[0]
		}
		return install.NewVaultCLI(addr, caCert, o.Dirs.Bin, o.Dirs.Home), nil
	default:
		return nil, fmt.Errorf("outil inconnu : %q", tool)
	}
}

// Build construit les étapes du setup à partir des options o.
func Build(o Options) ([]plan.Step, error) {
	a := o.Answers
	p := o.Profile
	if a == nil {
		return nil, errors.New("réponses de l'assistant manquantes")
	}

	deps := o.Deps
	state := deps.State
	reseau := effectiveNetwork(a, p)

	var steps []plan.Step

	// 1. Relais proxy px.
	_, usePx := network.Effective(reseau)
	pxStep := false
	if usePx {
		pxStep = true
		pxRelease, ok := o.Releases["px"]
		if !ok {
			return nil, errors.New("release manquante pour l'outil \"px\"")
		}
		pxIni := filepath.Join(o.Dirs.PxDir, "px.ini")
		upstream := a.Proxy
		if upstream == "" && p != nil {
			upstream = p.Network.Proxy
		}
		pxExe := filepath.Join(install.ToolDir(deps, "px", pxRelease.Version), "px.exe")
		steps = append(steps, plan.Step{
			ID:    "px",
			Title: "Relais proxy px",
			Check: func(ctx context.Context) (bool, error) {
				if state == nil {
					return false, nil
				}
				if _, ok := state.Installed("px", pxRelease.Version); !ok {
					return false, nil
				}
				return network.PxRunning("127.0.0.1:3128", 500*time.Millisecond), nil
			},
			Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
				if _, err := install.InstallZip(ctx, deps, "px", pxRelease, true, report); err != nil {
					return err
				}
				cfg := network.PxConfig{
					Upstream: upstream,
					NoProxy:  reseau.NoProxy,
					Gateway:  a.Podman,
				}
				if err := network.WritePxINI(pxIni, cfg, state); err != nil {
					return err
				}
				if err := network.RegisterAutostart(ctx, deps.Run, pxExe, pxIni); err != nil {
					return err
				}
				return network.StartPx(ctx, deps.Run, pxExe, pxIni)
			},
		})
	}

	// 2. Réseau d'entreprise.
	networkDeps := []string{}
	if pxStep {
		networkDeps = append(networkDeps, "px")
	}
	steps = append(steps, plan.Step{
		ID:        "network",
		Title:     "Réseau d'entreprise",
		DependsOn: networkDeps,
		Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
			return network.Apply(ctx, deps, reseau, o.Paths)
		},
	})

	// 3. Outils.
	for _, tool := range Tools(a, p) {
		release, ok := o.Releases[tool]
		if !ok {
			return nil, fmt.Errorf("release manquante pour l'outil %q", tool)
		}
		inst, err := newInstaller(tool, o, reseau)
		if err != nil {
			return nil, err
		}
		version := release.Version
		steps = append(steps, plan.Step{
			ID:        tool,
			Title:     toolTitle(tool, version),
			DependsOn: []string{"network"},
			Check: func(ctx context.Context) (bool, error) {
				if state == nil {
					return false, nil
				}
				if _, ok := state.Installed(tool, version); !ok {
					return false, nil
				}
				return state.Active[tool] == version, nil
			},
			Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
				if err := inst.Install(ctx, deps, release, report); err != nil {
					return err
				}
				return inst.Activate(deps, version)
			},
		})
	}

	// 4. Git.
	autocrlf := ""
	if p != nil {
		autocrlf = p.Git.Autocrlf
	}
	steps = append(steps, plan.Step{
		ID:    "git",
		Title: "Git",
		Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
			if err := gitx.Configure(ctx, deps.Run, a.GitName, a.GitEmail, autocrlf); err != nil {
				return err
			}
			if a.GenerateSSH {
				if _, _, err := gitx.EnsureSSHKey(ctx, deps.Run, o.Dirs.Home, a.GitEmail); err != nil {
					return err
				}
			}
			if o.GitHubToken != "" {
				if err := gitx.StoreToken(ctx, deps.Run, o.GitHubToken); err != nil {
					return err
				}
			}
			return nil
		},
	})

	// 5. Dépôts du projet.
	if len(a.Repos) > 0 {
		repos := append([]string(nil), a.Repos...)
		steps = append(steps, plan.Step{
			ID:        "repos",
			Title:     "Dépôts du projet",
			DependsOn: []string{"git", "network"},
			Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
				for _, name := range repos {
					url := ""
					if p != nil {
						for _, r := range p.Repos {
							if r.Name == name {
								url = r.URL
								break
							}
						}
					}
					if url == "" {
						return fmt.Errorf("dépôt inconnu dans le profil : %q", name)
					}
					if _, err := gitx.Clone(ctx, deps.Run, url, filepath.Join(o.Dirs.Workspace, name)); err != nil {
						return err
					}
				}
				return nil
			},
		})
	}

	// 6. Connexion au coffre.
	if a.Vault {
		auth := ""
		role := ""
		if p != nil {
			auth = p.Vault.Auth
			role = p.Vault.Role
		}
		if auth == "" {
			auth = "oidc"
		}
		vaultExe := filepath.Join(install.CurrentDir(deps, "vault"), "vault.exe")
		steps = append(steps, plan.Step{
			ID:        "vault-login",
			Title:     "Connexion au coffre",
			DependsOn: []string{"vault"},
			Run: func(ctx context.Context, report func(msg string, done, total int64)) error {
				return vaultx.Login(ctx, deps.Run, vaultExe, auth, role)
			},
		})
	}

	return steps, nil
}

// containsToken indique si s contient token (utilisé par les tests).
func containsToken(s, token string) bool {
	return token != "" && strings.Contains(s, token)
}
