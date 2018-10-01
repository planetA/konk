package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/scheduler"
)

var schedulerCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.SchedulerUse,
	Short:            docs.SchedulerShort,
	Long:             docs.SchedulerLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := scheduler.Run(); err != nil {
			return fmt.Errorf("Failed to init the scheduler: %v", err)
		}
		return nil
	},
}

func init() {
	KonkCmd.AddCommand(schedulerCmd)
}
