# DevStation

Environnement de développement Windows prêt à l'emploi en un `git clone`.

## Démarrage rapide

```powershell
git clone https://github.com/A-Djihadi/devstation.git
cd devstation; powershell -ExecutionPolicy Bypass -File .\setup.ps1
```

Variantes :

- Derrière un proxy d'entreprise Kerberos : `.\setup.ps1 -Kerberos -Proxy http://proxy.example.com:8080`
- Sans questions (réponses par défaut) : `.\setup.ps1 --yes`
- Simulation, sans rien modifier : `.\setup.ps1 --dry-run`

## Prérequis

- Windows 10/11 64 bits.
- Git for Windows.
- WSL2 installé pour Podman : la commande `wsl --install` doit avoir été faite une fois par un administrateur, suivie d'un redémarrage.
- Aucun droit administrateur n'est nécessaire pour le reste.
- Token GitHub facultatif : seulement pour pousser (les dépôts de démo sont publics). Créez un token « fine-grained » limité aux dépôts du projet avec la permission `Contents: Read and write`, puis collez-le quand l'assistant le demande. Il est rangé dans Git Credential Manager, jamais dans un fichier.

## Ce que DevStation installe et configure

- Java Temurin
- Maven
- Gradle
- Node.js + npm + Angular CLI
- Python
- VS Code ou IntelliJ IDEA (avec extensions)
- Podman + Compose + machine WSL
- Client Vault
- git (identité, clé SSH, token)
- proxy (`px` pour Kerberos)
- certificats d'entreprise
- Nexus (npm, pip, Maven, conteneurs)
- clonage des dépôts du projet

Tout est installé dans `%LOCALAPPDATA%\DevStation`, et seul le PATH utilisateur est modifié.

## Commandes

| Commande | Description |
| --- | --- |
| `devstation doctor` | Vérifie le poste |
| `devstation list` | Liste les versions installées (`--available` avec un outil) |
| `devstation repos sync` | Clone les dépôts manquants |
| `devstation secrets` | Lit les secrets du coffre de l'équipe (`list [chemin]`, `get <chemin> <champ> [--copy]`) |
| `devstation setup` | Installe et configure le poste (`--profile`, `--answers`, `--yes`, `--dry-run`) |
| `devstation uninstall` | Retire ce que DevStation a installé (`[outil]` \| `--all`, `--yes`) |
| `devstation use` | Change la version active d'un outil (`use <outil> <version>`) |
| `devstation version` | Affiche la version de DevStation |

Options globales : `--no-color`, `--verbose`, `--log <fichier>`.

## Profil d'équipe

Le fichier `profiles/demo.yaml` décrit les versions d'outils, la configuration réseau, les dépôts du projet et le coffre de secrets. Chaque clé est commentée dans `docs/profil-commente.yaml`.

Pour adapter DevStation à votre équipe, copiez ce fichier dans `profiles/<equipe>.yaml`, ajustez-le, puis lancez :

```powershell
.\setup.ps1 --profile profiles\<equipe>.yaml
```

## Coffre de secrets (Vault)

DevStation se connecte au Vault de l'équipe (client seulement, il ne monte jamais de serveur). La connexion se fait par OIDC par défaut.

- `devstation secrets list` : liste les chemins accessibles.
- `devstation secrets get <chemin> <champ> --copy` : copie un secret dans le presse-papiers.

Démo locale : `podman compose up` dans le dépôt `demo-backend` démarre un Vault de développement avec de faux secrets.

## Démo de bout en bout

Après l'installation :

```powershell
cd $env:USERPROFILE\workspace\demo\demo-backend
podman compose up --build
```

Ouvrez ensuite http://localhost:8080 : le frontend affiche le message du backend et « Secret Vault chargé : oui ».

## Dépannage

- **ExecutionPolicy** : utilisez la commande avec `-ExecutionPolicy Bypass` (voir le démarrage rapide).
- **Erreur de certificat** : ajoutez le PEM de l'autorité dans `ca_certs` du profil.
- **Proxy** : utilisez la variante `-Kerberos` ; `devstation doctor` vérifie `px`.
- **Un Java/Node installé par l'administrateur masque le nôtre** : le PATH système passe avant le PATH utilisateur ; `devstation doctor` le signale.
- **WSL absent** : faites installer WSL2 par un administrateur (`wsl --install`) puis redémarrez.
- **PATH non rechargé** : ouvrez un nouveau terminal après l'installation.

## Désinstallation

```powershell
devstation uninstall --all
```

Cette commande restaure les fichiers de configuration ; les sauvegardes `*.devstation.bak` sont conservées.

## Journaux

Les journaux se trouvent dans `%LOCALAPPDATA%\DevStation\logs`. Les secrets y sont masqués. Joignez-les en cas de problème.

## Pour contribuer

- Tests : `sh scripts/test.sh` (ou la CI).
- Conception : `docs/superpowers/specs`.
- Plan : `docs/superpowers/plans`.
