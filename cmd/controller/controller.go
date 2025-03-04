package main

import (
	"github.com/edgewize/edgeQ/cmd/controller/app"
	"log"
)

func main() {
	cmd := app.ModelServiceManagerCmd()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
