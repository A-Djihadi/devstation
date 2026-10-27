package sources

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// apacheMavenMeta est la racine du document maven-metadata.xml.
type apacheMavenMeta struct {
	Versions []string `xml:"versioning>versions>version"`
}

// Maven fournit les versions publiées d'Apache Maven depuis le dépôt Maven
// Central.
type Maven struct {
	fetcher Fetcher
	baseURL string
}

// NewMaven crée une source Maven interrogeant baseURL (sans slash final).
func NewMaven(f Fetcher, baseURL string) *Maven {
	return &Maven{fetcher: f, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Name retourne le nom de la source.
func (m *Maven) Name() string { return "maven" }

// apacheMavenMetadataPath est le chemin du maven-metadata.xml d'Apache Maven.
const apacheMavenMetadataPath = "/maven2/org/apache/maven/apache-maven/maven-metadata.xml"

// apacheMavenBinURL construit l'URL de l'archive binaire d'une version.
func apacheMavenBinURL(baseURL, version string) string {
	return baseURL + "/maven2/org/apache/maven/apache-maven/" + version +
		"/apache-maven-" + version + "-bin.zip"
}

// Releases retourne les versions stables publiées, triées par ordre décroissant.
func (m *Maven) Releases(ctx context.Context) ([]Release, error) {
	body, err := m.fetcher.GetText(ctx, m.baseURL+apacheMavenMetadataPath)
	if err != nil {
		return nil, err
	}
	var meta apacheMavenMeta
	if err := xml.Unmarshal([]byte(body), &meta); err != nil {
		return nil, fmt.Errorf("maven : métadonnées illisibles : %w", err)
	}
	seen := make(map[string]bool, len(meta.Versions))
	out := make([]Release, 0, len(meta.Versions))
	for _, v := range meta.Versions {
		v = normalize(v)
		if v == "" || strings.Contains(v, "-") || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, Release{
			Version: v,
			URL:     apacheMavenBinURL(m.baseURL, v),
			Archive: "zip",
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète la release avec l'empreinte SHA-512 publiée par le dépôt.
func (m *Maven) Resolve(ctx context.Context, r Release) (Release, error) {
	if r.URL == "" {
		return Release{}, fmt.Errorf("maven : URL absente pour %q : %w", r.Version, ErrNoRelease)
	}
	body, err := m.fetcher.GetText(ctx, r.URL+".sha512")
	if err != nil {
		return Release{}, err
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return Release{}, fmt.Errorf("maven : empreinte vide pour %q : %w", r.Version, ErrNoRelease)
	}
	r.Checksum = download.Checksum{Algo: "sha512", Hex: strings.ToLower(fields[0])}
	if r.Archive == "" {
		r.Archive = "zip"
	}
	return r, nil
}

// apacheGradleVersion décrit une entrée de l'API versions/all de Gradle.
type apacheGradleVersion struct {
	Version      string `json:"version"`
	Snapshot     bool   `json:"snapshot"`
	Nightly      bool   `json:"nightly"`
	RCFor        string `json:"rcFor"`
	MilestoneFor string `json:"milestoneFor"`
	Broken       bool   `json:"broken"`
	DownloadURL  string `json:"downloadUrl"`
}

// Gradle fournit les versions publiées de Gradle.
type Gradle struct {
	fetcher Fetcher
	baseURL string
}

// NewGradle crée une source Gradle interrogeant baseURL (sans slash final).
func NewGradle(f Fetcher, baseURL string) *Gradle {
	return &Gradle{fetcher: f, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Name retourne le nom de la source.
func (g *Gradle) Name() string { return "gradle" }

// Releases retourne les versions stables publiées, triées par ordre décroissant.
func (g *Gradle) Releases(ctx context.Context) ([]Release, error) {
	var all []apacheGradleVersion
	if err := g.fetcher.GetJSON(ctx, g.baseURL+"/versions/all", &all); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(all))
	out := make([]Release, 0, len(all))
	for _, v := range all {
		if v.Snapshot || v.Nightly || v.RCFor != "" || v.MilestoneFor != "" || v.Broken {
			continue
		}
		version := normalize(v.Version)
		if version == "" || strings.Contains(version, "-") || seen[version] {
			continue
		}
		seen[version] = true
		out = append(out, Release{
			Version: version,
			URL:     v.DownloadURL,
			Archive: "zip",
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète la release avec l'empreinte SHA-256 publiée par Gradle.
func (g *Gradle) Resolve(ctx context.Context, r Release) (Release, error) {
	if r.URL == "" {
		return Release{}, fmt.Errorf("gradle : URL absente pour %q : %w", r.Version, ErrNoRelease)
	}
	body, err := g.fetcher.GetText(ctx, r.URL+".sha256")
	if err != nil {
		return Release{}, err
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return Release{}, fmt.Errorf("gradle : empreinte vide pour %q : %w", r.Version, ErrNoRelease)
	}
	r.Checksum = download.Checksum{Algo: "sha256", Hex: strings.ToLower(fields[0])}
	if r.Archive == "" {
		r.Archive = "zip"
	}
	return r, nil
}

// apacheErrNoRelease vérifie qu'une erreur satisfait ErrNoRelease.
var _ = errors.Is
