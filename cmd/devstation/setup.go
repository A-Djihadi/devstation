package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/network"
	"github.com/A-Djihadi/devstation/internal/plan"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/setup"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/ui"
)

// SetupOptions regroupe les options de la commande setup.
type SetupOptions struct {
	// ProfilePath est le chemin du profil d'équipe.
	ProfilePath string
	// AnswersPath est le chemin d'un fichier de réponses existant.
	AnswersPath string
	// Yes exécute sans poser de questions.
	Yes bool
	// DryRun affiche les étapes sans rien installer.
	DryRun bool
}

// SetupHooks regroupe les points d'extension de runSetup (remplacés en test).
type SetupHooks struct {
	// Sources construit les sources de versions.
	Sources func(f sources.Fetcher, p *profile.Profile) map[string]sources.Source
	// Wizard lance l'assistant interactif.
	Wizard func(p *profile.Profile, a *profile.Answers, v map[string][]sources.Release, io ui.WizardIO) (*profile.Answers, error)
	// Token demande un token GitHub facultatif.
	Token func(io ui.WizardIO) (string, error)
	// Getenv lit une variable d'environnement.
	Getenv func(string) string
	// Out est la destination de l'affichage.
	Out io.Writer
}

// withDefaults complète les hooks non renseignés.
func (h SetupHooks) withDefaults() SetupHooks {
	if h.Sources == nil {
		h.Sources = DefaultSources
	}
	if h.Wizard == nil {
		h.Wizard = ui.Wizard
	}
	if h.Token == nil {
		h.Token = ui.AskToken
	}
	if h.Getenv == nil {
		h.Getenv = os.Getenv
	}
	if h.Out == nil {
		h.Out = os.Stdout
	}
	return h
}

// profileVersions retourne les versions autorisées du profil pour un outil.
func profileVersions(p *profile.Profile, tool string) []string {
	if p == nil {
		return nil
	}
	switch tool {
	case "java":
		return p.Tools.Java.Versions
	case "node":
		return p.Tools.Node.Versions
	case "angular_cli":
		return p.Tools.AngularCLI.Versions
	case "python":
		return p.Tools.Python.Versions
	case "maven":
		return p.Tools.Maven.Versions
	case "gradle":
		return p.Tools.Gradle.Versions
	}
	return nil
}

// profileDefault retourne la version par défaut du profil pour un outil.
func profileDefault(p *profile.Profile, tool string) string {
	if p == nil {
		return ""
	}
	switch tool {
	case "java":
		return p.Tools.Java.Default
	case "node":
		return p.Tools.Node.Default
	case "angular_cli":
		return p.Tools.AngularCLI.Default
	case "python":
		return p.Tools.Python.Default
	case "maven":
		return p.Tools.Maven.Default
	case "gradle":
		return p.Tools.Gradle.Default
	}
	return ""
}

// wantedVersion choisit la version demandée pour un outil.
func wantedVersion(a *profile.Answers, p *profile.Profile, tool string) string {
	if a != nil {
		if v := a.Versions[tool]; v != "" {
			return v
		}
	}
	if v := profileDefault(p, tool); v != "" {
		return v
	}
	if tool == "java" || tool == "node" {
		return "lts"
	}
	return "latest"
}

// setupTools retourne les outils à installer, complétés de px et compose.
func setupTools(a *profile.Answers, p *profile.Profile) []string {
	tools := setup.Tools(a, p)
	_, usePx := network.Effective(setupEffectiveNetwork(a, p))
	if usePx {
		tools = append(tools, "px")
	}
	for _, t := range tools {
		if t == "podman" {
			tools = append(tools, "compose")
			break
		}
	}
	return tools
}

