package main

import (
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/opencontainers/runc/libcontainer"

	"github.com/planetA/konk/cmd"
	"github.com/planetA/konk/pkg/launch"
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
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "launch" {
		launch.Launch(os.Args[2:])
		return
	}

	log.SetLevel(log.TraceLevel)
	cmd.ExecuteKonk()
}
