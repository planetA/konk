package config

import (
	"os"
	"log"

	"github.com/spf13/viper"
	"github.com/spf13/pflag"
)

// Type for a key, to make things more typesafe
type ViperKey string

// Constants used by viper to lookup configuration
const (
	NymphHost ViperKey = "nymph.host"
	NymphPort          = "nymph.port"

	CoordinatorHost = "coordinator.host"
	CoordinatorPort = "coordinator.port"

	ContainerIdEnv = "container.rank_env"

	KonkSysLauncher = "konk-sys.launcher"
	KonkSysInit     = "konk-sys.init"
)

func GetString(key ViperKey) (string) {
	return viper.GetString(string(key))
}

func GetInt(key ViperKey) (int) {
	return viper.GetInt(string(key))
}

func BindPFlag(key ViperKey, flag *pflag.Flag) {
	viper.BindPFlag(string(key), flag)
}


func InitConfig() {
	// Don't forget to read config either from cfgFile or from the default location
	if CfgFile != "" {
		viper.SetConfigFile(CfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".konk")
	}
	viper.SetEnvPrefix("konk")

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Cannot read config:", err)
		log.Println(os.Getpid())
		os.Exit(1)
	}
}
