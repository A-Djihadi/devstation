package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/sources"
)

// containersPodmanExe est le nom de l'exécutable Podman.
const containersPodmanExe = "podman.exe"

// containersComposeExe est le nom sous lequel Compose est installé pour que
// `podman compose` le trouve.
const containersComposeExe = "docker-compose.exe"

// containersDefaultBinDir est le sous-dossier d'exécutables par défaut de
// Podman, utilisé quand Install n'a pas encore tourné dans ce processus.
const containersDefaultBinDir = "usr\\bin"

// containersDefaultRetryWait est l'attente par défaut entre deux tentatives de
// démarrage de la machine Podman.
const containersDefaultRetryWait = 10 * time.Second

// containersStartAttempts est le nombre maximal de tentatives de démarrage.
const containersStartAttempts = 3

// Podman installe Podman, Docker Compose et initialise la machine WSL.
type Podman struct {
	// Compose décrit la release de Docker Compose à installer.
	Compose sources.Release
	// CACerts liste les chemins de certificats PEM à ajouter à la machine.
	CACerts []string
	// RegistryMirror est le miroir de registre à configurer (vide = aucun).
	RegistryMirror string

	// binDir est le dossier des exécutables relatif au dossier de version,
	// mémorisé par Install.
	binDir string
	// retryWait est l'attente entre deux tentatives de démarrage de la
	// machine ; 0 signifie containersDefaultRetryWait.
	retryWait time.Duration
}

var _ Installer = (*Podman)(nil)

// NewPodman crée un installeur Podman.
func NewPodman(compose sources.Release, caCerts []string, mirror string) *Podman {
	return &Podman{Compose: compose, CACerts: caCerts, RegistryMirror: mirror}
}

// Tool retourne le nom de l'outil géré.
func (p *Podman) Tool() string { return "podman" }

// containersFindExe cherche le dossier contenant name sous root et retourne son
// chemin relatif à root.
func containersFindExe(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), name) {
			rel, relErr := filepath.Rel(root, filepath.Dir(path))
			if relErr != nil {
				return relErr
			}
			found = rel
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s introuvable dans %q", name, root)
	}
	return found, nil
}

// containersRun exécute une commande et retourne une erreur contenant Stderr
// si le code de sortie est non nul.
func containersRun(ctx context.Context, d Deps, c execx.Cmd) (execx.Result, error) {
	if d.Run == nil {
		return execx.Result{}, errors.New("exécuteur de commandes manquant")
	}
	res, err := d.Run.Run(ctx, c)
	if err != nil {
		return res, err
	}
	if res.Code != 0 {
		return res, fmt.Errorf("échec de %q (code %d) : %s", c.String(), res.Code, strings.TrimSpace(res.Stderr))
	}
	return res, nil
}

// containersMachineSSH écrit data dans path sur la machine Podman via tee.
func containersMachineSSH(ctx context.Context, d Deps, podmanExe, path, data string) error {
	_, err := containersRun(ctx, d, execx.Cmd{
		Name:  podmanExe,
		Args:  []string{"machine", "ssh", "--", "sudo", "tee", path},
		Stdin: data,
	})
	return err
}

