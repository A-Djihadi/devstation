package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeZip cree une archive zip avec les entrees donnees (nom -> contenu).
func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creation zip : %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creation entree %q : %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("ecriture entree %q : %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("fermeture zip : %v", err)
	}
}

// writeTarGz cree une archive tar.gz avec les entrees donnees.
func writeTarGz(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creation tar.gz : %v", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("entete %q : %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("ecriture %q : %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("fermeture tar : %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("fermeture gzip : %v", err)
	}
}

// TestUnzipStripTop verifie que le dossier racine unique est retire.
func TestUnzipStripTop(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "jdk.zip")
	writeZip(t, src, map[string]string{
		"jdk-21/bin/java": "java",
		"jdk-21/lib/mod":  "mod",
	})

	dest := filepath.Join(dir, "out")
	if err := Unzip(src, dest, true); err != nil {
		t.Fatalf("Unzip : %v", err)
	}

	for _, rel := range []string{"bin/java", "lib/mod"} {
		p := filepath.Join(dest, filepath.FromSlash(rel))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("fichier attendu %q absent : %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "jdk-21")); !os.IsNotExist(err) {
		t.Errorf("le dossier racine jdk-21 aurait du etre retire")
	}
}

// TestZipSlipRejected verifie le rejet d'une entree remontant hors de dest.
func TestZipSlipRejected(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "evil.zip")
	writeZip(t, src, map[string]string{
		"../evil.txt": "pwned",
	})

	dest := filepath.Join(dir, "out")
	err := Unzip(src, dest, false)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("erreur attendue ErrUnsafePath, obtenu %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); !os.IsNotExist(err) {
		t.Errorf("fichier ecrit hors de dest : %v", err)
	}
}

// TestAbsolutePathRejected verifie le rejet des chemins absolus.
func TestAbsolutePathRejected(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")

	for _, name := range []string{"/etc/x", `C:\x`} {
		src := filepath.Join(dir, "abs.zip")
		writeZip(t, src, map[string]string{name: "x"})
		err := Unzip(src, dest, false)
		if !errors.Is(err, ErrUnsafePath) {
			t.Errorf("entree %q : erreur attendue ErrUnsafePath, obtenu %v", name, err)
		}
	}
}

// TestUnicodeAndSpaces verifie l'extraction vers un chemin accentue et espace.
func TestUnicodeAndSpaces(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "node.zip")
	writeZip(t, src, map[string]string{
		"node-v20/bin/node": "node",
	})

	dest := filepath.Join(dir, "Jean Émile")
	if err := Unzip(src, dest, true); err != nil {
		t.Fatalf("Unzip : %v", err)
	}

	p := filepath.Join(dest, "bin", "node")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("lecture %q : %v", p, err)
	}
	if string(data) != "node" {
		t.Errorf("contenu inattendu : %q", string(data))
	}
}