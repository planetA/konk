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
	WithPreDump     bool   = false
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
	Args: func(cmd *cobra.Command, args []string) error {
		if PreDump == true && WithPreDump == true {
			return fmt.Errorf("Flags pre-dump and with-pre-dump are conflicting")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Debug("Executing migration command")

		coord, err := coordinator.NewClient()
		if err != nil {
			return err
		}
		defer coord.Close()

		var migrationType container.MigrationType
		switch {
		case PreDump == true:
			migrationType = container.PreDump
		case WithPreDump == true:
			migrationType = container.WithPreDump
		default:
			migrationType = container.Migrate
		}

		log.WithFields(log.Fields{
			"rank": Rank,
			"dest": Destination,
			"type": migrationType,
		}).Debug("Requesting migration")

		if err := coord.Migrate(container.Rank(Rank), Destination, migrationType); err != nil {
			return fmt.Errorf("Migration failed: %v", err)
		}
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use: docs.ConsoleDeleteUse,
	Short: docs.ConsoleDeleteShort,
	Long: docs.ConsoleDeleteLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Debug("Deleting container")

		coord, err := coordinator.NewClient()
		if err != nil {
			return err
		}
		defer coord.Close()

		log.WithFields(log.Fields{
			"rank": Rank,
		}).Debug("Requesting container deletion")

		if err := coord.Delete(container.Rank(Rank)); err != nil {
			return fmt.Errorf("Deletion failed: %v", err)
		}
		return nil
	},
}

func init() {
	migrateCmd.Flags().IntVar(&Rank, "rank", -1, "Rank to migrate")
	migrateCmd.MarkFlagRequired("rank")

	migrateCmd.Flags().StringVar(&Destination, "dest", "", "New destination of a rank")
	migrateCmd.MarkFlagRequired("dest")

	migrateCmd.Flags().BoolVar(&PreDump, "pre-dump", false, "Run predump command")

	migrateCmd.Flags().BoolVar(&WithPreDump, "with-pre-dump", false, "Migrate, but run predump command")

	consoleCmd.AddCommand(migrateCmd)

	deleteCmd.Flags().IntVar(&Rank, "rank", -1, "Rank to delete")
	deleteCmd.MarkFlagRequired("rank")
	consoleCmd.AddCommand(deleteCmd)

	consoleCmd.Flags().BoolVarP(&InteractiveMode, "interactive", "i", false, "Run console in interactive mode")
	KonkCmd.AddCommand(consoleCmd)
}
