package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.Npm)(nil)

// npmFakeFetcher sert les fixtures locales et enregistre les URL demandées.
type npmFakeFetcher struct {
	files map[string]string
	urls  []string
}

// GetJSON décode la fixture correspondant à l'URL demandée.
func (f *npmFakeFetcher) GetJSON(_ context.Context, url string, v any) error {
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return errors.New("URL inconnue : " + url)
	}
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		return err
	}
	return npmDecodeJSON(data, v)
}

// GetText retourne le contenu de la fixture correspondant à l'URL demandée.
func (f *npmFakeFetcher) GetText(_ context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return "", errors.New("URL inconnue : " + url)
	}
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// npmDecodeJSON décode data dans v via encoding/json.
func npmDecodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// TestNpmReleasesExcludePrerelease vérifie que les pré-releases sont exclues.
func TestNpmReleasesExcludePrerelease(t *testing.T) {
	f := &npmFakeFetcher{files: map[string]string{
		"https://registry.example.com/@angular%2Fcli": "npm-angular-cli.json",
	}}
	src := sources.NewNpm(f, "https://registry.example.com", "@angular/cli", "angular_cli")
	rs, err := src.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	for _, r := range rs {
		if r.Version == "22.3.0-next.2" || r.Version == "22.3.0-next.3" {
			t.Fatalf("pré-release présente : %s", r.Version)
		}
	}
	if len(rs) != 4 {
		t.Fatalf("attendu 4 versions, obtenu %d : %v", len(rs), rs)
	}
	if rs[0].Version != "22.2.4" {
		t.Fatalf("première version attendue 22.2.4, obtenu %s", rs[0].Version)
	}
}

// TestNpmEscapesScopedName vérifie l'échappement du nom de paquet dans l'URL.
func TestNpmEscapesScopedName(t *testing.T) {
	f := &npmFakeFetcher{files: map[string]string{
		"https://registry.example.com/@angular%2Fcli": "npm-angular-cli.json",
	}}
	src := sources.NewNpm(f, "https://registry.example.com", "@angular/cli", "angular_cli")
	if _, err := src.Releases(context.Background()); err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(f.urls) != 1 {
		t.Fatalf("attendu 1 URL demandée, obtenu %d", len(f.urls))
	}
	if f.urls[0] != "https://registry.example.com/@angular%2Fcli" {
		t.Fatalf("URL inattendue : %s", f.urls[0])
	}
}

// TestNpmLatest vérifie la lecture de dist-tags.latest.
func TestNpmLatest(t *testing.T) {
	f := &npmFakeFetcher{files: map[string]string{
		"https://registry.example.com/@angular%2Fcli": "npm-angular-cli.json",
	}}
	src := sources.NewNpm(f, "https://registry.example.com", "@angular/cli", "angular_cli")
	got, err := src.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest : %v", err)
	}
	if got != "22.2.4" {
		t.Fatalf("latest attendu 22.2.4, obtenu %s", got)
	}
}
