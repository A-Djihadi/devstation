package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.Adoptium)(nil)

// adoptiumFetcher sert les fixtures locales selon l'URL demandée.
type adoptiumFetcher struct {
	t        *testing.T
	urls     []string
	contents map[string]string
}

func (f *adoptiumFetcher) GetJSON(ctx context.Context, url string, v any) error {
	f.urls = append(f.urls, url)
	content, ok := f.contents[url]
	if !ok {
		f.t.Fatalf("URL inattendue : %s", url)
	}
	return json.Unmarshal([]byte(content), v)
}

func (f *adoptiumFetcher) GetText(ctx context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	content, ok := f.contents[url]
	if !ok {
		return "", errors.New("URL inattendue : " + url)
	}
	return content, nil
}

func TestAdoptiumReleases(t *testing.T) {
	content, err := os.ReadFile("testdata/adoptium-21.json")
	if err != nil {
		t.Fatal(err)
	}
	base := "https://api.adoptium.example.com"
	url := base + "/v3/assets/feature_releases/21/ga?os=windows&architecture=x64&image_type=jdk&page_size=20"
	f := &adoptiumFetcher{t: t, contents: map[string]string{url: string(content)}}
	src := sources.NewAdoptium(f, base, []int{21})

	rels, err := src.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.urls) != 1 || f.urls[0] != url {
		t.Fatalf("URL appelée = %v, attendu %s", f.urls, url)
	}
	if len(rels) != 1 {
		t.Fatalf("nombre de releases = %d, attendu 1", len(rels))
	}
	r := rels[0]
	if r.Version != "21.0.12" {
		t.Errorf("version = %q, attendu 21.0.12", r.Version)
	}
	if !r.LTS {
		t.Error("LTS attendu pour la version 21")
	}
	if r.Archive != "zip" {
		t.Errorf("archive = %q, attendu zip", r.Archive)
	}
	wantURL := "https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.12.1%2B1/OpenJDK21U-jdk_x64_windows_hotspot_21.0.12.1_1.zip"
	if r.URL != wantURL {
		t.Errorf("URL = %q, attendu %q", r.URL, wantURL)
	}
	if r.Checksum.Algo != "sha256" || r.Checksum.Hex != "f9d6e191ab098c0d416e7d588a24420a8621cd2f4720dab2459b8b7b2d2d8b4e" {
		t.Errorf("checksum = %+v", r.Checksum)
	}
}

func TestAdoptiumResolve(t *testing.T) {
	src := sources.NewAdoptium(&adoptiumFetcher{t: t}, "https://api.adoptium.example.com", []int{21})
	in := sources.Release{Version: "21.0.12", URL: "https://example.com/jdk.zip"}
	out, err := src.Resolve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.URL != in.URL {
		t.Errorf("URL = %q, attendu %q", out.URL, in.URL)
	}
	if _, err := src.Resolve(context.Background(), sources.Release{Version: "21.0.12"}); !errors.Is(err, sources.ErrNoRelease) {
		t.Errorf("erreur = %v, attendu ErrNoRelease", err)
	}
}
