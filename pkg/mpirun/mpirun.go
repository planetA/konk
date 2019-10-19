package mpirun

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/coordinator"
)

// Compose the params, it consists of three parts:
//   1. Path and arguments to mpirun
//   2. Path and arguments to konk-suid
//   3. Path and arguments to the actual program (provided as freeArgs)
func composeParams(image string, freeArgs []string) []string {
	// Find arguments to mpirun
	numproc := config.GetInt(config.MpirunNumproc)
	hosts := config.GetString(config.MpirunHosts)

	numprocName := config.GetString(config.MpirunNameNumproc)
	hostsName := config.GetString(config.MpirunNameHosts)

	savedParams := config.GetStringSlice(config.MpirunParams)

	params := append(savedParams, hostsName, hosts, numprocName, fmt.Sprintf("%v", numproc))

	// Find arguments to konk-suid
	konkPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error when computing absolute path: %v", err)
	}
	konkSuidPath := fmt.Sprintf("%v-suid", konkPath)

	// forward config file
	params = append(params, konkSuidPath, "--config", config.CfgFile, "run")

	params = append(params, "--image", image)

	// Put everything together
	params = append(params, freeArgs...)

	return params
}

func forwardSignal(process *os.Process, signal os.Signal) error {
	// We send local signal anyway
	defer process.Signal(signal)

	// Try to create connection to the coordinator
	coord, err := coordinator.NewClient()
	if err != nil {
		return fmt.Errorf("Cannot reach the coordinator: %v", err)
	}

	switch signal.(type) {
	case syscall.Signal:
		// Send a signal
		if err := coord.Signal(signal.(syscall.Signal)); err != nil {
			return fmt.Errorf("Could not send signal to the coordinator: %v", err)
		}
	default:
		return fmt.Errorf("Signal type is not supported")
	}

	return nil
}

func Run(image string, args []string) error {
	mpiBinpath := config.GetString(config.MpirunBinpath)

	params := composeParams(image, args)
	cmd := exec.Command(mpiBinpath, params...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Wait until command ends
	cmdDone := make(chan error, 1)
	go func() {
		err := cmd.Run()
		if err != nil {
			cmdDone <- fmt.Errorf("MPI terminated with an error: %v", err)
		} else {
			cmdDone <- nil
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	for {
		select {
		case signal := <-sigChan:
			fmt.Println("Forward signal")
			if err := forwardSignal(cmd.Process, signal); err != nil {
				return fmt.Errorf("Signal forwarding failed: %v", err)
			}
		case res := <-cmdDone:
			fmt.Println("Finished with: ", res)
			return nil
		}
	}
}
