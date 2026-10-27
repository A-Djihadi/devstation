package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/A-Djihadi/devstation/internal/profile"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// ErrCancelled indique que l'utilisateur a refusé de lancer l'installation.
var ErrCancelled = errors.New("installation annulée")

// WizardIO regroupe les entrées et sorties de l'assistant interactif.
type WizardIO struct {
	// In est la source des réponses de l'utilisateur.
	In io.Reader
	// Out est la destination de l'affichage.
	Out io.Writer
	// Accessible active le mode texte ligne à ligne de huh (lecteurs
	// d'écran, tests).
	Accessible bool
}

// wizSteps liste les titres des étapes de l'assistant, dans l'ordre.
var wizSteps = []string{
	"Profil",
	"Rôle",
	"Éditeur",
	"Versions",
	"Réseau",
	"Git",
	"Dépôts",
	"Conteneurs",
	"Récapitulatif",
}

// Steps retourne les titres des étapes de l'assistant, dans l'ordre.
func Steps() []string {
	return append([]string(nil), wizSteps...)
}

// wizBreadcrumb rend le fil d'Ariane d'une étape : "<n>/9 · <titre>".
func wizBreadcrumb(n int) string {
	return fmt.Sprintf("%d/%d · %s", n, len(wizSteps), wizSteps[n-1])
}

// wizVersionOptions construit les libellés des versions proposées pour un
// outil. Une version LTS est annotée " (LTS)".
func wizVersionOptions(rs []sources.Release) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(rs))
	for _, r := range rs {
		label := r.Version
		if r.LTS {
			label += " (LTS)"
		}
		opts = append(opts, huh.NewOption(label, r.Version))
	}
	return opts
}

// wizToolsFor retourne les outils nécessaires selon les rôles choisis, dans
// un ordre stable.
func wizToolsFor(roles []string) []string {
	want := make(map[string]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}
	var out []string
	if want["back"] || want["fullstack"] {
		out = append(out, "java", "maven")
	}
	if want["front"] || want["fullstack"] {
		out = append(out, "node", "angular_cli")
	}
	if want["python"] {
		out = append(out, "python")
	}
	return out
}

// wizConfirmed traduit une confirmation en erreur : un refus renvoie
// ErrCancelled.
func wizConfirmed(ok bool) error {
	if !ok {
		return ErrCancelled
	}
	return nil
}

