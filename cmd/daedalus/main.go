package main

import (
	"context"
	"fmt"
	"os"

	"github.com/EstebanForge/daedalus/internal/app"
)

var version = "dev"

func main() {
	application := app.New(version)
	if err := application.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
