// +build linux cgo

package initial

import (
	"fmt"
	"os/exec"
	"os"

	"github.com/planetA/konk/config"
)

// Start the init process that is also the child of the nymph
func Run(socket *os.File) (error) {
	initPath := config.GetString(config.KonkSysInit)
	cmd := exec.Command(initPath)
	cmd.Env = append(os.Environ(),
		"KONK_INIT_FD=3",
	)
	cmd.ExtraFiles = []*os.File{socket}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Application exited with an error: %v", err)
	}

	return nil
}
