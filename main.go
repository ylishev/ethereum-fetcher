// main.go is located at root level, in order to satisfy the `go run .` requirement
package main

import (
	"context"
	"os"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/di"
	"ethereum-fetcher/internal/server"

	"github.com/spf13/viper"
	"go.uber.org/dig"

	log "github.com/sirupsen/logrus"
)

func main() {
	// initialize dependencies
	container := dig.New()
	err := di.SetupContainer(container)
	if err != nil {
		log.Fatalf("cannot initialize dependencies: %v", err)
	}

	err = container.Invoke(func(vp *viper.Viper, cancel context.CancelFunc, limeAPIProvider *server.WebServer) {
		cmd.LogInit(vp.GetString(cmd.LogLevel))

		log.WithFields(log.Fields{
			"status": "starting",
			"port":   vp.GetString(cmd.APIPort),
			"pid":    os.Getpid(),
		}).Info("lime ethereum fetcher server")

		cmd.InitShutdownHandler(cancel)

		limeAPIProvider.Run(vp.GetInt(cmd.APIPort))
		log.Info("nuit, nuit")
	})
	if err != nil {
		log.Fatalf("cannot invoke dependencies: %v", err)
	}
}
