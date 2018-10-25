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

var containerCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.ContainerUse,
	Short:            docs.ContainerShort,
	Long:             docs.ContainerLong,
	Run:              nil,
}

var containerRunCmd = &cobra.Command{
	Use:   docs.ContainerRunUse,
	Short: docs.ContainerRunShort,
	Long:  docs.ContainerRunLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		containerId, err := GetContainerId()
		if err != nil {
			return err
		}

		if err = coproc.Run(containerId, args); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	KonkCmd.AddCommand(containerCmd)

	containerCmd.PersistentFlags().Int("id", 0, "Container id")

	// Configure a unique Id of a container in a network
	containerRunCmd.Flags().String("rank_env", "", "Environment variable containing id")
	config.BindPFlag(config.ContainerIdEnv, containerRunCmd.Flags().Lookup("rank_env"))
	containerCmd.AddCommand(containerRunCmd)
}

// Return a unique Id of a container in a network. It either can be set over a command line,
// or obtained from an environment variable.
func GetContainerId() (container.Id, error) {
	// Environment variable containing the Id
	envVarId := config.GetString(config.ContainerIdEnv)

	if (len(envVarId) == 0) == (!KonkCmd.Flags().Changed("id")) {
		return -1, fmt.Errorf(`Expected to set either "id" or "env"`)
	}

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
