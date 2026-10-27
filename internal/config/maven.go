package config

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/A-Djihadi/devstation/internal/state"
)

// mavenMirrorKey est la clé d'état du miroir géré par DevStation.
const mavenMirrorKey = "mirror:devstation"

// mavenProxyKey est la clé d'état du proxy géré par DevStation.
const mavenProxyKey = "proxy:devstation"

// mavenID est l'identifiant des blocs gérés par DevStation.
const mavenID = "devstation"

// MavenSettings décrit les réglages Maven gérés par DevStation.
// MirrorURL vide laisse les miroirs intacts ; ProxyHost vide laisse les
// proxies intacts.
type MavenSettings struct {
	MirrorURL     string
	ProxyHost     string
	ProxyPort     int
	NonProxyHosts string
}

// mavenMinimalSettings est le contenu d'un settings.xml minimal valide.
const mavenMinimalSettings = `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 https://maven.apache.org/xsd/settings-1.0.0.xsd">
</settings>
`

// mavenEscape échappe une valeur pour insertion dans un document XML.
func mavenEscape(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return s
	}
	return b.String()
}

// mavenMirrorBlock construit le bloc <mirror> géré par DevStation.
func mavenMirrorBlock(url string) string {
	return "<mirror>\n" +
		"      <id>" + mavenID + "</id>\n" +
		"      <name>DevStation</name>\n" +
		"      <url>" + mavenEscape(url) + "</url>\n" +
		"      <mirrorOf>*</mirrorOf>\n" +
		"    </mirror>"
}

// mavenProxyBlock construit le bloc <proxy> géré par DevStation.
func mavenProxyBlock(s MavenSettings) string {
	port := s.ProxyPort
	if port <= 0 {
		port = 3128
	}
	return "<proxy>\n" +
		"      <id>" + mavenID + "</id>\n" +
		"      <active>true</active>\n" +
		"      <protocol>http</protocol>\n" +
		"      <host>" + mavenEscape(s.ProxyHost) + "</host>\n" +
		"      <port>" + mavenItoa(port) + "</port>\n" +
		"      <nonProxyHosts>" + mavenEscape(s.NonProxyHosts) + "</nonProxyHosts>\n" +
		"    </proxy>"
}

// mavenItoa convertit un entier positif en chaîne décimale.
func mavenItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// mavenFindBlock cherche le bloc d'élément tag dont l'<id> vaut mavenID.
// Renvoie les bornes [start, end) du bloc complet, ou -1 si absent.
func mavenFindBlock(content, tag string) (int, int) {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	pos := 0
	for {
		start := strings.Index(content[pos:], open)
		if start < 0 {
			return -1, -1
		}
		start += pos
		end := strings.Index(content[start:], closeTag)
		if end < 0 {
			return -1, -1
		}
		end += start + len(closeTag)
		block := content[start:end]
		if mavenBlockID(block) == mavenID {
			return start, end
		}
		pos = end
	}
}

// mavenBlockID extrait la valeur de l'élément <id> d'un bloc XML.
func mavenBlockID(block string) string {
	open := "<id>"
	closeTag := "</id>"
	start := strings.Index(block, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(block[start:], closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(block[start : start+end])
}

// mavenInsertBlock insère block dans la section section (par exemple
// "mirrors") avant </settings>, en créant la section si nécessaire.
func mavenInsertBlock(content, section, block string) string {
	open := "<" + section + ">"
	closeTag := "</" + section + ">"
	if idx := strings.Index(content, closeTag); idx >= 0 {
		insert := block + "\n    "
		return content[:idx] + insert + content[idx:]
	}
	sectionBlock := "  " + open + "\n    " + block + "\n  " + closeTag + "\n"
	if idx := strings.Index(content, "</settings>"); idx >= 0 {
		return content[:idx] + sectionBlock + content[idx:]
	}
	return content + sectionBlock
}

// mavenRemoveBlock retire le bloc d'élément tag dont l'<id> vaut mavenID.
func mavenRemoveBlock(content, tag string) string {
	start, end := mavenFindBlock(content, tag)
	if start < 0 {
		return content
	}
	// Retirer aussi l'indentation et la fin de ligne qui précèdent le bloc.
	lineStart := strings.LastIndex(content[:start], "\n")
	if lineStart >= 0 && strings.TrimSpace(content[lineStart+1:start]) == "" {
		start = lineStart + 1
	}
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + content[end:]
}

// mavenApplyBlock remplace ou insère le bloc géré d'élément tag et renvoie le
// nouveau contenu ainsi que le texte exact du bloc d'origine (nil si absent).
func mavenApplyBlock(content, tag, section, block string) (string, *string) {
	start, end := mavenFindBlock(content, tag)
	if start >= 0 {
		original := content[start:end]
		return content[:start] + block + content[end:], &original
	}
	return mavenInsertBlock(content, section, block), nil
}

// SetMavenSettings applique les réglages Maven au fichier settings.xml path.
// Le fichier est créé s'il est absent ; une sauvegarde unique est réalisée
// avant toute écriture. Les autres miroirs et proxies sont conservés.
func SetMavenSettings(path string, s MavenSettings, st *state.State) error {
	if err := BackupOnce(path, st); err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content := string(original)
	if len(strings.TrimSpace(content)) == 0 {
		content = mavenMinimalSettings
	}

	if s.MirrorURL != "" {
		var prev *string
		content, prev = mavenApplyBlock(content, "mirror", "mirrors", mavenMirrorBlock(s.MirrorURL))
		st.RecordKey(path, mavenMirrorKey, prev)
	}
	if s.ProxyHost != "" {
		var prev *string
		content, prev = mavenApplyBlock(content, "proxy", "proxies", mavenProxyBlock(s))
		st.RecordKey(path, mavenProxyKey, prev)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, []byte(content))
}

// RestoreMavenSettings restaure les blocs gérés du fichier path à partir de
// st. Un bloc dont le texte d'origine est nil est retiré ; les autres blocs
// sont remis à l'identique. Un fichier absent ne provoque pas d'erreur.
func RestoreMavenSettings(path string, st *state.State) error {
	managed := st.KeysFor(path)
	if len(managed) == 0 {
		return nil
	}
	original, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	content := string(original)

	for _, mk := range managed {
		var tag string
		switch mk.Key {
		case mavenMirrorKey:
			tag = "mirror"
		case mavenProxyKey:
			tag = "proxy"
		default:
			continue
		}
		if mk.Original == nil {
			content = mavenRemoveBlock(content, tag)
			continue
		}
		start, end := mavenFindBlock(content, tag)
		if start >= 0 {
			content = content[:start] + *mk.Original + content[end:]
		} else {
			section := tag + "s"
			content = mavenInsertBlock(content, section, *mk.Original)
		}
	}

	return writeAtomic(path, []byte(content))
}
