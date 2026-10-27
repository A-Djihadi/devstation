package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// editorsVSCodePath est le chemin de l'API de VS Code pour la dernière version
// stable de l'archive Windows x64.
const editorsVSCodePath = "/api/update/win32-x64-archive/stable/latest"

// editorsIntelliJPath est le chemin de l'API des versions JetBrains pour la
// dernière version publiée d'IntelliJ IDEA Community.
const editorsIntelliJPath = "/products/releases?code=IIC&latest=true&type=release"

// VSCode est la source des versions de Visual Studio Code.
type VSCode struct {
	fetcher Fetcher
	apiBase string
}

// NewVSCode crée une source VS Code interrogeant apiBase (sans slash final).
func NewVSCode(f Fetcher, apiBase string) *VSCode {
	return &VSCode{fetcher: f, apiBase: strings.TrimSuffix(apiBase, "/")}
}

// Name retourne le nom de la source.
func (s *VSCode) Name() string { return "vscode" }

// editorsVSCodeLatest est la réponse de l'API de VS Code.
type editorsVSCodeLatest struct {
	URL            string `json:"url"`
	ProductVersion string `json:"productVersion"`
	SHA256Hash     string `json:"sha256hash"`
}

// Releases retourne la dernière version stable de VS Code.
func (s *VSCode) Releases(ctx context.Context) ([]Release, error) {
	var latest editorsVSCodeLatest
	if err := s.fetcher.GetJSON(ctx, s.apiBase+editorsVSCodePath, &latest); err != nil {
		return nil, err
	}
	if latest.ProductVersion == "" {
		return nil, fmt.Errorf("vscode : version absente de la réponse : %w", ErrNoRelease)
	}
	return []Release{{
		Version:  normalize(latest.ProductVersion),
		URL:      latest.URL,
		Checksum: download.Checksum{Algo: "sha256", Hex: latest.SHA256Hash},
		Archive:  "zip",
	}}, nil
}

// Resolve retourne la release telle quelle.
func (s *VSCode) Resolve(_ context.Context, r Release) (Release, error) {
	return r, nil
}

// IntelliJ est la source des versions d'IntelliJ IDEA Community.
type IntelliJ struct {
	fetcher Fetcher
	apiBase string
}

// NewIntelliJ crée une source IntelliJ interrogeant apiBase (sans slash final).
func NewIntelliJ(f Fetcher, apiBase string) *IntelliJ {
	return &IntelliJ{fetcher: f, apiBase: strings.TrimSuffix(apiBase, "/")}
}

// Name retourne le nom de la source.
func (s *IntelliJ) Name() string { return "intellij" }

// editorsIntelliJDownload décrit un téléchargement JetBrains.
type editorsIntelliJDownload struct {
	Link         string `json:"link"`
	ChecksumLink string `json:"checksumLink"`
}

// editorsIntelliJRelease décrit une version publiée JetBrains.
type editorsIntelliJRelease struct {
	Version   string                             `json:"version"`
	Downloads map[string]editorsIntelliJDownload `json:"downloads"`
}

// Releases retourne les versions publiées d'IntelliJ IDEA Community.
func (s *IntelliJ) Releases(ctx context.Context) ([]Release, error) {
	var byCode map[string][]editorsIntelliJRelease
	if err := s.fetcher.GetJSON(ctx, s.apiBase+editorsIntelliJPath, &byCode); err != nil {
		return nil, err
	}
	var list []editorsIntelliJRelease
	for _, v := range byCode {
		list = v
		break
	}
	out := make([]Release, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, r := range list {
		dl, ok := r.Downloads["windowsZip"]
		if !ok || dl.Link == "" {
			continue
		}
		v := normalize(r.Version)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, Release{
			Version: v,
			URL:     dl.Link,
			Archive: "zip",
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète la release avec l'empreinte sha256 publiée par JetBrains.
func (s *IntelliJ) Resolve(ctx context.Context, r Release) (Release, error) {
	if r.URL == "" {
		return Release{}, fmt.Errorf("intellij : aucune archive Windows pour %s : %w", r.Version, ErrNoRelease)
	}
	text, err := s.fetcher.GetText(ctx, r.URL+".sha256")
	if err != nil {
		return Release{}, err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return Release{}, fmt.Errorf("intellij : empreinte vide pour %s : %w", r.Version, ErrNoRelease)
	}
	r.Checksum = download.Checksum{Algo: "sha256", Hex: fields[0]}
	return r, nil
}
