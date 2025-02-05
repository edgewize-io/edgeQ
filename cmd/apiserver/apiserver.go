package main

import (
	"log"

	"github.com/edgewize/edgeQ/cmd/apiserver/app"
)

var Version string
var BuildTime string

func main() {
	log.Printf("Current Version: %s", Version)
	log.Printf("Build Time: %s", BuildTime)

	cmd := app.NewAPIServerCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
