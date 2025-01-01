package main

import (
	"llm_term/pkg/ui"
	"log"
)

func main() {
	app := ui.New()
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}