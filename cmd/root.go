package cmd

import (
	"os"
	"log"

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
	KonkCmd.PersistentFlags().StringVar(&config.CfgFile, "config", "", "config file (default is $HOME/.konk.yaml)")
	KonkCmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", false, "verbose output")
}

func initConfig() {
	// Don't forget to read config either from cfgFile or from the default location
	if config.CfgFile != "" {
		viper.SetConfigFile(config.CfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".konk")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Can't read config:", err)
		os.Exit(1)
	}
}
