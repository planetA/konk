package main

import (
	"os"
	"log"
	"runtime"

	"github.com/planetA/konk/cmd"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			log.Fatal(err)
		}
		panic("--this line should have never been executed, congratulations--")
	}

	cmd.ExecuteKonk()
}
