package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.Maven)(nil)
var _ sources.Source = (*sources.Gradle)(nil)

// apacheFakeFetcher sert le contenu des fixtures selon l'URL demandée.
type apacheFakeFetcher struct {
	files map[string]string
	urls  []string
}

func (f *apacheFakeFetcher) GetJSON(ctx context.Context, url string, v any) error {
	return errors.New("apacheFakeFetcher : GetJSON non supporté")
}

func (f *apacheFakeFetcher) GetText(ctx context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return "", errors.New("apacheFakeFetcher : URL inconnue " + url)
	}
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// apacheJSONFetcher sert un contenu JSON fixe pour une URL donnée.
type apacheJSONFetcher struct {
	url  string
	body string
	urls []string
}

func (f *apacheJSONFetcher) GetJSON(ctx context.Context, url string, v any) error {
	f.urls = append(f.urls, url)
	if url != f.url {
		return errors.New("apacheJSONFetcher : URL inconnue " + url)
	}
	return json.Unmarshal([]byte(f.body), v)
}

func (f *apacheJSONFetcher) GetText(ctx context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	if url != f.url {
		return "", errors.New("apacheJSONFetcher : URL inconnue " + url)
	}
	return f.body, nil
}

func TestMavenReleasesExcludeRC(t *testing.T) {
	const base = "https://repo.example.com"
	f := &apacheFakeFetcher{files: map[string]string{
		base + "/maven2/org/apache/maven/apache-maven/maven-metadata.xml": "maven-meta.xml",
	}}
	m := sources.NewMaven(f, base)
	if m.Name() != "maven" {
		t.Fatalf("Name() = %q", m.Name())
	}
	rs, err := m.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("aucune version")
	}
	if rs[0].Version != "3.9.16" {
		t.Fatalf("première version = %q, attendu 3.9.16", rs[0].Version)
	}
	for _, r := range rs {
		if strings.Contains(r.Version, "-") {
			t.Fatalf("pré-release conservée : %q", r.Version)
		}
		if r.Archive != "zip" {
			t.Fatalf("archive = %q", r.Archive)
		}
	}
	want := base + "/maven2/org/apache/maven/apache-maven/3.9.16/apache-maven-3.9.16-bin.zip"
	if rs[0].URL != want {
		t.Fatalf("URL = %q, attendu %q", rs[0].URL, want)
	}
}

func TestMavenResolveSHA512(t *testing.T) {
	const base = "https://repo.example.com"
	binURL := base + "/maven2/org/apache/maven/apache-maven/3.9.16/apache-maven-3.9.16-bin.zip"
	f := &apacheFakeFetcher{files: map[string]string{
		binURL + ".sha512": "maven-bin-zip.sha512",
	}}
	m := sources.NewMaven(f, base)
	r, err := m.Resolve(context.Background(), sources.Release{Version: "3.9.16", URL: binURL, Archive: "zip"})
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if r.Checksum.Algo != "sha512" {
		t.Fatalf("algo = %q", r.Checksum.Algo)
	}
	if len(r.Checksum.Hex) != 128 {
		t.Fatalf("empreinte = %q", r.Checksum.Hex)
	}
	if !strings.HasPrefix(r.Checksum.Hex, "ed41650d") {
		t.Fatalf("empreinte inattendue : %q", r.Checksum.Hex)
	}
}

func TestGradleReleasesStableOnly(t *testing.T) {
	const base = "https://services.example.com"
	body, err := os.ReadFile("testdata/gradle-versions.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &apacheJSONFetcher{url: base + "/versions/all", body: string(body)}
	g := sources.NewGradle(f, base)
	if g.Name() != "gradle" {
		t.Fatalf("Name() = %q", g.Name())
	}
	rs, err := g.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("nombre de versions = %d, attendu 3", len(rs))
	}
	if rs[0].Version != "9.8.2" {
		t.Fatalf("première version = %q, attendu 9.8.2", rs[0].Version)
	}
	for _, r := range rs {
		if strings.Contains(r.Version, "-") {
			t.Fatalf("pré-release conservée : %q", r.Version)
		}
		if r.Archive != "zip" {
			t.Fatalf("archive = %q", r.Archive)
		}
	}
}

func TestGradleResolve(t *testing.T) {
	const base = "https://services.example.com"
	dl := "https://services.example.com/distributions/gradle-9.8.2-bin.zip"
	f := &apacheFakeFetcher{files: map[string]string{
		dl + ".sha256": "gradle-bin-zip.sha256",
	}}
	g := sources.NewGradle(f, base)
	r, err := g.Resolve(context.Background(), sources.Release{Version: "9.8.2", URL: dl, Archive: "zip"})
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if r.Checksum.Algo != "sha256" {
		t.Fatalf("algo = %q", r.Checksum.Algo)
	}
	if len(r.Checksum.Hex) != 64 {
		t.Fatalf("empreinte = %q", r.Checksum.Hex)
	}
}
