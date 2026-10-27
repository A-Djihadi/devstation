package sources_test

import (
	"context"
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

// fakeSource est une Source de test dont Resolve est piloté par une fonction.
type fakeSource struct {
	resolve func(sources.Release) (sources.Release, error)
}

func (f fakeSource) Name() string { return "fake" }

func (f fakeSource) Releases(ctx context.Context) ([]sources.Release, error) {
	return nil, nil
}

func (f fakeSource) Resolve(ctx context.Context, r sources.Release) (sources.Release, error) {
	return f.resolve(r)
}

func TestCandidatesOrderAndFilter(t *testing.T) {
	rs := []sources.Release{
		{Version: "21.0.4"},
		{Version: "21.0.10"},
		{Version: "17.0.9"},
		{Version: "20.19.0", LTS: true},
	}
	got := sources.Candidates(rs, "21")
	if len(got) != 2 {
		t.Fatalf("len = %d, attendu 2", len(got))
	}
	if got[0].Version != "21.0.10" || got[1].Version != "21.0.4" {
		t.Fatalf("ordre = %q, %q, attendu 21.0.10, 21.0.4", got[0].Version, got[1].Version)
	}
	if len(sources.Candidates(rs, "")) != 4 {
		t.Fatal("want vide doit retourner toutes les versions")
	}
	if len(sources.Candidates(rs, "latest")) != 4 {
		t.Fatal("latest doit retourner toutes les versions")
	}
	lts := sources.Candidates(rs, "lts")
	if len(lts) != 1 || lts[0].Version != "20.19.0" {
		t.Fatalf("lts = %v, attendu [20.19.0]", lts)
	}
}

func TestPickResolvedSkipsUnavailable(t *testing.T) {
	rs := []sources.Release{
		{Version: "3.13.18"},
		{Version: "3.13.9"},
	}
	s := fakeSource{resolve: func(r sources.Release) (sources.Release, error) {
		if r.Version == "3.13.18" {
			return sources.Release{}, sources.ErrNoRelease
		}
		r.URL = "https://example.com/python-" + r.Version + ".exe"
		return r, nil
	}}
	got, err := sources.PickResolved(context.Background(), s, rs, "3.13")
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if got.Version != "3.13.9" {
		t.Fatalf("version = %q, attendu 3.13.9", got.Version)
	}
}

func TestPickResolvedStopsOnOtherError(t *testing.T) {
	rs := []sources.Release{
		{Version: "3.13.18"},
		{Version: "3.13.9"},
	}
	netErr := errors.New("réseau indisponible")
	s := fakeSource{resolve: func(r sources.Release) (sources.Release, error) {
		return sources.Release{}, netErr
	}}
	_, err := sources.PickResolved(context.Background(), s, rs, "3.13")
	if !errors.Is(err, netErr) {
		t.Fatalf("erreur = %v, attendu %v", err, netErr)
	}
}

func TestPickResolvedNone(t *testing.T) {
	rs := []sources.Release{
		{Version: "3.13.18"},
		{Version: "3.13.9"},
	}
	s := fakeSource{resolve: func(r sources.Release) (sources.Release, error) {
		return sources.Release{}, sources.ErrNoRelease
	}}
	_, err := sources.PickResolved(context.Background(), s, rs, "3.13")
	if !errors.Is(err, sources.ErrNoRelease) {
		t.Fatalf("erreur = %v, attendu ErrNoRelease", err)
	}
}
