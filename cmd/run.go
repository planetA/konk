package cmd

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/docs"
	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
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
		hostname := config.GetString(config.RunHostname)

		nymph, err := nymph.NewClient(hostname)
		if err != nil {
			return fmt.Errorf("Failed to connect to Nymph: %v", err)
		}
		defer nymph.Close()

		if err := nymph.Run(containerId, image, args); err != nil {
			log.WithFields(log.Fields{
				"containerId": containerId,
				"image":       image,
				"args":        args}).Error("Failed to launch container")
		}

		log.Info("Running container")

		ret, err := nymph.Wait(containerId)
		if err != nil {
			return fmt.Errorf("Waiting failed: %v", err)
		}

		log.WithField("Return value", ret).Info("Process finished")

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

	RunCmd.Flags().String("hostname", "localhost", "Where the application should run")
	config.BindPFlag(config.RunHostname, RunCmd.Flags().Lookup("hostname"))

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
