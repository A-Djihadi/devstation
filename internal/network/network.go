// Package network applique la configuration réseau d'entreprise (proxy,
// dépôts Nexus, certificats) aux outils installés par DevStation.
package network

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/A-Djihadi/devstation/internal/config"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/install"
	"github.com/A-Djihadi/devstation/internal/profile"
)

// Paths regroupe les chemins des fichiers de configuration réseau des outils.
type Paths struct {
	// Npmrc est le chemin du fichier .npmrc.
	Npmrc string
	// PipIni est le chemin du fichier pip.ini.
	PipIni string
	// MavenSettings est le chemin du fichier settings.xml de Maven.
	MavenSettings string
	// GradleProps est le chemin du fichier gradle.properties.
	GradleProps string
	// CABundle est le chemin du bundle PEM des certificats d'entreprise.
	CABundle string
}

// DefaultPaths retourne les chemins par défaut des fichiers de configuration
// réseau à partir du profil utilisateur, du dossier de données applicatives et
// du dossier DevStation.
func DefaultPaths(home, appData, devstationDir string) Paths {
	return Paths{
		Npmrc:         filepath.Join(home, ".npmrc"),
		PipIni:        filepath.Join(appData, "pip", "pip.ini"),
		MavenSettings: filepath.Join(home, ".m2", "settings.xml"),
		GradleProps:   filepath.Join(home, ".gradle", "gradle.properties"),
		CABundle:      filepath.Join(devstationDir, "ca-bundle.pem"),
	}
}

// Effective retourne l'URL de proxy effective et indique si le proxy local
// px doit être utilisé. Une authentification Kerberos impose le proxy local.
func Effective(n profile.Network) (proxyURL string, usePx bool) {
	if n.ProxyAuth == "kerberos" {
		return "http://127.0.0.1:3128", true
	}
	return n.Proxy, false
}

// netProxyHostPort extrait l'hôte et le port d'une URL de proxy. Le port vaut
// 80 lorsqu'il n'est pas précisé.
func netProxyHostPort(proxyURL string) (host string, port int, err error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return "", 0, fmt.Errorf("URL de proxy invalide %q : %w", proxyURL, err)
	}
	host = u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("URL de proxy invalide %q : hôte manquant", proxyURL)
	}
	port = 80
	if p := u.Port(); p != "" {
		n, convErr := strconv.Atoi(p)
		if convErr != nil {
			return "", 0, fmt.Errorf("port de proxy invalide %q : %w", p, convErr)
		}
		port = n
	}
	return host, port, nil
}

// netWriteCABundle concatène les certificats PEM de paths dans bundle.
func netWriteCABundle(bundle string, paths []string) error {
	var b strings.Builder
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("lecture du certificat %q : %w", p, err)
		}
		b.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		return fmt.Errorf("création du dossier de %q : %w", bundle, err)
	}
	if err := os.WriteFile(bundle, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("écriture de %q : %w", bundle, err)
	}
	return nil
}

// netGitConfig exécute `git config --global key value`.
func netGitConfig(ctx context.Context, d install.Deps, key, value string) error {
	if d.Run == nil {
		return nil
	}
	res, err := d.Run.Run(ctx, execx.Cmd{Name: "git", Args: []string{"config", "--global", key, value}})
	if err != nil {
		return fmt.Errorf("git config %s : %w", key, err)
	}
	if res.Code != 0 {
		return fmt.Errorf("git config %s : code %d", key, res.Code)
	}
	return nil
}

// Apply applique la configuration réseau n aux fichiers p et aux outils.
// Seules les valeurs renseignées sont appliquées ; toutes les modifications
// de fichiers passent par le paquet config avec sauvegarde et restauration.
func Apply(ctx context.Context, d install.Deps, n profile.Network, p Paths) error {
	proxyURL, _ := Effective(n)

	// 1. Certificats : magasin utilisateur et bundle PEM.
	if len(n.CACerts) > 0 {
		if d.Certs != nil {
			for _, cert := range n.CACerts {
				if _, err := d.Certs.AddUserRoot(cert); err != nil {
					return fmt.Errorf("ajout du certificat %q : %w", cert, err)
				}
			}
		}
		if err := netWriteCABundle(p.CABundle, n.CACerts); err != nil {
			return err
		}
	}

	// 2. Proxy.
	if proxyURL != "" {
		if d.Env != nil {
			if err := d.Env.SetUserVar("HTTP_PROXY", proxyURL); err != nil {
				return fmt.Errorf("écriture de HTTP_PROXY : %w", err)
			}
			if err := d.Env.SetUserVar("HTTPS_PROXY", proxyURL); err != nil {
				return fmt.Errorf("écriture de HTTPS_PROXY : %w", err)
			}
			if err := d.Env.SetUserVar("NO_PROXY", strings.Join(n.NoProxy, ",")); err != nil {
				return fmt.Errorf("écriture de NO_PROXY : %w", err)
			}
		}

		if err := netGitConfig(ctx, d, "http.proxy", proxyURL); err != nil {
			return err
		}

		if err := config.SetKeys(p.Npmrc, map[string]string{
			"proxy":       proxyURL,
			"https-proxy": proxyURL,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.Npmrc, err)
		}

		if err := config.SetINI(p.PipIni, "global", map[string]string{
			"proxy": proxyURL,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.PipIni, err)
		}

		host, port, err := netProxyHostPort(proxyURL)
		if err != nil {
			return err
		}
		portStr := strconv.Itoa(port)
		if err := config.SetKeys(p.GradleProps, map[string]string{
			"systemProp.http.proxyHost":     host,
			"systemProp.http.proxyPort":     portStr,
			"systemProp.https.proxyHost":    host,
			"systemProp.https.proxyPort":    portStr,
			"systemProp.http.nonProxyHosts": strings.Join(n.NoProxy, "|"),
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.GradleProps, err)
		}
	}

	// 3. Nexus et proxy Maven combinés dans un seul appel.
	maven := config.MavenSettings{}
	mavenSet := false
	if proxyURL != "" {
		host, port, err := netProxyHostPort(proxyURL)
		if err != nil {
			return err
		}
		maven.ProxyHost = host
		maven.ProxyPort = port
		maven.NonProxyHosts = strings.Join(n.NoProxy, "|")
		mavenSet = true
	}
	if n.Nexus.Maven != "" {
		maven.MirrorURL = n.Nexus.Maven
		mavenSet = true
	}
	if mavenSet {
		if err := config.SetMavenSettings(p.MavenSettings, maven, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.MavenSettings, err)
		}
	}

	if n.Nexus.Npm != "" {
		if err := config.SetKeys(p.Npmrc, map[string]string{
			"registry": n.Nexus.Npm,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.Npmrc, err)
		}
	}
	if n.Nexus.Pypi != "" {
		if err := config.SetINI(p.PipIni, "global", map[string]string{
			"index-url": n.Nexus.Pypi,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.PipIni, err)
		}
	}

	// 4. Certificats pour les outils.
	if len(n.CACerts) > 0 {
		if err := config.SetKeys(p.Npmrc, map[string]string{
			"cafile": p.CABundle,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.Npmrc, err)
		}
		if err := config.SetINI(p.PipIni, "global", map[string]string{
			"cert": p.CABundle,
		}, d.State); err != nil {
			return fmt.Errorf("écriture de %q : %w", p.PipIni, err)
		}
		if err := netGitConfig(ctx, d, "http.sslBackend", "schannel"); err != nil {
			return err
		}
	}

	return nil
}
