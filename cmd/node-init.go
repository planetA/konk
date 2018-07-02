package cmd

import (
	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/node"
)

var (
	// Unique Id of a node in a network
	nodeId int
)

var nodeInitCmd = &cobra.Command{
	Use:   docs.NodeInitUse,
	Short: docs.NodeInitShort,
	Long:  docs.NodeInitLong,
	Run: func(cmd *cobra.Command, args []string) {
		node.Init(nodeId)
	},
}

func init() {
	nodeInitCmd.Flags().IntVarP(&nodeId, "id", "i", 0, "Node id")
	nodeInitCmd.MarkFlagRequired("id")
	nodeCmd.AddCommand(nodeInitCmd)
}
