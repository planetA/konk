package mpirun

import (
	"os"
	"log"
	"fmt"
	"os/exec"

	"github.com/planetA/konk/config"
)

// Compose the params, it consists of three parts:
//   1. Path and arguments to mpirun
//   2. Path and arguments to konk-suid
//   3. Path and arguments to the actual program (provided as freeArgs)
func composeParams(freeArgs []string) []string {
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
		log.Fatal("Error when computing absolute path: %v", err)
	}
	konkSuidPath := fmt.Sprintf("%v-suid", konkPath)

	// forward config file
	params = append(params, konkSuidPath, "--config", config.CfgFile, "run")

	// Put everything together
	params = append(params, freeArgs...)

	return params
}

func Run(args []string) error {
	mpiBinpath := config.GetString(config.MpirunBinpath)

	params := composeParams(args)
	cmd := exec.Command(mpiBinpath, params...)

	fmt.Println(cmd)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("MPI terminated with an error: %v", err)
	}

	return nil
}
