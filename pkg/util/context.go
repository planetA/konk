package util

import (
	"context"
	"os"
	"os/signal"
)

func NewContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c)
		defer signal.Stop(c)

		select {
		case <-c:
			cancel()
		}
	}()

	return ctx, cancel
}
