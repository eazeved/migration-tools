package main

import (
	"fmt"
	"os"

	"github.com/eazeved/migration-tools/cmd"
)

var VERSION = "dev"

func main() {
	if err := cmd.NewRootCmd(VERSION).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
