package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/A-Djihadi/devstation/internal/doctor"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/setup"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
)

// toolsDefaultProfile est le profil utilisé par défaut par les sous-commandes.
const toolsDefaultProfile = "profiles/demo.yaml"

// init enregistre les sous-commandes doctor, use, list et version.
func init() {
	extraCommands = append(extraCommands, newToolsDoctorCmd, newToolsUseCmd, newToolsListCmd, newToolsVersionCmd)
}

// toolsGlobalFlags lit les options globales héritées au moment de l'exécution.
func toolsGlobalFlags(cmd *cobra.Command) GlobalFlags {
	var g GlobalFlags
	g.NoColor, _ = cmd.Flags().GetBool("no-color")
	g.Verbose, _ = cmd.Flags().GetBool("verbose")
	g.LogPath, _ = cmd.Flags().GetString("log")
	return g
}

// toolsInstallers construit un installeur par outil de setup.Tools(a, p).
func toolsInstallers(app *App, p *profile.Profile, a *profile.Answers) map[string]install.Installer {
	out := map[string]install.Installer{}
	for _, tool := range setup.Tools(a, p) {
		switch tool {
		case "java":
			out[tool] = install.NewJava(a.CACerts)
		case "node":
			out[tool] = install.NewNode(a.Versions["npm"], a.Versions["angular_cli"], "")
		case "python":
			out[tool] = install.NewPython()
		case "maven":
			out[tool] = install.NewMaven()
		case "gradle":
			out[tool] = install.NewGradle()
		case "vscode":
			out[tool] = install.NewVSCode(nil)
		case "intellij":
			out[tool] = install.NewIntelliJ(nil, "")
		case "podman":
			mirror := ""
			if p != nil {
				mirror = p.Network.Nexus.Containers
			}
			out[tool] = install.NewPodman(sources.Release{}, a.CACerts, mirror)
		case "vault":
			addr := ""
			if p != nil {
				addr = p.Vault.Addr
			}
			out[tool] = install.NewVaultCLI(addr, "", filepath.Join(app.Root, "bin"), app.Home)
		}
	}
	return out
}

// toolsProbe vérifie la joignabilité HTTP d'une URL par une requête HEAD.
func toolsProbe(ctx context.Context, url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("réponse HTTP %d", resp.StatusCode)
	}
	return nil
}

// toolsRunDoctor exécute le diagnostic et retourne le code de sortie.
func toolsRunDoctor(ctx context.Context, out io.Writer, o doctor.Options) int {
	checks := doctor.Run(ctx, o)
	fails := 0
	for _, c := range checks {
		symbol := "✓"
		switch c.Status {
		case doctor.Warn:
			symbol = "⚠"
		case doctor.Fail:
			symbol = "✗"
			fails++
		}
		fmt.Fprintf(out, "%s %s : %s\n", symbol, c.Name, c.Detail)
		if c.Fix != "" {
			fmt.Fprintf(out, "  → %s\n", c.Fix)
		}
	}
	fmt.Fprintf(out, "%d problème(s) bloquant(s)\n", fails)
	if fails > 0 {
		return 1
	}
	return 0
}

// toolsUse active une version déjà installée ou l'installe puis l'active.
func toolsUse(ctx context.Context, out io.Writer, d install.Deps, inst install.Installer, src sources.Source, version string) error {
	if d.State != nil {
		if _, ok := d.State.Installed(inst.Tool(), version); ok {
			if err := inst.Activate(d, version); err != nil {
				return err
			}
			fmt.Fprintf(out, "%s %s est maintenant actif (ouvrez un nouveau terminal)\n", inst.Tool(), version)
			return nil
		}
	}

	if src == nil {
		return fmt.Errorf("source de versions manquante pour l'outil %q", inst.Tool())
	}
	rs, err := src.Releases(ctx)
	if err != nil {
		return fmt.Errorf("recherche des versions de l'outil %q : %w", inst.Tool(), err)
	}
	r, err := sources.PickResolved(ctx, src, rs, version)
	if err != nil {
		return fmt.Errorf("résolution de la version de l'outil %q : %w", inst.Tool(), err)
	}
	if err := inst.Install(ctx, d, r, nil); err != nil {
		return err
	}
	if err := inst.Activate(d, r.Version); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s %s est maintenant actif (ouvrez un nouveau terminal)\n", inst.Tool(), r.Version)
	return nil
}

// toolsList affiche les versions installées, filtrées par outil si tool est non vide.
func toolsList(out io.Writer, st *state.State, tool string) {
	if st == nil {
		return
	}
	installs := make([]state.Install, 0, len(st.Installs))
	for _, i := range st.Installs {
		if tool != "" && i.Tool != tool {
			continue
		}
		installs = append(installs, i)
	}
	sort.SliceStable(installs, func(i, j int) bool {
		if installs[i].Tool != installs[j].Tool {
			return installs[i].Tool < installs[j].Tool
		}
		return sources.CompareVersions(installs[i].Version, installs[j].Version) > 0
	})
	for _, i := range installs {
		line := i.Tool + " " + i.Version
		if st.Active[i.Tool] == i.Version {
			line += " (active)"
		}
		fmt.Fprintln(out, line)
	}
}

