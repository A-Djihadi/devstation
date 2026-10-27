// Package download fournit un client de téléchargement HTTPS avec vérification
// d'empreinte, cache local et nouvelles tentatives.
package download

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrChecksum indique que l'empreinte du fichier téléchargé ne correspond pas à
// celle attendue.
var ErrChecksum = errors.New("empreinte invalide")

// Checksum décrit une empreinte attendue. Algo vaut "sha256", "sha512" ou ""
// (chaîne vide : empreinte non vérifiée).
type Checksum struct {
	Algo string
	Hex  string
}

// Options configure un Client.
type Options struct {
	// ProxyURL est l'URL du proxy HTTP à utiliser (vide : pas de proxy explicite).
	ProxyURL string
	// CACertFiles liste des fichiers PEM de CA à ajouter au pool système.
	CACertFiles []string
	// CacheDir est le répertoire de cache des téléchargements.
	CacheDir string
	// Retries est le nombre de tentatives totales (0 : valeur par défaut 3).
	Retries int
	// HTTPClient permet d'injecter un client HTTP (tests). Si nil, un client est
	// construit à partir des autres options.
	HTTPClient *http.Client
	// Backoff permet d'injecter la temporisation entre tentatives (tests). Si nil,
	// les délais 1 s, 2 s, 4 s sont utilisés.
	Backoff func(attempt int) time.Duration
}

// Progress est appelé pendant le téléchargement avec le nombre d'octets déjà
// reçus et le total annoncé (-1 si inconnu).
type Progress func(done, total int64)

// Client télécharge des fichiers et du JSON avec vérification d'empreinte.
type Client struct {
	http    *http.Client
	cache   string
	retries int
	backoff func(attempt int) time.Duration
}

// New construit un Client à partir des options fournies.
func New(o Options) (*Client, error) {
	if o.CacheDir == "" {
		return nil, errors.New("répertoire de cache manquant")
	}
	if err := os.MkdirAll(o.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("création du cache : %w", err)
	}

	hc := o.HTTPClient
	if hc == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if len(o.CACertFiles) > 0 {
			pool, err := x509.SystemCertPool()
			if err != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			for _, f := range o.CACertFiles {
				pem, err := os.ReadFile(f)
				if err != nil {
					return nil, fmt.Errorf("lecture CA %s : %w", f, err)
				}
				if !pool.AppendCertsFromPEM(pem) {
					return nil, fmt.Errorf("CA invalide : %s", f)
				}
			}
			transport.TLSClientConfig = &tls.Config{RootCAs: pool}
		}
		if o.ProxyURL != "" {
			pu, err := url.Parse(o.ProxyURL)
			if err != nil {
				return nil, fmt.Errorf("proxy invalide : %w", err)
			}
			transport.Proxy = http.ProxyURL(pu)
		}
		hc = &http.Client{Transport: transport}
	}

	retries := o.Retries
	if retries <= 0 {
		retries = 3
	}
	backoff := o.Backoff
	if backoff == nil {
		backoff = func(attempt int) time.Duration {
			switch attempt {
			case 1:
				return time.Second
			case 2:
				return 2 * time.Second
			default:
				return 4 * time.Second
			}
		}
	}

	return &Client{http: hc, cache: o.CacheDir, retries: retries, backoff: backoff}, nil
}

// cachePath retourne le chemin de cache pour une URL donnée.
func (c *Client) cachePath(u string) string {
	sum := sha256.Sum256([]byte(u))
	return filepath.Join(c.cache, hex.EncodeToString(sum[:])+".part")
}

// newHash retourne la fonction de hachage correspondant à l'algorithme.
func newHash(algo string) (hash.Hash, error) {
	switch strings.ToLower(algo) {
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("algorithme d'empreinte inconnu : %s", algo)
	}
}

// verify vérifie l'empreinte du fichier. Retourne true si vérifiée.
func verify(path string, sum Checksum) (bool, error) {
	if sum.Algo == "" {
		return false, nil
	}
	h, err := newHash(sum.Algo)
	if err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, sum.Hex) {
		return false, ErrChecksum
	}
	return true, nil
}

// Fetch télécharge l'URL vers le cache, vérifie l'empreinte et retourne le
// chemin du fichier. Si un fichier en cache correspond déjà à l'empreinte, il
// est retourné sans requête réseau.
func (c *Client) Fetch(ctx context.Context, rawURL string, sum Checksum, p Progress) (string, bool, error) {
	dest := c.cachePath(rawURL)

	if _, err := os.Stat(dest); err == nil {
		ok, verr := verify(dest, sum)
		if verr == nil && ok {
			return dest, true, nil
		}
		if sum.Algo == "" {
			return dest, false, nil
		}
		_ = os.Remove(dest)
	}

	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return "", false, ctx.Err()
			case <-time.After(c.backoff(attempt - 1)):
			}
		}
		verified, err := c.download(ctx, rawURL, dest, sum, p)
		if err == nil {
			return dest, verified, nil
		}
		lastErr = err
		if errors.Is(err, ErrChecksum) {
			return "", false, err
		}
	}
	return "", false, lastErr
}

// download effectue une tentative de téléchargement.
func (c *Client) download(ctx context.Context, rawURL, dest string, sum Checksum, p Progress) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("statut HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return false, err
	}

	var h hash.Hash
	if sum.Algo != "" {
		h, err = newHash(sum.Algo)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return false, err
		}
	}

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return false, werr
			}
			if h != nil {
				h.Write(buf[:n])
			}
			done += int64(n)
			if p != nil {
				p(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return false, rerr
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return false, err
	}

	verified := false
	if h != nil {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, sum.Hex) {
			os.Remove(tmp)
			return false, ErrChecksum
		}
		verified = true
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return verified, nil
}

// GetJSON télécharge l'URL et décode le corps JSON dans v.
func (c *Client) GetJSON(ctx context.Context, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("statut HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}