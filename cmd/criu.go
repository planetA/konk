package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/criu"
)

var (
	// Pid of a process in a container
	Pid int
)

var criuCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.CriuUse,
	Short:            docs.CriuShort,
	Long:             docs.CriuLong,
	Run:              nil,
}

var criuDumpCmd = &cobra.Command{
	Use:   docs.CriuDumpUse,
	Short: docs.CriuDumpShort,
	Long:  docs.CriuDumpLong,
	Run: func(cmd *cobra.Command, args []string) {
		if err := criu.Dump(Pid); err != nil {
			log.Printf("Failed to dump: %v", err)
		}
	},
}

func init() {
	KonkCmd.AddCommand(criuCmd)

	criuCmd.PersistentFlags().IntVarP(&Pid, "pid", "p", 0, "Process id")
	criuDumpCmd.MarkFlagRequired("pid")

	criuCmd.AddCommand(criuDumpCmd)
}
