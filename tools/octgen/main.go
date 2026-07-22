// Command octgen runs the experimental external Oct-to-Go generator.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuechen-li-dev/oct/internal/octgen"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: octgen <generate|check> -input generator.oct -output generated.go")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet("octgen "+command, flag.ExitOnError)
	input := flags.String("input", "", "Oct generator source")
	output := flags.String("output", "", "generated Go output")
	flags.Parse(os.Args[2:])
	if *input == "" || *output == "" {
		fatal("-input and -output are required")
	}
	var err error
	switch command {
	case "generate":
		err = octgen.Write(*input, *output)
	case "check":
		err = octgen.Check(*input, *output)
	default:
		fatal("unknown command %q; expected generate or check", command)
	}
	if err != nil {
		fatal("octgen %s: %v", command, err)
	}
}

func fatal(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", arguments...)
	os.Exit(1)
}
