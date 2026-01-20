package main

import (
	"os"

	"github.com/songzhibin97/go-arthas/cli"
)

func main() {
	exitCode := cli.Run(os.Args[1:])
	os.Exit(exitCode)
}
