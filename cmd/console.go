package cmd

import (
	"fmt"
	"errors"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/console"
)

var (
	InteractiveMode bool = false
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
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if InteractiveMode == true {
			return fmt.Errorf("Interactive mode is not supported yet")
		}

		if err := console.Command(args[0], args[1:]); err != nil {
			return fmt.Errorf("Failed to init the console: %v", err)
		}
		return nil
	},
}

func init() {
	consoleCmd.Flags().BoolVarP(&InteractiveMode, "interactive", "i", false, "Run console in interactive mode")
	KonkCmd.AddCommand(consoleCmd)
}
