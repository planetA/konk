// prestart script
package prestart

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/planetA/konk/pkg/container"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func Run(args []string) error {
	var hookState configs.HookState

	err := json.NewDecoder(os.Stdin).Decode(&hookState)
	if err != nil {
		return fmt.Errorf("Failed to create JSON decoder: %v", err)
	}

	log.Println(hookState)

	id := 0
	if err := container.CreateNetwork(id); err != nil {
		return fmt.Errorf("Failed to create the network: %v", err)
	}
	return nil
}
