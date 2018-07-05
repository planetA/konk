package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
)

var (
	// Unique Id of a container in a network
	containerId int
	// Environment variable containing the Id
	envVarId string
)

var containerCmd = &cobra.Command{
	TraverseChildren: true,
	Use:              docs.ContainerUse,
	Short:            docs.ContainerShort,
	Long:             docs.ContainerLong,
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var containerCreateCmd = &cobra.Command{
	Use:   docs.ContainerCreateUse,
	Short: docs.ContainerCreateShort,
	Long:  docs.ContainerCreateLong,
	Run: func(cmd *cobra.Command, args []string) {
		container.Create(containerId)
	},
}

var containerDeleteCmd = &cobra.Command{
	Use:   docs.ContainerDeleteUse,
	Short: docs.ContainerDeleteShort,
	Long:  docs.ContainerDeleteLong,
	Run: func(cmd *cobra.Command, args []string) {
		container.Delete(containerId)
	},
}

var containerRunCmd = &cobra.Command{
	Use:   docs.ContainerRunUse,
	Short: docs.ContainerRunShort,
	Long:  docs.ContainerRunLong,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if (len(envVarId) == 0) == (!cmd.Flags().Changed("id")) {
			return fmt.Errorf(`Expected to set either "id" or "env"`)
		}

		if len(envVarId) != 0 {
			envVal := os.Getenv(envVarId)
			i, err := strconv.Atoi(envVal)
			if err != nil {
				return fmt.Errorf(`Could not parse variable %s: %s`, envVarId, envVal)
			}
			containerId = i
		}

		if containerId < 0 {
			return fmt.Errorf("Id should be >= 0: %v", containerId)
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := container.Run(containerId, args); err != nil {
			log.Print(err)
		}
	},
}

func init() {
	KonkCmd.AddCommand(containerCmd)

	containerCmd.PersistentFlags().IntVarP(&containerId, "id", "i", 0, "Container id")

	containerCreateCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerCreateCmd)

	containerDeleteCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerDeleteCmd)

	containerRunCmd.Flags().StringVar(&envVarId, "env", "", "Environment variable containing id")
	containerCmd.AddCommand(containerRunCmd)
}
