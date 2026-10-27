package sources_test

import (
	"errors"
	"testing"

	"github.com/A-Djihadi/devstation/internal/download"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// var _ vérifie que le client de téléchargement implémente Fetcher.
var _ sources.Fetcher = (*download.Client)(nil)

func sample() []sources.Release {
	return []sources.Release{
		{Version: "21.0.4"},
		{Version: "21.0.10"},
		{Version: "17.0.9"},
	}
}

func TestPickMajor(t *testing.T) {
	r, err := sources.Pick(sample(), "21")
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if r.Version != "21.0.10" {
		t.Fatalf("version = %q, attendu 21.0.10", r.Version)
	}
}

func TestPickExact(t *testing.T) {
	r, err := sources.Pick(sample(), "21.0.4")
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if r.Version != "21.0.4" {
		t.Fatalf("version = %q, attendu 21.0.4", r.Version)
	}
}

func TestPickLTS(t *testing.T) {
	rs := []sources.Release{
		{Version: "22.1.0", LTS: false},
		{Version: "20.19.0", LTS: true},
		{Version: "18.20.0", LTS: true},
	}
	r, err := sources.Pick(rs, "lts")
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if r.Version != "20.19.0" {
		t.Fatalf("version = %q, attendu 20.19.0", r.Version)
	}
}

func TestPickMissing(t *testing.T) {
	_, err := sources.Pick(sample(), "99")
	if !errors.Is(err, sources.ErrNoRelease) {
		t.Fatalf("erreur = %v, attendu ErrNoRelease", err)
	}
}

func TestRestrict(t *testing.T) {
	got := sources.Restrict(sample(), []string{"21"})
	if len(got) != 2 {
		t.Fatalf("len = %d, attendu 2", len(got))
	}
	for _, r := range got {
		if r.Version != "21.0.4" && r.Version != "21.0.10" {
			t.Fatalf("version inattendue : %q", r.Version)
		}
	}
	if len(sources.Restrict(sample(), nil)) != 3 {
		t.Fatal("allowed vide doit retourner rs inchangé")
	}
}

func TestSortDescNumeric(t *testing.T) {
	rs := []sources.Release{
		{Version: "21.0.9"},
		{Version: "21.0.10"},
		{Version: "v20.19.0"},
		{Version: "v22.1.0"},
	}
	sources.SortDesc(rs)
	want := []string{"v22.1.0", "21.0.10", "21.0.9", "v20.19.0"}
	for i, w := range want {
		if rs[i].Version != w {
			t.Fatalf("position %d = %q, attendu %q", i, rs[i].Version, w)
		}
	}
}

func TestDownloadClientIsFetcher(t *testing.T) {
	var _ sources.Fetcher = (*download.Client)(nil)
}
