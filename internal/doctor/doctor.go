// Package doctor fournit le diagnostic du poste : vérification des outils
// installés, du PATH, du proxy, des dépôts Nexus, de Vault et de Podman.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/network"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/vaultx"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// Status est le résultat d'une vérification.
type Status string

const (
	// OK indique une vérification réussie.
	OK Status = "ok"
	// Warn indique un avertissement non bloquant.
	Warn Status = "warn"
	// Fail indique une vérification en échec.
	Fail Status = "fail"
)

// Check décrit le résultat d'une vérification.
type Check struct {
	// Name est le nom lisible de la vérification.
	Name string
	// Status est le résultat de la vérification.
	Status Status
	// Detail précise le résultat.
	Detail string
	// Fix propose une action corrective.
	Fix string
}

// Options regroupe les entrées du diagnostic.
type Options struct {
	// Deps regroupe les dépendances partagées.
	Deps install.Deps
	// Answers contient les réponses de l'assistant ; peut être nil.
	Answers *profile.Answers
	// Profile est le profil chargé ; peut être nil.
	Profile *profile.Profile
	// Installers associe un outil à son installeur (pour Verify).
	Installers map[string]install.Installer
	// PxAddr est l'adresse d'écoute du relais px ("127.0.0.1:3128").
	PxAddr string
	// Probe vérifie la joignabilité HTTP d'une URL ; nil saute la vérification.
	Probe func(ctx context.Context, url string) error
}

// Run exécute toutes les vérifications et retourne leurs résultats.
func Run(ctx context.Context, o Options) []Check {
	var checks []Check
	checks = append(checks, checkTools(ctx, o)...)
	checks = append(checks, checkShadowing(o)...)
	checks = append(checks, checkProxy(o)...)
	checks = append(checks, checkNexus(ctx, o)...)
	checks = append(checks, checkVault(ctx, o)...)
	checks = append(checks, checkPodman(ctx, o)...)
	return checks
}

// versionMatches indique si la version disponible correspond à la version
// attendue, par comparaison de préfixe ("21" accepte "21.0.12").
func versionMatches(got, want string) bool {
	if want == "" {
		return true
	}
	return got == want || strings.HasPrefix(got, want+".") || strings.HasPrefix(got, want+"-")
}

// checkTools vérifie chaque outil installé.
func checkTools(ctx context.Context, o Options) []Check {
	names := make([]string, 0, len(o.Installers))
	for name := range o.Installers {
		names = append(names, name)
	}
	sort.Strings(names)

	var checks []Check
	for _, name := range names {
		inst := o.Installers[name]
		check := Check{Name: "outil " + name}
		got, err := inst.Verify(ctx, o.Deps)
		if err != nil {
			check.Status = Fail
			check.Detail = err.Error()
			check.Fix = "relancer devstation setup"
			checks = append(checks, check)
			continue
		}
		want := ""
		if o.Answers != nil {
			want = o.Answers.Versions[name]
		}
		if !versionMatches(got, want) {
			check.Status = Warn
			check.Detail = fmt.Sprintf("version %s installée, %s attendue", got, want)
			check.Fix = fmt.Sprintf("devstation use %s %s", name, want)
			checks = append(checks, check)
			continue
		}
		check.Status = OK
		check.Detail = got
		checks = append(checks, check)
	}
	return checks
}

// checkShadowing détecte les exécutables du PATH machine qui masquent nos outils.
func checkShadowing(o Options) []Check {
	if o.Deps.Env == nil {
		return nil
	}
	machinePath, err := o.Deps.Env.MachinePath()
	if err != nil {
		return []Check{{
			Name:   "PATH système",
			Status: Warn,
			Detail: err.Error(),
			Fix:    "vérifier le PATH système",
		}}
	}

	ours := []string{
		install.CurrentDir(o.Deps, "java"),
		install.CurrentDir(o.Deps, "node"),
		install.CurrentDir(o.Deps, "python"),
	}

	var checks []Check
	for _, exe := range []string{"java.exe", "node.exe", "python.exe"} {
		dirs := winenv.Shadowing(machinePath, exe, ours)
		if len(dirs) == 0 {
			continue
		}
		checks = append(checks, Check{
			Name:   "PATH système masque " + exe,
			Status: Warn,
			Detail: strings.Join(dirs, "; "),
			Fix: fmt.Sprintf("le PATH système passe avant le PATH utilisateur : demander au support de retirer %s ou désinstaller cette version",
				strings.Join(dirs, ", ")),
		})
	}
	return checks
}

