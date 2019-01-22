package nymph

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// Process reaper for init processes launched by the nymph
// XXX: Implement the reaper in the way that it will actually call all of its children, when
// nymph decides to exit, but the children are not done yet.
type Reaper struct {
	done          chan bool
	notifications chan os.Signal
}

// Stops the reaper one the nymph exits
func (r *Reaper) Stopper() {
	<-r.done
	log.Println("Stopping the reaper")
	signal.Stop(r.notifications)
	close(r.notifications)
}

// Reaper waits for SIGCHLD on a notification channel in a loop, and then calls waitpid.
// If notification is closed, reaper assumes there are no more children and exits.
//
// Theoretically, children still can exist at this point, if they ignore SIGINT,
// but I do not consider this case. The reason is that the reaper is only supposed to watch
// init-processes that are expected to react to SIGINT correctly. When an init process stops, all
// of its children should die automatically.
func (r *Reaper) Loop() {
	for {
		_, more := <-r.notifications
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

func NewReaper() (*Reaper, error) {
	reaper := &Reaper{
		done:          make(chan bool),
		notifications: make(chan os.Signal),
	}

	signal.Notify(reaper.notifications, syscall.SIGCHLD)

	// Become subreaper
	err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("Prctl: %v", err)
	}

	go reaper.Stopper()

	go reaper.Loop()

	return reaper, nil
}

func (r *Reaper) Close() {
	r.done <- true
}
