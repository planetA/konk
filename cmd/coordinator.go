package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/srv/coordinator"
)

var coordinatorCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.CoordinatorUse,
	Short:            docs.CoordinatorShort,
	Long:             docs.CoordinatorLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := coordinator.Run(); err != nil {
			return fmt.Errorf("Failed to init the scheduler: %v", err)
		}
		return nil
	},
}

func init() {
	KonkCmd.AddCommand(coordinatorCmd)
}
