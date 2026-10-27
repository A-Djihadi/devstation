// Package sources définit les types communs aux sources de versions d'outils
// (Node.js, Python, etc.) : description d'une version publiée, interface de
// source, sélection et tri de versions.
package sources

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// ErrNoRelease indique qu'aucune version publiée ne correspond à la demande.
var ErrNoRelease = errors.New("aucune version correspondante")

// Release décrit une version publiée d'un outil.
type Release struct {
	// Version est le numéro de version (par exemple "21.0.4").
	Version string
	// LTS indique si la version est une version à support long terme.
	LTS bool
	// URL est l'adresse de téléchargement de l'archive.
	URL string
	// Checksum est l'empreinte attendue de l'archive.
	Checksum download.Checksum
	// Archive est le format de l'archive : "zip", "tar.gz", "exe" ou "msi".
	Archive string
}

// Source fournit la liste des versions publiées d'un outil.
type Source interface {
	// Name retourne le nom de la source.
	Name() string
	// Releases retourne les versions publiées.
	Releases(ctx context.Context) ([]Release, error)
}

// Fetcher récupère des ressources distantes (JSON ou texte).
type Fetcher interface {
	// GetJSON télécharge l'URL et décode le corps JSON dans v.
	GetJSON(ctx context.Context, url string, v any) error
	// GetText télécharge l'URL et retourne le corps sous forme de texte.
	GetText(ctx context.Context, url string) (string, error)
}

// normalize retire un éventuel préfixe "v" et les espaces.
func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// segments découpe une version en segments numériques. Les segments non
// numériques valent 0.
func segments(v string) []int {
	v = normalize(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		out[i] = n
	}
	return out
}

// CompareVersions compare deux versions segment par segment. Elle retourne -1
// si a < b, 0 si a == b et 1 si a > b. Le préfixe "v" est ignoré.
func CompareVersions(a, b string) int {
	sa := segments(a)
	sb := segments(b)
	n := len(sa)
	if len(sb) > n {
		n = len(sb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(sa) {
			x = sa[i]
		}
		if i < len(sb) {
			y = sb[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

// SortDesc trie les versions par ordre décroissant.
func SortDesc(rs []Release) {
	sort.SliceStable(rs, func(i, j int) bool {
		return CompareVersions(rs[i].Version, rs[j].Version) > 0
	})
}

// matchesPrefix indique si version correspond à want selon la règle de
// préfixe de segments : égalité ou préfixe suivi d'un point.
func matchesPrefix(version, want string) bool {
	v := normalize(version)
	w := normalize(want)
	if w == "" {
		return false
	}
	if v == w {
		return true
	}
	return strings.HasPrefix(v, w+".")
}

// Pick sélectionne une version parmi rs selon want :
//   - "" ou "latest" : la version la plus récente ;
//   - "lts" : la version LTS la plus récente ;
//   - "21" ou "21.0.4" : la plus récente correspondant au préfixe de segments.
//
// Elle retourne ErrNoRelease si aucune version ne correspond.
func Pick(rs []Release, want string) (Release, error) {
	if len(rs) == 0 {
		return Release{}, ErrNoRelease
	}
	sorted := make([]Release, len(rs))
	copy(sorted, rs)
	SortDesc(sorted)

	w := strings.ToLower(strings.TrimSpace(want))
	switch w {
	case "", "latest":
		return sorted[0], nil
	case "lts":
		for _, r := range sorted {
			if r.LTS {
				return r, nil
			}
		}
		return Release{}, ErrNoRelease
	}
	for _, r := range sorted {
		if matchesPrefix(r.Version, want) {
			return r, nil
		}
	}
	return Release{}, ErrNoRelease
}

// Restrict conserve les versions dont le numéro correspond à une entrée de
// allowed selon la règle de préfixe de segments. Si allowed est vide, rs est
// retourné inchangé.
func Restrict(rs []Release, allowed []string) []Release {
	if len(allowed) == 0 {
		return rs
	}
	out := make([]Release, 0, len(rs))
	for _, r := range rs {
		for _, a := range allowed {
			if matchesPrefix(r.Version, a) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
