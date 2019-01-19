package config

import (
	"log"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Type for a key, to make things more typesafe
type ViperKey string

// Constants used by viper to lookup configuration
const (
	NymphHost ViperKey = "nymph.host"
	NymphPort          = "nymph.port"

	CoordinatorHost = "coordinator.host"
	CoordinatorPort = "coordinator.port"

	ContainerIdEnv    = "container.rank_env"
	ContainerRootDir  = "container.root_dir"
	ContainerBaseName = "container.base_name"

	KonkSysLauncher = "konk-sys.launcher"
	KonkSysInit     = "konk-sys.init"

	MpirunNumproc = "mpirun.numproc"
	MpirunHosts   = "mpirun.hosts"
	MpirunBinpath = "mpirun.binpath"
	MpirunParams  = "mpirun.params"

	MpirunNameNumproc = "mpirun.name.numproc"
	MpirunNameHosts   = "mpirun.name.hosts"
)

func GetString(key ViperKey) string {
	if !viper.IsSet(string(key)) {
		log.Panicf("The key '%v' was not set and does not have a default value", key)
	}
	return viper.GetString(string(key))
}

func GetStringSlice(key ViperKey) []string {
	if !viper.IsSet(string(key)) {
		log.Panicf("The key '%v' was not set and does not have a default value", key)
	}
	return viper.GetStringSlice(string(key))
}

func GetInt(key ViperKey) int {
	if !viper.IsSet(string(key)) {
		log.Panicf("The key '%v' was not set and does not have a default value", key)
	}
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
