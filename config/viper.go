package config

import (
	"log"
	"net"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Type for a key, to make things more typesafe
type ViperKey string

// Constants used by viper to lookup configuration
const (
	NymphHost     ViperKey = "nymph.host"
	NymphPort              = "nymph.port"
	NymphRootDir           = "nymph.root_dir"
	NymphCniPath           = "nymph.cni_path"
	NymphNetworks          = "nymph.networks"

	CoordinatorHost = "coordinator.host"
	CoordinatorPort = "coordinator.port"

	ContainerRankEnv  = "container.rank_env"
	ContainerImage    = "container.image"
	ContainerRootDir  = "container.root_dir"
	ContainerBaseName = "container.base_name"
	ContainerUsername = "container.user"
	ContainerHostname = "container.hostname"

	ContainerDevicePath = "container.device.path"

	KonkSysLauncher = "konk-sys.launcher"
	KonkSysInit     = "konk-sys.init"

	MpirunNumproc = "mpirun.numproc"
	MpirunHosts   = "mpirun.hosts"
	MpirunBinpath = "mpirun.binpath"
	MpirunParams  = "mpirun.params"
	MpirunTmpDir  = "mpirun.tmp_dir"

	MpirunNameNumproc = "mpirun.name.numproc"
	MpirunNameHosts   = "mpirun.name.hosts"

	OvsBridgeName = "ovs.bridge.name"
	OvsPeers      = "ovs.peers"

	VethBridgeName = "veth.bridge.name"

	VethVxlanName  = "veth.vxlan.name"
	VethVxlanId    = "veth.vxlan.id"
	VethVxlanPort  = "veth.vxlan.port"
	VethVxlanDev   = "veth.vxlan.dev"
	VethVxlanGroup = "veth.vxlan.group"

	RxeQpnpn = "rxe.qpnpn"
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

func GetUint(key ViperKey) uint {
	if !viper.IsSet(string(key)) {
		log.Panicf("The key '%v' was not set and does not have a default value", key)
	}
	return viper.GetUint(string(key))
}

func GetIP(key ViperKey) net.IP {
	if !viper.IsSet(string(key)) {
		log.Panicf("The key '%v' was not set and does not have a default value", key)
	}
	ip := net.ParseIP(viper.GetString(string(key)))
	if ip == nil {
		log.Panicf("The key '%v' does not point to a correct IP address.", key)
	}
	return ip
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
