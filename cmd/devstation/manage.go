package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A-Djihadi/devstation/internal/config"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/gitx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/vaultx"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// manageToolVars associe à chaque outil les variables d'environnement
// utilisateur qu'il définit et que la désinstallation doit supprimer.
var manageToolVars = map[string][]string{
	"java":   {"JAVA_HOME"},
	"maven":  {"MAVEN_HOME"},
	"gradle": {"GRADLE_HOME"},
	"node":   {"NODE_EXTRA_CA_CERTS"},
	"vault":  {"VAULT_ADDR", "VAULT_CACERT"},
}

// manageNetworkVars regroupe les variables d'environnement réseau supprimées
// par une désinstallation complète.
var manageNetworkVars = []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}

// manageConfirm lit la réponse de l'utilisateur sur stdin et indique si elle
// vaut « o » ou « oui » (insensible à la casse).
func manageConfirm(in io.Reader) bool {
	data, err := io.ReadAll(in)
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(string(data)))
	return answer == "o" || answer == "oui"
}

// manageUnderRoot indique si entry est située sous dir (insensible à la casse).
func manageUnderRoot(entry, dir string) bool {
	e := strings.TrimSuffix(strings.TrimSuffix(entry, `\`), "/")
	d := strings.TrimSuffix(strings.TrimSuffix(dir, `\`), "/")
	if d == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(e), strings.ToLower(d))
}

// manageCleanPath retire du Path utilisateur toutes les entrées situées sous
// le dossier de l'outil tool.
func manageCleanPath(d install.Deps, tool string) error {
	pathVar, _, err := d.Env.UserVar("Path")
	if err != nil {
		return fmt.Errorf("lecture du Path utilisateur : %w", err)
	}
	base := filepath.Join(d.Root, tool)
	entries := strings.Split(strings.TrimSuffix(pathVar, ";"), ";")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if manageUnderRoot(entry, base) {
			pathVar = winenv.RemovePath(pathVar, entry)
		}
	}
	if err := d.Env.SetUserVar("Path", pathVar); err != nil {
		return fmt.Errorf("écriture du Path utilisateur : %w", err)
	}
	return nil
}

// manageRemoveTool retire un outil du Path, des variables d'environnement, du
// disque et de l'état.
func manageRemoveTool(d install.Deps, tool string) error {
	if err := manageCleanPath(d, tool); err != nil {
		return err
	}
	for _, name := range manageToolVars[tool] {
		if err := d.Env.DeleteUserVar(name); err != nil {
			return fmt.Errorf("suppression de la variable %q : %w", name, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(d.Root, tool)); err != nil {
		return fmt.Errorf("suppression du dossier de l'outil %q : %w", tool, err)
	}
	if d.State != nil {
		kept := d.State.Installs[:0]
		for _, inst := range d.State.Installs {
			if inst.Tool != tool {
				kept = append(kept, inst)
			}
		}
		d.State.Installs = kept
		delete(d.State.Active, tool)
	}
	return nil
}

// manageConfigFiles retourne les fichiers de configuration gérés par l'état,
// sans doublon.
func manageConfigFiles(st *state.State) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range st.Keys {
		if seen[k.File] {
			continue
		}
		seen[k.File] = true
		out = append(out, k.File)
	}
	return out
}

// manageRestoreConfig restaure un fichier de configuration géré.
func manageRestoreConfig(path string, st *state.State) error {
	switch {
	case filepath.Base(path) == "settings.xml":
		return config.RestoreMavenSettings(path, st)
	case strings.EqualFold(filepath.Ext(path), ".ini"):
		sections := map[string]bool{}
		for _, k := range st.KeysFor(path) {
			if i := strings.Index(k.Key, "."); i > 0 {
				sections[k.Key[:i]] = true
			}
		}
		for section := range sections {
			if err := config.RestoreINI(path, section, st); err != nil {
				return err
			}
		}
		return nil
	default:
		return config.RestoreKeys(path, st)
	}
}

// manageUninstall retire les outils visés puis, si all est vrai, restaure la
// configuration gérée et la configuration réseau.
func manageUninstall(ctx context.Context, out io.Writer, d install.Deps, tool string, all bool) error {
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}

	var tools []string
	if all {
		if d.State != nil {
			seen := map[string]bool{}
			for _, inst := range d.State.Installs {
				if !seen[inst.Tool] {
					seen[inst.Tool] = true
					tools = append(tools, inst.Tool)
				}
			}
		}
	} else {
		if tool == "" {
			return errors.New("outil manquant : précisez un outil ou --all")
		}
		tools = []string{tool}
	}

	for _, t := range tools {
		if err := manageRemoveTool(d, t); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ %s désinstallé\n", t)
	}

	if all {
		if d.State != nil {
			for _, path := range manageConfigFiles(d.State) {
				if err := manageRestoreConfig(path, d.State); err != nil {
					return fmt.Errorf("restauration de %q : %w", path, err)
				}
				fmt.Fprintf(out, "✓ configuration restaurée : %s\n", path)
			}
		}
		for _, name := range manageNetworkVars {
			if err := d.Env.DeleteUserVar(name); err != nil {
				return fmt.Errorf("suppression de la variable %q : %w", name, err)
			}
		}
		if d.Run != nil {
			_, _ = d.Run.Run(ctx, execx.Cmd{
				Name: "reg.exe",
				Args: []string{"delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "DevStationPx", "/f"},
			})
		}
		if d.State != nil {
			d.State.Keys = nil
		}
	}

	if err := d.Env.Broadcast(); err != nil {
		return fmt.Errorf("notification d'environnement : %w", err)
	}

	fmt.Fprintln(out, "Désinstallation terminée.")
	fmt.Fprintln(out, "Les sauvegardes *.devstation.bak restent disponibles.")
	return nil
}

// manageReposSync clone les dépôts manquants du profil p dans l'espace de
// travail des réponses a.
func manageReposSync(ctx context.Context, out io.Writer, r execx.Runner, p *profile.Profile, a *profile.Answers) error {
	if p == nil || a == nil {
		return errors.New("profil ou réponses manquants")
	}
	for _, name := range a.Repos {
		url := ""
		for _, repo := range p.Repos {
			if repo.Name == name {
				url = repo.URL
				break
			}
		}
		if url == "" {
			return fmt.Errorf("dépôt inconnu dans le profil : %q", name)
		}
		dir := filepath.Join(a.Workspace, name)
		cloned, err := gitx.Clone(ctx, r, url, dir)
		if err != nil {
			return err
		}
		if cloned {
			fmt.Fprintf(out, "✓ %s cloné\n", name)
		} else {
			fmt.Fprintf(out, "↷ %s déjà présent\n", name)
		}
	}
	return nil
}

// manageSecretsGet lit un champ d'un secret et l'écrit sur la sortie, ou le
// copie dans le presse-papiers sans l'afficher.
func manageSecretsGet(ctx context.Context, out io.Writer, r execx.Runner, vaultExe, path, field string, copyToClipboard bool) error {
	value, err := vaultx.Get(ctx, r, vaultExe, path, field)
	if err != nil {
		return err
	}
	if copyToClipboard {
		if _, err := r.Run(ctx, execx.Cmd{Name: "clip.exe", Stdin: value, Secret: true}); err != nil {
			return fmt.Errorf("copie dans le presse-papiers impossible : %w", err)
		}
		fmt.Fprintln(out, "Copié dans le presse-papiers")
		return nil
	}
	fmt.Fprintln(out, value)
	return nil
}

// manageSecretsList liste les entrées d'un chemin du coffre.
func manageSecretsList(ctx context.Context, out io.Writer, r execx.Runner, vaultExe, path string) error {
	keys, err := vaultx.List(ctx, r, vaultExe, path)
	if err != nil {
		return err
	}
	for _, k := range keys {
		fmt.Fprintln(out, k)
	}
	return nil
}

// manageVaultExe retourne le chemin de l'exécutable vault.
func manageVaultExe(d install.Deps) string {
	return filepath.Join(install.CurrentDir(d, "vault"), "vault.exe")
}

// manageLoadProfileAnswers charge le profil et les réponses de l'application.
func manageLoadProfileAnswers(app *App, profilePath string) (*profile.Profile, *profile.Answers, error) {
	p, err := profile.Load(profilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("chargement du profil %q : %w", profilePath, err)
	}
	a, err := profile.LoadAnswers(app.AnswersPath)
	if err != nil {
		return nil, nil, fmt.Errorf("chargement des réponses %q : %w", app.AnswersPath, err)
	}
	return p, a, nil
}

// manageRunReposSync construit l'application et clone les dépôts manquants.
func manageRunReposSync(ctx context.Context, flags GlobalFlags, profilePath string) error {
	app, err := NewApp(flags)
	if err != nil {
		return err
	}
	defer app.Close()
	p, a, err := manageLoadProfileAnswers(app, profilePath)
	if err != nil {
		return err
	}
	return manageReposSync(ctx, os.Stdout, execx.OS(), p, a)
}

// manageRunUninstall construit l'application et désinstalle les outils visés.
func manageRunUninstall(ctx context.Context, flags GlobalFlags, profilePath, tool string, all, yes bool) error {
	app, err := NewApp(flags)
	if err != nil {
		return err
	}
	defer app.Close()
	p, err := profile.Load(profilePath)
	if err != nil {
		return fmt.Errorf("chargement du profil %q : %w", profilePath, err)
	}
	deps, err := app.Deps(p.Network)
	if err != nil {
		return err
	}
	if !yes {
		target := tool
		if all {
			target = "tous les outils installés"
		}
		fmt.Fprintf(os.Stdout, "Confirmer la désinstallation de %s ? [o/N] ", target)
		if !manageConfirm(os.Stdin) {
			fmt.Fprintln(os.Stdout, "Désinstallation annulée.")
			return nil
		}
	}
	return manageUninstall(ctx, os.Stdout, deps, tool, all)
}

// manageRunSecrets construit l'application et exécute une commande secrets.
func manageRunSecrets(ctx context.Context, flags GlobalFlags, profilePath, path, field string, copyToClipboard bool) error {
	app, err := NewApp(flags)
	if err != nil {
		return err
	}
	defer app.Close()
	p, err := profile.Load(profilePath)
	if err != nil {
		return fmt.Errorf("chargement du profil %q : %w", profilePath, err)
	}
	deps, err := app.Deps(p.Network)
	if err != nil {
		return err
	}
	vaultExe := manageVaultExe(deps)
	if field == "" {
		return manageSecretsList(ctx, os.Stdout, deps.Run, vaultExe, path)
	}
	return manageSecretsGet(ctx, os.Stdout, deps.Run, vaultExe, path, field, copyToClipboard)
}

// manageGlobalFlags retourne les options globales lues sur la commande racine.
func manageGlobalFlags(cmd *cobra.Command) GlobalFlags {
	var g GlobalFlags
	if f := cmd.Flags().Lookup("no-color"); f != nil {
		g.NoColor, _ = cmd.Flags().GetBool("no-color")
	}
	if f := cmd.Flags().Lookup("verbose"); f != nil {
		g.Verbose, _ = cmd.Flags().GetBool("verbose")
	}
	if f := cmd.Flags().Lookup("log"); f != nil {
		g.LogPath, _ = cmd.Flags().GetString("log")
	}
	return g
}

// newReposCmd construit la sous-commande repos.
func newReposCmd() *cobra.Command {
	profilePath := filepath.Join("profiles", "demo.yaml")
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "Gère les dépôts du projet",
	}
	sync := &cobra.Command{
		Use:   "sync",
		Short: "Clone les dépôts manquants",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manageRunReposSync(cmd.Context(), manageGlobalFlags(cmd), profilePath)
		},
	}
	sync.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	cmd.AddCommand(sync)
	return cmd
}

// newUninstallCmd construit la sous-commande uninstall.
func newUninstallCmd() *cobra.Command {
	profilePath := filepath.Join("profiles", "demo.yaml")
	var all, yes bool
	cmd := &cobra.Command{
		Use:   "uninstall [outil]",
		Short: "Retire ce que DevStation a installé",
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := ""
			if len(args) > 0 {
				tool = args[0]
			}
			return manageRunUninstall(cmd.Context(), manageGlobalFlags(cmd), profilePath, tool, all, yes)
		},
	}
	cmd.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	cmd.Flags().BoolVar(&all, "all", false, "désinstalle tous les outils et restaure la configuration")
	cmd.Flags().BoolVar(&yes, "yes", false, "ne pose aucune question")
	return cmd
}

// newSecretsCmd construit la sous-commande secrets.
func newSecretsCmd() *cobra.Command {
	profilePath := filepath.Join("profiles", "demo.yaml")
	var copyToClipboard bool
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Lit les secrets du coffre de l'équipe",
	}
	list := &cobra.Command{
		Use:   "list [chemin]",
		Short: "Liste les secrets d'un chemin",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 {
				path = args[0]
			}
			return manageRunSecrets(cmd.Context(), manageGlobalFlags(cmd), profilePath, path, "", false)
		},
	}
	list.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	get := &cobra.Command{
		Use:   "get <chemin> <champ>",
		Short: "Lit un champ d'un secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return manageRunSecrets(cmd.Context(), manageGlobalFlags(cmd), profilePath, args[0], args[1], copyToClipboard)
		},
	}
	get.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	get.Flags().BoolVar(&copyToClipboard, "copy", false, "copie la valeur dans le presse-papiers sans l'afficher")
	cmd.AddCommand(list, get)
	return cmd
}

// newVaultTokenHelperCmd construit la sous-commande cachée vault-token-helper.
func newVaultTokenHelperCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "vault-token-helper",
		Short:  "Helper de token Vault",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return vaultx.TokenHelper(args, os.Stdin, os.Stdout, winenv.NewCreds())
		},
	}
}

// init enregistre les sous-commandes de gestion.
func init() {
	extraCommands = append(extraCommands, newReposCmd, newUninstallCmd, newSecretsCmd, newVaultTokenHelperCmd)
}
