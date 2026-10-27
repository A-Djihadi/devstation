package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/A-Djihadi/devstation/internal/download"
)

// nodeIndexEntry décrit une entrée de l'index des versions Node.js.
type nodeIndexEntry struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
	LTS     any      `json:"lts"`
}

// nodeFileSuffix est le suffixe de l'archive Windows x64 de Node.js.
const nodeFileSuffix = "win-x64-zip"

// Node est la source des versions publiées de Node.js.
type Node struct {
	f       Fetcher
	baseURL string
}

// NewNode crée une source Node.js interrogeant baseURL (sans slash final).
func NewNode(f Fetcher, baseURL string) *Node {
	return &Node{f: f, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Name retourne le nom de la source.
func (n *Node) Name() string { return "node" }

// nodeHasFile indique si files contient le suffixe demandé.
func nodeHasFile(files []string, suffix string) bool {
	for _, f := range files {
		if f == suffix {
			return true
		}
	}
	return false
}

// nodeIsLTS indique si le champ lts de l'index désigne une version LTS.
func nodeIsLTS(v any) bool {
	_, ok := v.(string)
	return ok
}

// nodeZipName retourne le nom de l'archive Windows x64 d'une version.
func nodeZipName(version string) string {
	return "node-v" + version + "-win-x64.zip"
}

// Releases retourne les versions publiées de Node.js pour Windows x64.
func (n *Node) Releases(ctx context.Context) ([]Release, error) {
	var entries []nodeIndexEntry
	if err := n.f.GetJSON(ctx, n.baseURL+"/dist/index.json", &entries); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(entries))
	out := make([]Release, 0, len(entries))
	for _, e := range entries {
		if !nodeHasFile(e.Files, nodeFileSuffix) {
			continue
		}
		version := normalize(e.Version)
		if version == "" || seen[version] {
			continue
		}
		seen[version] = true
		out = append(out, Release{
			Version: version,
			LTS:     nodeIsLTS(e.LTS),
			URL:     n.baseURL + "/dist/v" + version + "/" + nodeZipName(version),
			Archive: "zip",
		})
	}
	SortDesc(out)
	return out, nil
}

// Resolve complète l'empreinte SHA-256 de l'archive Windows x64.
func (n *Node) Resolve(ctx context.Context, r Release) (Release, error) {
	version := normalize(r.Version)
	text, err := n.f.GetText(ctx, n.baseURL+"/dist/v"+version+"/SHASUMS256.txt")
	if err != nil {
		return Release{}, err
	}
	want := nodeZipName(version)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] == want {
			r.Version = version
			r.URL = n.baseURL + "/dist/v" + version + "/" + want
			r.Archive = "zip"
			r.Checksum = download.Checksum{Algo: "sha256", Hex: fields[0]}
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("aucune archive Windows x64 pour Node.js %s : %w", version, ErrNoRelease)
}
