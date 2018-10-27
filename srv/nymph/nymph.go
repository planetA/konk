// The node-daemon service called nymph is responsible for setting up the node
// for konk applications to run. It is also responsible for notifying the
// coordinator about new containers. Additionally, nymph deamons coordinate migration.
package nymph

import (
	"log"
	"net/rpc"

	"github.com/planetA/konk/config"
	"github.com/planetA/konk/pkg/util"
)

// The node daemon controls node local operations. In the beginning the daemon opens a socket
// and waits for commands from scheduler. The scheduler sends migration commands to the daemon:
// either send or receive a process.
func Run() error {
	listener, err := util.CreateListener(config.GetInt(config.NymphPort))
	if err != nil {
		return err
	}
	defer listener.Close()

	ctx, cancel := util.NewContext()
	defer cancel()

	nymph := NewNymph()
	util.CrashHandler(ctx, func() {
		log.Println("Nymph is exiting")
		nymph._Close()
	})

	rpc.Register(nymph)

	if err := util.ServerLoop(listener); err != nil {
		return err
	}

	return nil
}
