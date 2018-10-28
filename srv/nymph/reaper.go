package nymph

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Process reaper for init processes launched by the nymph
// XXX: Implement the reaper in the way that it will actually call all of its children, when
// nymph decides to exit, but the children are not done yet.
type Reaper struct {
	done chan bool
}

// Stops the reaper one the nymph exits
func reaperStopper(done chan bool, notifications chan os.Signal) {
	<-done
	log.Println("Stopping the reaper")
	signal.Stop(notifications)
	close(notifications)
}

// Reaper waits for SIGCHLD on a notification channel in a loop, and then calls waitpid.
// If notification is closed, reaper assumes there are no more children and exits.
//
// Theoretically, children still can exist at this point, if they ignore SIGINT,
// but I do not consider this case. The reason is that the reaper is only supposed to watch
// init-processes that are expected to react to SIGINT correctly. When an init process stops, all
// of its children should die automatically.
func reaperLoop(done chan bool, notifications chan os.Signal) {
	for {
		_, more := <-notifications
		if !more {
			log.Println("No more children. Leaving the reaper")
			return
		}

	waitpidLoop:
		for {
			var wstatus syscall.WaitStatus
			switch pid, err := syscall.Wait4(-1, &wstatus, 0, nil); err {
			case syscall.EINTR:
				continue
			case syscall.ECHILD:
				log.Println("No more children. Waiting for more.")
				break waitpidLoop
			case nil:
				log.Printf("Reaped a child: %v", pid)
			default:
				log.Panicf("Unexpected signal in reaper (pid=%v): %v", pid, err)
			}
		}
	}
}

func NewReaper() *Reaper {
	done := make(chan bool)
	var notifications = make(chan os.Signal)
	signal.Notify(notifications, syscall.SIGCHLD)

	go reaperStopper(done, notifications)

	go reaperLoop(done, notifications)

	return &Reaper{
		done: done,
	}
}

func (r *Reaper) Close() {
	r.done <- true
}