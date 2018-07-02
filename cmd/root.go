package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
)

var KonkCmd = &cobra.Command{
	TraverseChildren: true,
	Run:              nil,

	Use:   docs.KonkUse,
	Short: docs.KonkShort,
	Long:  docs.KonkLong,
}

func ExecuteKonk() {
	if err := KonkCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
