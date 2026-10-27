# DevStation — conception

Date : 2026-10-27 · Auteur : A-Djihadi (conception menée avec Claude)

## 1. Intention

Monter un poste de dev Windows pour un nouveau projet prend du temps et se fait de mémoire :
choisir l'éditeur, installer Java / Node / Angular / Python dans les bonnes versions, poser les
certificats d'entreprise, brancher le Nexus, configurer git, cloner les dépôts, installer de quoi
lancer des conteneurs. DevStation rend cela reproductible :

```powershell
git clone https://github.com/A-Djihadi/devstation.git
cd devstation
.\setup.ps1
```

Une CLI belle et guidée pose les questions utiles (front ? back ? quel éditeur ? quelles
versions ?), télécharge depuis les sources officielles, installe, configure, clone les dépôts du
projet, et laisse un poste prêt à coder. Les réponses sont sauvegardées pour rejouer
l'installation ailleurs ou la partager à l'équipe.

**Succès** : un dev sur un Windows neuf (avec Git et WSL2 déjà présents) passe de `git clone` à
« le backend et le frontend de démo tournent en local, dans Podman, et s'ouvrent dans mon
éditeur » en une seule session guidée, sans droits administrateur et sans lire de doc annexe.

## 2. Objectifs, non-objectifs, anti-objectifs

### Objectifs (v1)
- Windows 10/11 x64, PowerShell 5.1 (présent partout) pour le démarrage.
- Installation **sans droits admin**, tout dans le profil utilisateur.
- Choix interactif : profil (front / back / fullstack), éditeur (VS Code ou IntelliJ IDEA),
  versions de Java, Node/npm, Angular CLI, Python, outil de build (Maven et/ou Gradle).
- Contexte entreprise : proxy, certificats CA, dépôt de binaires (Nexus) propagés à tous les outils.
- Git : identité, `core.autocrlf`, clé SSH optionnelle, token GitHub stocké de façon sûre.
- Clonage des dépôts du projet listés dans le profil d'équipe.
- Podman (client Windows + machine WSL2) pour builder et lancer les conteneurs en local et
  rejouer la CI/CD.
- Extensions d'éditeur selon le profil.
- Commandes : assistant, `doctor`, changement de version, désinstallation, rejeu non interactif.
- Deux dépôts de démo : `demo-backend` (Java) et `demo-frontend` (Angular, micro-frontend).

### Non-objectifs (v1)
- macOS / Linux (l'architecture ne doit pas l'empêcher plus tard).
- Installer ou activer WSL2 (prérequis documenté dans le README).
- Installer Git (prérequis : il a servi au `git clone`).
- Créer des dépôts ou quoi que ce soit sur GitHub : la CLI ne fait que cloner (et rend possible le push).
- Gérer les licences IntelliJ Ultimate.

### Anti-objectifs (ce qui serait un échec même si « ça marche »)
- Exiger les droits admin ou modifier la configuration machine (HKLM, PATH système, store machine).
- Écraser silencieusement une config existante de l'utilisateur (`.npmrc`, `settings.xml`,
  `pip.ini`, `.gitconfig`…) : toujours sauvegarder avant et ne toucher que les clés gérées.
- Laisser fuiter un secret (token GitHub, mot de passe Nexus) dans un log, un fichier d'état,
  la sortie console ou un dépôt.
- Une relance qui casse quelque chose ou réinstalle tout : la CLI est idempotente.
- Télécharger depuis des miroirs non officiels ou sans vérifier l'intégrité.
- Une sortie illisible, des murs de texte, ou une CLI qui fige sans dire ce qu'elle fait.

## 3. Contraintes

- Les trois dépôts (`devstation`, `demo-backend`, `demo-frontend`) sont **publics** (décision
  utilisateur) : téléchargement de la release et clonage sans authentification ; le token GitHub
  n'est demandé que pour le push.
- Le proxy d'entreprise impose **Kerberos** (décision utilisateur) : voir §6.4.

- **Hybride décidé** : script PowerShell de démarrage + CLI écrite en **Go**.
- **Implémentation par les agents DeepSeek** via `agentic-coder` ; Claude planifie
  (`/plan-tasks`) et relit les PR ; l'humain merge. Voir §9.
- Aucun secret dans le code ni dans les dépôts ; configuration via fichiers de profil et
  variables d'environnement (convention `agentic-coder`).
- Le code part chez DeepSeek : le profil d'exemple ne contient **aucune URL ni donnée réelle
  d'entreprise** (Nexus, proxy et CA fictifs du type `nexus.example.com`).

