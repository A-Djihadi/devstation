package config

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A-Djihadi/devstation/internal/state"
)

// mavenTestSettings est la vue de décodage utilisée par les tests.
type mavenTestSettings struct {
	XMLName xml.Name `xml:"settings"`
	Mirrors []struct {
		ID string `xml:"id"`
	} `xml:"mirrors>mirror"`
	Proxies []struct {
		ID   string `xml:"id"`
		Host string `xml:"host"`
		Port int    `xml:"port"`
	} `xml:"proxies>proxy"`
}

// mavenDecode décode le contenu XML d'un settings.xml pour les tests.
func mavenDecode(t *testing.T, path string) mavenTestSettings {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture %s : %v", path, err)
	}
	var s mavenTestSettings
	if err := xml.Unmarshal(data, &s); err != nil {
		t.Fatalf("décodage XML : %v\n%s", err, data)
	}
	return s
}

// mavenTempPath renvoie un chemin de settings.xml dans un répertoire temporaire.
func mavenTempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "settings.xml")
}

func TestMavenCreatesMinimalSettings(t *testing.T) {
	path := mavenTempPath(t)
	st := &state.State{}
	s := MavenSettings{MirrorURL: "https://repo.example.com/maven", ProxyHost: "127.0.0.1", ProxyPort: 3128}
	if err := SetMavenSettings(path, s, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	got := mavenDecode(t, path)
	if len(got.Mirrors) != 1 || got.Mirrors[0].ID != "devstation" {
		t.Fatalf("miroirs = %+v", got.Mirrors)
	}
	if len(got.Proxies) != 1 || got.Proxies[0].ID != "devstation" {
		t.Fatalf("proxies = %+v", got.Proxies)
	}
	if got.Proxies[0].Host != "127.0.0.1" || got.Proxies[0].Port != 3128 {
		t.Fatalf("proxy = %+v", got.Proxies[0])
	}
}

func TestMavenReplacesOnlyDevstationMirror(t *testing.T) {
	path := mavenTempPath(t)
	corp := "<mirror>\n      <id>corp</id>\n      <url>https://corp.example.com/maven</url>\n      <mirrorOf>*</mirrorOf>\n    </mirror>"
	initial := "<?xml version=\"1.0\"?>\n<settings>\n  <mirrors>\n    " + corp + "\n  </mirrors>\n</settings>\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &state.State{}
	if err := SetMavenSettings(path, MavenSettings{MirrorURL: "https://repo.example.com/maven"}, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	if err := SetMavenSettings(path, MavenSettings{MirrorURL: "https://repo2.example.com/maven"}, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, corp) {
		t.Fatalf("le miroir corp a été modifié :\n%s", content)
	}
	got := mavenDecode(t, path)
	if len(got.Mirrors) != 2 {
		t.Fatalf("miroirs = %+v", got.Mirrors)
	}
	if !strings.Contains(content, "https://repo2.example.com/maven") {
		t.Fatalf("le miroir devstation n'a pas été remplacé :\n%s", content)
	}
}

func TestMavenAddsProxiesSection(t *testing.T) {
	path := mavenTempPath(t)
	initial := "<?xml version=\"1.0\"?>\n<settings>\n  <mirrors>\n  </mirrors>\n</settings>\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &state.State{}
	s := MavenSettings{MirrorURL: "https://repo.example.com/maven", ProxyHost: "127.0.0.1", ProxyPort: 3128, NonProxyHosts: "localhost|*.example.com"}
	if err := SetMavenSettings(path, s, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	got := mavenDecode(t, path)
	if len(got.Proxies) != 1 || got.Proxies[0].ID != "devstation" {
		t.Fatalf("proxies = %+v", got.Proxies)
	}
	if len(got.Mirrors) != 1 || got.Mirrors[0].ID != "devstation" {
		t.Fatalf("miroirs = %+v", got.Mirrors)
	}
}

func TestMavenEscapesValues(t *testing.T) {
	path := mavenTempPath(t)
	st := &state.State{}
	s := MavenSettings{MirrorURL: "https://repo.example.com/maven?a=1&b=2", ProxyHost: "127.0.0.1", ProxyPort: 3128}
	if err := SetMavenSettings(path, s, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "a=1&amp;b=2") {
		t.Fatalf("valeur non échappée :\n%s", data)
	}
	if err := xml.Unmarshal(data, new(mavenTestSettings)); err != nil {
		t.Fatalf("XML invalide : %v", err)
	}
}

func TestMavenRestore(t *testing.T) {
	path := mavenTempPath(t)
	initial := "<?xml version=\"1.0\"?>\n<settings>\n  <mirrors>\n  </mirrors>\n</settings>\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &state.State{}
	s := MavenSettings{MirrorURL: "https://repo.example.com/maven", ProxyHost: "127.0.0.1", ProxyPort: 3128}
	if err := SetMavenSettings(path, s, st); err != nil {
		t.Fatalf("SetMavenSettings : %v", err)
	}
	if err := RestoreMavenSettings(path, st); err != nil {
		t.Fatalf("RestoreMavenSettings : %v", err)
	}
	got := mavenDecode(t, path)
	if len(got.Mirrors) != 0 {
		t.Fatalf("miroirs restants = %+v", got.Mirrors)
	}
	if len(got.Proxies) != 0 {
		t.Fatalf("proxies restants = %+v", got.Proxies)
	}
}