// wizSummary rend le récapitulatif des choix de l'utilisateur.
func wizSummary(a *profile.Answers) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Profil : %s\n", a.Profile)
	fmt.Fprintf(&b, "Rôles : %s\n", strings.Join(a.Roles, ", "))
	fmt.Fprintf(&b, "Éditeur : %s\n", a.Editor)
	if len(a.Versions) > 0 {
		b.WriteString("Versions :\n")
		for _, tool := range []string{"java", "node", "angular_cli", "python", "maven", "gradle"} {
			if v, ok := a.Versions[tool]; ok && v != "" {
				fmt.Fprintf(&b, "  %s : %s\n", tool, v)
			}
		}
	}
	if a.Proxy != "" {
		fmt.Fprintf(&b, "Proxy : %s\n", a.Proxy)
	}
	if a.ProxyAuth != "" {
		fmt.Fprintf(&b, "Authentification proxy : %s\n", a.ProxyAuth)
	}
	if len(a.NoProxy) > 0 {
		fmt.Fprintf(&b, "Sans proxy : %s\n", strings.Join(a.NoProxy, ", "))
	}
	if a.GitName != "" || a.GitEmail != "" {
		fmt.Fprintf(&b, "Git : %s <%s>\n", a.GitName, a.GitEmail)
	}
	if a.GenerateSSH {
		b.WriteString("Clé SSH : génération demandée\n")
	}
	if len(a.Repos) > 0 {
		fmt.Fprintf(&b, "Dépôts : %s\n", strings.Join(a.Repos, ", "))
	}
	if a.Workspace != "" {
		fmt.Fprintf(&b, "Dossier de travail : %s\n", a.Workspace)
	}
	if a.Podman {
		b.WriteString("Podman : installation demandée\n")
	}
	if a.Vault {
		b.WriteString("Coffre Vault : connexion demandée\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// wizForm construit un formulaire huh configuré selon io.
func wizForm(io WizardIO, groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).
		WithAccessible(io.Accessible).
		WithInput(io.In).
		WithOutput(io.Out)
}

// Wizard lance l'assistant interactif de configuration et retourne une copie
// mise à jour des réponses. Le profil p et les réponses a ne sont pas
// modifiés. Un refus au récapitulatif renvoie ErrCancelled.
func Wizard(p *profile.Profile, a *profile.Answers, versions map[string][]sources.Release, io WizardIO) (*profile.Answers, error) {
	out := *a
	out.Roles = append([]string(nil), a.Roles...)
	out.NoProxy = append([]string(nil), a.NoProxy...)
	out.CACerts = append([]string(nil), a.CACerts...)
	out.Repos = append([]string(nil), a.Repos...)
	out.Versions = make(map[string]string, len(a.Versions))
	for k, v := range a.Versions {
		out.Versions[k] = v
	}

	// 1/9 · Profil
	profileGroup := huh.NewGroup(
		huh.NewNote().
			Title(wizBreadcrumb(1)).
			Description(fmt.Sprintf("Profil : %s", p.Name)),
	)
	if err := wizForm(io, profileGroup).Run(); err != nil {
		return nil, err
	}

	// 2/9 · Rôle
	roleGroup := huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title(wizBreadcrumb(2)).
			Options(
				huh.NewOption("Front", "front"),
				huh.NewOption("Back", "back"),
				huh.NewOption("Fullstack", "fullstack"),
				huh.NewOption("Python", "python"),
			).
			Value(&out.Roles).
			Validate(func(v []string) error {
				if len(v) == 0 {
					return errors.New("sélectionnez au moins un rôle")
				}
				return nil
			}),
	)
	if err := wizForm(io, roleGroup).Run(); err != nil {
		return nil, err
	}

	// 3/9 · Éditeur
	editorGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title(wizBreadcrumb(3)).
			Options(
				huh.NewOption("VS Code", "vscode"),
				huh.NewOption("IntelliJ", "intellij"),
				huh.NewOption("Les deux", "both"),
			).
			Value(&out.Editor),
	)
	if err := wizForm(io, editorGroup).Run(); err != nil {
		return nil, err
	}

	// 4/9 · Versions
	var versionFields []huh.Field
	for _, tool := range wizToolsFor(out.Roles) {
		rs := versions[tool]
		if len(rs) == 0 {
			continue
		}
		val := out.Versions[tool]
		versionFields = append(versionFields, huh.NewSelect[string]().
			Title(tool).
			Options(wizVersionOptions(rs)...).
			Value(&val))
	}
	if len(versionFields) > 0 {
		versionGroup := huh.NewGroup(versionFields...).Title(wizBreadcrumb(4))
		if err := wizForm(io, versionGroup).Run(); err != nil {
			return nil, err
		}
		// Relit les valeurs choisies.
		for _, tool := range wizToolsFor(out.Roles) {
			rs := versions[tool]
			if len(rs) == 0 {
				continue
			}
			if out.Versions[tool] == "" {
				out.Versions[tool] = rs[0].Version
			}
		}
	}

	// 5/9 · Réseau
	var noProxyText string
	networkGroup := huh.NewGroup(
		huh.NewInput().
			Title(wizBreadcrumb(5)).
			Description("Proxy").
			Value(&out.Proxy),
		huh.NewSelect[string]().
			Title("Authentification proxy").
			Options(
				huh.NewOption("Aucune", "none"),
				huh.NewOption("Kerberos", "kerberos"),
			).
			Value(&out.ProxyAuth),
		huh.NewInput().
			Title("Sans proxy").
			Description("Hôtes séparés par des virgules").
			Value(&noProxyText),
	)
	if err := wizForm(io, networkGroup).Run(); err != nil {
		return nil, err
	}
	out.NoProxy = wizSplitList(noProxyText)

	// 6/9 · Git
	gitGroup := huh.NewGroup(
		huh.NewInput().
			Title(wizBreadcrumb(6)).
			Description("Nom Git").
			Value(&out.GitName),
		huh.NewInput().
			Title("Email Git").
			Value(&out.GitEmail),
		huh.NewConfirm().
			Title("Générer une clé SSH ?").
			Value(&out.GenerateSSH),
	)
	if err := wizForm(io, gitGroup).Run(); err != nil {
		return nil, err
	}

	// 7/9 · Dépôts
	repos := p.ReposFor(out.Roles)
	if len(repos) > 0 {
		repoOpts := make([]huh.Option[string], 0, len(repos))
		for _, r := range repos {
			repoOpts = append(repoOpts, huh.NewOption(r.Name, r.Name))
		}
		selected := append([]string(nil), out.Repos...)
		repoGroup := huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(wizBreadcrumb(7)).
				Options(repoOpts...).
				Value(&selected),
			huh.NewInput().
				Title("Dossier de travail").
				Value(&out.Workspace),
		)
		if err := wizForm(io, repoGroup).Run(); err != nil {
			return nil, err
		}
		out.Repos = selected
	}

	// 8/9 · Conteneurs
	containerFields := []huh.Field{
		huh.NewConfirm().
			Title(wizBreadcrumb(8)).
			Description("Installer Podman ?").
			Value(&out.Podman),
	}
	if p.Vault.Addr != "" {
		containerFields = append(containerFields, huh.NewConfirm().
			Title("Se connecter au coffre Vault ?").
			Value(&out.Vault))
	}
	if err := wizForm(io, huh.NewGroup(containerFields...)).Run(); err != nil {
		return nil, err
	}

	// 9/9 · Récapitulatif
	confirmed := true
	summaryGroup := huh.NewGroup(
		huh.NewNote().
			Title(wizBreadcrumb(9)).
			Description(wizSummary(&out)),
		huh.NewConfirm().
			Title("Lancer l'installation ?").
			Value(&confirmed),
	)
	if err := wizForm(io, summaryGroup).Run(); err != nil {
		return nil, err
	}
	if err := wizConfirmed(confirmed); err != nil {
		return nil, err
	}

	return &out, nil
}

// wizSplitList découpe une liste séparée par des virgules en retirant les
// espaces et les entrées vides.
func wizSplitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// AskToken demande un token GitHub facultatif. Le champ est masqué et un
// champ vide est accepté.
func AskToken(io WizardIO) (string, error) {
	var token string
	group := huh.NewGroup(
		huh.NewInput().
			Title("Token GitHub (facultatif, pour pousser)").
			EchoMode(huh.EchoModePassword).
			Value(&token),
	)
	if err := wizForm(io, group).Run(); err != nil {
		return "", err
	}
	return token, nil
}
