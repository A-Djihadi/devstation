// Package vaultx fournit un client minimal pour HashiCorp Vault : helper de
// token (protocole token helper), connexion interactive et lecture de secrets
// via la CLI `vault`.
package vaultx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// TokenTarget est la cible du Gestionnaire d'identifiants Windows où est
// stocké le token Vault.
const TokenTarget = "DevStation/vault-token"

// TokenHelper implémente le protocole token helper de Vault. Le binaire vault
// appelle le helper avec l'argument "get", "store" ou "erase" :
//   - "get" écrit le token stocké sur stdout, sans saut de ligne ; si aucun
//     token n'est stocké, rien n'est écrit et la fonction renvoie nil ;
//   - "store" lit le token sur stdin et l'enregistre ;
//   - "erase" supprime le token stocké.
//
// Le token n'apparaît jamais dans un message d'erreur.
func TokenHelper(args []string, stdin io.Reader, stdout io.Writer, c winenv.Creds) error {
	if len(args) == 0 {
		return errors.New("token helper : action manquante (get, store ou erase)")
	}
	switch args[0] {
	case "get":
		_, secret, err := c.Get(TokenTarget)
		if err != nil {
			if errors.Is(err, winenv.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("token helper : lecture du token impossible : %w", err)
		}
		if _, err := io.WriteString(stdout, secret); err != nil {
			return fmt.Errorf("token helper : écriture du token impossible : %w", err)
		}
		return nil
	case "store":
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("token helper : lecture de l'entrée impossible : %w", err)
		}
		token := strings.TrimRight(string(data), " \t\r\n")
		if err := c.Set(TokenTarget, "vault", token); err != nil {
			return fmt.Errorf("token helper : enregistrement du token impossible : %w", err)
		}
		return nil
	case "erase":
		if err := c.Delete(TokenTarget); err != nil {
			return fmt.Errorf("token helper : suppression du token impossible : %w", err)
		}
		return nil
	default:
		return fmt.Errorf("token helper : action inconnue %q", args[0])
	}
}

// Login ouvre une session Vault via la CLI. La commande est interactive : la
// méthode OIDC ouvre le navigateur et la méthode token demande le token sur la
// console, jamais en argument. role est facultatif.
func Login(ctx context.Context, r execx.Runner, vaultExe, method, role string) error {
	args := []string{"login", "-method=" + method}
	if role != "" {
		args = append(args, "role="+role)
	}
	res, err := r.Run(ctx, execx.Cmd{Name: vaultExe, Args: args, Interactive: true})
	if err != nil {
		return fmt.Errorf("connexion à Vault impossible : %s", stderrOr(res, err))
	}
	if res.Code != 0 {
		return fmt.Errorf("connexion à Vault impossible : %s", stderrOr(res, err))
	}
	return nil
}

// List retourne les entrées d'un chemin du moteur KV de Vault.
func List(ctx context.Context, r execx.Runner, vaultExe, path string) ([]string, error) {
	res, err := r.Run(ctx, execx.Cmd{Name: vaultExe, Args: []string{"kv", "list", "-format=json", path}})
	if err != nil {
		return nil, fmt.Errorf("liste des secrets impossible : %s", stderrOr(res, err))
	}
	if res.Code != 0 {
		return nil, fmt.Errorf("liste des secrets impossible : %s", stderrOr(res, err))
	}
	var keys []string
	if err := json.Unmarshal([]byte(res.Stdout), &keys); err != nil {
		return nil, fmt.Errorf("liste des secrets illisible : %w", err)
	}
	return keys, nil
}

// Get retourne la valeur d'un champ d'un secret. La valeur n'est jamais
// journalisée ni incluse dans un message d'erreur.
func Get(ctx context.Context, r execx.Runner, vaultExe, path, field string) (string, error) {
	res, err := r.Run(ctx, execx.Cmd{Name: vaultExe, Args: []string{"kv", "get", "-field=" + field, path}})
	if err != nil {
		return "", fmt.Errorf("lecture du secret %s impossible : %s", path, stderrOr(res, err))
	}
	if res.Code != 0 {
		return "", fmt.Errorf("lecture du secret %s impossible : %s", path, stderrOr(res, err))
	}
	return strings.TrimRight(res.Stdout, " \t\r\n"), nil
}

// TTL retourne la durée de vie restante du token Vault courant.
func TTL(ctx context.Context, r execx.Runner, vaultExe string) (time.Duration, error) {
	res, err := r.Run(ctx, execx.Cmd{Name: vaultExe, Args: []string{"token", "lookup", "-format=json"}})
	if err != nil {
		return 0, fmt.Errorf("lecture de la durée de vie du token impossible : %s", stderrOr(res, err))
	}
	if res.Code != 0 {
		return 0, fmt.Errorf("lecture de la durée de vie du token impossible : %s", stderrOr(res, err))
	}
	var payload struct {
		Data struct {
			TTL int64 `json:"ttl"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		return 0, fmt.Errorf("réponse de Vault illisible : %w", err)
	}
	return time.Duration(payload.Data.TTL) * time.Second, nil
}

// stderrOr retourne la sortie d'erreur de la commande, ou le message de err si
// elle est vide.
func stderrOr(res execx.Result, err error) string {
	if s := strings.TrimSpace(res.Stderr); s != "" {
		return s
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("code de sortie %d", res.Code)
}
