// Container-process service
package coproc

import (
	"log"
	"runtime"
	"path"
	"io/ioutil"
	"fmt"
	"strconv"

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/nymph"
)

func readNumber(containerPath, fileName string) (int, error) {
	filePath := path.Join(containerPath, fileName)
	buffer, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("Failed to open file (%v): %v", filePath, err)
	}

	number, err := strconv.Atoi(string(buffer))
	if err != nil {
		return 0, fmt.Errorf("Failed to read file %v: %v", err)
	}

	return number, nil
}

func Run(id container.Id, args []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	nymph, err := nymph.NewClient("localhost")
	if err != nil {
		return fmt.Errorf("Failed to connect to nymph: %v", err)
	}
	defer nymph.Close()

	// Nymph returns the path to the container directory and waits until the container is created
	path, err := nymph.CreateContainer(id)
	if err != nil {
		return fmt.Errorf("Container creation failed: %v", err)
	}

	pid, err := readNumber(path, "pid")
	if err != nil {
		return err
	}

	_, err = container.LaunchCommandInitProc(pid, args)
	if err != nil {
		return err
	}

	log.Println("The command is launched")

	if err := nymph.NotifyProcess(id); err != nil {
		return err
	}

	return nil
}