## 4. Expérience utilisateur

### 4.1 Démarrage (`setup.ps1`)
1. Vérifie Windows x64, PowerShell ≥ 5.1, Git présent, WSL2 disponible (avertit seulement si absent :
   Podman sera sauté).
2. Si `ExecutionPolicy` bloque, le README donne la commande
   `powershell -ExecutionPolicy Bypass -File .\setup.ps1`.
3. Télécharge `devstation.exe` à la version **épinglée dans le script** depuis les Releases GitHub
   de `A-Djihadi/devstation`, vérifie son SHA-256 contre `checksums.txt` de la même release,
   le place dans `%LOCALAPPDATA%\DevStation\bin`.
4. Si le téléchargement direct échoue, demande un proxy, réessaie avec les identifiants Windows
   (Kerberos, §6.4), installe et lance px si besoin, puis transmet la config à la CLI.
5. Mode dev : si `-Dev` est passé et que Go est présent, lance `go run ./cmd/devstation` à la place.
6. Lance `devstation setup` en transmettant les arguments reçus.

### 4.2 L'assistant (`devstation setup`)
Écran d'accueil (logo, version), puis des étapes avec un fil d'Ariane visible
(« 3/9 · Versions ») :

1. **Profil d'équipe** : choisir un fichier dans `profiles/` (défaut `profiles/demo.yaml`) ou un chemin/URL.
2. **Rôle** : Front · Back · Fullstack (multi-sélection possible : + Data/Python).
3. **Éditeur** : VS Code · IntelliJ IDEA · les deux.
4. **Versions** : listes alimentées en direct par les sources officielles, filtrées / pré-sélectionnées
   par le profil (§5.2). Java (LTS en tête), Node (LTS en tête) + version de npm, Angular CLI, Python,
   Maven / Gradle. Incompatibilités signalées (ex. Angular X exige Node ≥ Y).
5. **Réseau d'entreprise** : proxy (avec exclusions), certificats CA (fichiers PEM), Nexus (URLs par
   écosystème). Pré-remplis par le profil, modifiables.
6. **Git** : nom, email (pré-remplis depuis la config existante), génération d'une clé SSH ed25519
   optionnelle, token GitHub **optionnel** (saisie masquée, « passer » possible) : les trois dépôts
   sont publics, le token ne sert qu'à pousser.
7. **Dépôts** : cases à cocher des dépôts du profil, dossier de travail (défaut `%USERPROFILE%\workspace\<projet>`).
8. **Conteneurs** : installer Podman et initialiser la machine WSL (oui / non).
9. **Récapitulatif** : tout ce qui sera fait, avec confirmation.

Exécution : une ligne par étape avec spinner puis ✓ / ✗ / ↷ (déjà fait), barre de progression
des téléchargements (taille, débit). Un échec n'arrête pas tout : on continue ce qui ne dépend pas
de l'étape échouée, et le bilan final liste les erreurs avec le chemin du log.