// checkProxy vérifie que le relais px tourne lorsque Kerberos est utilisé.
func checkProxy(o Options) []Check {
	if o.Profile == nil {
		return nil
	}
	_, usePx := network.Effective(o.Profile.Network)
	if !usePx {
		return nil
	}
	addr := o.PxAddr
	if addr == "" {
		addr = "127.0.0.1:3128"
	}
	if network.PxRunning(addr, 500*time.Millisecond) {
		return []Check{{Name: "relais proxy px", Status: OK, Detail: addr}}
	}
	return []Check{{
		Name:   "relais proxy px",
		Status: Fail,
		Detail: "aucune écoute sur " + addr,
		Fix:    "lancer px ou rouvrir la session Windows",
	}}
}

// checkNexus vérifie la joignabilité des dépôts Nexus configurés.
func checkNexus(ctx context.Context, o Options) []Check {
	if o.Profile == nil || o.Probe == nil {
		return nil
	}
	nexus := o.Profile.Network.Nexus
	urls := []struct {
		name string
		url  string
	}{
		{"Nexus Maven", nexus.Maven},
		{"Nexus Npm", nexus.Npm},
		{"Nexus Pypi", nexus.Pypi},
	}
	var checks []Check
	for _, u := range urls {
		if u.url == "" {
			continue
		}
		check := Check{Name: u.name}
		if err := o.Probe(ctx, u.url); err != nil {
			check.Status = Warn
			check.Detail = err.Error()
			check.Fix = "vérifier le proxy et les certificats d'entreprise"
		} else {
			check.Status = OK
			check.Detail = u.url
		}
		checks = append(checks, check)
	}
	return checks
}

// checkVault vérifie la durée de vie restante du token Vault.
func checkVault(ctx context.Context, o Options) []Check {
	if o.Answers == nil || !o.Answers.Vault {
		return nil
	}
	if o.Deps.Run == nil {
		return nil
	}
	exe := filepath.Join(install.CurrentDir(o.Deps, "vault"), "vault.exe")
	ttl, err := vaultx.TTL(ctx, o.Deps.Run, exe)
	if err != nil || ttl <= 0 {
		detail := "token Vault absent ou expiré"
		if err != nil {
			detail = err.Error()
		}
		return []Check{{
			Name:   "Vault",
			Status: Warn,
			Detail: detail,
			Fix:    "devstation setup (étape Connexion au coffre) ou vault login",
		}}
	}
	return []Check{{
		Name:   "Vault",
		Status: OK,
		Detail: "token valide encore " + ttl.Round(time.Second).String(),
	}}
}

// findPodmanExe cherche podman.exe sous le dossier de la version active.
func findPodmanExe(root string) string {
	var found string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "podman.exe") {
			found = path
		}
		return nil
	})
	return found
}

// checkPodman vérifie qu'une machine Podman est démarrée.
func checkPodman(ctx context.Context, o Options) []Check {
	if o.Answers == nil || !o.Answers.Podman {
		return nil
	}
	if o.Deps.Run == nil {
		return nil
	}
	exe := findPodmanExe(install.CurrentDir(o.Deps, "podman"))
	if exe == "" {
		return []Check{{
			Name:   "Podman",
			Status: Warn,
			Detail: "podman.exe introuvable",
			Fix:    "relancer devstation setup",
		}}
	}
	res, err := o.Deps.Run.Run(ctx, execx.Cmd{
		Name: exe,
		Args: []string{"machine", "list", "--format", "json"},
	})
	if err != nil || res.Code != 0 {
		detail := "impossible de lister les machines Podman"
		if err != nil {
			detail = err.Error()
		}
		return []Check{{
			Name:   "Podman",
			Status: Warn,
			Detail: detail,
			Fix:    "podman machine start",
		}}
	}
	var machines []struct {
		Running bool `json:"Running"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &machines); err != nil {
		return []Check{{
			Name:   "Podman",
			Status: Warn,
			Detail: "réponse de podman illisible",
			Fix:    "podman machine start",
		}}
	}
	for _, m := range machines {
		if m.Running {
			return []Check{{Name: "Podman", Status: OK, Detail: "machine démarrée"}}
		}
	}
	return []Check{{
		Name:   "Podman",
		Status: Warn,
		Detail: "aucune machine démarrée",
		Fix:    "podman machine start",
	}}
}
