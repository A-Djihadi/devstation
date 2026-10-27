// Package network fournit les utilitaires réseau de DevStation : configuration
// du relais proxy px et vérification de sa disponibilité.
package network

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/A-Djihadi/devstation/internal/config"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/state"
)

// pxDefaultPort est le port d'écoute par défaut du relais px.
const pxDefaultPort = 3128

// PxConfig décrit la configuration du relais proxy px.
type PxConfig struct {
	// Upstream est l'URL du proxy amont (schéma optionnel).
	Upstream string
	// NoProxy liste les hôtes exclus du proxy.
	NoProxy []string
	// Port est le port d'écoute local ; 0 signifie le port par défaut.
	Port int
	// Gateway active l'écoute sur toutes les interfaces pour les invités.
	Gateway bool
	// Allow liste les adresses autorisées lorsque Gateway est actif.
	Allow string
}

// pxStripScheme retire le schéma d'une URL de proxy et sa barre oblique finale.
func pxStripScheme(upstream string) string {
	s := strings.TrimSpace(upstream)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return strings.TrimSuffix(s, "/")
}

// WritePxINI écrit la configuration du relais px dans le fichier INI path,
// section [proxy], en enregistrant les clés modifiées dans st.
func WritePxINI(path string, c PxConfig, st *state.State) error {
	port := c.Port
	if port == 0 {
		port = pxDefaultPort
	}

	kv := map[string]string{
		"server":  pxStripScheme(c.Upstream),
		"port":    strconv.Itoa(port),
		"noproxy": strings.Join(c.NoProxy, ","),
	}

	if c.Gateway {
		kv["gateway"] = "1"
		kv["allow"] = c.Allow
	} else {
		kv["listen"] = "127.0.0.1"
	}

	if err := config.SetINI(path, "proxy", kv, st); err != nil {
		return fmt.Errorf("écriture de la configuration px : %w", err)
	}
	return nil
}

// RegisterAutostart enregistre le démarrage automatique de px dans la clé
// HKCU\Software\Microsoft\Windows\CurrentVersion\Run (aucun droit administrateur).
func RegisterAutostart(ctx context.Context, r execx.Runner, pxExe, iniPath string) error {
	if r == nil {
		return fmt.Errorf("exécuteur manquant")
	}
	data := fmt.Sprintf(`"%s" --config="%s"`, pxExe, iniPath)
	c := execx.Cmd{
		Name: "reg.exe",
		Args: []string{
			"add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", "DevStationPx",
			"/t", "REG_SZ",
			"/d", data,
			"/f",
		},
	}
	res, err := r.Run(ctx, c)
	if err != nil {
		return fmt.Errorf("enregistrement du démarrage automatique de px : %w", err)
	}
	if res.Code != 0 {
		return fmt.Errorf("enregistrement du démarrage automatique de px : code %d", res.Code)
	}
	return nil
}

// StartPx lance le relais px en arrière-plan et rend la main immédiatement.
func StartPx(ctx context.Context, r execx.Runner, pxExe, iniPath string) error {
	if r == nil {
		return fmt.Errorf("exécuteur manquant")
	}
	c := execx.Cmd{
		Name: "cmd.exe",
		Args: []string{"/c", "start", "", "/b", pxExe, "--config=" + iniPath},
	}
	res, err := r.Run(ctx, c)
	if err != nil {
		return fmt.Errorf("démarrage de px : %w", err)
	}
	if res.Code != 0 {
		return fmt.Errorf("démarrage de px : code %d", res.Code)
	}
	return nil
}

// PxRunning indique si une connexion TCP vers addr s'ouvre dans le délai donné.
func PxRunning(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
