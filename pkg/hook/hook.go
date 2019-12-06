package hook

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/network"
)

func executeSafe(args []string) error {
	if len(args) != 2 {
		log.WithField("args", args).Error("Expected exactly two arguments")
		return fmt.Errorf("Expected exactly two arguments")
	}

	hookType := args[0]
	netType := args[1]

	logName := fmt.Sprintf("/tmp/konk/nymph/hook-%v-%v-%v.log", hookType, netType, time.Now().Unix())
	logFile, err := os.OpenFile(logName, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetLevel(log.TraceLevel)

	if err := network.RunHook(hookType, netType); err != nil {
		return err
	}

	return nil
}

func Execute(args []string) {
	if err := executeSafe(args); err != nil {
		log.WithError(err).Error("Failed to run hook command")
		os.Exit(1)
	}

	os.Exit(0)
}
