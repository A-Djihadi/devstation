package profile

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAnswersRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "answers.yaml")
	in := &Answers{
		Profile:     "demo",
		Roles:       []string{"back", "front"},
		Editor:      "vscode",
		Versions:    map[string]string{"java": "21", "node": "20"},
		Proxy:       "http://proxy.example.com:8080",
		ProxyAuth:   "kerberos",
		NoProxy:     []string{"localhost", ".example.com"},
		CACerts:     []string{"/tmp/ca.pem"},
		GitName:     "Jean Dupont",
		GitEmail:    "jean.dupont@example.com",
		GenerateSSH: true,
		Repos:       []string{"api", "web"},
		Workspace:   filepath.Join(dir, "workspace", "demo"),
		Podman:      true,
		Vault:       true,
	}
	if err := in.Save(path); err != nil {
		t.Fatalf("Save : %v", err)
	}
	out, err := LoadAnswers(path)
	if err != nil {
		t.Fatalf("LoadAnswers : %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("aller-retour différent :\nattendu %+v\nobtenu  %+v", in, out)
	}
}

func TestAnswersHaveNoSecretFields(t *testing.T) {
	typ := reflect.TypeOf(Answers{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, bad := range []string{"Token", "Password", "Secret"} {
			if strings.Contains(name, bad) {
				t.Fatalf("champ interdit dans Answers : %s", name)
			}
		}
	}
}

func TestDefaultsFromDemoProfile(t *testing.T) {
	p, err := Load(filepath.Join("..", "..", "profiles", "demo.yaml"))
	if err != nil {
		t.Fatalf("Load : %v", err)
	}
	home := filepath.Join(string(filepath.Separator), "home", "user")
	a := Defaults(p, home)
	if a.Profile != p.Name {
		t.Fatalf("Profile = %q, attendu %q", a.Profile, p.Name)
	}
	want := filepath.Join(home, "workspace", p.Name)
	if a.Workspace != want {
		t.Fatalf("Workspace = %q, attendu %q", a.Workspace, want)
	}
	if !reflect.DeepEqual(a.Roles, p.Defaults.Roles) {
		t.Fatalf("Roles = %v, attendu %v", a.Roles, p.Defaults.Roles)
	}
	if a.Editor != p.Defaults.Editor {
		t.Fatalf("Editor = %q, attendu %q", a.Editor, p.Defaults.Editor)
	}
	if a.Proxy != p.Network.Proxy || a.ProxyAuth != p.Network.ProxyAuth {
		t.Fatalf("proxy = %q/%q, attendu %q/%q", a.Proxy, a.ProxyAuth, p.Network.Proxy, p.Network.ProxyAuth)
	}
	if a.Versions["java"] != p.Tools.Java.Default {
		t.Fatalf("Versions[java] = %q, attendu %q", a.Versions["java"], p.Tools.Java.Default)
	}
	if a.Versions["node"] != p.Tools.Node.Default {
		t.Fatalf("Versions[node] = %q, attendu %q", a.Versions["node"], p.Tools.Node.Default)
	}
	wantRepos := p.ReposFor(p.Defaults.Roles)
	if len(a.Repos) != len(wantRepos) {
		t.Fatalf("Repos = %v, attendu %d dépôts", a.Repos, len(wantRepos))
	}
	for i, r := range wantRepos {
		if a.Repos[i] != r.Name {
			t.Fatalf("Repos[%d] = %q, attendu %q", i, a.Repos[i], r.Name)
		}
	}
}
