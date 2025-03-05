package main

import (
	"github.com/tomtom-international/macos-actions-runner-controller/cmd/tarter/app"
	"os"
)

func main() {
	command := app.NewTarterCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
