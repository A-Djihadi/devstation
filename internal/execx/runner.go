// Package execx fournit une abstraction d'exécution de commandes externes,
// testable via une implémentation factice.
package execx

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	// Interactive relie la commande directement au terminal du processus
	// (os.Stdin, os.Stdout, os.Stderr) au lieu de capturer ses sorties.
	// Sert aux commandes qui attendent une saisie sur la console.
	Interactive bool
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

// Run exécute la commande via os/exec et capture ses sorties. Si la commande
// est Interactive, elle est reliée directement au terminal du processus et
// ses sorties ne sont pas capturées.
func (osRunner) Run(ctx context.Context, c Cmd) (Result, error) {
	if c.Interactive && c.Stdin != "" {
		return Result{}, errors.New("commande interactive : Stdin ne peut pas être fourni")
	}
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.Env...)
	}
	var stdout, stderr strings.Builder
	if c.Interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		if c.Stdin != "" {
			cmd.Stdin = strings.NewReader(c.Stdin)
		}
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.Code = cmd.ProcessState.ExitCode()
	}
	if err != nil && !c.Interactive && res.Stderr != "" {
		if extrait := StderrExtract(res.Stderr); extrait != "" {
			err = fmt.Errorf("%w : %s", err, extrait)
		}
	}
	return res, err
}

// StderrExtract retourne un extrait lisible de stderr : les 20 dernières
// lignes non vides, limité à 2000 caractères.
func StderrExtract(stderr string) string {
	lines := strings.Split(stderr, "\n")
	kept := make([]string, 0, 20)
	for i := len(lines) - 1; i >= 0 && len(kept) < 20; i-- {
		l := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		kept = append(kept, l)
	}
	// Remet dans l'ordre d'origine.
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	out := strings.Join(kept, "\n")
	if len(out) > 2000 {
		out = out[len(out)-2000:]
	}
	return out
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
// les arguments sont remplacés par "***". Une commande interactive est
// suffixée par " (interactif)".
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
	s := strings.Join(parts, " ")
	if c.Interactive {
		s += " (interactif)"
	}
	return s
}
