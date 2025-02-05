package main

import (
	"github.com/edgewize/edgeQ/cmd/controller/app"
	"log"
)

var Version string
var BuildTime string

func main() {
	log.Printf("Current Version: %s", Version)
	log.Printf("Build Time: %s", BuildTime)

	cmd := app.ModelServiceManagerCmd()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
