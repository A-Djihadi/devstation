package profile

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Answers regroupe les réponses de l'assistant de configuration DevStation.
// Aucun champ ne doit contenir de secret : ils sont stockés dans le
// Gestionnaire d'identifiants Windows.
type Answers struct {
	Profile     string            `yaml:"profile"`
	Roles       []string          `yaml:"roles"`
	Editor      string            `yaml:"editor"`
	Versions    map[string]string `yaml:"versions"`
	Proxy       string            `yaml:"proxy"`
	ProxyAuth   string            `yaml:"proxy_auth"`
	NoProxy     []string          `yaml:"no_proxy"`
	CACerts     []string          `yaml:"ca_certs"`
	GitName     string            `yaml:"git_name"`
	GitEmail    string            `yaml:"git_email"`
	GenerateSSH bool              `yaml:"generate_ssh"`
	Repos       []string          `yaml:"repos"`
	Workspace   string            `yaml:"workspace"`
	Podman      bool              `yaml:"podman"`
	Vault       bool              `yaml:"vault"`
}

// LoadAnswers lit les réponses de l'assistant depuis path.
func LoadAnswers(path string) (*Answers, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a Answers
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Save écrit les réponses de l'assistant dans path.
func (a *Answers) Save(path string) error {
	data, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}

// Defaults construit les réponses par défaut à partir du profil p.
// Workspace vaut home\workspace\<p.Name>.
func Defaults(p *Profile, home string) *Answers {
	a := &Answers{
		Profile:   p.Name,
		Roles:     append([]string(nil), p.Defaults.Roles...),
		Editor:    p.Defaults.Editor,
		Versions:  map[string]string{},
		Proxy:     p.Network.Proxy,
		ProxyAuth: p.Network.ProxyAuth,
		NoProxy:   append([]string(nil), p.Network.NoProxy...),
		CACerts:   append([]string(nil), p.Network.CACerts...),
		Workspace: filepath.Join(home, "workspace", p.Name),
		Vault:     p.Vault.Addr != "",
	}
	tools := map[string]ToolSpec{
		"java":        p.Tools.Java,
		"node":        p.Tools.Node,
		"angular_cli": p.Tools.AngularCLI,
		"python":      p.Tools.Python,
		"maven":       p.Tools.Maven,
		"gradle":      p.Tools.Gradle,
	}
	for name, spec := range tools {
		if spec.Default != "" {
			a.Versions[name] = spec.Default
		}
	}
	for _, repo := range p.ReposFor(a.Roles) {
		a.Repos = append(a.Repos, repo.Name)
	}
	return a
}