// toolsListAvailable affiche les dix premières versions disponibles d'un outil.
func toolsListAvailable(ctx context.Context, out io.Writer, src sources.Source, tool string) error {
	if src == nil {
		return fmt.Errorf("source de versions manquante pour l'outil %q", tool)
	}
	rs, err := src.Releases(ctx)
	if err != nil {
		return fmt.Errorf("recherche des versions de l'outil %q : %w", tool, err)
	}
	sources.SortDesc(rs)
	if len(rs) > 10 {
		rs = rs[:10]
	}
	for _, r := range rs {
		line := tool + " " + r.Version
		if r.LTS {
			line += " (LTS)"
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

// newToolsDoctorCmd construit la sous-commande doctor.
func newToolsDoctorCmd() *cobra.Command {
	profilePath := toolsDefaultProfile
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Vérifie le poste",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(toolsGlobalFlags(cmd))
			if err != nil {
				return err
			}
			defer app.Close()

			p, err := profile.Load(profilePath)
			if err != nil {
				return fmt.Errorf("chargement du profil %q : %w", profilePath, err)
			}
			a, err := profile.LoadAnswers(app.AnswersPath)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Aucune installation trouvée : lancez d'abord devstation setup")
				return exitCode(1)
			}
			deps, err := app.Deps(p.Network)
			if err != nil {
				return err
			}
			code := toolsRunDoctor(cmd.Context(), cmd.OutOrStdout(), doctor.Options{
				Deps:       deps,
				Answers:    a,
				Profile:    p,
				Installers: toolsInstallers(app, p, a),
				PxAddr:     "127.0.0.1:3128",
				Probe:      toolsProbe,
			})
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	return cmd
}

// newToolsUseCmd construit la sous-commande use.
func newToolsUseCmd() *cobra.Command {
	profilePath := toolsDefaultProfile
	cmd := &cobra.Command{
		Use:   "use <outil> <version>",
		Short: "Change la version active d'un outil",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, version := args[0], args[1]

			app, err := NewApp(toolsGlobalFlags(cmd))
			if err != nil {
				return err
			}
			defer app.Close()

			p, err := profile.Load(profilePath)
			if err != nil {
				return fmt.Errorf("chargement du profil %q : %w", profilePath, err)
			}
			a, err := profile.LoadAnswers(app.AnswersPath)
			if err != nil {
				a = profile.Defaults(p, app.Home)
			}
			deps, err := app.Deps(p.Network)
			if err != nil {
				return err
			}

			inst, ok := toolsInstallers(app, p, a)[tool]
			if !ok {
				names := make([]string, 0, len(toolsInstallers(app, p, a)))
				for name := range toolsInstallers(app, p, a) {
					names = append(names, name)
				}
				sort.Strings(names)
				return fmt.Errorf("outil inconnu : %q (outils possibles : %v)", tool, names)
			}

			src := DefaultSources(deps.DL, p)[tool]
			return toolsUse(cmd.Context(), cmd.OutOrStdout(), deps, inst, src, version)
		},
	}
	cmd.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	return cmd
}

// newToolsListCmd construit la sous-commande list.
func newToolsListCmd() *cobra.Command {
	profilePath := toolsDefaultProfile
	available := false
	cmd := &cobra.Command{
		Use:   "list [outil]",
		Short: "Liste les versions installées",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := ""
			if len(args) > 0 {
				tool = args[0]
			}

			app, err := NewApp(toolsGlobalFlags(cmd))
			if err != nil {
				return err
			}
			defer app.Close()

			if available && tool != "" {
				p, err := profile.Load(profilePath)
				if err != nil {
					return fmt.Errorf("chargement du profil %q : %w", profilePath, err)
				}
				deps, err := app.Deps(p.Network)
				if err != nil {
					return err
				}
				return toolsListAvailable(cmd.Context(), cmd.OutOrStdout(), DefaultSources(deps.DL, p)[tool], tool)
			}

			if app.State == nil || len(app.State.Installs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Aucun outil installé. Lancez devstation setup.")
				return nil
			}
			toolsList(cmd.OutOrStdout(), app.State, tool)
			return nil
		},
	}
	cmd.Flags().StringVar(&profilePath, "profile", profilePath, "profil d'équipe (YAML)")
	cmd.Flags().BoolVar(&available, "available", false, "liste les versions disponibles d'un outil")
	return cmd
}

// newToolsVersionCmd construit la sous-commande version.
func newToolsVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Affiche la version de DevStation",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "devstation %s\n", version)
			return nil
		},
	}
}
