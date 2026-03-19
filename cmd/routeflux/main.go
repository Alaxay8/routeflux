package main

import (
	"os"

	"github.com/Alaxay8/routeflux/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
