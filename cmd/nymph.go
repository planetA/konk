package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/node"
	"github.com/planetA/konk/srv/nymph"
)

var (
	// Init node only
	initOnly bool
	// Do not do initialisation
	noInit bool
)

var nymphCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.NymphUse,
	Short:            docs.NymphShort,
	Long:             docs.NymphLong,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("no-init") && cmd.Flags().Changed("init") {
			return fmt.Errorf("Requested to do only initialisation and do no initialisation simultaneously")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !noInit {
			if err := node.Init(); err != nil {
				return fmt.Errorf("Failed to init the node: %v", err)
			}
		}

		if initOnly {
			return nil
		}

		if err := nymph.Run(); err != nil {
			return fmt.Errorf("Nymph failed: %v", err)
		}
		return nil
	},
}

func init() {
	nymphCmd.Flags().BoolVar(&initOnly, "init", false, "Initialise the node and exit immediately")
	nymphCmd.Flags().BoolVar(&noInit, "no-init", false, "Do not initialise the node")
	KonkCmd.AddCommand(nymphCmd)
}
