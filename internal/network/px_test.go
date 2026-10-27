package network

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/A-Djihadi/devstation/internal/execx"
	"github.com/A-Djihadi/devstation/internal/state"
)

// pxReadINI lit le contenu du fichier INI de test.
func pxReadINI(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de %q : %v", path, err)
	}
	return string(data)
}

// pxNewState retourne un état vide utilisable par les tests.
func pxNewState() *state.State {
	return &state.State{Active: map[string]string{}}
}

func TestWritePxINIStripsScheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "px.ini")
	c := PxConfig{
		Upstream: "http://proxy.example.com:8080",
		NoProxy:  []string{"localhost", "*.example.com"},
	}
	if err := WritePxINI(path, c, pxNewState()); err != nil {
		t.Fatalf("WritePxINI : %v", err)
	}
	got := pxReadINI(t, path)
	if !strings.Contains(got, "[proxy]") {
		t.Fatalf("section [proxy] absente :\n%s", got)
	}
	if !strings.Contains(got, "server=proxy.example.com:8080") {
		t.Fatalf("server incorrect :\n%s", got)
	}
	if !strings.Contains(got, "port=3128") {
		t.Fatalf("port par défaut attendu :\n%s", got)
	}
	if !strings.Contains(got, "noproxy=localhost,*.example.com") {
		t.Fatalf("noproxy incorrect :\n%s", got)
	}
	if !strings.Contains(got, "listen=127.0.0.1") {
		t.Fatalf("listen attendu :\n%s", got)
	}
	if strings.Contains(got, "gateway") {
		t.Fatalf("gateway ne doit pas être présent :\n%s", got)
	}
}

func TestWritePxINIGateway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "px.ini")
	c := PxConfig{
		Upstream: "proxy.example.com:8080",
		Port:     8080,
		Gateway:  true,
		Allow:    "172.16.0.0/12",
	}
	if err := WritePxINI(path, c, pxNewState()); err != nil {
		t.Fatalf("WritePxINI : %v", err)
	}
	got := pxReadINI(t, path)
	if !strings.Contains(got, "gateway=1") {
		t.Fatalf("gateway attendu :\n%s", got)
	}
	if !strings.Contains(got, "allow=172.16.0.0/12") {
		t.Fatalf("allow attendu :\n%s", got)
	}
	if !strings.Contains(got, "port=8080") {
		t.Fatalf("port attendu :\n%s", got)
	}
	if strings.Contains(got, "listen=") {
		t.Fatalf("listen ne doit pas être présent :\n%s", got)
	}
}

func TestRegisterAutostartArgs(t *testing.T) {
	fake := &execx.Fake{}
	pxExe := `C:\Tools\px.exe`
	iniPath := `C:\Users\demo\px.ini`
	if err := RegisterAutostart(context.Background(), fake, pxExe, iniPath); err != nil {
		t.Fatalf("RegisterAutostart : %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("appels attendus : 1, obtenus : %d", len(fake.Calls))
	}
	call := fake.Calls[0]
	if call.Name != "reg.exe" {
		t.Fatalf("exécutable attendu reg.exe, obtenu %q", call.Name)
	}
	want := []string{
		"add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "DevStationPx",
		"/t", "REG_SZ",
		"/d", `"` + pxExe + `" --config="` + iniPath + `"`,
		"/f",
	}
	if len(call.Args) != len(want) {
		t.Fatalf("arguments attendus %v, obtenus %v", want, call.Args)
	}
	for i := range want {
		if call.Args[i] != want[i] {
			t.Fatalf("argument %d : attendu %q, obtenu %q", i, want[i], call.Args[i])
		}
	}
	if strings.Contains(strings.Join(call.Args, " "), "HKLM") {
		t.Fatalf("HKLM interdit : %v", call.Args)
	}
}

func TestPxRunning(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("écoute locale : %v", err)
	}
	addr := ln.Addr().String()
	if !PxRunning(addr, 500*time.Millisecond) {
		t.Fatalf("PxRunning devrait être vrai sur %s", addr)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("fermeture : %v", err)
	}
	if PxRunning(addr, 200*time.Millisecond) {
		t.Fatalf("PxRunning devrait être faux après fermeture de %s", addr)
	}
}
