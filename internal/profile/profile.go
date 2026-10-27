// Package profile charge et valide les profils d'équipe DevStation.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolSpec décrit un outil installable et ses versions disponibles.
type ToolSpec struct {
	Versions     []string `yaml:"versions"`
	Default      string   `yaml:"default"`
	Distribution string   `yaml:"distribution"`
	Npm          string   `yaml:"npm"`
	Optional     bool     `yaml:"optional"`
	Enabled      *bool    `yaml:"enabled"`
}

// Tools regroupe les outils gérés par DevStation.
type Tools struct {
	Java       ToolSpec `yaml:"java"`
	Node       ToolSpec `yaml:"node"`
	AngularCLI ToolSpec `yaml:"angular_cli"`
	Python     ToolSpec `yaml:"python"`
	Maven      ToolSpec `yaml:"maven"`
	Gradle     ToolSpec `yaml:"gradle"`
}

// Nexus décrit les dépôts Nexus d'entreprise.
type Nexus struct {
	Maven      string `yaml:"maven"`
	Npm        string `yaml:"npm"`
	Pypi       string `yaml:"pypi"`
	Containers string `yaml:"containers"`
	Auth       bool   `yaml:"auth"`
}

// Network décrit la configuration réseau et proxy.
type Network struct {
	Proxy     string   `yaml:"proxy"`
	ProxyAuth string   `yaml:"proxy_auth"`
	NoProxy   []string `yaml:"no_proxy"`
	CACerts   []string `yaml:"ca_certs"`
	Nexus     Nexus    `yaml:"nexus"`
}

// Repo décrit un dépôt Git et les rôles qui le clonent.
type Repo struct {
	Name  string   `yaml:"name"`
	URL   string   `yaml:"url"`
	Roles []string `yaml:"roles"`
}

// SecretRef décrit un secret à lire dans Vault et sa cible.
type SecretRef struct {
	Path   string `yaml:"path"`
	Field  string `yaml:"field"`
	Target string `yaml:"target"`
}

// Vault décrit la configuration Vault du profil.
type Vault struct {
	Addr    string      `yaml:"addr"`
	Auth    string      `yaml:"auth"`
	Mount   string      `yaml:"mount"`
	Role    string      `yaml:"role"`
	Secrets []SecretRef `yaml:"secrets"`
}

// Profile est un profil d'équipe chargé depuis un fichier YAML.
type Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Defaults    struct {
		Roles  []string `yaml:"roles"`
		Editor string   `yaml:"editor"`
	} `yaml:"defaults"`
	Tools  Tools  `yaml:"tools"`
	Network Network `yaml:"network"`
	Git    struct {
		Autocrlf string `yaml:"autocrlf"`
	} `yaml:"git"`
	Repos            []Repo                         `yaml:"repos"`
	EditorExtensions map[string]map[string][]string `yaml:"editor_extensions"`
	Vault            Vault                          `yaml:"vault"`
	Dir              string                         `yaml:"-"`
}

// ValidationError décrit une erreur de validation d'un profil.
type ValidationError struct {
	Key  string
	Line int
	Msg  string
}

// Error implémente l'interface error.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("profil invalide : %s (ligne %d) : %s", e.Key, e.Line, e.Msg)
}

var (
	validProxyAuth  = map[string]bool{"": true, "none": true, "kerberos": true}
	validVaultAuth  = map[string]bool{"": true, "oidc": true, "ldap": true, "kerberos": true, "token": true}
	validEditor     = map[string]bool{"": true, "vscode": true, "intellij": true, "both": true}
	validRoles      = map[string]bool{"front": true, "back": true, "fullstack": true, "python": true}
	validSecretTgt  = map[string]bool{"nexus.username": true, "nexus.password": true}
)