// containersMachineRunning indique si au moins une machine Podman est
// actuellement démarrée.
func containersMachineRunning(ctx context.Context, d Deps, podmanExe string) (bool, error) {
	res, err := containersRun(ctx, d, execx.Cmd{
		Name: podmanExe,
		Args: []string{"machine", "list", "--format", "json"},
	})
	if err != nil {
		return false, err
	}
	var machines []struct {
		Running bool `json:"Running"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &machines); err != nil {
		return false, fmt.Errorf("lecture de la liste des machines Podman : %w", err)
	}
	for _, m := range machines {
		if m.Running {
			return true, nil
		}
	}
	return false, nil
}

// containersStartMachine démarre la machine Podman en réessayant jusqu'à
// containersStartAttempts fois, avec un arrêt et une attente entre deux
// tentatives. Elle retourne la dernière erreur rencontrée.
func containersStartMachine(ctx context.Context, d Deps, podmanExe string, wait time.Duration) error {
	if wait <= 0 {
		wait = containersDefaultRetryWait
	}
	var lastErr error
	for attempt := 0; attempt < containersStartAttempts; attempt++ {
		if attempt > 0 {
			// Arrêt best-effort : l'erreur est ignorée.
			_, _ = d.Run.Run(ctx, execx.Cmd{Name: podmanExe, Args: []string{"machine", "stop"}})
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if _, err := containersRun(ctx, d, execx.Cmd{Name: podmanExe, Args: []string{"machine", "start"}}); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// Install installe Podman, copie Compose et prépare la machine WSL.
func (p *Podman) Install(ctx context.Context, d Deps, r sources.Release, report Report) error {
	dir, err := InstallZip(ctx, d, "podman", r, true, report)
	if err != nil {
		return err
	}

	binDir, err := containersFindExe(dir, containersPodmanExe)
	if err != nil {
		return err
	}
	p.binDir = binDir
	podmanExe := filepath.Join(dir, binDir, containersPodmanExe)

	if p.Compose.URL != "" {
		if d.DL == nil {
			return errors.New("client de téléchargement manquant")
		}
		composePath, _, err := d.DL.Fetch(ctx, p.Compose.URL, p.Compose.Checksum, nil)
		if err != nil {
			return err
		}
		if err := containersCopyFile(composePath, filepath.Join(dir, binDir, containersComposeExe)); err != nil {
			return err
		}
	}

	if _, err := containersRun(ctx, d, execx.Cmd{Name: "wsl.exe", Args: []string{"--status"}}); err != nil {
		return errors.New("WSL2 est requis pour Podman : voir le README")
	}

	res, err := containersRun(ctx, d, execx.Cmd{
		Name: podmanExe,
		Args: []string{"machine", "list", "--format", "json"},
	})
	if err != nil {
		return err
	}
	var machines []json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &machines); err != nil {
		return fmt.Errorf("lecture de la liste des machines Podman : %w", err)
	}
	if len(machines) == 0 {
		if _, err := containersRun(ctx, d, execx.Cmd{
			Name: podmanExe,
			Args: []string{"machine", "init"},
		}); err != nil {
			return err
		}
	}

	running, err := containersMachineRunning(ctx, d, podmanExe)
	if err != nil {
		return err
	}
	if !running {
		if err := containersStartMachine(ctx, d, podmanExe, p.retryWait); err != nil {
			return fmt.Errorf("la machine Podman ne démarre pas : %w\nEssayez : wsl --shutdown, puis relancez devstation setup (Docker Desktop peut aussi bloquer le premier démarrage : fermez-le le temps de l'installation).", err)
		}
	}

	for i, cert := range p.CACerts {
		data, err := os.ReadFile(cert)
		if err != nil {
			return fmt.Errorf("lecture du certificat %q : %w", cert, err)
		}
		target := fmt.Sprintf("/etc/pki/ca-trust/source/anchors/devstation-%d.pem", i)
		if err := containersMachineSSH(ctx, d, podmanExe, target, string(data)); err != nil {
			return err
		}
	}
	if len(p.CACerts) > 0 {
		if _, err := containersRun(ctx, d, execx.Cmd{
			Name: podmanExe,
			Args: []string{"machine", "ssh", "--", "sudo", "update-ca-trust"},
		}); err != nil {
			return err
		}
	}

	if p.RegistryMirror != "" {
		conf := "[[registry]]\nlocation = \"docker.io\"\n\n[[registry.mirror]]\nlocation = \"" + p.RegistryMirror + "\"\n"
		if err := containersMachineSSH(ctx, d, podmanExe, "/etc/containers/registries.conf.d/devstation.conf", conf); err != nil {
			return err
		}
	}

	return nil
}

// containersCopyFile copie src vers dst.
func containersCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("ouverture de %q : %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("création de %q : %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copie vers %q : %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("fermeture de %q : %w", dst, err)
	}
	return nil
}

// Activate active la version donnée de Podman.
func (p *Podman) Activate(d Deps, version string) error {
	binDir := p.binDir
	if binDir == "" {
		binDir = containersDefaultBinDir
	}
	return SetCurrent(d, "podman", version, []string{binDir})
}

// Verify retourne la version de Podman effectivement disponible.
func (p *Podman) Verify(ctx context.Context, d Deps) (string, error) {
	exe := filepath.Join(CurrentDir(d, "podman"), p.binDir, containersPodmanExe)
	res, err := containersRun(ctx, d, execx.Cmd{Name: exe, Args: []string{"--version"}})
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(res.Stdout))
	if len(fields) == 0 {
		return "", errors.New("sortie de podman --version vide")
	}
	return fields[len(fields)-1], nil
}
