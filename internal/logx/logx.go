// Package logx fournit un journal de fichier qui masque les secrets.
package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// motifs de secrets connus, masqués même s'ils n'ont pas été enregistrés.
// Les motifs à clé/valeur conservent la clé et ne masquent que la valeur.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9]+`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`hvs\.[A-Za-z0-9._-]+`),
	regexp.MustCompile(`(?i)_authToken=[^\s&]+`),
	regexp.MustCompile(`(?i)password\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)"password"\s*:\s*"[^"]*"`),
	regexp.MustCompile(`(?i)authorization:\s*bearer\s+\S+`),
	regexp.MustCompile(`(?i)(token|secret)\s*=\s*\S+`),
}

// Logger écrit des lignes dans un fichier en masquant les secrets connus.
type Logger struct {
	mu      sync.Mutex
	f       *os.File
	secrets []string
}

// New ouvre (ou crée) le fichier de journal situé à path.
func New(path string) (*Logger, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

// Close ferme le fichier de journal.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// AddSecret enregistre une valeur à masquer dans les écritures suivantes.
func (l *Logger) AddSecret(s string) {
	if s == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.secrets = append(l.secrets, s)
}

// Printf écrit une ligne formatée dans le journal après masquage des secrets.
func (l *Logger) Printf(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	l.mu.Lock()
	defer l.mu.Unlock()
	line = Redact(line, l.secrets)
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	_, _ = l.f.WriteString(line)
}

// Redact remplace les secrets fournis et les motifs connus par « *** ».
func Redact(s string, secrets []string) string {
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		s = strings.ReplaceAll(s, sec, "***")
	}
	for _, re := range patterns {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			if i := strings.IndexByte(m, '='); i >= 0 {
				return m[:i+1] + "***"
			}
			if i := strings.IndexByte(m, ':'); i >= 0 {
				return m[:i+1] + "***"
			}
			return "***"
		})
	}
	return s
}