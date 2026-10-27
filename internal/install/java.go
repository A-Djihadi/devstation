package install

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// javaVersionRe extrait le numéro de version d'une ligne de sortie de
// `java -version`.
var javaVersionRe = regexp.MustCompile(`version "([^"]+)"`)

// Java installe et active un JDK Temurin (Adoptium).
type Java struct {
	// CACerts liste les chemins de certificats racine à importer dans le
	// magasin de confiance du JDK.
	CACerts []string
}

var _ Installer = (*Java)(nil)

// NewJava crée un installeur Java important les certificats donnés.
func NewJava(caCerts []string) *Java {
	return &Java{CACerts: caCerts}
}

// Tool retourne le nom de l'outil géré.
func (j *Java) Tool() string {
	return "java"
}

// Install installe la version r puis importe les certificats racine dans le
// magasin de confiance du JDK.
func (j *Java) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir, err := InstallZip(ctx, d, "java", r, true, report)
	if err != nil {
		return err
	}

	keytool := filepath.Join(dir, "bin", "keytool.exe")
	for i, cert := range j.CACerts {
		alias := fmt.Sprintf("devstation-%d", i+1)
		res, err := d.Run.Run(ctx, execx.Cmd{
			Name: keytool,
			Args: []string{
				"-importcert",
				"-noprompt",
				"-cacerts",
				"-storepass", "changeit",
				"-alias", alias,
				"-file", cert,
			},
		})
		if err != nil {
			return fmt.Errorf("import du certificat %q : %w", cert, err)
		}
		if res.Code != 0 {
			out := res.Stdout + res.Stderr
			if strings.Contains(out, "already exists") {
				continue
			}
			return fmt.Errorf("import du certificat %q (code %d) : %s", cert, res.Code, strings.TrimSpace(res.Stderr))
		}
	}

	return nil
}

// Activate active la version donnée et définit JAVA_HOME.
func (j *Java) Activate(d Deps, version string) error {
	if err := SetCurrent(d, "java", version, []string{"bin"}); err != nil {
		return err
	}
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}
	if err := d.Env.SetUserVar("JAVA_HOME", CurrentDir(d, "java")); err != nil {
		return fmt.Errorf("écriture de JAVA_HOME : %w", err)
	}
	return nil
}

// Verify retourne la version de Java disponible via la jonction « current ».
func (j *Java) Verify(ctx context.Context, d Deps) (string, error) {
	java := filepath.Join(CurrentDir(d, "java"), "bin", "java.exe")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: java, Args: []string{"-version"}})
	if err != nil {
		return "", fmt.Errorf("exécution de java -version : %w", err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("java -version (code %d) : %s", res.Code, strings.TrimSpace(res.Stderr))
	}
	m := javaVersionRe.FindStringSubmatch(res.Stderr)
	if m == nil {
		return "", fmt.Errorf("format de version Java inattendu : %q", strings.TrimSpace(res.Stderr))
	}
	return m[1], nil
}
