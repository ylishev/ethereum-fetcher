package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LogInit the logrus level with appropriate settings and log level from command line
func LogInit(level string) {
	logrus.SetFormatter(&logrus.TextFormatter{})

	logrus.SetOutput(os.Stdout)

	logrusLevel, err := logrus.ParseLevel(level)
	if err != nil {
		// Fallback to InfoLevel
		logrusLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logrusLevel)
}
