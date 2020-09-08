package config

var (
	CfgFile string

	VarCoordinatorHost string
	VarCoordinatorPort int
)

const (
	DefaultCoordinatorHost string = "localhost"
	DefaultCoordinatorPort int    = 8990
)
