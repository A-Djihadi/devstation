// Command devstation installe et configure un poste de dev Windows.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// exitCode est un code de sortie porté comme erreur : main l'utilise sans
// afficher de message supplémentaire.
type exitCode int

// Error implémente error ; le message est vide car le code est silencieux.
func (e exitCode) Error() string { return "" }

// errorMessage traduit une erreur en message à afficher et code de sortie.
// Un exitCode ne produit aucun message.
func errorMessage(err error) (msg string, code int) {
	var ec exitCode
	if errors.As(err, &ec) {
		return "", int(ec)
	}
	return "erreur : " + err.Error(), 1
}

// version est remplacée à la compilation : -ldflags "-X main.version=<tag>".
var version = "dev"

// extraCommands regroupe les fabriques de sous-commandes ajoutées par les
// autres fichiers du paquet via init.
var extraCommands []func() *cobra.Command

// newRootCmd construit la commande racine devstation et ses sous-commandes.
func newRootCmd() *cobra.Command {
	var flags GlobalFlags
	root := &cobra.Command{
		Use:           "devstation",
		Short:         "Prépare un poste de développement Windows",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupCmd(cmd.Context(), flags, SetupOptions{})
		},
	}
	root.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "désactive les couleurs")
	root.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "affiche les détails")
	root.PersistentFlags().StringVar(&flags.LogPath, "log", "", "fichier de journal (défaut : dossier logs de DevStation)")
	root.PersistentFlags().BoolP("help", "h", false, "affiche l'aide")
	root.Flags().BoolP("version", "v", false, "affiche la version")
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetUsageTemplate(usageTemplateFR)

	root.AddCommand(newSetupCmd(&flags))
	for _, f := range extraCommands {
		root.AddCommand(f())
	}
	return root
}

// usageTemplateFR est le gabarit d'aide de cobra traduit en français.
const usageTemplateFR = `Utilisation :{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [commande]{{end}}{{if gt (len .Aliases) 0}}

Alias :
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Exemples :
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Commandes disponibles :{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Options :
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Options globales :
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Commandes d'aide supplémentaires :{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Utilisez "{{.CommandPath}} [commande] --help" pour plus d'informations.{{end}}
`

// main exécute la commande racine et sort avec le code 1 en cas d'erreur.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		msg, code := errorMessage(err)
		if msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(code)
	}
}
