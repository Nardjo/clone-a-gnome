package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Options regroupe les paramètres configurables de la CLI.
type Options struct {
	SourceURL       string
	Reuse           bool
	ContainerName   string
	LocalDBName     string
	LocalPort       int
	AnonymizeScript string
	ExcludeTables   []string
	LogLevel        string
}

// New construit la commande root de l’outil.
func New() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "clone-a-gnome <SOURCE_URL>",
		Short: "Clone rapidement une base PostgreSQL distante dans un conteneur local",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("merci de fournir l’URL de connexion source")
			}
			opts.SourceURL = args[0]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), opts)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().BoolVar(&opts.Reuse, "reuse", false, "réutiliser un conteneur existant si disponible")
	cmd.Flags().StringVar(&opts.ContainerName, "container-name", "clone-a-gnome", "nom explicite du conteneur Docker local")
	cmd.Flags().StringVar(&opts.LocalDBName, "local-db", "localdb", "nom de la base locale à créer")
	cmd.Flags().IntVar(&opts.LocalPort, "port", 0, "port local à utiliser (0 = aléatoire)")
	cmd.Flags().StringVar(&opts.AnonymizeScript, "anonymize-script", "", "chemin vers un script SQL à exécuter après l’import")
	cmd.Flags().StringSliceVar(&opts.ExcludeTables, "exclude-table", nil, "table à exclure lors du dump (répéter le drapeau)")
	cmd.Flags().StringVar(&opts.LogLevel, "log-level", "info", "niveau de log (debug,info,warn,error)")

	return cmd
}
