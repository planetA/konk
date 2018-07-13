package cmd

import (
	"log"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/criu"
)

var (
	// Pid of a process in a container
	Pid int
	// Hostname and port of the recipient node
	Hostname string
	Port     int
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

var criuMigrateCmd = &cobra.Command{
	Use:   docs.CriuMigrateUse,
	Short: docs.CriuMigrateShort,
	Long:  docs.CriuMigrateLong,
	Run: func(cmd *cobra.Command, args []string) {
		recipient := fmt.Sprintf("%s:%v", Hostname, Port)

		if err := criu.Migrate(Pid, recipient); err != nil {
			log.Printf("Failed to migrate: %v", err)
		}
	},
}

var criuReceiveCmd = &cobra.Command{
	Use:   docs.CriuReceiveUse,
	Short: docs.CriuReceiveShort,
	Long:  docs.CriuReceiveLong,
	Run: func(cmd *cobra.Command, args []string) {
		if err := criu.Receive(Port); err != nil {
			log.Printf("Failed to receive: %v", err)
		}
	},
}

func init() {
	KonkCmd.AddCommand(criuCmd)

	criuDumpCmd.Flags().IntVarP(&Pid, "pid", "p", 0, "Process id")
	criuDumpCmd.MarkFlagRequired("pid")
	criuCmd.AddCommand(criuDumpCmd)

	criuMigrateCmd.Flags().IntVarP(&Pid, "pid", "p", 0, "Target process id")
	criuMigrateCmd.MarkFlagRequired("pid")
	criuMigrateCmd.Flags().StringVarP(&Hostname, "host", "H", "", "Hosname of the recipient node")
	criuMigrateCmd.MarkFlagRequired("host")
	criuMigrateCmd.Flags().IntVarP(&Port, "port", "P", 0, "Port on the recipient node")
	criuMigrateCmd.MarkFlagRequired("port")
	criuCmd.AddCommand(criuMigrateCmd)

	criuReceiveCmd.Flags().IntVarP(&Port, "port", "p", 0, "Port to listen on")
	criuReceiveCmd.MarkFlagRequired("port")
	criuCmd.AddCommand(criuReceiveCmd)

}
