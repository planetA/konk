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

	cont, err := nymph.CreateContainer(id);
	if  err != nil {
		return err
	}

	pid, err := readNumber(cont.Path, "pid")
	if err != nil {
		return err
	}

	cmd, err := container.LaunchCommandInitProc(pid, args)
	if err != nil {
		return err
	}

	return nil

	cmd, err = cont.LaunchCommand(args)
	if err != nil {
		return err
	}

	log.Println("Launched command. No waiting. Pid: ", cmd.Process.Pid)

	cmd.Wait()

	return nil
}

