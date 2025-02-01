package main

import (
	"github.com/edgewize/modelmesh/cmd/msc/app"
	"log"
)

func main() {
	cmd := app.ModelServiceManagerCmd()

	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
