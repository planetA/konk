package config

var (
	CfgFile string

	VarVerbose bool

	VarCoordinatorHost string
	VarCoordinatorPort int
)

const (
	DefaultVerbose bool = false

	DefaultCoordinatorHost string = "localhost"
	DefaultCoordinatorPort int    = 8990
)
