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
	19: "^18.19.1 || ^20.11.1 || ^22",
	20: "^20.19 || ^22.12 || ^24",
	21: "^20.19 || ^22.12 || ^24",
	22: "^22.22 || ^24.13",
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

// satisfies évalue une version contre une plage "^X.Y[.Z]" : même majeur et
// version supérieure ou égale au minimum.
func satisfies(version, rng string) bool {
	rng = strings.TrimPrefix(strings.TrimSpace(rng), "^")
	min, ok := parseVersion(rng)
	if !ok {
		return false
	}
	v, ok := parseVersion(version)
	if !ok {
		return false
	}
	if v[0] != min[0] {
		return false
	}
	for i := 1; i < 3; i++ {
		if v[i] > min[i] {
			return true
		}
		if v[i] < min[i] {
			return false
		}
	}
	return true
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