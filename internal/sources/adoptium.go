package sources

import (
	"context"
	"strconv"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// adoptiumLTSTFeatures liste les versions Java à support long terme.
var adoptiumLTSFeatures = map[int]bool{8: true, 11: true, 17: true, 21: true, 25: true}

// adoptiumPackage décrit le paquet téléchargeable d'un binaire Adoptium.
type adoptiumPackage struct {
	Name     string `json:"name"`
	Link     string `json:"link"`
	Checksum string `json:"checksum"`
}

// adoptiumBinary décrit un binaire publié pour une plateforme donnée.
type adoptiumBinary struct {
	OS           string          `json:"os"`
	Architecture string          `json:"architecture"`
	ImageType    string          `json:"image_type"`
	Package      adoptiumPackage `json:"package"`
}

// adoptiumVersionData décrit les composantes de version d'une release.
type adoptiumVersionData struct {
	Major    int    `json:"major"`
	Minor    int    `json:"minor"`
	Security int    `json:"security"`
	Semver   string `json:"semver"`
}

// adoptiumRelease décrit une release telle que renvoyée par l'API Adoptium.
type adoptiumRelease struct {
	ReleaseName string              `json:"release_name"`
	VersionData adoptiumVersionData `json:"version_data"`
	Binaries    []adoptiumBinary    `json:"binaries"`
}

// Adoptium est une source de versions Java Temurin (Adoptium).
type Adoptium struct {
	fetcher  Fetcher
	baseURL  string
	features []int
}

// NewAdoptium crée une source Adoptium pour les versions majeures indiquées.
func NewAdoptium(f Fetcher, baseURL string, features []int) *Adoptium {
	return &Adoptium{fetcher: f, baseURL: baseURL, features: features}
}

// Name retourne le nom de la source.
func (a *Adoptium) Name() string { return "java" }

// adoptiumVersion construit la version numérique d'une release.
func adoptiumVersion(v adoptiumVersionData) string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Security)
}

// adoptiumZipBinary retourne le binaire Windows x64 de type jdk dont le paquet
// est une archive zip.
func adoptiumZipBinary(bs []adoptiumBinary) (adoptiumBinary, bool) {
	for _, b := range bs {
		if b.OS != "windows" || b.Architecture != "x64" || b.ImageType != "jdk" {
			continue
		}
		if strings.HasSuffix(b.Package.Name, ".zip") {
			return b, true
		}
	}
	return adoptiumBinary{}, false
}

// Releases retourne les versions publiées pour chaque version majeure demandée.
func (a *Adoptium) Releases(ctx context.Context) ([]Release, error) {
	seen := make(map[string]bool)
	out := make([]Release, 0)
	for _, feature := range a.features {
		url := a.baseURL + "/v3/assets/feature_releases/" + strconv.Itoa(feature) +
			"/ga?os=windows&architecture=x64&image_type=jdk&page_size=20"
		var rels []adoptiumRelease
		if err := a.fetcher.GetJSON(ctx, url, &rels); err != nil {
			return nil, err
		}
		for _, rel := range rels {
			version := adoptiumVersion(rel.VersionData)
			if seen[version] {
				continue
			}
			b, ok := adoptiumZipBinary(rel.Binaries)
			if !ok {
				continue
			}
			seen[version] = true
			out = append(out, Release{
				Version:  version,
				LTS:      adoptiumLTSFeatures[feature],
				URL:      b.Package.Link,
				Checksum: download.Checksum{Algo: "sha256", Hex: b.Package.Checksum},
				Archive:  "zip",
			})
		}
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète une release choisie. Elle retourne ErrNoRelease quand la
// release n'a pas d'URL de téléchargement.
func (a *Adoptium) Resolve(ctx context.Context, r Release) (Release, error) {
	if r.URL == "" {
		return Release{}, ErrNoRelease
	}
	return r, nil
}