Fin : bilan, prochaines commandes (`cd …`, `podman compose up`, ouvrir l'éditeur), rappel
« ouvre un nouveau terminal pour que le PATH soit pris en compte ».

### 4.3 Commandes

| Commande | Rôle |
|---|---|
| `devstation setup` | assistant (défaut sans argument) |
| `devstation setup --answers <fichier> --yes` | rejeu non interactif (secrets demandés ou lus en variable d'env.) |
| `devstation doctor` | vérifie chaque outil, version, PATH, CA, proxy, Nexus joignable, Podman ; dit quoi faire |
| `devstation use <outil> <version>` | change de version active (installe si besoin) — java, node, python, maven, gradle |
| `devstation list [outil]` | versions installées / disponibles |
| `devstation repos sync` | clone les dépôts manquants du profil |
| `devstation uninstall [outil\|--all]` | retire ce que DevStation a installé et restaure les configs sauvegardées |
| `devstation version` | version de la CLI |

Options globales : `--no-color`, `--verbose`, `--log <fichier>`. Sans TTY (CI, redirection), la sortie
devient du texte simple sans animation.

### 4.4 Rendu
Go + bibliothèques Charm : `huh` (formulaires), `lipgloss` (styles), `bubbles` (spinner, progression).
Cible : Windows Terminal, avec rendu correct dans la console classique (conhost) et respect de `NO_COLOR`.
Palette sobre, textes en **français**.

## 5. Données

### 5.1 Profil d'équipe (`profiles/*.yaml`, versionné)
Décrit ce qu'une équipe impose ou propose. Exemple `profiles/demo.yaml` (valeurs fictives) :

```yaml
name: demo
description: Projet de démonstration Java + Angular
defaults:
  roles: [fullstack]
  editor: vscode
tools:
  java:   { versions: ["21", "17"], default: "21", distribution: temurin }
  node:   { versions: ["22", "20"], default: "22", npm: "bundled" }
  angular_cli: { default: "20" }
  python: { default: "3.12", optional: true }
  maven:  { default: "3.9" }
  gradle: { enabled: false }
network:
  proxy: ""                      # ex. http://proxy.example.com:8080
  proxy_auth: kerberos           # none | kerberos (relais local px)
  no_proxy: [localhost, 127.0.0.1, .example.com]
  ca_certs: []                   # chemins PEM relatifs au profil, ex. certs/corp-root.pem
  nexus:
    maven: ""                    # ex. https://nexus.example.com/repository/maven-public/
    npm: ""
    pypi: ""
    containers: ""
git:
  autocrlf: "true"
repos:
  - { name: demo-backend,  url: https://github.com/A-Djihadi/demo-backend.git,  roles: [back, fullstack] }
  - { name: demo-frontend, url: https://github.com/A-Djihadi/demo-frontend.git, roles: [front, fullstack] }
editor_extensions:
  vscode:
    common: [eamodio.gitlens, redhat.vscode-yaml]
    back:   [vscjava.vscode-java-pack, vmware.vscode-boot-dev-pack]
    front:  [angular.ng-template, dbaeumer.vscode-eslint, esbenp.prettier-vscode]
    python: [ms-python.python]
  intellij:
    common: []
```

Le schéma exact est à la main du builder ; il doit être validé au chargement avec des messages
d'erreur clairs (ligne, clé). Le README contient un profil commenté.

### 5.2 Sources des versions
Listes en direct depuis les sources officielles (le profil restreint ou épingle) :
Java → API Adoptium (Temurin) ; Node → `nodejs.org/dist/index.json` ; npm / Angular CLI → registre npm
(ou le Nexus npm s'il est configuré) ; Python → python.org ; Maven → `archive.apache.org` / Maven
Central ; Gradle → `services.gradle.org` ; VS Code → `update.code.visualstudio.com` (build *user*) ;
IntelliJ → API de téléchargement JetBrains ; Podman → Releases GitHub `containers/podman`.
Si une source est injoignable, se replier sur les versions du profil.

### 5.3 État local (`%USERPROFILE%\.devstation\`)
- `answers.yaml` : réponses de l'assistant (rejouable, partageable), **sans aucun secret**.
- `state.json` : ce que DevStation a installé (outil, version, chemin, date), configs modifiées et
  emplacement de leur sauvegarde — sert à l'idempotence, `doctor` et `uninstall`.
- Logs : `%LOCALAPPDATA%\DevStation\logs\<date-heure>.log`, avec masquage des secrets.

## 6. Installation et configuration

### 6.1 Disposition
`%LOCALAPPDATA%\DevStation\tools\<outil>\<version>\` + un lien `current` (jonction NTFS, sans admin)
vers la version active. Le PATH **utilisateur** (HKCU\Environment) reçoit les dossiers `current\bin`,
`JAVA_HOME` etc. pointent sur `current` ; diffusion de `WM_SETTINGCHANGE` après modification.
`use` ne fait que déplacer la jonction.

Piège à traiter dans `doctor` : sous Windows le PATH machine passe **avant** le PATH utilisateur ; un
Java ou un Node installé par l'admin masquera le nôtre → le détecter et l'expliquer.

### 6.2 Téléchargements
HTTPS uniquement, proxy et CA honorés, reprise ou nouvelle tentative (3 essais), cache dans
`%LOCALAPPDATA%\DevStation\cache`. Vérification de l'empreinte publiée par l'éditeur dès qu'elle
existe (SHA-256 / SHA-512) ; sinon l'écrire dans le log comme « non vérifiée ». Les archives zip
sont préférées aux installeurs ; un installeur n'est utilisé qu'en mode *per-user* silencieux.

### 6.3 Par outil (comportement attendu)
- **Java** : JDK Temurin zip. `JAVA_HOME`. CA importés dans le truststore du JDK (`keytool`).
- **Node** : zip officiel ; version de npm épinglée si le profil le demande ; `npm` prefix global
  dans le dossier DevStation ; Angular CLI installé globalement dans ce prefix.
- **Python** : installation per-user sans admin (choix de la méthode laissé au builder) ; `pip`.
- **Maven / Gradle** : zips officiels ; `~/.m2/settings.xml` (miroir Nexus, proxy) ;
  `~/.gradle/gradle.properties` (proxy).
- **VS Code** : build *user* (zip ou installeur per-user silencieux) + `code --install-extension`.
- **IntelliJ IDEA** : distribution zip/portable ; plugins si le profil en liste (meilleur effort).
- **Podman** : client Windows sans admin ; `podman machine init` + `start` sur WSL2 ; CA copiés dans
  la machine ; registre Nexus éventuel dans `registries.conf` ; `podman compose` utilisable.
  *Choix de conception (Claude)* : le fournisseur de `podman compose` est le binaire
  **docker-compose** officiel (Releases GitHub `docker/compose`, empreinte vérifiée), placé dans le
  PATH DevStation — pas `podman-compose`, pour ne pas rendre Python obligatoire.
- **Git** : `user.name`, `user.email`, `core.autocrlf`, `http.proxy`, CA (`http.sslBackend schannel` ou
  `http.sslCAInfo`) ; clé SSH ed25519 dans `~/.ssh` si demandée (sans écraser une clé existante) ;
  token GitHub enregistré via `git credential approve` dans **Git Credential Manager**
  (fourni avec Git for Windows) — jamais écrit ailleurs.

### 6.4 Réseau d'entreprise, propagation
Une seule saisie, appliquée à : variables `HTTP(S)_PROXY` / `NO_PROXY` utilisateur, git, npm
(`.npmrc` : `proxy`, `https-proxy`, `registry`, `cafile`), Maven, Gradle, pip (`pip.ini` : `proxy`,
`index-url`, `cert`), Podman, `NODE_EXTRA_CA_CERTS`. Les CA sont aussi ajoutés au magasin
**CurrentUser\Root** de Windows (pas d'admin ; Windows affiche une confirmation à l'utilisateur).
Chaque fichier modifié est sauvegardé (`<fichier>.devstation.bak`) et seules les clés gérées sont
touchées.

*Choix de conception (Claude)* — sauvegarde et restauration au niveau **clé** : `state.json`
enregistre, pour chaque clé gérée, sa valeur d'origine (ou son absence). `uninstall` remet ces
valeurs et laisse intactes les autres modifications de l'utilisateur. Le `.bak` complet est écrit
**une seule fois** (premier passage) et n'est jamais écrasé : c'est un filet de sécurité manuel,
pas le mécanisme de restauration.

*Choix de conception (Claude)* — l'ajout au magasin CurrentUser\Root déclenche une confirmation
Windows par certificat ; elle reste affichée même en rejeu `--yes` (impossible à contourner sans
admin, et c'est voulu). Un certificat déjà présent est sauté, donc la confirmation n'apparaît
qu'au premier passage.

**Proxy authentifié par Kerberos (décision utilisateur).** Le proxy d'entreprise exige Kerberos.
npm, Maven, pip, Podman et le client HTTP Go ne le gèrent pas (ou mal) : DevStation installe donc
**px** (relais proxy local, `genotrance/px`, binaire Windows officiel, empreinte vérifiée), qui
s'authentifie avec la session Windows de l'utilisateur via SSPI — **aucun mot de passe saisi ni
stocké**. Tous les outils pointent vers `http://127.0.0.1:3128` ; px relaie vers le proxy amont et
applique `no_proxy`. Profil : `network.proxy_auth: none | kerberos`.
- Démarrage : avant que px existe, `setup.ps1` télécharge `devstation.exe` **et** px avec
  `Invoke-WebRequest -Proxy … -ProxyUseDefaultCredentials` (PowerShell sait faire Kerberos), lance px,
  puis la CLI qui passe par px dès sa première requête.
- px est lancé à l'ouverture de session (raccourci dans le dossier Démarrage utilisateur, sans admin)
  et `doctor` vérifie qu'il tourne et qu'il obtient bien un ticket.
- La machine Podman (WSL2) doit aussi joindre px : px écoute alors sur l'interface atteignable depuis
  WSL avec une liste d'autorisation restreinte au sous-réseau WSL (ou mode réseau WSL « mirrored »
  s'il est actif). Le mécanisme exact est laissé au builder, le résultat attendu ne l'est pas :
  `podman pull` et les builds d'images passent par le proxy.
- `proxy_auth: none` : comportement simple, l'URL du proxy est écrite directement dans chaque outil.

Authentification Nexus : v1 = lecture anonyme ; si le profil déclare `auth: true`, identifiant +
mot de passe demandés et stockés dans le Gestionnaire d'identifiants Windows, puis injectés là où
l'outil l'exige (le builder choisit le mécanisme le moins exposé par outil et le documente).

## 7. Dépôts de démo

Deux dépôts séparés publiés par l'utilisateur sur son GitHub ; DevStation ne fait que les cloner,
côte à côte dans le dossier de travail.

- **demo-backend** : Java 21, Spring Boot 3, Maven Wrapper. `GET /api/greeting?name=…` → JSON
  `{ "message": "Bonjour, …" }`, `GET /actuator/health`. CORS pour le frontend en dev. Tests JUnit
  (contrôleur + service). `Containerfile` multi-étapes. Workflow GitHub Actions (build + test + image).
- **demo-frontend** : Angular (version par défaut du profil), exposé en **micro-frontend remote**
  (Native Federation) et utilisable aussi seul ; un composant appelle `/api/greeting` (proxy de dev
  vers le backend) et affiche la réponse. Tests unitaires. `Containerfile` (build Node → nginx qui
  sert l'app et relaie `/api` au backend). Workflow GitHub Actions.
- **Stack locale** : `compose.yaml` dans `demo-backend` (contextes de build `.` et `../demo-frontend`)
  lançant les deux conteneurs ; `podman compose up` puis `http://localhost:8080` affiche le message
  du backend. C'est le scénario de « rejeu CI/CD en local ».

## 8. Organisation du dépôt `devstation` (indicative, à affiner par le builder)

```
setup.ps1                 démarrage
cmd/devstation/           main
internal/ui/              assistant et rendu (Charm)
internal/profile/         chargement + validation des profils et réponses
internal/sources/         listes de versions par source officielle
internal/download/        HTTP, proxy, CA, cache, vérif. empreinte, progression
internal/install/         un installeur par outil derrière une interface commune
internal/winenv/          PATH/variables HKCU, jonctions, magasin de certificats, credential manager
internal/config/          écriture des configs outils (npmrc, settings.xml, pip.ini, gitconfig…)
internal/state/           state.json, sauvegardes, idempotence
internal/doctor/
profiles/demo.yaml
docs/
README.md
```

Les appels propres à Windows (registre, jonctions, certificats, credential manager) sont derrière
des interfaces avec une implémentation factice, pour que la logique soit testable sous Linux (§9).

## 9. Implémentation par `agentic-coder`

- Trois dépôts, trois runs distincts : `devstation`, `demo-backend`, `demo-frontend`. Chacun reçoit
  son `CLAUDE.md`, ses sous-agents (`new_project.py --profile …`) et son `Dockerfile` de sandbox :
  Go avec le cache de modules pré-rempli pour `devstation` ; `maven:3-eclipse-temurin-21` avec `~/.m2`
  pré-rempli ; image Node avec `node_modules`.
- Le sandbox est **Linux et sans réseau** : les tests unitaires ne téléchargent rien (serveurs HTTP
  de test, faux systèmes de fichiers) et n'utilisent pas d'API Windows. Le code Windows est compilé
  et testé par la CI GitHub Actions sur `windows-latest` (`go vet`, `go test`,
  `GOOS=windows go build`), plus un test d'intégration manuel.
- Contraintes `/plan-tasks` : 1 à 3 fichiers par tâche, chaque tâche testable, tâches d'un même lot
  indépendantes. Le travail est donc découpé en **lots successifs** : on merge un lot avant de
  planifier le suivant (le premier pose `go.mod`, les interfaces et le squelette).
- Claude relit chaque PR (conformité à ce document, secrets, sécurité des téléchargements) ;
  l'humain merge.
- Les clés DeepSeek restent dans `agentic-coder/.env`, jamais lues, affichées ni copiées ailleurs.

## 10. Livraison

- Workflow GitHub Actions sur tag `v*` : build `windows/amd64` (et `arm64`), `checksums.txt`, publication
  dans les Releases. `setup.ps1` est mis à jour à chaque release pour épingler la nouvelle version.
- README : prérequis (Git, WSL2 ; token GitHub *fine-grained* limité aux dépôts du projet, droits
  `Contents: read/write`, nécessaire seulement pour pousser), démarrage en deux commandes, captures, profil commenté, dépannage
  (ExecutionPolicy, proxy, certificats, PATH machine qui masque, WSL absent), désinstallation, logs.

## 11. Laissé au builder

Schéma exact des fichiers YAML, choix des bibliothèques Go hors Charm, méthode d'installation de
Python sans admin, choix zip vs installeur per-user par outil, mise en page précise des écrans,
découpage exact des lots, outil de release (goreleaser ou script).
