package main

import (
	"os"
	"runtime"

	"github.com/planetA/konk/cmd"
	"github.com/planetA/konk/pkg/launch"
)

func init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "launch" {
		launch.Launch(os.Args[2:])
		return
	}

	cmd.ExecuteKonk()
}
