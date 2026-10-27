// Package archive fournit l'extraction d'archives zip et tar.gz en
// protegeant la destination contre les chemins dangereux (zip slip).
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafePath est renvoye lorsqu'une entree d'archive tente d'ecrire
// hors du repertoire de destination (chemin relatif remontant ou absolu).
var ErrUnsafePath = errors.New("chemin dangereux dans l'archive")

// Unzip extrait l'archive zip src dans le repertoire dest.
// Si stripTop est vrai, le dossier racine unique est retire des chemins.
func Unzip(src, dest string, stripTop bool) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("ouverture de l'archive zip %q : %w", src, err)
	}
	defer r.Close()

	prefix := ""
	if stripTop {
		prefix = commonTopDir(zipNames(r.File))
	}

	for _, f := range r.File {
		name := f.Name
		if prefix != "" {
			if name == prefix || name == prefix+"/" {
				continue
			}
			if !strings.HasPrefix(name, prefix+"/") {
				continue
			}
			name = strings.TrimPrefix(name, prefix+"/")
		}
		if name == "" {
			continue
		}

		target, err := safeJoin(dest, name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creation du repertoire %q : %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("creation du repertoire %q : %w", filepath.Dir(target), err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("ouverture de l'entree %q : %w", f.Name, err)
		}
		err = writeFile(target, rc, f.Mode())
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Untar extrait l'archive tar.gz src dans le repertoire dest.
// Si stripTop est vrai, le dossier racine unique est retire des chemins.
func Untar(src, dest string, stripTop bool) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("ouverture de l'archive tar.gz %q : %w", src, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("lecture gzip de %q : %w", src, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	prefix := ""
	if stripTop {
		names, err := tarNames(src)
		if err != nil {
			return err
		}
		prefix = commonTopDir(names)
	}

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("lecture de l'archive tar.gz %q : %w", src, err)
		}

		name := hdr.Name
		if prefix != "" {
			if name == prefix || name == prefix+"/" {
				continue
			}
			if !strings.HasPrefix(name, prefix+"/") {
				continue
			}
			name = strings.TrimPrefix(name, prefix+"/")
		}
		if name == "" {
			continue
		}

		target, err := safeJoin(dest, name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creation du repertoire %q : %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("creation du repertoire %q : %w", filepath.Dir(target), err)
			}
			mode := os.FileMode(hdr.Mode).Perm()
			if err := writeFile(target, tr, mode); err != nil {
				return err
			}
		default:
			// Liens, peripheriques et autres types ignores.
		}
	}
	return nil
}

// safeJoin joint name a dest et verifie que le resultat reste dans dest.
func safeJoin(dest, name string) (string, error) {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return "", fmt.Errorf("%w : %q", ErrUnsafePath, name)
	}
	if filepath.IsAbs(name) || isWindowsAbs(name) {
		return "", fmt.Errorf("%w : %q", ErrUnsafePath, name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w : %q", ErrUnsafePath, name)
	}
	target := filepath.Join(dest, clean)
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		return "", fmt.Errorf("%w : %q", ErrUnsafePath, name)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w : %q", ErrUnsafePath, name)
	}
	return target, nil
}

// isWindowsAbs detecte les chemins absolus Windows (C:\x, \\serveur\partage).
func isWindowsAbs(name string) bool {
	if strings.HasPrefix(name, `\\`) || strings.HasPrefix(name, `//`) {
		return true
	}
	if len(name) >= 3 && name[1] == ':' && (name[2] == '\\' || name[2] == '/') {
		c := name[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// writeFile ecrit le contenu de r dans target avec le mode donne.
func writeFile(target string, r io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("creation du fichier %q : %w", target, err)
	}
	_, err = io.Copy(out, r)
	cerr := out.Close()
	if err != nil {
		return fmt.Errorf("ecriture du fichier %q : %w", target, err)
	}
	if cerr != nil {
		return fmt.Errorf("fermeture du fichier %q : %w", target, cerr)
	}
	return nil
}

// zipNames retourne les noms des entrees zip.
func zipNames(files []*zip.File) []string {
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	return names
}

// tarNames relit l'archive tar.gz pour en extraire les noms d'entrees.
func tarNames(src string) ([]string, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("ouverture de l'archive tar.gz %q : %w", src, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("lecture gzip de %q : %w", src, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("lecture de l'archive tar.gz %q : %w", src, err)
		}
		names = append(names, hdr.Name)
	}
	return names, nil
}

// commonTopDir retourne le dossier racine unique commun a tous les noms,
// ou "" si les entrees ne partagent pas un unique dossier racine.
func commonTopDir(names []string) string {
	top := ""
	for _, n := range names {
		n = strings.TrimPrefix(n, "./")
		if n == "" {
			continue
		}
		idx := strings.IndexAny(n, `/\`)
		if idx < 0 {
			return ""
		}
		first := n[:idx]
		if first == "" {
			return ""
		}
		if top == "" {
			top = first
		} else if top != first {
			return ""
		}
	}
	return top
}