// unknownField extrait la clé inconnue et sa ligne depuis un message
// yaml.v3 du type "line 3: field toolz not found in type profile.Profile".
func unknownField(msg string) (string, int) {
	line := 0
	if i := strings.Index(msg, "line "); i >= 0 {
		rest := msg[i+len("line "):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j > 0 {
			if n, err := strconv.Atoi(rest[:j]); err == nil {
				line = n
			}
		}
	}
	key := ""
	if i := strings.Index(msg, "field "); i >= 0 {
		rest := msg[i+len("field "):]
		if j := strings.Index(rest, " "); j >= 0 {
			key = rest[:j]
		} else {
			key = rest
		}
	}
	if key == "" {
		key = "yaml"
	}
	return key, line
}

// Load lit et valide le profil YAML situé à path.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) && len(typeErr.Errors) > 0 {
			key, line := unknownField(typeErr.Errors[0])
			return nil, &ValidationError{Key: key, Line: line, Msg: typeErr.Errors[0]}
		}
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p.Dir = filepath.Dir(abs)
	for i, c := range p.Network.CACerts {
		if c != "" && !filepath.IsAbs(c) {
			p.Network.CACerts[i] = filepath.Join(p.Dir, c)
		}
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate vérifie la cohérence du profil.
func (p *Profile) Validate() error {
	if p.Network.ProxyAuth != "" && !validProxyAuth[p.Network.ProxyAuth] {
		return &ValidationError{Key: "network.proxy_auth", Msg: "valeur attendue : none ou kerberos"}
	}
	if p.Network.ProxyAuth == "kerberos" && p.Network.Proxy == "" {
		return &ValidationError{Key: "network.proxy", Msg: "proxy requis avec proxy_auth: kerberos"}
	}
	if p.Vault.Auth != "" && !validVaultAuth[p.Vault.Auth] {
		return &ValidationError{Key: "vault.auth", Msg: "valeur attendue : oidc, ldap, kerberos ou token"}
	}
	if p.Defaults.Editor != "" && !validEditor[p.Defaults.Editor] {
		return &ValidationError{Key: "defaults.editor", Msg: "valeur attendue : vscode, intellij ou both"}
	}
	for _, r := range p.Defaults.Roles {
		if !validRoles[r] {
			return &ValidationError{Key: "defaults.roles", Msg: fmt.Sprintf("rôle inconnu : %s", r)}
		}
	}
	for _, repo := range p.Repos {
		for _, r := range repo.Roles {
			if !validRoles[r] {
				return &ValidationError{Key: "repos." + repo.Name + ".roles", Msg: fmt.Sprintf("rôle inconnu : %s", r)}
			}
		}
	}
	for _, s := range p.Vault.Secrets {
		if !validSecretTgt[s.Target] {
			return &ValidationError{Key: "vault.secrets", Msg: fmt.Sprintf("cible inconnue : %s", s.Target)}
		}
	}
	tools := []struct {
		key  string
		spec ToolSpec
	}{
		{"tools.java", p.Tools.Java},
		{"tools.node", p.Tools.Node},
		{"tools.angular_cli", p.Tools.AngularCLI},
		{"tools.python", p.Tools.Python},
		{"tools.maven", p.Tools.Maven},
		{"tools.gradle", p.Tools.Gradle},
	}
	for _, t := range tools {
		if t.spec.Default == "" || len(t.spec.Versions) == 0 {
			continue
		}
		found := false
		for _, v := range t.spec.Versions {
			if v == t.spec.Default {
				found = true
				break
			}
		}
		if !found {
			return &ValidationError{Key: t.key + ".default", Msg: fmt.Sprintf("version %q absente de versions", t.spec.Default)}
		}
	}
	return nil
}

// ReposFor retourne les dépôts dont un rôle figure dans roles.
func (p *Profile) ReposFor(roles []string) []Repo {
	want := make(map[string]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}
	var out []Repo
	for _, repo := range p.Repos {
		for _, r := range repo.Roles {
			if want[r] {
				out = append(out, repo)
				break
			}
		}
	}
	return out
}