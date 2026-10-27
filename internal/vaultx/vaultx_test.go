package vaultx

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/winenv"
)

// TestTokenHelperGetMissingIsEmpty vérifie que "get" sans token n'écrit rien et ne renvoie pas d'erreur.
func TestTokenHelperGetMissingIsEmpty(t *testing.T) {
	c := winenv.NewFake()
	var out bytes.Buffer
	if err := TokenHelper([]string{"get"}, strings.NewReader(""), &out, c); err != nil {
		t.Fatalf("TokenHelper(get) : %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("sortie attendue vide, obtenu %q", out.String())
	}
}

// TestTokenHelperStoreThenGet vérifie l'aller-retour "store" puis "get".
func TestTokenHelperStoreThenGet(t *testing.T) {
	c := winenv.NewFake()
	if err := TokenHelper([]string{"store"}, strings.NewReader("s.abc123\n"), &bytes.Buffer{}, c); err != nil {
		t.Fatalf("TokenHelper(store) : %v", err)
	}
	user, secret, err := c.Get(TokenTarget)
	if err != nil {
		t.Fatalf("Get : %v", err)
	}
	if user != "vault" || secret != "s.abc123" {
		t.Fatalf("identifiants inattendus : %q %q", user, secret)
	}
	var out bytes.Buffer
	if err := TokenHelper([]string{"get"}, strings.NewReader(""), &out, c); err != nil {
		t.Fatalf("TokenHelper(get) : %v", err)
	}
	if out.String() != "s.abc123" {
		t.Fatalf("token attendu %q, obtenu %q", "s.abc123", out.String())
	}
}

// TestTokenHelperErase vérifie que "erase" supprime le token stocké.
func TestTokenHelperErase(t *testing.T) {
	c := winenv.NewFake()
	if err := c.Set(TokenTarget, "vault", "s.abc123"); err != nil {
		t.Fatalf("Set : %v", err)
	}
	if err := TokenHelper([]string{"erase"}, strings.NewReader(""), &bytes.Buffer{}, c); err != nil {
		t.Fatalf("TokenHelper(erase) : %v", err)
	}
	if _, _, err := c.Get(TokenTarget); err == nil {
		t.Fatal("token attendu supprimé")
	}
}

// TestLoginIsInteractive vérifie que la connexion est interactive et transmet le rôle.
func TestLoginIsInteractive(t *testing.T) {
	r := &execx.Fake{}
	if err := Login(context.Background(), r, "vault.exe", "oidc", "demo"); err != nil {
		t.Fatalf("Login : %v", err)
	}
	if len(r.Calls) != 1 {
		t.Fatalf("1 appel attendu, obtenu %d", len(r.Calls))
	}
	call := r.Calls[0]
	if !call.Interactive {
		t.Fatal("commande attendue interactive")
	}
	if call.Name != "vault.exe" {
		t.Fatalf("exécutable attendu vault.exe, obtenu %q", call.Name)
	}
	joined := strings.Join(call.Args, " ")
	if !strings.Contains(joined, "-method=oidc") {
		t.Fatalf("méthode absente des arguments : %q", joined)
	}
	if !strings.Contains(joined, "role=demo") {
		t.Fatalf("rôle absent des arguments : %q", joined)
	}
}

// TestListParsesJSON vérifie le décodage de la liste JSON renvoyée par Vault.
func TestListParsesJSON(t *testing.T) {
	r := &execx.Fake{Responses: map[string]execx.Result{
		"vault.exe kv list -format=json secret/apps": {Stdout: `["api","web"]`},
	}}
	keys, err := List(context.Background(), r, "vault.exe", "secret/apps")
	if err != nil {
		t.Fatalf("List : %v", err)
	}
	if len(keys) != 2 || keys[0] != "api" || keys[1] != "web" {
		t.Fatalf("liste inattendue : %v", keys)
	}
}

// TestGetTrimsValue vérifie que la valeur lue est débarrassée des espaces de fin.
func TestGetTrimsValue(t *testing.T) {
	r := &execx.Fake{Responses: map[string]execx.Result{
		"vault.exe kv get -field=password secret/apps/api": {Stdout: "s3cr3t\n"},
	}}
	v, err := Get(context.Background(), r, "vault.exe", "secret/apps/api", "password")
	if err != nil {
		t.Fatalf("Get : %v", err)
	}
	if v != "s3cr3t" {
		t.Fatalf("valeur attendue %q, obtenue %q", "s3cr3t", v)
	}
}

// TestTTLParsesSeconds vérifie le décodage du champ data.ttl en secondes.
func TestTTLParsesSeconds(t *testing.T) {
	r := &execx.Fake{Responses: map[string]execx.Result{
		"vault.exe token lookup -format=json": {Stdout: `{"data":{"ttl":3600}}`},
	}}
	d, err := TTL(context.Background(), r, "vault.exe")
	if err != nil {
		t.Fatalf("TTL : %v", err)
	}
	if d != time.Hour {
		t.Fatalf("durée attendue %v, obtenue %v", time.Hour, d)
	}
}
