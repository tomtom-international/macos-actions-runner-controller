package main

import (
	"github.com/tomtom-international/macos-actions-runner-controller/cmd/controller/app"
	"os"
)

func main() {
	command := app.NewControllerCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
