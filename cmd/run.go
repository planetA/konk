package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/planetA/konk/pkg/nymph"
)

func requestCoordinatorAllocation(rank container.Rank) (string, error) {
	c, err := coordinator.NewClient()
	if err != nil {
		return "", err
	}
	defer c.Close()

	return c.AllocateHost(rank)
}

var RunCmd = &cobra.Command{
	Use:   docs.RunUse,
	Short: docs.RunShort,
	Long:  docs.RunLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		containerRank, err := GetContainerRank()
		if err != nil {
			return err
		}

		image := config.GetString(config.ContainerImage)
		hostname, ok := config.GetStringOk(config.ContainerHostname)
		if ok != true {
			hostname, err = requestCoordinatorAllocation(containerRank)
			if err != nil {
				return fmt.Errorf("Failed to get host allocation: %v", err)
			}
		}

		n, err := nymph.NewClient(hostname)
		if err != nil {
			return fmt.Errorf("Failed to connect to Nymph: %v", err)
		}
		defer n.Close()

		init, err := config.GetBool(config.ContainerInit)
		if err != nil {
			init = false
		}

		if err := n.Run(&nymph.RunArgs{
			Rank:  containerRank,
			Image: image,
			Args:  args,
			Init:  init,
		}); err != nil {
			log.WithFields(log.Fields{
				"containerRank": containerRank,
				"image":         image,
				"args":          args}).Error("Failed to launch container")
		}

		log.Info("Running container")

		ret, err := n.Wait(containerRank)
		if err != nil {
			return fmt.Errorf("Waiting failed: %v", err)
		}

		log.WithField("Return value", ret).Info("Process finished")

		return nil
	},
}

func init() {
	// Configure a unique Rank of a container in a network
	RunCmd.Flags().Int("rank", 0, "Rank number for parallel jobs")
	config.BindPFlag(config.ContainerRank, RunCmd.Flags().Lookup("rank"))
	config.BindPFlagDefault(config.ContainerRank, RunCmd.Flags().Lookup("rank"), 1)

	RunCmd.Flags().String("image", "", "Container image")
	RunCmd.MarkFlagRequired("image")
	config.BindPFlag(config.ContainerImage, RunCmd.Flags().Lookup("image"))

	RunCmd.Flags().String("hostname", "", "Name of the host where the application should run")
	RunCmd.MarkFlagRequired("hostname")
	config.BindPFlag(config.ContainerHostname, RunCmd.Flags().Lookup("hostname"))

	RunCmd.Flags().String("user", "user", "The user that should launch the process")
	config.BindPFlag(config.ContainerUsername, RunCmd.Flags().Lookup("user"))

	RunCmd.Flags().Bool("init", true, "Tell if the process should be init process")
	config.BindPFlag(config.ContainerInit, RunCmd.Flags().Lookup("init"))

	KonkCmd.AddCommand(RunCmd)
}

// Return a unique Rank of a container in a network. It either can be set over a command line,
// or obtained from an environment variable.
func GetContainerRank() (container.Rank, error) {
	if containerRank, ok := config.GetIntOk(config.ContainerRank); ok == true {
		if containerRank < 0 {
			return -1, fmt.Errorf("Rank should be >= 0: %v", containerRank)
		}

		return container.Rank(containerRank), nil
	}

	return -1, fmt.Errorf("Rank was not set")
}
