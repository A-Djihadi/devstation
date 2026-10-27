//go:build windows

package winenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/A-Djihadi/devstation/internal/execx"
)

// Constantes de l'API Win32 utilisées pour diffuser un changement d'environnement.
const (
	hwndBroadcast     = 0xFFFF
	wmSettingChange   = 0x001A
	smtoAbortIfHung   = 0x0002
	broadcastTimeout  = 5000
	envRegistrySubkey = `Environment`
	machineEnvSubkey  = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// NewEnv retourne l'implémentation réelle de Env basée sur le registre Windows.
func NewEnv() Env {
	return &windowsEnv{}
}

// NewLinks retourne l'implémentation réelle de Links basée sur mklink /J.
func NewLinks(r execx.Runner) Links {
	return &windowsLinks{runner: r}
}

// windowsEnv implémente Env via HKCU\Environment et user32.dll.
type windowsEnv struct{}

// UserVar retourne la valeur d'une variable utilisateur et indique si elle existe.
func (windowsEnv) UserVar(name string) (string, bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, envRegistrySubkey, registry.QUERY_VALUE)
	if err != nil {
		return "", false, err
	}
	defer k.Close()
	val, _, err := k.GetStringValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	return val, true, nil
}

// SetUserVar définit une variable utilisateur dans HKCU\Environment.
func (windowsEnv) SetUserVar(name, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envRegistrySubkey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if strings.Contains(value, "%") || strings.EqualFold(name, "Path") {
		return k.SetExpandStringValue(name, value)
	}
	return k.SetStringValue(name, value)
}

// DeleteUserVar supprime une variable utilisateur ; une variable absente n'est pas une erreur.
func (windowsEnv) DeleteUserVar(name string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envRegistrySubkey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	err = k.DeleteValue(name)
	if err != nil && errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

// MachinePath retourne la valeur du PATH machine en lecture seule.
func (windowsEnv) MachinePath() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, machineEnvSubkey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	val, _, err := k.GetStringValue("Path")
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// Broadcast notifie Windows d'un changement d'environnement.
func (windowsEnv) Broadcast() error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("SendMessageTimeoutW")
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return err
	}
	var result uintptr
	r1, _, callErr := proc.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(smtoAbortIfHung),
		uintptr(broadcastTimeout),
		uintptr(unsafe.Pointer(&result)),
	)
	if r1 == 0 {
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return callErr
		}
		return errors.New("échec de la diffusion du changement d'environnement")
	}
	return nil
}

// windowsLinks implémente Links via mklink /J et os.Readlink.
type windowsLinks struct {
	runner execx.Runner
}

// SetJunction crée ou remplace une jonction de link vers target.
func (l *windowsLinks) SetJunction(link, target string) error {
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	res, err := l.runner.Run(context.Background(), execx.Cmd{
		Name: "cmd",
		Args: []string{"/c", "mklink", "/J", link, target},
	})
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("mklink /J a échoué (code %d) : %s", res.Code, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// ReadJunction retourne la cible d'une jonction.
func (windowsLinks) ReadJunction(link string) (string, error) {
	return os.Readlink(link)
}
