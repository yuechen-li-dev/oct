// Command oct is the command-line driver for Oct, a scientific programming language focused on correctness, testing, artifacts, and compiled execution.
package main

import (
	"os"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
