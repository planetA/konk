package nymph

import (
	"fmt"
	"log"

	"github.com/planetA/konk/pkg/container"
)

// Connect to the local nymph daemon and ask it to create a container
func CreateContainer(id container.Id) (*container.Container, error) {
	client, err := NewClient("localhost")
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to nymph: %v", err)
	}
	defer client.Close()

	// Nymph returns the path to the container directory and waits until the container is created
	path, err := client.CreateContainer(id)
	if err != nil {
		return nil, fmt.Errorf("Container creation failed: %v", err)
	}

	// Once we know the path, we can attach to it
	cont, err := container.ContainerAttachInit(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a container: %v", err)
	}

	log.Println("Created")

	return cont, nil
}
