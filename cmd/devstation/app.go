package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/logx"
	"github.com/A-Djihadi/devstation/internal/network"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
	"github.com/A-Djihadi/devstation/internal/state"
	"github.com/A-Djihadi/devstation/internal/ui"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// GlobalFlags regroupe les options communes à toutes les commandes.
type GlobalFlags struct {
	// NoColor désactive les couleurs et styles.
	NoColor bool
	// Verbose active l'affichage détaillé.
	Verbose bool
	// LogPath force le chemin du fichier de journal.
	LogPath string
}

// App regroupe les chemins, l'état et les services partagés par les commandes.
type App struct {
	// Home est le dossier utilisateur.
	Home string
	// AppData est le dossier des données applicatives itinérantes.
	AppData string
	// LocalAppData est le dossier des données applicatives locales.
	LocalAppData string
	// Root est le dossier DevStation (%LOCALAPPDATA%\DevStation).
	Root string
	// StatePath est le chemin de state.json.
	StatePath string
	// AnswersPath est le chemin de answers.yaml.
	AnswersPath string
	// LogPath est le chemin du fichier de journal.
	LogPath string
	// Log est le journal ; peut être nil.
	Log *logx.Logger
	// State est l'état local persistant.
	State *state.State
	// Theme est le thème d'affichage.
	Theme ui.Theme
	// Interactive indique si la sortie standard est un terminal.
	Interactive bool
}

// isTerminal indique si f est un terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// envOr retourne la variable d'environnement name ou fallback si elle est vide.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// NewApp construit l'application à partir des options globales g.
func NewApp(g GlobalFlags) (*App, error) {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("dossier utilisateur introuvable : %w", err)
		}
		home = h
	}
	appData := envOr("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	localAppData := envOr("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))

	root := filepath.Join(localAppData, "DevStation")
	statePath := filepath.Join(home, ".devstation", "state.json")
	answersPath := filepath.Join(home, ".devstation", "answers.yaml")

	for _, dir := range []string{
		root,
		filepath.Join(root, "logs"),
		filepath.Join(root, "cache"),
		filepath.Join(root, "tools"),
		filepath.Join(root, "bin"),
		filepath.Join(root, "px"),
		filepath.Dir(statePath),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("création de %q : %w", dir, err)
		}
	}

	logPath := g.LogPath
	if logPath == "" {
		logPath = filepath.Join(root, "logs", time.Now().Format("20060102-150405")+".log")
	}
	log, err := logx.New(logPath)
	if err != nil {
		return nil, fmt.Errorf("ouverture du journal %q : %w", logPath, err)
	}

	st, err := state.Load(statePath)
	if err != nil {
		log.Close()
		return nil, fmt.Errorf("lecture de l'état %q : %w", statePath, err)
	}

	interactive := isTerminal(os.Stdout)
	color := ui.ColorEnabled(interactive, os.Getenv) && !g.NoColor

	return &App{
		Home:         home,
		AppData:      appData,
		LocalAppData: localAppData,
		Root:         root,
		StatePath:    statePath,
		AnswersPath:  answersPath,
		LogPath:      logPath,
		Log:          log,
		State:        st,
		Theme:        ui.NewTheme(color),
		Interactive:  interactive,
	}, nil
}

// Deps construit les dépendances réelles des installeurs pour la configuration
// réseau n.
func (a *App) Deps(n profile.Network) (install.Deps, error) {
	proxyURL, _ := network.Effective(n)
	dl, err := download.New(download.Options{
		ProxyURL:    proxyURL,
		CACertFiles: n.CACerts,
		CacheDir:    filepath.Join(a.Root, "cache"),
	})
	if err != nil {
		return install.Deps{}, fmt.Errorf("initialisation du téléchargement : %w", err)
	}
	var run execx.Runner = execx.OS()
	if a.Log != nil {
		run = execx.Logged(run, a.Log.Printf)
	}
	return install.Deps{
		DL:    dl,
		Env:   winenv.NewEnv(),
		Links: winenv.NewLinks(run),
		Certs: winenv.NewCertStore(),
		Creds: winenv.NewCreds(),
		Run:   run,
		State: a.State,
		Log:   a.Log,
		Root:  filepath.Join(a.Root, "tools"),
	}, nil
}

// Close sauvegarde l'état puis ferme le journal.
func (a *App) Close() error {
	var errs []error
	if a.State != nil {
		if err := a.State.Save(a.StatePath); err != nil {
			errs = append(errs, fmt.Errorf("écriture de l'état %q : %w", a.StatePath, err))
		}
	}
	if a.Log != nil {
		if err := a.Log.Close(); err != nil {
			errs = append(errs, err)
		}
		a.Log = nil
	}
	return errors.Join(errs...)
}

// javaFeatures extrait les versions majeures de Java du profil p ; à défaut,
// elle retourne 21 et 17.
func javaFeatures(p *profile.Profile) []int {
	var out []int
	if p != nil {
		for _, v := range p.Tools.Java.Versions {
			n := 0
			for _, c := range v {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			if n > 0 {
				out = append(out, n)
			}
		}
	}
	if len(out) == 0 {
		out = []int{21, 17}
	}
	return out
}

// DefaultSources construit les sources de versions officielles.
func DefaultSources(f sources.Fetcher, p *profile.Profile) map[string]sources.Source {
	registry := "https://registry.npmjs.org"
	if p != nil && p.Network.Nexus.Npm != "" {
		registry = p.Network.Nexus.Npm
	}
	return map[string]sources.Source{
		"java":        sources.NewAdoptium(f, "https://api.adoptium.net", javaFeatures(p)),
		"node":        sources.NewNode(f, "https://nodejs.org"),
		"angular_cli": sources.NewNpm(f, registry, "@angular/cli", "angular_cli"),
		"python":      sources.NewPython(f, "https://www.python.org"),
		"maven":       sources.NewMaven(f, "https://repo.maven.apache.org"),
		"gradle":      sources.NewGradle(f, "https://services.gradle.org"),
		"vscode":      sources.NewVSCode(f, "https://update.code.visualstudio.com"),
		"intellij":    sources.NewIntelliJ(f, "https://data.services.jetbrains.com"),
		"podman":      sources.NewPodman(f, "https://api.github.com"),
		"compose":     sources.NewCompose(f, "https://api.github.com"),
		"px":          sources.NewPx(f, "https://api.github.com"),
		"vault":       sources.NewVault(f, "https://releases.hashicorp.com"),
	}
}
