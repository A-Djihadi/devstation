package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.GitHub)(nil)

// githubFetcher sert le contenu des fixtures selon l'URL demandée.
type githubFetcher struct {
	t     *testing.T
	files map[string]string
	texts map[string]string
	urls  []string
}

func (f *githubFetcher) GetJSON(ctx context.Context, url string, v any) error {
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		f.t.Fatalf("URL JSON inattendue : %s", url)
	}
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		f.t.Fatalf("lecture fixture %s : %v", name, err)
	}
	return githubDecodeJSON(f.t, data, v)
}

func (f *githubFetcher) GetText(ctx context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	if txt, ok := f.texts[url]; ok {
		return txt, nil
	}
	return "", errors.New("URL texte inconnue : " + url)
}

func githubDecodeJSON(t *testing.T, data []byte, v any) error {
	t.Helper()
	return json.Unmarshal(data, v)
}

func TestPodmanSkipsPrerelease(t *testing.T) {
	const api = "https://api.example.com"
	f := &githubFetcher{
		t:     t,
		files: map[string]string{api + "/repos/containers/podman/releases?per_page=20": "gh-podman.json"},
	}
	s := sources.NewPodman(f, api)
	if s.Name() != "podman" {
		t.Fatalf("Name() = %q", s.Name())
	}
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 2 {
		t.Fatalf("attendu 2 releases, obtenu %d", len(rs))
	}
	if rs[0].Version != "6.1.4" {
		t.Fatalf("tête = %q, attendu 6.1.4", rs[0].Version)
	}
	for _, r := range rs {
		if strings.Contains(r.Version, "rc") {
			t.Fatalf("pré-release présente : %q", r.Version)
		}
	}
	if rs[0].Archive != "zip" {
		t.Fatalf("Archive = %q, attendu zip", rs[0].Archive)
	}
	if rs[0].Checksum.Algo != "sha256" || rs[0].Checksum.Hex != "8b70cd2511bfac9b65e58858d8fe54eeb68e3420eeb600e3f869b9a4b8576572" {
		t.Fatalf("Checksum = %+v", rs[0].Checksum)
	}
	if !strings.HasSuffix(rs[0].URL, "podman-remote-release-windows_amd64.zip") {
		t.Fatalf("URL = %q", rs[0].URL)
	}
}

func TestComposeExe(t *testing.T) {
	const api = "https://api.example.com"
	f := &githubFetcher{
		t:     t,
		files: map[string]string{api + "/repos/docker/compose/releases?per_page=20": "gh-compose.json"},
	}
	s := sources.NewCompose(f, api)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("attendu 1 release, obtenu %d", len(rs))
	}
	if rs[0].Version != "5.6.1" {
		t.Fatalf("Version = %q", rs[0].Version)
	}
	if rs[0].Archive != "exe" {
		t.Fatalf("Archive = %q, attendu exe", rs[0].Archive)
	}
	if !strings.HasSuffix(rs[0].URL, "docker-compose-windows-x86_64.exe") {
		t.Fatalf("URL = %q", rs[0].URL)
	}
}

func TestPxNotWheels(t *testing.T) {
	const api = "https://api.example.com"
	f := &githubFetcher{
		t:     t,
		files: map[string]string{api + "/repos/genotrance/px/releases?per_page=20": "gh-px.json"},
	}
	s := sources.NewPx(f, api)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("attendu 1 release, obtenu %d", len(rs))
	}
	if rs[0].Version != "0.11.0" {
		t.Fatalf("Version = %q, attendu 0.11.0", rs[0].Version)
	}
	if !strings.HasSuffix(rs[0].URL, "px-v0.11.0-windows-amd64.zip") {
		t.Fatalf("URL = %q", rs[0].URL)
	}
	if strings.Contains(rs[0].URL, "wheels") {
		t.Fatalf("variante -wheels retenue : %q", rs[0].URL)
	}
}

func TestResolveFallsBackToSha256File(t *testing.T) {
	const api = "https://api.example.com"
	const zipURL = "https://example.com/x.zip"
	const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	f := &githubFetcher{
		t:     t,
		texts: map[string]string{zipURL + ".sha256": hex + "  x.zip\n"},
	}
	s := sources.NewPodman(f, api)
	r, err := s.Resolve(context.Background(), sources.Release{Version: "1.0.0", URL: zipURL})
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if r.Checksum.Algo != "sha256" || r.Checksum.Hex != hex {
		t.Fatalf("Checksum = %+v", r.Checksum)
	}
	if len(f.urls) != 1 || f.urls[0] != zipURL+".sha256" {
		t.Fatalf("URL demandées = %v", f.urls)
	}
}

func TestResolveKeepsExistingChecksum(t *testing.T) {
	const api = "https://api.example.com"
	f := &githubFetcher{t: t}
	s := sources.NewPodman(f, api)
	in := sources.Release{
		Version:  "1.0.0",
		URL:      "https://example.com/x.zip",
		Checksum: download.Checksum{Algo: "sha256", Hex: "abc"},
	}
	r, err := s.Resolve(context.Background(), in)
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if r.Checksum.Hex != "abc" {
		t.Fatalf("Checksum = %+v", r.Checksum)
	}
	if len(f.urls) != 0 {
		t.Fatalf("aucune requête attendue, obtenu %v", f.urls)
	}
}
