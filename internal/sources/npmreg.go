package sources

import (
	"context"
	"net/url"
	"sort"
	"strings"
)

// npmRegistryResponse décrit la réponse du registre npm pour un paquet.
type npmRegistryResponse struct {
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]npmVersionEntry `json:"versions"`
}

// npmVersionEntry décrit une version publiée d'un paquet npm.
type npmVersionEntry struct {
	Version string `json:"version"`
}

// Npm est une source de versions adossée à un registre npm (npmjs.org ou un
// dépôt Nexus compatible).
type Npm struct {
	fetcher     Fetcher
	registryURL string
	pkg         string
	name        string
}

// NewNpm crée une source npm pour le paquet pkg servi par registryURL (sans
// slash final). name est le nom retourné par Name.
func NewNpm(f Fetcher, registryURL, pkg, name string) *Npm {
	return &Npm{
		fetcher:     f,
		registryURL: strings.TrimRight(registryURL, "/"),
		pkg:         pkg,
		name:        name,
	}
}

// Name retourne le nom de la source.
func (n *Npm) Name() string {
	return n.name
}

// packageURL retourne l'URL du document du paquet sur le registre.
func (n *Npm) packageURL() string {
	return n.registryURL + "/" + url.PathEscape(n.pkg)
}

// fetch récupère le document du paquet depuis le registre.
func (n *Npm) fetch(ctx context.Context) (npmRegistryResponse, error) {
	var doc npmRegistryResponse
	if err := n.fetcher.GetJSON(ctx, n.packageURL(), &doc); err != nil {
		return npmRegistryResponse{}, err
	}
	return doc, nil
}

// Releases retourne les versions publiées du paquet, hors pré-releases, triées
// par version décroissante et sans doublon.
func (n *Npm) Releases(ctx context.Context) ([]Release, error) {
	doc, err := n.fetch(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(doc.Versions))
	out := make([]Release, 0, len(doc.Versions))
	for key := range doc.Versions {
		if strings.Contains(key, "-") {
			continue
		}
		v := normalize(key)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, Release{Version: v})
	}
	SortDesc(out)
	return out, nil
}

// Resolve retourne la release inchangée : un paquet npm est installé par npm,
// il n'y a rien à télécharger.
func (n *Npm) Resolve(_ context.Context, r Release) (Release, error) {
	return r, nil
}

// Latest retourne la version désignée par dist-tags.latest.
func (n *Npm) Latest(ctx context.Context) (string, error) {
	doc, err := n.fetch(ctx)
	if err != nil {
		return "", err
	}
	return doc.DistTags["latest"], nil
}

// npmSortedKeys retourne les clés d'une map triées (utilitaire interne).
func npmSortedKeys(m map[string]npmVersionEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
