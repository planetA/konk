package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/config"
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

func init() {
	cobra.OnInitialize(config.InitConfig)
	KonkCmd.PersistentFlags().StringVar(&config.CfgFile, "config", config.CfgFile, "config file")
	KonkCmd.PersistentFlags().BoolVarP(&config.VarVerbose, "verbose", "v", config.DefaultVerbose, "verbose output")
	KonkCmd.PersistentFlags().StringVar(&config.VarCoordinatorHost, "coordinator_host", config.DefaultCoordinatorHost, "Hostname running the coordinator")
	KonkCmd.PersistentFlags().IntVar(&config.VarCoordinatorPort, "coordinator_port", config.DefaultCoordinatorPort, "Coordinator server port")

	config.BindPFlag(config.CoordinatorHost, KonkCmd.PersistentFlags().Lookup("coordinator_host"))
	config.BindPFlag(config.CoordinatorPort, KonkCmd.PersistentFlags().Lookup("coordinator_port"))
}
