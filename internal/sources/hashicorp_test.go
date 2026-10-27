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

var _ sources.Source = (*sources.HashiCorp)(nil)

// hashicorpFetcher sert les fixtures locales selon l'URL demandée.
type hashicorpFetcher struct {
	t     *testing.T
	files map[string]string
	urls  []string
}

func (f *hashicorpFetcher) read(url string) (string, error) {
	f.urls = append(f.urls, url)
	name, ok := f.files[url]
	if !ok {
		return "", errors.New("URL inconnue : " + url)
	}
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (f *hashicorpFetcher) GetJSON(ctx context.Context, url string, v any) error {
	text, err := f.read(url)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), v)
}

func (f *hashicorpFetcher) GetText(ctx context.Context, url string) (string, error) {
	return f.read(url)
}

const hashicorpBase = "https://releases.example.com"

func hashicorpNewFetcher(t *testing.T) *hashicorpFetcher {
	return &hashicorpFetcher{
		t: t,
		files: map[string]string{
			hashicorpBase + "/vault/index.json":                   "vault-index.json",
			hashicorpBase + "/vault/2.2.1/vault_2.2.1_SHA256SUMS": "vault-sha256sums.txt",
		},
	}
}

func TestVaultReleasesExcludeEntAndRC(t *testing.T) {
	f := hashicorpNewFetcher(t)
	s := sources.NewVault(f, hashicorpBase)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	var got []string
	for _, r := range rs {
		got = append(got, r.Version)
	}
	want := []string{"2.2.1", "2.2.0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("versions = %v, attendu %v", got, want)
	}
	if len(f.urls) != 1 || f.urls[0] != hashicorpBase+"/vault/index.json" {
		t.Fatalf("URL demandées = %v", f.urls)
	}
}

func TestVaultResolveChecksum(t *testing.T) {
	f := hashicorpNewFetcher(t)
	s := sources.NewVault(f, hashicorpBase)
	rs, err := s.Releases(context.Background())
	if err != nil {
		t.Fatalf("Releases : %v", err)
	}
	r, err := sources.PickResolved(context.Background(), s, rs, "2.2.1")
	if err != nil {
		t.Fatalf("PickResolved : %v", err)
	}
	if r.Checksum.Algo != "sha256" {
		t.Fatalf("Algo = %q", r.Checksum.Algo)
	}
	if r.Checksum.Hex != "ec0544b354adf70e11db5247b9fa5e7046f70bf0c654944e0095780b5f056342" {
		t.Fatalf("Hex = %q", r.Checksum.Hex)
	}
	if !strings.HasSuffix(r.URL, "vault_2.2.1_windows_amd64.zip") {
		t.Fatalf("URL = %q", r.URL)
	}
}

func TestVaultResolveMissingLine(t *testing.T) {
	f := hashicorpNewFetcher(t)
	s := sources.NewVault(f, hashicorpBase)
	r := sources.Release{
		Version: "2.2.1",
		URL:     hashicorpBase + "/vault/2.2.1/vault_2.2.1_windows_arm64.zip",
		Archive: "zip",
	}
	_, err := s.Resolve(context.Background(), r)
	if !errors.Is(err, sources.ErrNoRelease) {
		t.Fatalf("err = %v, attendu ErrNoRelease", err)
	}
}
