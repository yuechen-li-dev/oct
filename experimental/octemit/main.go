// Command octemit is the bounded driver for the compiled-backend source probe.
// It is intentionally experimental and is not part of the stable Oct CLI.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/build"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 || (arguments[0] != "generate" && arguments[0] != "check") {
		return fmt.Errorf("usage: octemit <generate|check> -input <source.oct> -output <sibling.generated.go> -package <go-package>")
	}
	command := arguments[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	input := flags.String("input", "", "ordinary Oct source or project")
	output := flags.String("output", "", "sibling .generated.go destination")
	packageName := flags.String("package", "", "destination Go package")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *output == "" || *packageName == "" {
		return fmt.Errorf("usage: octemit <generate|check> -input <source.oct> -output <sibling.generated.go> -package <go-package>")
	}
	if err := validateDestination(*input, *output); err != nil {
		return err
	}
	source, err := build.EmitGoSource(*input, build.GoSourceOptions{PackageName: *packageName})
	if err != nil {
		return fmt.Errorf("emit compiled Go: %w", err)
	}
	if command == "check" {
		existing, err := os.ReadFile(*output)
		if err != nil || !bytes.Equal(existing, source) {
			return fmt.Errorf("generated Go is stale: %s; run octemit generate", *output)
		}
		return nil
	}
	if err := os.WriteFile(*output, source, 0o644); err != nil {
		return fmt.Errorf("write generated Go %s: %w", *output, err)
	}
	return nil
}

func validateDestination(input, output string) error {
	inputDirectory, err := filepath.Abs(filepath.Dir(input))
	if err != nil {
		return fmt.Errorf("resolve input directory: %w", err)
	}
	outputPath, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if filepath.Dir(outputPath) != inputDirectory || !strings.HasSuffix(filepath.Base(outputPath), ".generated.go") {
		return fmt.Errorf("output must be a sibling .generated.go file beside %s", input)
	}
	return nil
}
