package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// editorsExtensions retourne les extensions d'éditeur à installer pour un
// éditeur et une liste de rôles. Elle concatène la section "common" puis, pour
// chaque rôle dans l'ordre, la section du rôle ; le rôle "fullstack" ajoute
// aussi "back" et "front". Les doublons sont retirés en conservant l'ordre de
// première apparition.
func Extensions(p *profile.Profile, editor string, roles []string) []string {
	if p == nil {
		return nil
	}
	byRole := p.EditorExtensions[editor]
	if byRole == nil {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	add := func(ids []string) {
		for _, id := range ids {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}

	add(byRole["common"])
	for _, role := range roles {
		add(byRole[role])
		if role == "fullstack" {
			add(byRole["back"])
			add(byRole["front"])
		}
	}
	return out
}

// VSCode est l'installeur de Visual Studio Code.
type VSCode struct {
	// Extensions liste les identifiants d'extensions à installer.
	Extensions []string
}

var _ Installer = (*VSCode)(nil)

// NewVSCode crée un installeur VS Code avec les extensions données.
func NewVSCode(ext []string) *VSCode {
	return &VSCode{Extensions: ext}
}

// Tool retourne le nom de l'outil géré.
func (v *VSCode) Tool() string {
	return "vscode"
}

// Install télécharge et extrait VS Code puis installe les extensions. Aucun
// dossier "data" n'est créé : sa présence ferait basculer VS Code en mode
// portable. L'échec d'une extension n'empêche pas les suivantes ; une erreur
// listant les extensions en échec est retournée à la fin.
func (v *VSCode) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir, err := InstallZip(ctx, d, "vscode", r, false, report)
	if err != nil {
		return err
	}

	code := filepath.Join(dir, "bin", "code.cmd")
	var failed []string
	for _, ext := range v.Extensions {
		cmd := execx.Cmd{Name: code, Args: []string{"--install-extension", ext, "--force"}}
		res, err := d.Run.Run(ctx, cmd)
		if err != nil || res.Code != 0 {
			failed = append(failed, ext)
			if d.Log != nil {
				msg := strings.TrimSpace(res.Stderr)
				if msg == "" && err != nil {
					msg = err.Error()
				}
				d.Log.Printf("extension %s en échec : %s", ext, msg)
			}
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("extensions VS Code en échec : %s", strings.Join(failed, ", "))
	}
	return nil
}

// Activate active la version installée de VS Code.
func (v *VSCode) Activate(d Deps, version string) error {
	return SetCurrent(d, "vscode", version, []string{"bin"})
}

// Verify retourne la version de VS Code effectivement disponible.
func (v *VSCode) Verify(ctx context.Context, d Deps) (string, error) {
	code := filepath.Join(CurrentDir(d, "vscode"), "bin", "code.cmd")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: code, Args: []string{"--version"}})
	if err != nil {
		return "", fmt.Errorf("exécution de %q : %w", code, err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("échec de %q : %s", code, strings.TrimSpace(res.Stderr))
	}
	line := res.Stdout
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line), nil
}

// IntelliJ est l'installeur d'IntelliJ IDEA.
type IntelliJ struct {
	// Plugins liste les identifiants de plugins à installer.
	Plugins []string
	// StartMenuDir est le dossier du menu Démarrer où créer le raccourci.
	StartMenuDir string
}

var _ Installer = (*IntelliJ)(nil)

// NewIntelliJ crée un installeur IntelliJ IDEA.
func NewIntelliJ(plugins []string, startMenuDir string) *IntelliJ {
	return &IntelliJ{Plugins: plugins, StartMenuDir: startMenuDir}
}

// Tool retourne le nom de l'outil géré.
func (i *IntelliJ) Tool() string {
	return "intellij"
}

// editorsQuote échappe une chaîne pour un littéral PowerShell entre apostrophes.
func editorsQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// editorsShortcutScript construit le script PowerShell qui crée le raccourci
// du menu Démarrer vers l'exécutable donné.
func editorsShortcutScript(lnk, target string) string {
	return fmt.Sprintf(
		"$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s');"+
			"$s.TargetPath='%s';$s.Save()",
		editorsQuote(lnk), editorsQuote(target))
}

// Install télécharge et extrait IntelliJ IDEA, crée le raccourci du menu
// Démarrer puis installe les plugins en meilleur effort.
func (i *IntelliJ) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir, err := InstallZip(ctx, d, "intellij", r, false, report)
	if err != nil {
		return err
	}

	exe := filepath.Join(dir, "bin", "idea64.exe")

	if i.StartMenuDir != "" {
		lnk := filepath.Join(i.StartMenuDir, "IntelliJ IDEA.lnk")
		script := editorsShortcutScript(lnk, exe)
		cmd := execx.Cmd{
			Name: "powershell.exe",
			Args: []string{"-NoProfile", "-NonInteractive", "-Command", script},
		}
		res, err := d.Run.Run(ctx, cmd)
		if err != nil || res.Code != 0 {
			msg := strings.TrimSpace(res.Stderr)
			if msg == "" && err != nil {
				msg = err.Error()
			}
			return fmt.Errorf("création du raccourci %q : %s", lnk, msg)
		}
	}

	if len(i.Plugins) > 0 {
		args := append([]string{"installPlugins"}, i.Plugins...)
		res, err := d.Run.Run(ctx, execx.Cmd{Name: exe, Args: args})
		if err != nil || res.Code != 0 {
			if d.Log != nil {
				msg := strings.TrimSpace(res.Stderr)
				if msg == "" && err != nil {
					msg = err.Error()
				}
				d.Log.Printf("installation des plugins IntelliJ en échec : %s", msg)
			}
		}
	}

	return nil
}

// Activate active la version installée d'IntelliJ IDEA.
func (i *IntelliJ) Activate(d Deps, version string) error {
	return SetCurrent(d, "intellij", version, []string{"bin"})
}

// Verify retourne la version d'IntelliJ IDEA effectivement disponible, lue
// dans le fichier product-info.json de la version active.
func (i *IntelliJ) Verify(ctx context.Context, d Deps) (string, error) {
	path := filepath.Join(CurrentDir(d, "intellij"), "product-info.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("lecture de %q : %w", path, err)
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("décodage de %q : %w", path, err)
	}
	if info.Version == "" {
		return "", errors.New("version absente de product-info.json")
	}
	return info.Version, nil
}
