package main

import (
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"

	"github.com/planetA/konk/cmd"
	"github.com/planetA/konk/pkg/launch"
	"github.com/planetA/konk/pkg/hook"
)

func init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()

	if len(os.Args) > 1 && os.Args[1] == "init" {
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			log.Fatal(err)
		}
		panic("--this line should have never been executed, congratulations--")
	} else if len(os.Args) > 1 && os.Args[1] == "hook" {
		log.WithField("args", os.Args).Trace("Running hook")
		hook.Execute(os.Args[2:])
		panic("--this line should have never been executed, congratulations--")
	}
}

func main() {
	log.SetLevel(log.TraceLevel)

	if len(os.Args) > 1 && os.Args[1] == "launch" {
		launch.Launch(os.Args[2:])
		return
	}

	cmd.ExecuteKonk()
}
