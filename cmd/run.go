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
		containerRank, err := GetContainerRank()
		if err != nil {
			return err
		}

		image := config.GetString(config.ContainerImage)
		hostname := config.GetString(config.ContainerHostname)

		nymph, err := nymph.NewClient(hostname)
		if err != nil {
			return fmt.Errorf("Failed to connect to Nymph: %v", err)
		}
		defer nymph.Close()

		if err := nymph.Run(containerRank, image, args); err != nil {
			log.WithFields(log.Fields{
				"containerRank": containerRank,
				"image":         image,
				"args":          args}).Error("Failed to launch container")
		}

		log.Info("Running container")

		ret, err := nymph.Wait(containerRank)
		if err != nil {
			return fmt.Errorf("Waiting failed: %v", err)
		}

		log.WithField("Return value", ret).Info("Process finished")

		return nil
	},
}

func init() {
	// Configure a unique Rank of a container in a network
	RunCmd.Flags().String("rank_env", "", "Environment variable containing rank")
	config.BindPFlag(config.ContainerRankEnv, RunCmd.Flags().Lookup("rank_env"))

	RunCmd.Flags().String("image", "", "Location of the container image")
	RunCmd.MarkFlagRequired("image")
	config.BindPFlag(config.ContainerImage, RunCmd.Flags().Lookup("image"))

	RunCmd.Flags().String("hostname", "localhost", "Where the application should run")
	config.BindPFlag(config.ContainerHostname, RunCmd.Flags().Lookup("hostname"))

	RunCmd.Flags().String("user", "user", "Where the application should run")
	config.BindPFlag(config.ContainerUsername, RunCmd.Flags().Lookup("user"))

	KonkCmd.AddCommand(RunCmd)
}

// Return a unique Rank of a container in a network. It either can be set over a command line,
// or obtained from an environment variable.
func GetContainerRank() (container.Rank, error) {
	// Environment variable containing the Rank
	envVarRank := config.GetString(config.ContainerRankEnv)

	var containerRank container.Rank
	if len(envVarRank) != 0 {
		envVal := os.Getenv(envVarRank)
		i, err := strconv.Atoi(envVal)
		if err != nil {
			return -1, fmt.Errorf(`Could not parse variable %s: %s`, envVarRank, envVal)
		}
		containerRank = container.Rank(i)
	}

	if containerRank < 0 {
		return -1, fmt.Errorf("Rank should be >= 0: %v", containerRank)
	}

	return containerRank, nil
}
