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
)

var nodeCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.NodeUse,
	Short:            docs.NodeShort,
	Long:             docs.NodeLong,
	Run: nil,
}

var nodeInitCmd = &cobra.Command{
	Use:   docs.NodeInitUse,
	Short: docs.NodeInitShort,
	Long:  docs.NodeInitLong,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("id") {
			return fmt.Errorf(`Don't update "id" for now`)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		node.Init(nodeId)
	},
}

func init() {
	nodeInitCmd.Flags().IntVarP(&nodeId, "id", "i", 0, "Node id")
	nodeCmd.AddCommand(nodeInitCmd)

	KonkCmd.AddCommand(nodeCmd)
}

func init() {
}
