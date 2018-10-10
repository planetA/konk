package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
	cobra.OnInitialize(initConfig)
	KonkCmd.PersistentFlags().StringVar(&config.CfgFile, "config", config.CfgFile, "config file")
	KonkCmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", config.Verbose, "verbose output")
	KonkCmd.PersistentFlags().StringVar(&config.CoordinatorHost, "coordinator_host", config.CoordinatorHost, "Hostname running the coordinator")
	KonkCmd.PersistentFlags().IntVar(&config.CoordinatorPort, "coordinator_port", config.CoordinatorPort, "Coordinator server port")

	viper.BindPFlag(config.ViperCoordinatorHost, KonkCmd.PersistentFlags().Lookup("coordinator_host"))
	viper.BindPFlag(config.ViperCoordinatorPort, KonkCmd.PersistentFlags().Lookup("coordinator_port"))
}

func initConfig() {
	// Don't forget to read config either from cfgFile or from the default location
	if config.CfgFile != "" {
		viper.SetConfigFile(config.CfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".konk")
	}
	viper.SetEnvPrefix("konk")

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Can't read config:", err)
		os.Exit(1)
	}
}
