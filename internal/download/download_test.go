package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, o Options) *Client {
	t.Helper()
	if o.CacheDir == "" {
		o.CacheDir = t.TempDir()
	}
	if o.Backoff == nil {
		o.Backoff = func(int) time.Duration { return time.Millisecond }
	}
	c, err := New(o)
	if err != nil {
		t.Fatalf("New : %v", err)
	}
	return c
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestFetchVerifiesSHA256(t *testing.T) {
	body := []byte("contenu de test")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, Options{})
	path, verified, err := c.Fetch(context.Background(), srv.URL, Checksum{Algo: "sha256", Hex: sha256Hex(body)}, nil)
	if err != nil {
		t.Fatalf("Fetch : %v", err)
	}
	if !verified {
		t.Fatal("empreinte attendue vérifiée")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("contenu = %q", got)
	}
}

func TestChecksumMismatchRemovesPart(t *testing.T) {
	body := []byte("mauvais contenu")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := newTestClient(t, Options{CacheDir: dir})
	_, _, err := c.Fetch(context.Background(), srv.URL, Checksum{Algo: "sha256", Hex: sha256Hex([]byte("autre"))}, nil)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, attendu ErrChecksum", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("fichiers restants dans le cache : %v", entries)
	}
}

func TestRetriesOn500(t *testing.T) {
	body := []byte("ok")
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, Options{})
	_, verified, err := c.Fetch(context.Background(), srv.URL, Checksum{Algo: "sha256", Hex: sha256Hex(body)}, nil)
	if err != nil {
		t.Fatalf("Fetch : %v", err)
	}
	if !verified {
		t.Fatal("empreinte attendue vérifiée")
	}
	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("requêtes = %d, attendu 3", got)
	}
}

func TestCacheHitNoRequest(t *testing.T) {
	body := []byte("cache")
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, Options{})
	sum := Checksum{Algo: "sha256", Hex: sha256Hex(body)}
	if _, _, err := c.Fetch(context.Background(), srv.URL, sum, nil); err != nil {
		t.Fatalf("premier Fetch : %v", err)
	}
	if _, verified, err := c.Fetch(context.Background(), srv.URL, sum, nil); err != nil || !verified {
		t.Fatalf("second Fetch : %v verified=%v", err, verified)
	}
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("requêtes = %d, attendu 1", got)
	}
}

func TestConnectionDropMidBody(t *testing.T) {
	body := make([]byte, 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2048")
		w.Write(body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := newTestClient(t, Options{CacheDir: dir, Retries: 1})
	_, _, err := c.Fetch(context.Background(), srv.URL, Checksum{Algo: "sha256", Hex: sha256Hex(body)}, nil)
	if err == nil {
		t.Fatal("erreur attendue")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".part" {
			t.Fatalf("fichier final présent : %s", e.Name())
		}
	}
}

func TestShortBodyWithoutChecksumFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Write(make([]byte, 10))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := newTestClient(t, Options{CacheDir: dir, Retries: 1})
	_, _, err := c.Fetch(context.Background(), srv.URL, Checksum{}, nil)
	if err == nil {
		t.Fatal("erreur attendue")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("fichiers restants dans le cache : %v", entries)
	}
}

func TestUsesProxy(t *testing.T) {
	body := []byte("via proxy")
	var proxied int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxied, 1)
		w.Write(body)
	}))
	defer proxy.Close()

	c := newTestClient(t, Options{ProxyURL: proxy.URL})
	_, verified, err := c.Fetch(context.Background(), "http://cible.example.com/fichier", Checksum{Algo: "sha256", Hex: sha256Hex(body)}, nil)
	if err != nil {
		t.Fatalf("Fetch : %v", err)
	}
	if !verified {
		t.Fatal("empreinte attendue vérifiée")
	}
	if atomic.LoadInt32(&proxied) == 0 {
		t.Fatal("le proxy n'a pas reçu la requête")
	}
}
