package mpirun

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/coordinator"
	"github.com/sirupsen/logrus"
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

func createContainerFactory() (libcontainer.Factory, error) {
	tmpDir := config.GetString(config.MpirunTmpDir)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		logrus.WithFields(logrus.Fields{
			"path": tmpDir,
		}).Info("Temp directory already exists. Purging.")
		os.RemoveAll(tmpDir)
	}
	logrus.WithFields(logrus.Fields{
		"path": tmpDir,
	}).Trace("Creating temporary directory")

	if err := os.MkdirAll(tmpDir, 0770); err != nil {
		return nil, fmt.Errorf("Failed to create temporary directory %v: %v", tmpDir, err)
	}

	containersPath := path.Join(tmpDir, "containers")
	factory, err := libcontainer.New(containersPath, libcontainer.Cgroupfs, libcontainer.InitArgs(os.Args[0], "init"))
	if err != nil {
		return nil, fmt.Errorf("Failed to create container factory: %v", err)
	}

	return factory, nil
}

// Launch mpirun inside the container, request nymphs to create local containers
// and let mpirun to launch the processes in the newly created containers
func Run(image string, args []string) error {
	containerFactory, err := createContainerFactory(); 
	if err != nil {
		return fmt.Errorf("Failed to create mpirun container factory: %v", err)
	}

	containerConfig := &configs.Config{}
	container, err := containerFactory.Create("mpirun", containerConfig)
	if err != nil {
		return fmt.Errorf("Creating container failed", err)
	}
	defer container.Destroy()

	mpiBinpath := config.GetString(config.MpirunBinpath)

	params := composeParams(image, args)
	log.Println(params)

	process := &libcontainer.Process{
		Args: append([]string{mpiBinpath}, params...),
		Env: []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		User: "user",
		Stdin: os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Init: true,
	}

	// Wait until command ends
	if err := container.Run(process); err != nil {
		log.Println("MPI terminated with an error: %v", err)
		return fmt.Errorf("MPI terminated with an error: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)
	go func() {
		for {
			signal := <-sigChan
			fmt.Println("Forward signal")
			if err := process.Signal(signal); err != nil {
				log.Println("Signal forwarding failed: ", err)
				return
			}
		}
	}()

	ret, err := process.Wait()
	if err != nil {
		return fmt.Errorf("Waiting for process failed: %v", err)
	}

	log.Println("Process finished with state: ", ret)

	return nil
}
