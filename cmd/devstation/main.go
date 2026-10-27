// Command devstation installe et configure un poste de dev Windows.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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

	root.AddCommand(newSetupCmd(&flags))
	for _, f := range extraCommands {
		root.AddCommand(f())
	}
	return root
}

// main exécute la commande racine et sort avec le code 1 en cas d'erreur.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "erreur : "+err.Error())
		os.Exit(1)
	}
}
