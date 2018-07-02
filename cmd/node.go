package cmd

import (
	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
)

var nodeCmd = &cobra.Command{
	TraverseChildren: true,
	Use:   docs.NodeUse,
	Short: docs.NodeShort,
	Long:  docs.NodeLong,
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	KonkCmd.AddCommand(nodeCmd)
}
