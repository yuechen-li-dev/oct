package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/yuechen-li-dev/oct/internal/jsoninfer"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

const jsonInferUsage = "usage: oct json infer <file.json> [--name <Name>] [--explain]"

func writeJSONHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, jsonInferUsage+"\n\nPrint the concept and record table declarations a JSON document loads into,\nand the Json.Load call that loads it. A program does not infer: it declares\na type, and Json.Load<T> reads the document as that type. This command writes\nthe declarations to start from.\n\n  --name <Name>   Name the declaration the whole document loads into.\n                  The default is the file's name.\n  --explain       Also print each choice that had more than one reading:\n                  record or keyed table, table or tagged array.\n\nThe output is Oct source and comments. A value with no declaration is listed\nwith its place, and the command fails.")
	return err
}

// executeJSON runs `oct json infer`. The work is internal/jsoninfer's; this
// reads the file and writes what was inferred.
func executeJSON(args []string, stdout io.Writer, stderr io.Writer, workingDir string) error {
	if len(args) == 0 || isHelpArg(args) || (args[0] == "infer" && isHelpArg(args[1:])) {
		return writeJSONHelp(stdout)
	}
	if args[0] != "infer" {
		return reportCommandError(stderr, "json", fmt.Errorf("unknown json command %q; expected infer", args[0]))
	}
	source, name, explain := "", "", false
	for index := 1; index < len(args); index++ {
		switch arg := args[index]; {
		case arg == "--explain":
			explain = true
		case arg == "--name":
			index++
			if index == len(args) {
				return reportCommandError(stderr, "json infer", errors.New("--name needs a name; "+jsonInferUsage))
			}
			name = args[index]
		case len(arg) > 0 && arg[0] == '-':
			return reportCommandError(stderr, "json infer", fmt.Errorf("unknown option %s; %s", arg, jsonInferUsage))
		case source != "":
			return reportCommandError(stderr, "json infer", errors.New("one file at a time; "+jsonInferUsage))
		default:
			source = arg
		}
	}
	if source == "" {
		return reportCommandError(stderr, "json infer", errors.New("missing file; "+jsonInferUsage))
	}
	text, err := os.ReadFile(resolveWorkingPath(workingDir, source))
	if err != nil {
		return reportCommandError(stderr, "json infer", fmt.Errorf("%s: %w", source, err))
	}
	document, parseErr := octjson.Parse(text)
	if parseErr != nil {
		return reportCommandError(stderr, "json infer", errors.New(parseErr.Text("oct json infer", source)))
	}
	result, err := jsoninfer.Infer(document, jsoninfer.Options{Source: source, Name: name})
	if err != nil {
		return reportCommandError(stderr, "json infer", err)
	}
	output := result.Text()
	if explain {
		output += result.Explain()
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		return err
	}
	if !result.Loads() {
		noun := "values have"
		if len(result.Refusals) == 1 {
			noun = "value has"
		}
		return reportCommandError(stderr, "json infer", fmt.Errorf("%d %s no declaration in %s; see the end of the output", len(result.Refusals), noun, source))
	}
	return nil
}
