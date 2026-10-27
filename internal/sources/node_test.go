package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/sources"
)

var _ sources.Source = (*sources.Node)(nil)

// nodeFakeFetcher sert les fixtures locales selon l'URL demandée.
type nodeFakeFetcher struct {
	baseURL string
	urls    []string
}

// nodeFixtureFor associe une URL à un fichier de testdata.
func nodeFixtureFor(url string) (string, bool) {
	switch {
	case strings.HasSuffix(url, "/dist/index.json"):
		return "testdata/node-index.json", true
	case strings.HasSuffix(url, "/SHASUMS256.txt"):
		return "testdata/node-shasums.txt", true
	}
	return "", false
}

func (f *nodeFakeFetcher) GetJSON(_ context.Context, url string, v any) error {
	text, err := f.GetText(context.Background(), url)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), v)
}

func (f *nodeFakeFetcher) GetText(_ context.Context, url string) (string, error) {
	f.urls = append(f.urls, url)
	name, ok := nodeFixtureFor(url)
	if !ok {
		return "", fmt.Errorf("URL inconnue : %s", url)
	}
	b, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TestNodeReleases(t *testing.T) {
	f := &nodeFakeFetcher{baseURL: "https://node.example.com"}
	s := sources.NewNode(f, f.baseURL)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 4 {
		t.Fatalf("attendu 4 versions, obtenu %d", len(rs))
	}
	if rs[0].Version != "26.12.1" {
		t.Errorf("première version = %q, attendu 26.12.1", rs[0].Version)
	}
	if rs[0].LTS {
		t.Errorf("26.12.1 ne doit pas être LTS")
	}
	var lts *sources.Release
	for i := range rs {
		if rs[i].Version == "24.22.1" {
			lts = &rs[i]
		}
	}
	if lts == nil {
		t.Fatalf("version 24.22.1 absente")
	}
	if !lts.LTS {
		t.Errorf("24.22.1 doit être LTS")
	}
	wantURL := "https://node.example.com/dist/v24.22.1/node-v24.22.1-win-x64.zip"
	if lts.URL != wantURL {
		t.Errorf("URL = %q, attendu %q", lts.URL, wantURL)
	}
	if lts.Archive != "zip" {
		t.Errorf("Archive = %q, attendu zip", lts.Archive)
	}
	if len(f.urls) != 1 || f.urls[0] != "https://node.example.com/dist/index.json" {
		t.Errorf("URL demandées = %v", f.urls)
	}
}

func TestNodeResolveChecksum(t *testing.T) {
	f := &nodeFakeFetcher{baseURL: "https://node.example.com"}
	s := sources.NewNode(f, f.baseURL)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	first, err := sources.Pick(rs, "lts")
	if err != nil {
		t.Fatalf("Pick : %v", err)
	}
	if first.Version != "24.22.1" {
		t.Fatalf("première LTS = %q, attendu 24.22.1", first.Version)
	}
	r, err := s.Resolve(context.Background(), first)
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if r.Checksum.Algo != "sha256" {
		t.Errorf("Algo = %q, attendu sha256", r.Checksum.Algo)
	}
	const want = "d2046d28c2c3dcd211ac5145c0bf30e2047ef9dfb306693c8c6e2eccd8b95fa5"
	if r.Checksum.Hex != want {
		t.Errorf("Hex = %q, attendu %q", r.Checksum.Hex, want)
	}
}

func TestNodeResolveMissing(t *testing.T) {
	f := &nodeFakeFetcher{baseURL: "https://node.example.com"}
	s := sources.NewNode(f, f.baseURL)
	_, err := s.Resolve(context.Background(), sources.Release{Version: "1.2.3"})
	if !errors.Is(err, sources.ErrNoRelease) {
		t.Fatalf("attendu ErrNoRelease, obtenu %v", err)
	}
}
