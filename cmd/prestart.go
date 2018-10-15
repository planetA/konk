package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/srv/prestart"
)

var prestartCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.PrestartUse,
	Short:            docs.PrestartShort,
	Long:             docs.PrestartLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := prestart.Run(args); err != nil {
			return fmt.Errorf("Prestart failed: %v", err)
		}
		return nil
	},
}

func init() {
	prestartCmd.Flags().BoolVar(&initOnly, "init", false, "Initialise the node and exit immediately")
	prestartCmd.Flags().BoolVar(&noInit, "no-init", false, "Do not initialise the node")
	KonkCmd.AddCommand(prestartCmd)
}
