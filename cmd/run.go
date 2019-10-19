package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/srv/coproc"
)

var RunCmd = &cobra.Command{
	Use:   docs.RunUse,
	Short: docs.RunShort,
	Long:  docs.RunLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		containerId, err := GetContainerId()
		if err != nil {
			return err
		}

		image := config.GetString(config.ContainerImage)

		if err = coproc.Run(containerId, image, args); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	// Configure a unique Id of a container in a network
	RunCmd.Flags().String("rank_env", "", "Environment variable containing id")
	config.BindPFlag(config.ContainerIdEnv, RunCmd.Flags().Lookup("rank_env"))

	RunCmd.Flags().String("image", "", "Location of the container image")
	RunCmd.MarkFlagRequired("image")
	config.BindPFlag(config.ContainerImage, RunCmd.Flags().Lookup("image"))

	KonkCmd.AddCommand(RunCmd)
}

// Return a unique Id of a container in a network. It either can be set over a command line,
// or obtained from an environment variable.
func GetContainerId() (container.Id, error) {
	// Environment variable containing the Id
	envVarId := config.GetString(config.ContainerIdEnv)

	var containerId container.Id
	if len(envVarId) != 0 {
		envVal := os.Getenv(envVarId)
		i, err := strconv.Atoi(envVal)
		if err != nil {
			return -1, fmt.Errorf(`Could not parse variable %s: %s`, envVarId, envVal)
		}
		containerId = container.Id(i)
	}

	if containerId < 0 {
		return -1, fmt.Errorf("Id should be >= 0: %v", containerId)
	}

	return containerId, nil
}
