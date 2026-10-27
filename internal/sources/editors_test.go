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

var _ sources.Source = (*sources.VSCode)(nil)
var _ sources.Source = (*sources.IntelliJ)(nil)

// editorsFetcher sert le contenu des fixtures selon l'URL demandée.
type editorsFetcher struct {
	t     *testing.T
	files map[string]string
	urls  []string
}

func (f *editorsFetcher) read(name string) []byte {
	f.t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		f.t.Fatalf("lecture fixture %s : %v", name, err)
	}
	return b
}

func (f *editorsFetcher) GetJSON(_ context.Context, url string, v any) error {
	f.t.Helper()
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return errors.New("URL inconnue : " + url)
	}
	return editorsDecodeJSON(f.t, f.read(name), v)
}

// editorsDecodeJSON décode le contenu JSON d'une fixture dans v.
func editorsDecodeJSON(t *testing.T, data []byte, v any) error {
	t.Helper()
	return json.Unmarshal(data, v)
}

func (f *editorsFetcher) GetText(_ context.Context, url string) (string, error) {
	f.t.Helper()
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return "", errors.New("URL inconnue : " + url)
	}
	return string(f.read(name)), nil
}

func TestVSCodeRelease(t *testing.T) {
	const base = "https://update.example.com"
	f := &editorsFetcher{t: t, files: map[string]string{
		base + "/api/update/win32-x64-archive/stable/latest": "vscode-latest.json",
	}}
	s := sources.NewVSCode(f, base)
	if s.Name() != "vscode" {
		t.Fatalf("Name = %q", s.Name())
	}
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("len = %d", len(rs))
	}
	r := rs[0]
	if r.Version != "1.143.1" {
		t.Errorf("Version = %q", r.Version)
	}
	if r.Archive != "zip" {
		t.Errorf("Archive = %q", r.Archive)
	}
	if r.Checksum.Algo != "sha256" || r.Checksum.Hex != "b4f9643b8a549ca2c85a1bbc9063fc76f1e08fc0036a7f6a1a181cdc05235eca" {
		t.Errorf("Checksum = %+v", r.Checksum)
	}
	if !strings.HasSuffix(r.URL, "VSCode-win32-x64-1.143.1.zip") {
		t.Errorf("URL = %q", r.URL)
	}
	got, err := s.Resolve(context.Background(), r)
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if got.URL != r.URL || got.Checksum != r.Checksum {
		t.Errorf("Resolve = %+v", got)
	}
	if len(f.urls) != 1 || f.urls[0] != base+"/api/update/win32-x64-archive/stable/latest" {
		t.Errorf("URLs demandées = %v", f.urls)
	}
}

func TestIntelliJWindowsZip(t *testing.T) {
	const base = "https://jetbrains.example.com"
	f := &editorsFetcher{t: t, files: map[string]string{
		base + "/products/releases?code=IIC&latest=true&type=release": "jetbrains.json",
	}}
	s := sources.NewIntelliJ(f, base)
	if s.Name() != "intellij" {
		t.Fatalf("Name = %q", s.Name())
	}
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("len = %d", len(rs))
	}
	r := rs[0]
	if r.Version != "2025.3" {
		t.Errorf("Version = %q", r.Version)
	}
	if r.Archive != "zip" {
		t.Errorf("Archive = %q", r.Archive)
	}
	if !strings.HasSuffix(r.URL, ".win.zip") || strings.HasSuffix(r.URL, ".exe") {
		t.Errorf("URL = %q", r.URL)
	}
}

func TestIntelliJResolveChecksum(t *testing.T) {
	const base = "https://jetbrains.example.com"
	const zipURL = "https://download.example.com/idea/idea-2025.3.win.zip"
	f := &editorsFetcher{t: t, files: map[string]string{
		zipURL + ".sha256": "maven-bin-zip.sha512",
	}}
	s := sources.NewIntelliJ(f, base)
	got, err := s.Resolve(context.Background(), sources.Release{Version: "2025.3", URL: zipURL, Archive: "zip"})
	if err != nil {
		t.Fatalf("Resolve : %v", err)
	}
	if got.Checksum.Algo != "sha256" {
		t.Errorf("Algo = %q", got.Checksum.Algo)
	}
	if got.Checksum.Hex == "" || strings.ContainsAny(got.Checksum.Hex, " *") {
		t.Errorf("Hex = %q", got.Checksum.Hex)
	}
	if len(f.urls) != 1 || f.urls[0] != zipURL+".sha256" {
		t.Errorf("URLs demandées = %v", f.urls)
	}
}
