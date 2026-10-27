# DevStation — plan d'implémentation

> **Pour les agents :** ce plan est exécuté par `agentic-coder` (DeepSeek code, Claude relit les PR,
> l'humain merge). Chaque tâche ci-dessous devient une entrée de `.agent/tasks.json` (via `/plan-tasks`)
> au moment où son lot est lancé. Une tâche ne voit que son propre bloc : le bloc **Interfaces** est la
> seule source des noms et types partagés.

**Objectif :** un `git clone` + `.\setup.ps1` qui monte un poste de dev Windows complet, sans admin, via une CLI Go guidée.

**Architecture :** script PowerShell de démarrage → `devstation.exe` (Go). La logique est en paquets purs
testables sous Linux ; tout ce qui touche Windows (registre, jonctions, certificats, identifiants) passe par
des interfaces de `internal/winenv` avec une implémentation factice. Les installeurs suivent tous le même
flux (source → téléchargement vérifié → extraction → jonction `current` → PATH).

**Stack :** Go (version stable courante, épinglée en préparation), cobra, charmbracelet huh / lipgloss /
bubbles / bubbletea, gopkg.in/yaml.v3, golang.org/x/sys/windows, github.com/danieljoos/wincred,
PowerShell 5.1 + Pester 5. Démos : Spring Boot 4.1 / Java 21 / Maven ; Angular 22 + Native Federation 22, tests Vitest + jsdom.

**Spec :** `docs/superpowers/specs/2026-10-27-devstation-design.md`

## Contraintes globales

- Windows 10/11 x64, **aucune action admin** : HKCU uniquement, jamais HKLM ni PATH machine ; jonctions NTFS (pas de symlink).
- Racine des outils : `%LOCALAPPDATA%\DevStation\tools\<outil>\<version>\`, version active `…\<outil>\current`.
- Cache : `%LOCALAPPDATA%\DevStation\cache` ; logs : `%LOCALAPPDATA%\DevStation\logs\<AAAAMMJJ-HHMMSS>.log` ; binaire : `%LOCALAPPDATA%\DevStation\bin`.
- État : `%USERPROFILE%\.devstation\answers.yaml` (sans secret) et `%USERPROFILE%\.devstation\state.json`.
- Sauvegarde : `<fichier>.devstation.bak` écrit au premier passage seulement ; restauration clé par clé depuis `state.json`.
- Proxy Kerberos : relais **px** sur `http://127.0.0.1:3128` ; `network.proxy_auth: none | kerberos`.
- Téléchargements : HTTPS, 3 essais, empreinte éditeur vérifiée si publiée, sinon log « non vérifiée ».
- Secrets (token GitHub, mot de passe Nexus, token Vault) : jamais dans un log, `answers.yaml`, `state.json`, la console ou un dépôt. Stockage : Gestionnaire d'identifiants Windows / Git Credential Manager.
- Textes affichés en **français** ; `NO_COLOR` et absence de TTY ⇒ texte simple sans animation.
- Valeurs d'exemple fictives uniquement (`*.example.com`) : le code part chez DeepSeek.
- Sandbox de test : Linux, **sans réseau**, pas de nouvelle dépendance Go hors `go.mod` posé en préparation.
- Commande de test (`TEST_CMD`) du dépôt `devstation` : `sh scripts/test.sh` (même script en CI), soit :
  `export GOCACHE=/tmp/gocache GOPROXY=off GOTOOLCHAIN=local; go vet ./... && go test ./... && GOOS=windows go vet ./... && GOOS=windows go build ./... && pwsh -NoProfile -Command "Invoke-Pester -CI -Path tests/ps"`
- Code Windows : fichiers `*_windows.go` ; leur pendant factice/stub `*_other.go` (`//go:build !windows`).

## Points de vigilance (relecture)

1. **Relance après un échec partiel** : un outil déjà installé à la bonne version est sauté (↷), rien n'est retéléchargé ; seules les étapes manquantes s'exécutent. → testé en T1.7 et T3.1.
2. **Fichier de config existant avec du contenu non géré** (commentaires, `//registry.npmjs.org/:_authToken=…`) : conservé à l'octet près hors clés gérées, et son contenu n'apparaît jamais dans les logs. → testé en T2.12.
3. **Archive malveillante** (`../`, chemin absolu, lettre de lecteur) : extraction refusée, rien n'est écrit hors du dossier cible. → testé en T1.4.
4. **Profil utilisateur avec espaces/accents** (`C:\Users\Jean Émile`) : PATH, arguments exec et fichiers de config restent valides. → testé en T1.5 et T1.6.
5. **Coupure réseau ou empreinte fausse en cours de téléchargement** : fichier partiel supprimé, cache non pollué, erreur explicite. → testé en T1.3.

---

## Préparation (Claude, avant le lot 1 — pas de code produit)

Le sandbox étant hors-ligne, l'outillage doit exister avant que DeepSeek commence.

- [ ] **P1 — Dépôts GitHub publics** `A-Djihadi/devstation`, `A-Djihadi/demo-backend`, `A-Djihadi/demo-frontend` (avec ton accord explicite : action publique).
- [ ] **P2 — Squelette `devstation`** : `go.mod` / `go.sum` résolus via une image `golang` (réseau autorisé) avec toutes les dépendances de la Stack ; `cmd/devstation/main.go` minimal qui affiche `devstation dev` ; `internal/doc.go` vide ; `.gitignore` (`.agent/`, `dist/`) ; `CLAUDE.md` (stack, `TEST_CMD`, conventions) ; sous-agents via `new_project.py --profile infra,backend` ; `Dockerfile.sandbox` = image `golang` + `pwsh` + module Pester 5 + `go mod download` ; `.github/workflows/ci.yml` (ubuntu : `TEST_CMD` ; windows-latest : `go test ./...` + Pester).
  Vérification : `docker build` OK puis `TEST_CMD` passe dans le conteneur `--network none`.
- [ ] **P3 — Squelette `demo-backend`** : Spring Initializr (Java 21, Maven, web, actuator, cloud-vault-config, validation) ; sandbox `maven:3-eclipse-temurin-21` + `mvn dependency:go-offline` ; `TEST_CMD="sh mvnw -o -q test"` ; CI GitHub Actions.
- [ ] **P4 — Squelette `demo-frontend`** : `ng new demo-frontend --standalone --routing=false --style=css` puis `ng add @angular-architects/native-federation --type remote --port 4201` ; sandbox Node avec `node_modules` + Chromium headless ; `TEST_CMD="ng test --watch=false && ng build"` (Vitest + jsdom, pas de navigateur) ; CI.

Lancement d'un lot (exemple `devstation`) : `/plan-tasks` dans le dépôt cible avec ce plan → `.agent/tasks.json`, puis
`SANDBOX_IMAGE=devstation-sandbox:latest TEST_CMD="…" python -m agent.main --repo D:\code\devstation --tasks D:\code\devstation\.agent\tasks.json`.
Un lot est mergé entièrement avant de lancer le suivant.

---

## Lot 1 — Fondations (10 tâches indépendantes ; T1.9 part avec le lot 2)

### T1.1 Profil d'équipe : chargement et validation
**Fichiers :** créer `internal/profile/profile.go`, `internal/profile/profile_test.go`, `profiles/demo.yaml` (contenu du §5.1 de la spec, avec `proxy_auth: none` et `vault.addr: http://localhost:8200`).
**Produit :**
```go
type ToolSpec struct { Versions []string; Default string; Distribution string; Npm string; Optional bool; Enabled *bool }
type Tools struct { Java, Node, AngularCLI, Python, Maven, Gradle ToolSpec } // yaml: java, node, angular_cli, python, maven, gradle
type Nexus struct { Maven, Npm, Pypi, Containers string; Auth bool }
type Network struct { Proxy string; ProxyAuth string; NoProxy []string; CACerts []string; Nexus Nexus } // yaml: proxy, proxy_auth, no_proxy, ca_certs, nexus
type Repo struct { Name, URL string; Roles []string }
type SecretRef struct { Path, Field, Target string } // Target ∈ {"nexus.username","nexus.password"}
type Vault struct { Addr, Auth, Mount, Role string; Secrets []SecretRef }
type Profile struct { Name, Description string; Defaults struct{ Roles []string; Editor string }; Tools Tools; Network Network; Git struct{ Autocrlf string }; Repos []Repo; EditorExtensions map[string]map[string][]string; Vault Vault; Dir string `yaml:"-"` }
type ValidationError struct { Key string; Line int; Msg string } // Error() = "profil invalide : <Key> (ligne <Line>) : <Msg>"
func Load(path string) (*Profile, error)          // yaml.v3 KnownFields(true) ; Dir = dossier du fichier ; appelle Validate
func (p *Profile) Validate() error                 // *ValidationError
func (p *Profile) ReposFor(roles []string) []Repo
```
Règles : `proxy_auth` ∈ {"", "none", "kerberos"} ; `kerberos` exige `proxy` non vide ; `vault.auth` ∈ {"", "oidc", "ldap", "kerberos", "token"} ; `editor` ∈ {"vscode", "intellij", "both"} ; rôle ∈ {"front", "back", "fullstack", "python"} ; `Default` doit appartenir à `Versions` si `Versions` non vide ; `ca_certs` résolus relativement à `Dir`.
**Tests :** `TestLoadDemoProfile` (charge `../../profiles/demo.yaml`, `Name == "demo"`, 2 dépôts) · `TestUnknownKeyRejected` (clé `toolz:` → `*ValidationError` avec `Key == "toolz"`, `Line > 0`) · `TestKerberosNeedsProxy` · `TestDefaultNotInVersions` (java default `"11"` avec versions `["21","17"]` → `Key == "tools.java.default"`) · `TestReposForRoles` (`["front"]` → seulement `demo-frontend`, `["fullstack"]` → les deux).

### T1.2 État local
**Fichiers :** `internal/state/state.go`, `internal/state/state_test.go`.
**Produit :**
```go
type Install struct { Tool, Version, Path string; At time.Time }
type ManagedKey struct { File, Key string; Original *string } // nil = clé absente à l'origine
type State struct { Installs []Install; Keys []ManagedKey; Backups []string; Active map[string]string }
func Load(path string) (*State, error)          // fichier absent ⇒ état vide, pas d'erreur
func (s *State) Save(path string) error          // écriture atomique (fichier temporaire + rename)
func (s *State) RecordInstall(i Install)         // remplace l'entrée (Tool, Version) existante
func (s *State) Installed(tool, version string) (Install, bool)
func (s *State) RecordKey(file, key string, original *string) // la PREMIÈRE valeur enregistrée gagne
func (s *State) KeysFor(file string) []ManagedKey
```
**Tests :** `TestLoadMissingIsEmpty` · `TestSaveLoadRoundTrip` · `TestRecordKeyFirstWins` (deux appels, `Original` du premier conservé) · `TestRecordInstallReplaces`.

### T1.3 Téléchargement vérifié
**Fichiers :** `internal/download/download.go`, `internal/download/download_test.go`.
**Produit :**
```go
type Checksum struct { Algo string; Hex string } // Algo ∈ {"sha256","sha512",""} ; "" = non vérifiée
type Options struct { ProxyURL string; CACertFiles []string; CacheDir string; Retries int; HTTPClient *http.Client }
type Progress func(done, total int64)
type Client struct{ /* … */ }
func New(o Options) (*Client, error)
func (c *Client) Fetch(ctx context.Context, url string, sum Checksum, p Progress) (path string, verified bool, err error)
func (c *Client) GetJSON(ctx context.Context, url string, v any) error
var ErrChecksum = errors.New("empreinte invalide")
```
Le fichier est écrit en `<cache>/<sha256(url)>.part`, vérifié, puis renommé ; un fichier en cache dont l'empreinte correspond est rendu sans requête. CA ajoutés au pool système. `Retries` défaut 3, backoff 1 s/2 s/4 s (injecté pour les tests).
**Tests (httptest) :** `TestFetchVerifiesSHA256` · `TestChecksumMismatchRemovesPart` (`errors.Is(err, ErrChecksum)`, aucun fichier restant dans le cache) · `TestRetriesOn500` (2×500 puis 200 ⇒ OK, 3 requêtes) · `TestCacheHitNoRequest` · `TestConnectionDropMidBody` (serveur qui coupe après la moitié ⇒ erreur, pas de fichier final) · `TestUsesProxy` (proxy httptest reçoit la requête).

### T1.4 Extraction d'archives
**Fichiers :** `internal/archive/archive.go`, `internal/archive/archive_test.go`.
**Produit :** `func Unzip(src, dest string, stripTop bool) error` ; `func Untar(src, dest string, stripTop bool) error` (tar.gz) ; `var ErrUnsafePath = errors.New("chemin dangereux dans l'archive")`. `stripTop` retire le dossier racine unique (JDK, Node, Maven).
**Tests :** `TestUnzipStripTop` · `TestZipSlipRejected` (entrée `../evil.txt` ⇒ `ErrUnsafePath`, rien hors `dest`) · `TestAbsolutePathRejected` (`/etc/x` et `C:\x`) · `TestUnicodeAndSpaces` (dest `t.TempDir()+"/Jean Émile"`).

### T1.5 Environnement Windows : interfaces, factice, PATH pur
**Fichiers :** `internal/winenv/winenv.go`, `internal/winenv/fake.go`, `internal/winenv/winenv_test.go`.
**Produit :**
```go
type Env interface {
  UserVar(name string) (string, bool, error); SetUserVar(name, value string) error; DeleteUserVar(name string) error
  MachinePath() (string, error); Broadcast() error
}
type Links interface { SetJunction(link, target string) error; ReadJunction(link string) (string, error) }
type CertStore interface { AddUserRoot(pemPath string) (added bool, err error); HasUserRoot(pemPath string) (bool, error) }
type Creds interface { Get(target string) (user, secret string, err error); Set(target, user, secret string) error; Delete(target string) error }
var ErrNotFound = errors.New("introuvable")
type Fake struct { Vars map[string]string; Machine string; Junctions map[string]string; Roots map[string]bool; Secrets map[string][2]string; Broadcasts int }
func NewFake() *Fake // implémente Env, Links, CertStore, Creds
func PrependPath(pathVar, dir string) string   // insensible à la casse, sans doublon, ';' final ignoré
func RemovePath(pathVar, dir string) string
func Shadowing(machinePath string, exe string, dirs []string) []string // dossiers du PATH machine contenant exe (ex. "java.exe")
```
**Tests :** `TestPrependPathDedupCaseInsensitive` (`C:\A;c:\a\;C:\B` + `C:\A` ⇒ `C:\A;C:\B`) · `TestPrependPathWithSpaces` (`C:\Users\Jean Émile\x`) · `TestRemovePath` · `TestShadowing` (dossiers créés dans `t.TempDir()`) · `TestFakeCredsRoundTrip`.

### T1.6 Exécution de commandes
**Fichiers :** `internal/execx/runner.go`, `internal/execx/runner_test.go`.
**Produit :**
```go
type Cmd struct { Name string; Args []string; Dir string; Env []string; Stdin string; Secret bool } // Secret ⇒ Stdin et Args jamais loggés
type Result struct { Stdout, Stderr string; Code int }
type Runner interface { Run(ctx context.Context, c Cmd) (Result, error) }
func OS() Runner
type Fake struct { Calls []Cmd; Responses map[string]Result } // clé = Name + " " + strings.Join(Args, " ")
func (f *Fake) Run(ctx context.Context, c Cmd) (Result, error)
func (c Cmd) String() string // forme loggable : arguments avec espaces entre guillemets, "***" si Secret
```
**Tests :** `TestOSRunnerEcho` (`go version` ou `sh -c "echo ok"` sous Linux) · `TestStringQuotesSpaces` · `TestStringMasksSecret` · `TestFakeRecordsCalls`.

### T1.7 Exécuteur d'étapes
**Fichiers :** `internal/plan/plan.go`, `internal/plan/plan_test.go`.
**Produit :**
```go
type Status int // Pending, Running, Done, Skipped (déjà fait), Failed, Blocked (dépendance échouée)
type Step struct { ID, Title string; DependsOn []string; Check func(ctx context.Context) (bool, error); Run func(ctx context.Context, report func(msg string, done, total int64)) error }
type Event struct { StepID string; Status Status; Msg string; Done, Total int64; Err error }
type Summary struct { Done, Skipped, Failed, Blocked []string; Errors map[string]error }
func Execute(ctx context.Context, steps []Step, onEvent func(Event)) (Summary, error) // erreur seulement si graphe invalide
```
`Check` vrai ⇒ `Skipped` sans appeler `Run`. Ordre topologique stable (ordre de déclaration). Échec ⇒ ses descendants `Blocked`, les autres continuent.
**Tests :** `TestSkipWhenCheckTrue` · `TestFailureBlocksDependentsOnly` · `TestCycleRejected` · `TestUnknownDependencyRejected` · `TestEventsOrder`.

### T1.8 Compatibilité Angular ↔ Node
**Fichiers :** `internal/compat/compat.go`, `internal/compat/compat_test.go`.
**Produit :** `func AngularNode(angularMajor int, nodeVersion string) error` (nil si compatible, sinon message « Angular 20 exige Node ^20.19 || ^22.12 || ^24 ») ; table en dur, vérifiée contre angular.dev/reference/versions : 19 → `^18.19.1 || ^20.11.1 || ^22`, 20 → `^20.19 || ^22.12 || ^24`, 21 → `^20.19 || ^22.12 || ^24`, 22 → `^22.22 || ^24.13` (à confirmer sur angular.dev). Majeur inconnu ⇒ nil (pas de blocage).
**Tests :** `TestAngular20Node2212OK` · `TestAngular20Node18Rejected` · `TestUnknownAngularMajorAllowed`.

### T1.9 Thème et rendu texte
**Fichiers :** `internal/ui/theme.go`, `internal/ui/render.go`, `internal/ui/render_test.go`.
**Produit :** `type Theme struct{…}` ; `func NewTheme(color bool) Theme` ; `func ColorEnabled(isTTY bool, getenv func(string) string) bool` (faux si `NO_COLOR` défini ou pas de TTY) ; `func (t Theme) StepLine(st plan.Status, title, detail string) string` (symboles `✓` Done, `↷` Skipped, `✗` Failed, `⊘` Blocked) ; `func (t Theme) Banner(version string) string` ; `func (t Theme) Summary(s plan.Summary, logPath string) string`.
**Consomme :** `plan.Status`, `plan.Summary` (T1.7). Les branches n'étant pas chaînées, **cette tâche part avec le lot 2** (T1.7 sera alors mergé) ; elle garde son numéro pour la traçabilité.
**Tests :** `TestNoColorEnv` · `TestStepLineSymbolsPlain` (sans couleur : `"✓ Java 21"`) · `TestSummaryListsFailuresAndLog`.

### T1.10 Masquage des secrets et journal
**Fichiers :** `internal/logx/logx.go`, `internal/logx/logx_test.go`.
**Produit :** `func New(path string) (*Logger, error)` ; `func (l *Logger) AddSecret(s string)` ; `func (l *Logger) Printf(format string, a ...any)` ; `func Redact(s string, secrets []string) string` ; masque aussi les motifs `ghp_…`, `github_pat_…`, `hvs.…`, `_authToken=…`, `password=…`.
**Tests :** `TestRedactRegistered` · `TestRedactGitHubPatterns` · `TestRedactNpmAuthToken` · `TestLoggerNeverWritesSecret` (relit le fichier).

### T1.11 Réponses de l'assistant
**Fichiers :** `internal/profile/answers.go`, `internal/profile/answers_test.go`.
**Produit :**
```go
type Answers struct { Profile string; Roles []string; Editor string; Versions map[string]string; Proxy, ProxyAuth string; NoProxy, CACerts []string; GitName, GitEmail string; GenerateSSH bool; Repos []string; Workspace string; Podman bool; Vault bool }
func LoadAnswers(path string) (*Answers, error)
func (a *Answers) Save(path string) error
func Defaults(p *Profile, home string) *Answers // Workspace = home\workspace\<p.Name>
```
**Tests :** `TestAnswersRoundTrip` · `TestAnswersHaveNoSecretFields` (réflexion : aucun champ dont le nom contient Token, Password, Secret) · `TestDefaultsFromDemoProfile`.

---

## Lot 2 — Windows, sources de versions, éditeurs de config (dépend du lot 1)

Chaque source implémente :
```go
// internal/sources/sources.go (T2.1)
type Release struct { Version string; LTS bool; URL string; Checksum download.Checksum; Archive string } // Archive ∈ {"zip","tar.gz","exe","msi"}
type Source interface { Name() string; Releases(ctx context.Context) ([]Release, error) }
type Fetcher interface { GetJSON(ctx context.Context, url string, v any) error; GetText(ctx context.Context, url string) (string, error) }
func Pick(rs []Release, want string) (Release, error) // "21" ⇒ plus récente 21.x ; "21.0.4" exact ; "lts" ⇒ première LTS
func Restrict(rs []Release, allowed []string) []Release
```
Chaque source prend `baseURL` en paramètre et est testée sur un fixture JSON/texte dans `internal/sources/testdata/` (pas de réseau). Tests communs à chaque source : versions triées décroissantes, URL Windows x64, empreinte renseignée quand l'éditeur la publie.

| Tâche | Fichiers | Source et détail |
|---|---|---|
| T2.1 | `sources.go`, `sources_test.go` | types ci-dessus, `Pick`, `Restrict` (`TestPickMajor`, `TestPickExact`, `TestPickLTS`, `TestPickMissing`) ; `download.Client` doit satisfaire `Fetcher` (ajouter `GetText` dans `internal/download/download.go`) |
| T2.2 | `adoptium.go`, test, `testdata/adoptium-21.json` | `api.adoptium.net/v3/assets/feature_releases/{v}/ga?os=windows&architecture=x64&image_type=jdk` ; SHA-256 fourni |
| T2.3 | `node.go`, test, `testdata/node-index.json` + `testdata/node-shasums.txt` | `nodejs.org/dist/index.json` (champ `lts`), zip `win-x64`, `SHASUMS256.txt` |
| T2.4 | `npmreg.go`, test, `testdata/npm-angular-cli.json` | registre npm (ou URL Nexus npm) : versions de `@angular/cli` et `npm`, tags `latest` |
| T2.5 | `python.go`, test, `testdata/python-ftp.html` | `python.org/ftp/python/` : installeur `amd64.exe` per-user ; méthode d'install choisie et documentée en commentaire |
| T2.6 | `apache.go`, test, `testdata/maven-meta.xml` | Maven : `maven-metadata.xml` + `.sha512` ; Gradle : `services.gradle.org/versions/all` (`sha256` des zips `-bin`) |
| T2.7 | `github.go`, test, `testdata/gh-podman.json` | releases GitHub génériques `NewGitHub(name, repo, assetPattern, sumsAsset string)` : podman (`containers/podman`, zip Windows), compose (`docker/compose`, `docker-compose-windows-x86_64.exe` + `.sha256`), px (`genotrance/px`, zip windows-amd64) |
| T2.8 | `hashicorp.go`, test, `testdata/vault-index.json` | `releases.hashicorp.com/vault/index.json`, zip `windows_amd64`, `SHA256SUMS` |
| T2.9 | `editors.go`, test, `testdata/jetbrains.json` | VS Code `update.code.visualstudio.com/api/update/win32-x64-archive/stable/latest` ; IntelliJ `data.services.jetbrains.com/products/releases?code=IIC&latest=true&type=release` (zip Windows + checksum) |

| Tâche | Fichiers | Contenu |
|---|---|---|
| T2.10 | `internal/winenv/env_windows.go`, `env_other.go`, `env_windows_test.go` | `Env` + `Links` réels : HKCU\Environment (`REG_EXPAND_SZ` pour Path), `SendMessageTimeout(HWND_BROADCAST, WM_SETTINGCHANGE, "Environment")`, jonction via `cmd /c mklink /J`, PATH machine en lecture seule ; `env_other.go` renvoie une erreur « Windows uniquement ». Test Windows (CI) : variable `DEVSTATION_TEST_VAR` posée puis supprimée. |
| T2.11 | `internal/winenv/certs_windows.go`, `creds_windows.go`, `secrets_other.go` | `CertStore` (CertOpenStore CurrentUser\ROOT, `CertAddEncodedCertificateToStore` ; présence par empreinte SHA-1) ; `Creds` via `wincred` (cible `DevStation/<nom>`). |
| T2.12 | `internal/config/kv.go`, `kv_test.go` | `func SetKeys(path string, kv map[string]string, st *state.State) error` pour `.npmrc` / `gradle.properties` / `px.ini`-like : lignes `clé=valeur`, commentaires et clés non gérées intacts, `.devstation.bak` au premier passage, `RecordKey` avec l'ancienne valeur ; `func RestoreKeys(path string, st *state.State) error`. Tests : `TestPreservesUnmanagedBytes` (fichier avec `_authToken` et commentaires), `TestBackupOnlyOnce`, `TestRestoreRemovesAddedKeys`, `TestRestoreKeepsLaterUserEdits`. |
| T2.13 | `internal/config/ini.go`, `ini_test.go` | même contrat avec sections : `SetINI(path, section string, kv map[string]string, st *state.State) error`, `RestoreINI` (pip.ini `[global]`, px.ini `[proxy]`). |
| T2.14 | `internal/config/maven.go`, `maven_test.go` | `SetMavenSettings(path string, s MavenSettings, st *state.State) error` avec `type MavenSettings struct { MirrorURL, ProxyHost string; ProxyPort int; NonProxyHosts string }` : ajoute/remplace uniquement `<mirror id="devstation">` et `<proxy id="devstation">`, conserve le reste du XML ; `RestoreMavenSettings`. |

---

## Lot 3 — Installeurs, réseau, git, Vault (dépend du lot 2)

```go
// internal/install/install.go (T3.1)
type Deps struct { DL *download.Client; Env winenv.Env; Links winenv.Links; Certs winenv.CertStore; Creds winenv.Creds; Run execx.Runner; State *state.State; Log *logx.Logger; Root string /* …\DevStation\tools */ }
type Installer interface {
  Tool() string
  Install(ctx context.Context, d Deps, r sources.Release, report func(string, int64, int64)) error
  Activate(d Deps, version string) error   // déplace la jonction current + variables
  Verify(ctx context.Context, d Deps) (version string, err error)
}
func InstallZip(ctx context.Context, d Deps, tool string, r sources.Release, stripTop bool, report func(string, int64, int64)) (dir string, err error) // fetch → unzip dans Root\tool\version → RecordInstall ; déjà installé ⇒ no-op
func SetCurrent(d Deps, tool, version string, binSubdirs []string) error // jonction + PrependPath du PATH utilisateur + Broadcast
func Steps(d Deps, a *profile.Answers, p *profile.Profile, pick map[string]sources.Release) []plan.Step
```
Tests de chaque installeur avec `winenv.Fake`, `execx.Fake` et un serveur httptest servant un zip construit dans le test.

| Tâche | Fichiers (`internal/install/`) | Attendu testé |
|---|---|---|
| T3.1 | `install.go`, `install_test.go` | `TestInstallZipIdempotent` (2ᵉ appel : aucune requête HTTP), `TestSetCurrentPathAndJunction` |
| T3.2 | `java.go`, test | `JAVA_HOME` = `…\java\current` ; import de chaque CA via `keytool -importcert -noprompt -cacerts -alias devstation-<n>` (Fake vérifie les arguments) ; `Verify` lit `java -version` |
| T3.3 | `node.go`, test | `npm config set prefix <Root>\node\global` ; npm épinglé ⇒ `npm i -g npm@<v>` ; Angular CLI ⇒ `npm i -g @angular/cli@<v>` ; `NODE_EXTRA_CA_CERTS` |
| T3.4 | `python.go`, test | installeur per-user silencieux (`InstallAllUsers=0 PrependPath=0 Include_launcher=0 TargetDir=…`) ; pip.ini via `config.SetINI` |
| T3.5 | `buildtools.go`, test | Maven + Gradle (zip) ; `settings.xml` via `config.SetMavenSettings` ; `gradle.properties` proxy via `config.SetKeys` |
| T3.6 | `editors.go`, test | VS Code archive zip **sans** dossier `data` (profil utilisateur standard, pas de mode portable), `bin` ajouté au PATH, et `code --install-extension <id> --force` selon `EditorExtensions[editor][rôle]` + `common` ; IntelliJ zip + raccourci menu Démarrer utilisateur ; plugins IntelliJ du profil via `idea64.exe installPlugins <id…>` (meilleur effort : échec = avertissement) |
| T3.7 | `containers.go`, test | podman zip + docker-compose `.exe` renommé `docker-compose.exe` dans `…\podman\current\bin` ; `podman machine init --now` si aucune machine ; `podman machine ssh` pour copier les CA et écrire `registries.conf` si Nexus `containers` |
| T3.8 | `vaultcli.go`, test | zip vault ; `VAULT_ADDR`, `VAULT_CACERT` ; `~/.vault` avec `token_helper = "<bin>\\vault-token-helper.cmd"` ; `.cmd` = `@"%~dp0devstation.exe" vault-token-helper %*` |

| Tâche | Fichiers | Attendu testé |
|---|---|---|
| T3.9 | `internal/network/network.go`, test | `func Effective(n profile.Network) (proxyURL string, usePx bool)` (kerberos ⇒ `http://127.0.0.1:3128`) ; `func Apply(d install.Deps, n profile.Network, nexus profile.Nexus) error` : variables `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`, `.npmrc` (`proxy`, `https-proxy`, `registry`, `cafile`), pip.ini, git (`http.proxy`, `http.sslBackend schannel`), CA dans CurrentUser\Root (déjà présent ⇒ sauté). Tests sur Fake pour `none` et `kerberos`. |
| T3.10 | `internal/network/px.go`, test | install px (zip), `px.ini` (`[proxy] server=<amont> port=3128 noproxy=<…> listen=127.0.0.1` ; si Podman : `gateway=1` + `allow=<sous-réseau WSL>`), valeur HKCU `Software\Microsoft\Windows\CurrentVersion\Run\DevStationPx` ; `func PxRunning(addr string) bool` (dial TCP 500 ms). |
| T3.11 | `internal/gitx/gitx.go`, test | `Configure(ctx, r execx.Runner, name, email, autocrlf string) error` ; `EnsureSSHKey(ctx, r, home, email string) (pubPath string, created bool, err error)` (ne réécrit jamais une clé existante) ; `StoreToken(ctx, r, token string) error` = `git credential approve` avec stdin `protocol=https\nhost=github.com\nusername=x-access-token\npassword=<token>\n\n` et `Secret: true` ; `Clone(ctx, r, url, dir string) (cloned bool, err error)` (dossier existant ⇒ `false`, nil). |
| T3.12 | `internal/vaultx/vaultx.go`, test | `TokenHelper(args []string, stdin io.Reader, stdout io.Writer, c winenv.Creds) error` (get/store/erase, cible `DevStation/vault-token`) ; `Login(ctx, r, method, role string) error` (`vault login -method=<m> role=<r>` interactif) ; `List(ctx, r, path string) ([]string, error)` ; `Get(ctx, r, path, field string) (string, error)` (`vault kv get -format=json`) ; `TTL(ctx, r) (time.Duration, error)` (`vault token lookup -format=json`). |

---

## Lot 4 — Diagnostic, assistant, commandes (dépend du lot 3)

| Tâche | Fichiers | Attendu testé |
|---|---|---|
| T4.1 | `internal/doctor/doctor.go`, test | `type Check struct { Name string; Status string /* ok|warn|fail */; Detail, Fix string }` ; `func Run(ctx, d install.Deps, a *profile.Answers, p *profile.Profile) []Check` : version de chaque outil vs `Answers.Versions`, `winenv.Shadowing` pour `java.exe`/`node.exe`/`python.exe` (warn + Fix en français), px joignable si kerberos, Nexus joignable (HEAD), Vault TTL > 0, machine Podman `Running`. Tests : `TestShadowingWarns`, `TestPxDownFails`, `TestAllGreen`. |
| T4.2 | `internal/ui/wizard.go`, `wizard_test.go` | `func Wizard(p *profile.Profile, a *profile.Answers, versions map[string][]sources.Release, accessible bool) (*profile.Answers, error)` (huh, 9 étapes du §4.2, fil d'Ariane `n/9 · Titre`, token GitHub masqué et facultatif, renvoyé à part : `func AskToken() (string, error)`) ; test en mode `accessible` (entrées scriptées) qui produit des `Answers` attendues. |
| T4.3 | `internal/ui/progress.go`, test | modèle bubbletea qui consomme `plan.Event` (spinner, barre de progression, lignes `StepLine`) ; mode texte simple si pas de TTY ; test du mode texte. |
| T4.4 | `cmd/devstation/main.go`, `cmd/devstation/setup.go`, `cmd/devstation/setup_test.go` | cobra : racine = `setup` ; `setup --answers <f> --yes --profile <f> --no-color --verbose --log <f>` ; enchaîne profil → sources → `Wizard` (sauf `--yes`) → récapitulatif → `install.Steps` → `plan.Execute` → `Summary` → sauvegarde `answers.yaml` / `state.json` ; secrets `--yes` lus dans `DEVSTATION_GITHUB_TOKEN` (optionnel) ; si `Answers.Vault` et `profile.Vault.Secrets` : étape « Coffre » (`vaultx.Login`) puis chaque `SecretRef` lu par `vaultx.Get` et rangé via `Creds.Set("DevStation/nexus", …)` avant `network.Apply` (qui lit ces identifiants s'ils existent) ; Nexus `auth: true` sans Vault ⇒ saisie masquée. Test : `--yes` avec Fakes produit les étapes attendues, dans l'ordre réseau → coffre → outils → git → dépôts → conteneurs. |
| T4.5 | `cmd/devstation/tools.go`, test | `doctor`, `use <outil> <version>`, `list [outil]`, `version` |
| T4.6 | `cmd/devstation/manage.go`, test | `repos sync`, `uninstall [outil|--all]` (Restore* puis suppression des dossiers et entrées PATH, `state.json` mis à jour), `secrets list|get [--copy]`, `vault-token-helper` (caché) |

## Lot 5 — Démarrage, livraison, documentation (dépend du lot 4)

| Tâche | Fichiers | Attendu testé |
|---|---|---|
| T5.1 | `bootstrap/DevStation.Bootstrap.psm1`, `tests/ps/Bootstrap.Tests.ps1` | fonctions `Test-Prerequisites`, `Get-PinnedRelease` (lit `$DevStationVersion`), `Test-FileHash256 -Path -Expected`, `Invoke-Download -Url -Out [-Proxy] [-UseDefaultCredentials]`, `Start-Px` ; Pester avec `Mock` (tourne sous pwsh Linux) : empreinte fausse ⇒ fichier supprimé + erreur ; échec direct ⇒ nouvel essai avec proxy et `-ProxyUseDefaultCredentials`. |
| T5.2 | `setup.ps1`, `tests/ps/Setup.Tests.ps1` | PowerShell 5.1, `$DevStationVersion = "0.1.0"`, importe le module, `-Dev` ⇒ `go run ./cmd/devstation`, transmet `$args` ; test : `-Dev` appelle `go run` (Mock). |
| T5.3 | `.github/workflows/release.yml`, `scripts/checksums.ps1` | sur tag `v*` : build `windows/amd64` + `arm64` (`-ldflags "-X main.version=<tag>"`), `checksums.txt`, release GitHub. Vérifié en CI, pas dans le sandbox. |
| T5.4 | `README.md`, `docs/profil-commente.yaml` | contenu §10 de la spec ; vérifié en relecture. |

---

## Dépôt `demo-backend` (après P3)

| Tâche | Fichiers | Attendu testé |
|---|---|---|
| B1 | `GreetingService.java`, `GreetingController.java`, `GreetingControllerTest.java` | `GET /api/greeting?name=Ana` ⇒ `{"message":"Bonjour, Ana","secretLoaded":false}` ; `name` absent ⇒ `"Bonjour, inconnu"` ; `name` > 50 caractères ⇒ 400 ; CORS `http://localhost:4201` (`@WebMvcTest`). |
| B2 | `src/main/resources/application.properties` (déjà : `spring.cloud.vault.enabled=${VAULT_ENABLED:false}`, port 8081), `VaultSecretHolder.java`, `VaultSecretHolderTest.java` | Spring Cloud Vault, `spring.cloud.vault.enabled=${VAULT_ENABLED:false}`, secret `secret/demo-backend` clé `api-key` ; `secretLoaded=true` quand la propriété est présente (test avec propriété injectée, Vault désactivé). |
| B3 | `Containerfile`, `compose.yaml`, `vault/init.sh` | multi-étapes Temurin 21 ; services `vault` (`hashicorp/vault`, mode dev, token fictif `devstation-demo`), `vault-init` (écrit `secret/demo-backend api-key=demo-not-a-secret`), `backend` (`VAULT_ENABLED=true`), `frontend` (contexte `../demo-frontend`, port `8080`). Vérifié par la CI (`podman compose config` / build) et manuellement. |

## Dépôt `demo-frontend` (après P4)

| Tâche | Fichiers | Attendu testé |
|---|---|---|
| F1 | `src/app/greeting.service.ts`, `src/app/greeting.component.ts`, `src/app/greeting.component.spec.ts` | appelle `/api/greeting?name=<saisie>` (HttpClient), affiche `message` et « Secret Vault chargé : oui/non » ; état d'erreur « Backend injoignable » ; tests avec `HttpTestingController`. |
| F2 | `federation.config.mjs`, `proxy.conf.json`, `src/app/app.config.ts` | expose `./Greeting` (composant F1) ; `ng serve` port 4201 avec proxy `/api` → `http://localhost:8081`. Test : `federation.config.mjs` exporte `./Greeting` (spec Jasmine qui lit le manifeste généré, ou relecture). |
| F3 | `Containerfile`, `nginx.conf` | build Node → nginx ; `location /api/ { proxy_pass http://backend:8081; }` ; sert l'app sur 8080. Vérifié par la CI et `podman compose up`. |

---

## Ordre d'exécution

1. Préparation P1–P4 (Claude, avec ton accord pour P1).
2. `demo-backend` B1–B3 puis `demo-frontend` F1–F3 : petits, ils valident la chaîne agentique Java/Angular.
3. `devstation` lots 1 → 5, chaque lot mergé avant le suivant.
4. Essai de bout en bout sur un Windows réel (ou Windows Sandbox) : `git clone` → `.\setup.ps1` → `podman compose up` → `http://localhost:8080`.
