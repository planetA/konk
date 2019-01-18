package cmd

import (
	"github.com/spf13/cobra"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/mpirun"
)

var MpirunCmd = &cobra.Command{
	Use:   docs.MpirunUse,
	Short: docs.MpirunShort,
	Long:  docs.MpirunLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := mpirun.Run(args); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	MpirunCmd.Flags().IntP("numproc", "n", 1, "Number of processes to create")
	config.BindPFlag(config.MpirunNumproc, MpirunCmd.Flags().Lookup("numproc"))

	MpirunCmd.Flags().StringP("hosts", "H", "localhost", "Hoststring to pass to mpirun")
	config.BindPFlag(config.MpirunHosts, MpirunCmd.Flags().Lookup("hosts"))

	KonkCmd.AddCommand(MpirunCmd)
}
