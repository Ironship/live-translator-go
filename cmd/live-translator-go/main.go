//go:build windows

package main

import (
	"log"

	"live-translator-go/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}