package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/A-Djihadi/devstation/internal/config"
	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// VaultCLI installe et active le client HashiCorp Vault ainsi que son
// token helper fourni par DevStation.
type VaultCLI struct {
	// Addr est l'adresse du serveur Vault (VAULT_ADDR).
	Addr string
	// CACert est le chemin du certificat d'autorité (VAULT_CACERT).
	CACert string
	// BinDir est le dossier contenant devstation.exe.
	BinDir string
	// HomeDir est le dossier utilisateur où écrire .vault.
	HomeDir string
}

var _ Installer = (*VaultCLI)(nil)

// vaultcliHelperName est le nom du script de token helper écrit dans BinDir.
const vaultcliHelperName = "vault-token-helper.cmd"

// vaultcliVersionRe extrait le numéro de version de la sortie de "vault version".
var vaultcliVersionRe = regexp.MustCompile(`Vault v([0-9][0-9A-Za-z.\-]*)`)

// NewVaultCLI crée un installeur du client Vault.
func NewVaultCLI(addr, caCert, binDir, homeDir string) *VaultCLI {
	return &VaultCLI{Addr: addr, CACert: caCert, BinDir: binDir, HomeDir: homeDir}
}

// Tool retourne le nom de l'outil géré.
func (v *VaultCLI) Tool() string {
	return "vault"
}

// vaultcliHelperPath retourne le chemin du script de token helper.
func (v *VaultCLI) vaultcliHelperPath() string {
	return filepath.Join(v.BinDir, vaultcliHelperName)
}

// vaultcliConfigPath retourne le chemin du fichier de configuration Vault.
func (v *VaultCLI) vaultcliConfigPath() string {
	return filepath.Join(v.HomeDir, ".vault")
}

// vaultcliHelperContent retourne le contenu exact du script de token helper.
func vaultcliHelperContent() string {
	return `@"%~dp0devstation.exe" vault-token-helper %*` + "\r\n"
}

// vaultcliEscapeBackslashes double chaque antislash pour le format HCL.
func vaultcliEscapeBackslashes(p string) string {
	return strings.ReplaceAll(p, `\`, `\\`)
}

// vaultcliConfigContent retourne le contenu du fichier .vault.
func vaultcliConfigContent(helperPath string) string {
	return "token_helper = \"" + vaultcliEscapeBackslashes(helperPath) + "\"\n"
}

// Install installe la version r du client Vault, écrit le token helper et
// configure le fichier .vault.
func (v *VaultCLI) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	if _, err := InstallZip(ctx, d, "vault", r, false, report); err != nil {
		return err
	}

	if v.BinDir == "" {
		return errors.New("dossier des exécutables manquant")
	}
	if err := os.MkdirAll(v.BinDir, 0o755); err != nil {
		return fmt.Errorf("création de %q : %w", v.BinDir, err)
	}
	helper := v.vaultcliHelperPath()
	if err := os.WriteFile(helper, []byte(vaultcliHelperContent()), 0o644); err != nil {
		return fmt.Errorf("écriture de %q : %w", helper, err)
	}

	if v.HomeDir == "" {
		return errors.New("dossier utilisateur manquant")
	}
	if err := os.MkdirAll(v.HomeDir, 0o755); err != nil {
		return fmt.Errorf("création de %q : %w", v.HomeDir, err)
	}
	cfgPath := v.vaultcliConfigPath()
	if d.State != nil {
		if err := config.BackupOnce(cfgPath, d.State); err != nil {
			return fmt.Errorf("sauvegarde de %q : %w", cfgPath, err)
		}
	}
	if err := os.WriteFile(cfgPath, []byte(vaultcliConfigContent(helper)), 0o644); err != nil {
		return fmt.Errorf("écriture de %q : %w", cfgPath, err)
	}

	return nil
}

// Activate active la version donnée et définit les variables d'environnement
// Vault non vides.
func (v *VaultCLI) Activate(d Deps, version string) error {
	if err := SetCurrent(d, "vault", version, []string{""}); err != nil {
		return err
	}
	if d.Env == nil {
		return errors.New("gestion de l'environnement manquante")
	}
	if v.Addr != "" {
		if err := d.Env.SetUserVar("VAULT_ADDR", v.Addr); err != nil {
			return fmt.Errorf("écriture de VAULT_ADDR : %w", err)
		}
	}
	if v.CACert != "" {
		if err := d.Env.SetUserVar("VAULT_CACERT", v.CACert); err != nil {
			return fmt.Errorf("écriture de VAULT_CACERT : %w", err)
		}
	}
	return nil
}

// Verify retourne la version du client Vault effectivement disponible.
func (v *VaultCLI) Verify(ctx context.Context, d Deps) (string, error) {
	if d.Run == nil {
		return "", errors.New("exécution de commandes manquante")
	}
	exe := filepath.Join(CurrentDir(d, "vault"), "vault.exe")
	res, err := d.Run.Run(ctx, execx.Cmd{Name: exe, Args: []string{"version"}})
	if err != nil {
		return "", fmt.Errorf("exécution de %q : %w", exe, err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("échec de %q (code %d) : %s", exe, res.Code, strings.TrimSpace(res.Stderr))
	}
	m := vaultcliVersionRe.FindStringSubmatch(res.Stdout)
	if m == nil {
		return "", fmt.Errorf("version illisible dans la sortie de %q : %q", exe, strings.TrimSpace(res.Stdout))
	}
	return m[1], nil
}
