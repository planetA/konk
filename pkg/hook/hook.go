package hook

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/planetA/konk/pkg/network"
)

func Execute(args []string) {
	if len(args) != 2 {
		log.WithField("args", args).Error("Expected exactly two arguments")
		os.Exit(1)
	}
	log.SetLevel(log.TraceLevel)

	hookType := args[0]
	netType := args[1]
	if err := network.RunHook(hookType, netType); err != nil {
		log.WithError(err).Error("Failed to run hook command")
		os.Exit(1)
	}

	os.Exit(0)
}
