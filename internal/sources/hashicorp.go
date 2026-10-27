package sources

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// hashicorpVersionRe ne retient que les versions numériques propres
// (exclut les suffixes "+ent" et les pré-releases "-rc1").
var hashicorpVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// hashicorpIndex décrit le document index.json d'un produit HashiCorp.
type hashicorpIndex struct {
	Versions map[string]hashicorpIndexVersion `json:"versions"`
}

// hashicorpIndexVersion décrit une version dans index.json.
type hashicorpIndexVersion struct {
	Version string                `json:"version"`
	Builds  []hashicorpIndexBuild `json:"builds"`
}

// hashicorpIndexBuild décrit un artefact publié pour une version.
type hashicorpIndexBuild struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// HashiCorp est une source de versions pour un produit HashiCorp (Vault, etc.)
// publié sur releases.hashicorp.com.
type HashiCorp struct {
	fetcher Fetcher
	baseURL string
	product string
}

// NewHashiCorp crée une source HashiCorp pour le produit donné. baseURL ne
// doit pas se terminer par un slash.
func NewHashiCorp(f Fetcher, baseURL, product string) *HashiCorp {
	return &HashiCorp{fetcher: f, baseURL: baseURL, product: product}
}

// NewVault crée une source HashiCorp pour le produit Vault.
func NewVault(f Fetcher, baseURL string) *HashiCorp {
	return NewHashiCorp(f, baseURL, "vault")
}

// Name retourne le nom du produit.
func (h *HashiCorp) Name() string {
	return h.product
}

// Releases retourne les versions numériques publiées pour Windows x64, triées
// par version décroissante.
func (h *HashiCorp) Releases(ctx context.Context) ([]Release, error) {
	var idx hashicorpIndex
	url := h.baseURL + "/" + h.product + "/index.json"
	if err := h.fetcher.GetJSON(ctx, url, &idx); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(idx.Versions))
	out := make([]Release, 0, len(idx.Versions))
	for key, v := range idx.Versions {
		version := v.Version
		if version == "" {
			version = key
		}
		if !hashicorpVersionRe.MatchString(version) {
			continue
		}
		if seen[version] {
			continue
		}
		var build *hashicorpIndexBuild
		for i := range v.Builds {
			b := &v.Builds[i]
			if b.OS == "windows" && b.Arch == "amd64" {
				build = b
				break
			}
		}
		if build == nil {
			continue
		}
		seen[version] = true
		out = append(out, Release{
			Version: version,
			URL:     build.URL,
			Archive: "zip",
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète l'empreinte SHA-256 de la release choisie à partir du
// fichier SHA256SUMS publié pour cette version.
func (h *HashiCorp) Resolve(ctx context.Context, r Release) (Release, error) {
	version := normalize(r.Version)
	url := fmt.Sprintf("%s/%s/%s/%s_%s_SHA256SUMS", h.baseURL, h.product, version, h.product, version)
	text, err := h.fetcher.GetText(ctx, url)
	if err != nil {
		return Release{}, err
	}
	name := path.Base(r.URL)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == name {
			r.Checksum = download.Checksum{Algo: "sha256", Hex: fields[0]}
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("empreinte introuvable pour %s : %w", name, ErrNoRelease)
}
