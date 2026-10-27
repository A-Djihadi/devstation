package sources_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.Python)(nil)

// pythonFetcher sert le contenu des fixtures selon l'URL demandée et
// enregistre les URL demandées.
type pythonFetcher struct {
	urls map[string]string
	seen []string
}

// pythonFetcherFor construit un fetcher servant les fixtures données.
func pythonFetcherFor(t *testing.T, urls map[string]string) *pythonFetcher {
	t.Helper()
	return &pythonFetcher{urls: urls}
}

func (f *pythonFetcher) GetJSON(ctx context.Context, url string, v any) error {
	return fmt.Errorf("pythonFetcher : GetJSON inattendu pour %q", url)
}

func (f *pythonFetcher) GetText(ctx context.Context, url string) (string, error) {
	f.seen = append(f.seen, url)
	name, ok := f.urls[url]
	if !ok {
		return "", fmt.Errorf("pythonFetcher : URL inconnue %q", url)
	}
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// pythonFixtureURLs associe les URL attendues aux fixtures.
func pythonFixtureURLs(base string) map[string]string {
	return map[string]string{
		base + "/ftp/python/":        "python-ftp.html",
		base + "/ftp/python/3.16.0/": "python-3.16.0.html",
		base + "/ftp/python/3.15.2/": "python-3.15.2.html",
	}
}

func TestPythonReleases(t *testing.T) {
	const base = "https://www.python.org"
	f := pythonFetcherFor(t, pythonFixtureURLs(base))
	src := sources.NewPython(f, base)

	if src.Name() != "python" {
		t.Fatalf("Name() = %q, attendu %q", src.Name(), "python")
	}

	rs, err := src.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}

	want := []string{"3.16.0", "3.15.2", "3.13.18", "3.13.17", "3.12.16", "3.9.25"}
	if len(rs) != len(want) {
		t.Fatalf("Releases : %d versions, attendu %d (%v)", len(rs), len(want), rs)
	}
	for i, w := range want {
		if rs[i].Version != w {
			t.Errorf("Releases[%d] = %q, attendu %q", i, rs[i].Version, w)
		}
	}
	for _, r := range rs {
		if r.Version == "2.7.18" {
			t.Errorf("Releases contient 2.7.18, attendu exclu")
		}
		if r.LTS {
			t.Errorf("Releases[%s].LTS = true, attendu false", r.Version)
		}
		if r.URL != "" {
			t.Errorf("Releases[%s].URL = %q, attendu vide", r.Version, r.URL)
		}
	}

	if len(f.seen) != 1 || f.seen[0] != base+"/ftp/python/" {
		t.Errorf("URL demandées = %v, attendu [%s]", f.seen, base+"/ftp/python/")
	}
}

func TestPythonResolveNoInstaller(t *testing.T) {
	const base = "https://www.python.org"
	f := pythonFetcherFor(t, pythonFixtureURLs(base))
	src := sources.NewPython(f, base)

	_, err := src.Resolve(context.Background(), sources.Release{Version: "3.16.0"})
	if !errors.Is(err, sources.ErrNoRelease) {
		t.Fatalf("Resolve(3.16.0) : err = %v, attendu ErrNoRelease", err)
	}
}

func TestPythonPickResolvedSkipsPrerelease(t *testing.T) {
	const base = "https://www.python.org"
	f := pythonFetcherFor(t, pythonFixtureURLs(base))
	src := sources.NewPython(f, base)

	rs, err := src.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}

	r, err := sources.PickResolved(context.Background(), src, rs, "latest")
	if err != nil {
		t.Fatalf("PickResolved : %v", err)
	}
	if r.Version != "3.15.2" {
		t.Errorf("PickResolved(latest).Version = %q, attendu %q", r.Version, "3.15.2")
	}
	if r.Archive != "exe" {
		t.Errorf("PickResolved(latest).Archive = %q, attendu %q", r.Archive, "exe")
	}
	wantURL := base + "/ftp/python/3.15.2/python-3.15.2-amd64.exe"
	if r.URL != wantURL {
		t.Errorf("PickResolved(latest).URL = %q, attendu %q", r.URL, wantURL)
	}
	if r.Checksum.Algo != "" || r.Checksum.Hex != "" {
		t.Errorf("PickResolved(latest).Checksum = %+v, attendu vide", r.Checksum)
	}
}
