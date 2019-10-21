package nymph

import (
	"github.com/opencontainers/runc/libcontainer/configs"

	"github.com/planetA/konk/pkg/container"
)

// func prestartNetworkSetup(s *specs.State) error {
// 	cniPath := filepath.SplitList(os.Getenv(config.GetString(config.NymphCniPath)))

// 	_ = libcni.NewCNIConfig(cniPath, nil)

// 	log.Trace("prestartNetworkSetup")

// 	return nil
// }

// func appendPrestartHook(config *configs.Config, f func(*specs.State) error) {
// 	hook := configs.NewFunctionHook(f)
// 	config.Hooks.Prestart = append(config.Hooks.Prestart, hook)
// }

func setupNetwork(image *container.Image, ociConfig *configs.Config) error {

	// appendPrestartHook(ociConfig, prestartNetworkSetup)

	return nil
}

func instantiateConfig(image *container.Image) (*configs.Config, error) {
	var newConfig configs.Config

	newConfig = *image.Config

	if err := setupNetwork(image, &newConfig); err != nil {
		return nil, err
	}

	return &newConfig, nil
}
