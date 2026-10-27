// Package execx fournit une abstraction d'exécution de commandes externes,
// testable via une implémentation factice.
package execx

import (
	"context"
	"os/exec"
	"strings"
)

// Cmd décrit une commande à exécuter.
type Cmd struct {
	// Name est le nom ou le chemin de l'exécutable.
	Name string
	// Args sont les arguments passés à l'exécutable.
	Args []string
	// Dir est le répertoire de travail (vide = répertoire courant).
	Dir string
	// Env contient des variables d'environnement additionnelles au format "CLE=valeur".
	Env []string
	// Stdin est le contenu envoyé sur l'entrée standard.
	Stdin string
	// Secret indique que Stdin et Args ne doivent jamais être journalisés.
	Secret bool
}

// Result contient le résultat d'une exécution de commande.
type Result struct {
	// Stdout est la sortie standard capturée.
	Stdout string
	// Stderr est la sortie d'erreur capturée.
	Stderr string
	// Code est le code de sortie du processus.
	Code int
}

// Runner exécute des commandes.
type Runner interface {
	// Run exécute la commande c et retourne son résultat.
	Run(ctx context.Context, c Cmd) (Result, error)
}

// OS retourne un Runner qui exécute réellement les commandes du système.
func OS() Runner {
	return osRunner{}
}

// osRunner est l'implémentation réelle basée sur os/exec.
type osRunner struct{}

// Run exécute la commande via os/exec et capture ses sorties.
func (osRunner) Run(ctx context.Context, c Cmd) (Result, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.Env...)
	}
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.Code = cmd.ProcessState.ExitCode()
	}
	return res, err
}

// Fake est un Runner factice qui enregistre les appels et renvoie des réponses prédéfinies.
type Fake struct {
	// Calls contient les commandes reçues, dans l'ordre d'appel.
	Calls []Cmd
	// Responses associe une clé (Name + " " + strings.Join(Args, " ")) à un résultat.
	Responses map[string]Result
}

// Run enregistre la commande et retourne la réponse associée, ou un résultat vide.
func (f *Fake) Run(ctx context.Context, c Cmd) (Result, error) {
	f.Calls = append(f.Calls, c)
	if f.Responses != nil {
		if r, ok := f.Responses[c.key()]; ok {
			return r, nil
		}
	}
	return Result{}, nil
}

// key retourne la clé de recherche dans Responses.
func (c Cmd) key() string {
	return c.Name + " " + strings.Join(c.Args, " ")
}

// String retourne une forme journalisable de la commande. Les arguments
// contenant des espaces sont entourés de guillemets. Si Secret est vrai,
// les arguments sont remplacés par "***".
func (c Cmd) String() string {
	if c.Secret {
		return c.Name + " ***"
	}
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, c.Name)
	for _, a := range c.Args {
		if strings.ContainsAny(a, " \t") {
			parts = append(parts, `"`+a+`"`)
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}