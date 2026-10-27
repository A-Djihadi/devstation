package sources

import (
	"context"
	"regexp"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// githubRelease est la représentation JSON d'une release de l'API GitHub.
type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Prerelease bool          `json:"prerelease"`
	Draft      bool          `json:"draft"`
	Assets     []githubAsset `json:"assets"`
}

// githubAsset est la représentation JSON d'un asset d'une release GitHub.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// GitHub est une source de versions basée sur les releases GitHub.
type GitHub struct {
	f       Fetcher
	apiBase string
	name    string
	repo    string
	asset   *regexp.Regexp
}

// NewGitHub crée une source GitHub pour le dépôt repo (format "owner/name").
// Seul le premier asset dont le nom correspond à asset est retenu.
func NewGitHub(f Fetcher, apiBase, name, repo string, asset *regexp.Regexp) *GitHub {
	return &GitHub{
		f:       f,
		apiBase: strings.TrimSuffix(apiBase, "/"),
		name:    name,
		repo:    repo,
		asset:   asset,
	}
}

// NewPodman crée la source des releases de Podman (archive Windows x64).
func NewPodman(f Fetcher, apiBase string) *GitHub {
	return NewGitHub(f, apiBase, "podman", "containers/podman",
		regexp.MustCompile(`^podman-remote-release-windows_amd64\.zip$`))
}

// NewCompose crée la source des releases de Docker Compose (exécutable Windows x64).
func NewCompose(f Fetcher, apiBase string) *GitHub {
	return NewGitHub(f, apiBase, "compose", "docker/compose",
		regexp.MustCompile(`^docker-compose-windows-x86_64\.exe$`))
}

// NewPx crée la source des releases de px (archive Windows x64, hors variante -wheels).
func NewPx(f Fetcher, apiBase string) *GitHub {
	return NewGitHub(f, apiBase, "px", "genotrance/px",
		regexp.MustCompile(`^px-v[0-9.]+-windows-amd64\.zip$`))
}

// Name retourne le nom de la source.
func (g *GitHub) Name() string {
	return g.name
}

// githubArchive déduit le format d'archive de l'extension du nom d'asset.
func githubArchive(name string) string {
	switch {
	case strings.HasSuffix(name, ".zip"):
		return "zip"
	case strings.HasSuffix(name, ".exe"):
		return "exe"
	case strings.HasSuffix(name, ".msi"):
		return "msi"
	}
	return ""
}

// githubChecksum convertit un digest GitHub ("sha256:<hex>") en Checksum.
func githubChecksum(digest string) download.Checksum {
	algo, hex, ok := strings.Cut(strings.TrimSpace(digest), ":")
	if !ok || algo == "" || hex == "" {
		return download.Checksum{}
	}
	return download.Checksum{Algo: strings.ToLower(algo), Hex: strings.ToLower(hex)}
}

// Releases retourne les versions publiées, hors brouillons et pré-releases.
func (g *GitHub) Releases(ctx context.Context) ([]Release, error) {
	var raw []githubRelease
	url := g.apiBase + "/repos/" + g.repo + "/releases?per_page=20"
	if err := g.f.GetJSON(ctx, url, &raw); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(raw))
	out := make([]Release, 0, len(raw))
	for _, rel := range raw {
		if rel.Draft || rel.Prerelease {
			continue
		}
		version := normalize(rel.TagName)
		if version == "" || seen[version] {
			continue
		}
		var chosen *githubAsset
		for i := range rel.Assets {
			if g.asset.MatchString(rel.Assets[i].Name) {
				chosen = &rel.Assets[i]
				break
			}
		}
		if chosen == nil {
			continue
		}
		seen[version] = true
		out = append(out, Release{
			Version:  version,
			URL:      chosen.BrowserDownloadURL,
			Checksum: githubChecksum(chosen.Digest),
			Archive:  githubArchive(chosen.Name),
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète la release : si l'empreinte est absente, elle est lue dans
// le fichier "<URL>.sha256" publié à côté de l'archive.
func (g *GitHub) Resolve(ctx context.Context, r Release) (Release, error) {
	if r.Checksum.Algo != "" && r.Checksum.Hex != "" {
		return r, nil
	}
	text, err := g.f.GetText(ctx, r.URL+".sha256")
	if err != nil {
		return Release{}, err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return Release{}, ErrNoRelease
	}
	r.Checksum = download.Checksum{Algo: "sha256", Hex: strings.ToLower(fields[0])}
	return r, nil
}
