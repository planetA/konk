package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/srv/nymph"
)

var nymphCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.NymphUse,
	Short:            docs.NymphShort,
	Long:             docs.NymphLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := nymph.Run(); err != nil {
			return fmt.Errorf("Nymph failed: %v", err)
		}
		return nil
	},
}

func init() {
	KonkCmd.AddCommand(nymphCmd)
}
