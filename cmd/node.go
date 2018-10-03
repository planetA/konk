package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/node"
)

var (
	// Unique Id of a node in a network
	nodeId int
	// Init node only
	initOnly bool
	// Do not do initialisation
	noInit bool
)

var nodeCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.NodeUse,
	Short:            docs.NodeShort,
	Long:             docs.NodeLong,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("id") {
			return fmt.Errorf(`Don't update "id" for now`)
		}

		if cmd.Flags().Changed("no-init") && cmd.Flags().Changed("init") {
			return fmt.Errorf("Requested to do only initialisation and do no initialisation simultaneously")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if ! noInit {
			if err := node.Init(nodeId); err != nil {
				return fmt.Errorf("Failed to init the node: %v", err)
			}
		}

		if initOnly {
			return nil
		}

		if err := node.RunDaemon(nodeId); err != nil {
			return fmt.Errorf("Node-monitoring konk daemon failed: %v", err)
		}
		return nil
	},
}

func init() {
	nodeCmd.Flags().IntVarP(&nodeId, "id", "i", 0, "Node id")
	nodeCmd.Flags().BoolVar(&initOnly, "init", false, "Initialise the node and exit immediately")
	nodeCmd.Flags().BoolVar(&noInit, "no-init", false, "Initialise the node and exit immediately")
	KonkCmd.AddCommand(nodeCmd)
}
