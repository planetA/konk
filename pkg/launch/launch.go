package launch

import (
	"fmt"
	"os"
	"os/exec"
)

// Runs from inside a container to create a process that is get adopted by a nymph.
// Similar to
func Launch(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Failed to launch: %v", err)
	}

	return nil
}
