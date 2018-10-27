package util

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
)

var (
	wg = &sync.WaitGroup{}
)

type CrashFunction func()

func CrashHandler(ctx context.Context, cf CrashFunction) {
	wg.Add(1)
	go func() {
		<-ctx.Done()
		cf()
		wg.Done()
	}()
}

func NewContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt)
		defer signal.Stop(c)

		log.Println("Expect signal")
		select {
		case <-c:
			log.Println("Received Ctrl-C")
			cancel()
		case <-ctx.Done():
			log.Println("Got cancel")
			wg.Wait()
			log.Println("Wg ready")
			os.Exit(0)
		}
	}()

	return ctx, cancel
}
