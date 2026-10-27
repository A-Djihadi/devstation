// Package sources : source des versions de Python publiées sur python.org.
//
// L'installation se fera par l'installeur officiel en mode par utilisateur
// silencieux (InstallAllUsers=0), sans droits admin.
package sources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/A-Djihadi/devstation/internal/download"
)

// pythonLinkRe extrait les liens de répertoires de version du listing FTP.
var pythonLinkRe = regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`)

// pythonInstallerRe extrait les liens d'installeurs Windows du listing d'un
// répertoire de version.
var pythonInstallerRe = regexp.MustCompile(`href="(python-[^"]+-amd64\.exe)"`)

// Python est la source des versions de Python publiées sur python.org.
type Python struct {
	fetcher Fetcher
	baseURL string
}

// NewPython crée une source Python à partir du fetcher et de l'URL de base
// (sans slash final) de python.org.
func NewPython(f Fetcher, baseURL string) *Python {
	return &Python{fetcher: f, baseURL: baseURL}
}

// Name retourne le nom de la source.
func (p *Python) Name() string {
	return "python"
}

// Releases retourne les versions Python 3 publiées, triées par version
// décroissante. Les versions LTS ne sont pas signalées (LTS false) et aucune
// URL n'est renseignée : python.org ne permet pas de la connaître sans lire
// chaque dossier de version.
func (p *Python) Releases(ctx context.Context) ([]Release, error) {
	body, err := p.fetcher.GetText(ctx, p.baseURL+"/ftp/python/")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]Release, 0)
	for _, m := range pythonLinkRe.FindAllStringSubmatch(body, -1) {
		v := m[1]
		if seen[v] {
			continue
		}
		if !pythonMajorIs3(v) {
			continue
		}
		seen[v] = true
		out = append(out, Release{Version: v})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète la release choisie avec l'URL de l'installeur Windows x64
// et le format d'archive "exe". python.org ne publie pas d'empreinte SHA-256 à
// côté des installeurs : le Checksum reste vide et la vérification se fera par
// signature plus tard. Elle retourne une erreur satisfaisant
// errors.Is(err, ErrNoRelease) quand aucun installeur Windows x64 n'est publié
// pour cette version (pré-release ou release de sécurité).
func (p *Python) Resolve(ctx context.Context, r Release) (Release, error) {
	v := normalize(r.Version)
	body, err := p.fetcher.GetText(ctx, p.baseURL+"/ftp/python/"+v+"/")
	if err != nil {
		return Release{}, err
	}
	want := "python-" + v + "-amd64.exe"
	for _, m := range pythonInstallerRe.FindAllStringSubmatch(body, -1) {
		if m[1] != want {
			continue
		}
		r.Version = v
		r.URL = p.baseURL + "/ftp/python/" + v + "/" + want
		r.Archive = "exe"
		r.Checksum = download.Checksum{}
		return r, nil
	}
	return Release{}, fmt.Errorf("python %s : aucun installeur Windows x64 : %w", v, ErrNoRelease)
}

// pythonMajorIs3 indique si le numéro majeur de la version vaut 3.
func pythonMajorIs3(v string) bool {
	segs := segments(v)
	return len(segs) > 0 && segs[0] == 3
}
