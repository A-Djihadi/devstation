package install

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// Maven installe et active Apache Maven.
type Maven struct{}

// NewMaven crée un installeur Maven.
func NewMaven() *Maven {
	return &Maven{}
}

// Tool retourne le nom de l'outil géré.
func (m *Maven) Tool() string {
	return "maven"
}

// Install télécharge et extrait Maven dans le dossier de version.
func (m *Maven) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	_, err := InstallZip(ctx, d, "maven", r, true, report)
	return err
}

// Activate active la version donnée de Maven et définit MAVEN_HOME.
func (m *Maven) Activate(d Deps, version string) error {
	if err := SetCurrent(d, "maven", version, []string{"bin"}); err != nil {
		return err
	}
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}
	if err := d.Env.SetUserVar("MAVEN_HOME", CurrentDir(d, "maven")); err != nil {
		return fmt.Errorf("écriture de MAVEN_HOME : %w", err)
	}
	return nil
}

// Verify retourne la version de Maven effectivement disponible.
func (m *Maven) Verify(ctx context.Context, d Deps) (string, error) {
	if d.Run == nil {
		return "", errors.New("exécuteur de commandes manquant")
	}
	exe := filepath.Join(CurrentDir(d, "maven"), "bin", "mvn.cmd")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: exe, Args: []string{"-v"}})
	if err != nil {
		return "", fmt.Errorf("exécution de %q : %w", exe, err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("échec de %q (code %d) : %s", exe, res.Code, strings.TrimSpace(res.Stderr))
	}
	return buildtoolsParseMavenVersion(res.Stdout)
}

// Gradle installe et active Gradle.
type Gradle struct{}

// NewGradle crée un installeur Gradle.
func NewGradle() *Gradle {
	return &Gradle{}
}

// Tool retourne le nom de l'outil géré.
func (g *Gradle) Tool() string {
	return "gradle"
}

// Install télécharge et extrait Gradle dans le dossier de version.
func (g *Gradle) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	_, err := InstallZip(ctx, d, "gradle", r, true, report)
	return err
}

// Activate active la version donnée de Gradle et définit GRADLE_HOME.
func (g *Gradle) Activate(d Deps, version string) error {
	if err := SetCurrent(d, "gradle", version, []string{"bin"}); err != nil {
		return err
	}
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}
	if err := d.Env.SetUserVar("GRADLE_HOME", CurrentDir(d, "gradle")); err != nil {
		return fmt.Errorf("écriture de GRADLE_HOME : %w", err)
	}
	return nil
}

// Verify retourne la version de Gradle effectivement disponible.
func (g *Gradle) Verify(ctx context.Context, d Deps) (string, error) {
	if d.Run == nil {
		return "", errors.New("exécuteur de commandes manquant")
	}
	exe := filepath.Join(CurrentDir(d, "gradle"), "bin", "gradle.bat")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: exe, Args: []string{"--version"}})
	if err != nil {
		return "", fmt.Errorf("exécution de %q : %w", exe, err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("échec de %q (code %d) : %s", exe, res.Code, strings.TrimSpace(res.Stderr))
	}
	return buildtoolsParseGradleVersion(res.Stdout)
}

// buildtoolsParseMavenVersion extrait la version de la sortie de « mvn -v ».
func buildtoolsParseMavenVersion(out string) (string, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Apache Maven ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			break
		}
		return fields[2], nil
	}
	return "", errors.New("version de Maven introuvable dans la sortie de mvn -v")
}

// buildtoolsParseGradleVersion extrait la version de la sortie de « gradle --version ».
func buildtoolsParseGradleVersion(out string) (string, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Gradle ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		return fields[1], nil
	}
	return "", errors.New("version de Gradle introuvable dans la sortie de gradle --version")
}

var _ Installer = (*Maven)(nil)
var _ Installer = (*Gradle)(nil)
