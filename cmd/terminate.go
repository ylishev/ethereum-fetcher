package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// InitShutdownHandler capture shutdown signals from OS and stops the main application
func InitShutdownHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	go func() {
		s := <-c
		log.Infof("got night, night signal: %v", s)
		cancel()
	}()

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
}
