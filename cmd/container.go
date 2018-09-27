package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
)

var containerCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.ContainerUse,
	Short:            docs.ContainerShort,
	Long:             docs.ContainerLong,
	Run:              nil,
}

var containerCreateCmd = &cobra.Command{
	Use:   docs.ContainerCreateUse,
	Short: docs.ContainerCreateShort,
	Long:  docs.ContainerCreateLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		containerId, err := GetContainerId()
		if err != nil {
			return err
		}

		return container.Create(containerId)
	},
}

var containerDeleteCmd = &cobra.Command{
	Use:   docs.ContainerDeleteUse,
	Short: docs.ContainerDeleteShort,
	Long:  docs.ContainerDeleteLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		containerId, err := GetContainerId()
		if err != nil {
			return err
		}

		container.Delete(containerId)
		return nil
	},
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

		if err = container.Run(containerId, args); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	KonkCmd.AddCommand(containerCmd)

	containerCmd.PersistentFlags().Int("id", 0, "Container id")

	containerCreateCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerCreateCmd)

	containerDeleteCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerDeleteCmd)

	// Configure a unique Id of a container in a network
	containerRunCmd.Flags().String("rank_env", "", "Environment variable containing id")
	viper.BindPFlag("container.rank_env", containerRunCmd.Flags().Lookup("rank_env"))
	containerCmd.AddCommand(containerRunCmd)
}

// Return a unique Id of a container in a network. It either can be set over a command line,
// or obtained from an environment variable.
func GetContainerId() (int, error) {
	// Environment variable containing the Id
	envVarId := viper.GetString("container.rank_env")

	if (len(envVarId) == 0) == (!KonkCmd.Flags().Changed("id")) {
		return -1, fmt.Errorf(`Expected to set either "id" or "env"`)
	}

	var containerId int
	if len(envVarId) != 0 {
		envVal := os.Getenv(envVarId)
		i, err := strconv.Atoi(envVal)
		if err != nil {
			return -1, fmt.Errorf(`Could not parse variable %s: %s`, envVarId, envVal)
		}
		containerId = i
	}

	if containerId < 0 {
		return -1, fmt.Errorf("Id should be >= 0: %v", containerId)
	}

	return containerId, nil
}