// setupEffectiveNetwork retourne la configuration réseau effective des réponses.
func setupEffectiveNetwork(a *profile.Answers, p *profile.Profile) profile.Network {
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

// runSetup charge le profil, résout les versions et exécute le plan.
func runSetup(ctx context.Context, app *App, deps install.Deps, o SetupOptions, h SetupHooks) error {
	h = h.withDefaults()

	p, err := profile.Load(o.ProfilePath)
	if err != nil {
		return fmt.Errorf("chargement du profil %q : %w", o.ProfilePath, err)
	}

	var a *profile.Answers
	switch {
	case o.AnswersPath != "":
		a, err = profile.LoadAnswers(o.AnswersPath)
		if err != nil {
			return fmt.Errorf("chargement des réponses %q : %w", o.AnswersPath, err)
		}
	default:
		if _, statErr := os.Stat(app.AnswersPath); statErr == nil {
			a, err = profile.LoadAnswers(app.AnswersPath)
			if err != nil {
				return fmt.Errorf("chargement des réponses %q : %w", app.AnswersPath, err)
			}
		} else {
			a = profile.Defaults(p, app.Home)
		}
	}

	srcs := h.Sources(deps.DL, p)

	// Résolution des versions disponibles.
	fmt.Fprintln(h.Out, "Recherche des versions…")
	releases := map[string]sources.Release{}
	available := map[string][]sources.Release{}
	for _, tool := range setupTools(a, p) {
		src, ok := srcs[tool]
		if !ok {
			return fmt.Errorf("source de versions manquante pour l'outil %q", tool)
		}
		rs, err := src.Releases(ctx)
		if err != nil {
			return fmt.Errorf("recherche des versions de l'outil %q : %w", tool, err)
		}
		rs = sources.Restrict(rs, profileVersions(p, tool))
		available[tool] = rs
	}

	// Assistant interactif.
	var token string
	if !o.Yes {
		wio := ui.WizardIO{In: os.Stdin, Out: h.Out}
		a, err = h.Wizard(p, a, available, wio)
		if err != nil {
			return err
		}
		token, err = h.Token(wio)
		if err != nil {
			return err
		}
	} else {
		token = h.Getenv("DEVSTATION_GITHUB_TOKEN")
	}
	if token != "" && app.Log != nil {
		app.Log.AddSecret(token)
	}

	// L'assistant a pu ajouter des rôles : complète les versions manquantes.
	for _, tool := range setupTools(a, p) {
		if _, ok := available[tool]; ok {
			continue
		}
		src, ok := srcs[tool]
		if !ok {
			return fmt.Errorf("source de versions manquante pour l'outil %q", tool)
		}
		rs, err := src.Releases(ctx)
		if err != nil {
			return fmt.Errorf("recherche des versions de l'outil %q : %w", tool, err)
		}
		available[tool] = sources.Restrict(rs, profileVersions(p, tool))
	}

	// Résolution des releases retenues.
	for _, tool := range setupTools(a, p) {
		src, ok := srcs[tool]
		if !ok {
			return fmt.Errorf("source de versions manquante pour l'outil %q", tool)
		}
		r, err := sources.PickResolved(ctx, src, available[tool], wantedVersion(a, p, tool))
		if err != nil {
			return fmt.Errorf("résolution de la version de l'outil %q : %w", tool, err)
		}
		releases[tool] = r
	}

	steps, err := setup.Build(setup.Options{
		Deps:     deps,
		Answers:  a,
		Profile:  p,
		Releases: releases,
		Paths:    network.DefaultPaths(app.Home, app.AppData, app.Root),
		Dirs: setup.Dirs{
			Bin:       filepath.Join(app.Root, "bin"),
			Home:      app.Home,
			StartMenu: filepath.Join(app.AppData, "Microsoft", "Windows", "Start Menu", "Programs"),
			Workspace: a.Workspace,
			PxDir:     filepath.Join(app.Root, "px"),
		},
		GitHubToken: token,
	})
	if err != nil {
		return fmt.Errorf("construction du plan : %w", err)
	}

	if o.DryRun {
		for i, s := range steps {
			fmt.Fprintf(h.Out, "%d. %s\n", i+1, s.Title)
		}
		fmt.Fprintln(h.Out, "(simulation : rien n'a été installé)")
		return nil
	}

	titles := make(map[string]string, len(steps))
	for _, s := range steps {
		titles[s.ID] = s.Title
	}
	progress := ui.NewProgress(h.Out, app.Theme, titles, app.Interactive)

	if app.Log != nil {
		app.Log.Printf("devstation %s setup", version)
		app.Log.Printf("profil : %s", o.ProfilePath)
	}

	handle := func(ev plan.Event) {
		progress.Handle(ev)
		if app.Log == nil {
			return
		}
		switch ev.Status {
		case plan.Running:
			app.Log.Printf("étape %s : en cours", ev.StepID)
		case plan.Done:
			app.Log.Printf("étape %s : réussie", ev.StepID)
		case plan.Skipped:
			app.Log.Printf("étape %s : ignorée", ev.StepID)
		case plan.Failed:
			app.Log.Printf("étape %s : échec : %v", ev.StepID, ev.Err)
		case plan.Blocked:
			app.Log.Printf("étape %s : bloquée", ev.StepID)
		}
	}

	summary, err := plan.Execute(ctx, steps, handle)
	progress.Close(summary, app.LogPath)
	if app.Log != nil {
		app.Log.Printf("résumé : %d réussite(s), %d échec(s)", len(summary.Done), len(summary.Failed))
	}
	if err != nil {
		return err
	}

	if err := a.Save(app.AnswersPath); err != nil {
		return fmt.Errorf("écriture des réponses %q : %w", app.AnswersPath, err)
	}

	if len(summary.Failed) > 0 {
		return fmt.Errorf("%d étape(s) en échec, voir %s", len(summary.Failed), app.LogPath)
	}
	return nil
}

// runSetupCmd construit l'application et exécute runSetup.
func runSetupCmd(ctx context.Context, flags GlobalFlags, o SetupOptions) error {
	app, err := NewApp(flags)
	if err != nil {
		return err
	}
	defer app.Close()

	p, err := profile.Load(o.ProfilePath)
	if err != nil {
		return fmt.Errorf("chargement du profil %q : %w", o.ProfilePath, err)
	}
	deps, err := app.Deps(p.Network)
	if err != nil {
		return err
	}
	return runSetup(ctx, app, deps, o, SetupHooks{})
}

// newSetupCmd construit la sous-commande setup.
func newSetupCmd(flags *GlobalFlags) *cobra.Command {
	o := SetupOptions{ProfilePath: filepath.Join("profiles", "demo.yaml")}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Installe et configure le poste",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupCmd(cmd.Context(), *flags, o)
		},
	}
	cmd.Flags().StringVar(&o.ProfilePath, "profile", o.ProfilePath, "profil d'équipe (YAML)")
	cmd.Flags().StringVar(&o.AnswersPath, "answers", "", "fichier de réponses existant")
	cmd.Flags().BoolVar(&o.Yes, "yes", false, "exécute sans poser de questions")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "affiche les étapes sans rien installer")
	return cmd
}

// errNoSteps indique qu'aucune étape n'a été construite.
var errNoSteps = errors.New("aucune étape à exécuter")
