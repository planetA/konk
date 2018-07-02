package cmd

import (
	"github.com/spf13/cobra"

	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
)

var (
	// Unique Id of a container in a network
	containerId int
)

var containerCmd = &cobra.Command{
	TraverseChildren: true,
	Use:   docs.ContainerUse,
	Short: docs.ContainerShort,
	Long:  docs.ContainerLong,
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

func init() {
	KonkCmd.AddCommand(containerCmd)

	containerCreateCmd.Flags().IntVarP(&containerId, "id", "i", 0, "Container id")
	containerCreateCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerCreateCmd)

	containerDeleteCmd.Flags().IntVarP(&containerId, "id", "i", 0, "Container id")
	containerDeleteCmd.MarkFlagRequired("id")
	containerCmd.AddCommand(containerDeleteCmd)
}
