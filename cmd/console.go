package cmd

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
)

var (
	InteractiveMode bool   = false
	Rank            int    = -1
	Destination     string = ""
	PreDump         bool   = false
)

var consoleCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.ConsoleUse,
	Short:            docs.ConsoleShort,
	Long:             docs.ConsoleLong,
	Args: func(cmd *cobra.Command, args []string) error {
		if InteractiveMode == false && len(args) < 1 {
			return errors.New("need to specify a command")
		}

		if InteractiveMode == true {
			return fmt.Errorf("Interactive mode is not supported yet")
		}

		return nil
	},
}

var migrateCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.ConsoleMigrateUse,
	Short:            docs.ConsoleMigrateShort,
	Long:             docs.ConsoleMigrateLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Debug("Executing migration command")

		coord, err := coordinator.NewClient()
		if err != nil {
			return err
		}
		defer coord.Close()

		log.WithFields(log.Fields{
			"rank":     Rank,
			"dest":     Destination,
			"pre-dump": PreDump,
		}).Debug("Requesting migration")

		if err := coord.Migrate(container.Rank(Rank), Destination, PreDump); err != nil {
			return fmt.Errorf("Migration failed: %v", err)
		}
		return nil
	},
}

func init() {
	migrateCmd.Flags().IntVar(&Rank, "rank", -1, "Rank to migrate")
	migrateCmd.MarkFlagRequired("rank")

	migrateCmd.Flags().StringVar(&Destination, "dest", "", "New destination of a rank")
	migrateCmd.MarkFlagRequired("dest")

	migrateCmd.Flags().BoolVarP(&PreDump, "pre-dump", "p", false, "Run predump command")
	migrateCmd.MarkFlagRequired("dest")

	consoleCmd.AddCommand(migrateCmd)

	consoleCmd.Flags().BoolVarP(&InteractiveMode, "interactive", "i", false, "Run console in interactive mode")
	KonkCmd.AddCommand(consoleCmd)
}
