// Package compat vérifie la compatibilité entre versions d'outils (Angular ↔ Node).
package compat

import (
	"fmt"
	"strconv"
	"strings"
)

// angularNodeRanges associe chaque version majeure d'Angular à la plage de
// versions Node.js qu'elle exige, au format semver simplifié.
// Source : https://angular.dev/reference/versions
var angularNodeRanges = map[int]string{
	19: "^18.19.1 || ^20.11.1 || >=22.0.0",
	20: "^20.19.0 || ^22.12.0 || >=24.0.0",
	21: "^20.19.0 || ^22.12.0 || >=24.0.0",
	22: "^22.22.3 || ^24.15.0 || >=26.0.0",
}

// AngularNode vérifie que la version de Node.js fournie satisfait la plage
// exigée par la version majeure d'Angular indiquée. Elle retourne nil si la
// combinaison est compatible ou si le majeur Angular est inconnu (pas de
// blocage), sinon une erreur décrivant la plage attendue.
func AngularNode(angularMajor int, nodeVersion string) error {
	rng, ok := angularNodeRanges[angularMajor]
	if !ok {
		return nil
	}
	if satisfiesAny(nodeVersion, rng) {
		return nil
	}
	return fmt.Errorf("Angular %d exige Node %s", angularMajor, rng)
}

// satisfiesAny indique si version satisfait au moins une des plages séparées
// par "||".
func satisfiesAny(version, ranges string) bool {
	for _, r := range strings.Split(ranges, "||") {
		if satisfies(version, strings.TrimSpace(r)) {
			return true
		}
	}
	return false
}

// satisfies évalue une version contre une plage "^X.Y[.Z]" (même majeur et
// version supérieure ou égale au minimum) ou ">=X.Y[.Z]" (version supérieure
// ou égale au minimum, sans contrainte de majeur).
func satisfies(version, rng string) bool {
	rng = strings.TrimSpace(rng)
	v, ok := parseVersion(version)
	if !ok {
		return false
	}
	if rest, ok := strings.CutPrefix(rng, ">="); ok {
		min, ok := parseVersion(rest)
		if !ok {
			return false
		}
		return compareVersion(v, min) >= 0
	}
	rng = strings.TrimPrefix(rng, "^")
	min, ok := parseVersion(rng)
	if !ok {
		return false
	}
	if v[0] != min[0] {
		return false
	}
	return compareVersion(v, min) >= 0
}

// compareVersion compare deux triplets de version et retourne -1, 0 ou 1.
func compareVersion(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// parseVersion convertit "X", "X.Y" ou "X.Y.Z" en triplet [3]int. Le préfixe
// "v" est toléré. Retourne false si la chaîne n'est pas un triplet valide.
func parseVersion(s string) ([3]int, bool) {
	var out [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return out, false
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
