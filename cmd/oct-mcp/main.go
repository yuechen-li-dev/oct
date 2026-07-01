// Command oct-mcp is a Model Context Protocol server for the Oct toolchain.
//
// Design note: this binary lives inside the Oct module (github.com/yuechen-li-dev/oct)
// specifically so it can import internal/cli directly. Go's "internal/" import
// visibility rule restricts internal/cli to importers within this module tree,
// so an MCP server built as a *separate* Go module could not reach it and would
// be limited to shelling out to a built `oct` binary via os/exec. Living inside
// the module lets every tool call below run cli.Execute in-process against
// bytes.Buffer stdout/stderr — no subprocess spawn, no PATH lookup for a
// pre-built binary, and no text-scraping of a child process's output.
//
// cli.Execute(args []string, stdout, stderr io.Writer) error is the same
// function cmd/oct/main.go calls; every tool here just constructs the
// argv cli.Execute already knows how to parse (test/build/run/artifact/new/fmt)
// and reports back stdout, stderr, and success/failure. This keeps the MCP
// surface automatically in sync with the CLI's own flag parsing and help text,
// rather than duplicating tester/build/run logic at a lower level.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yuechen-li-dev/oct/internal/cli"
)

// runCLI invokes internal/cli.Execute in-process, capturing stdout/stderr into
// buffers instead of the process's real streams. cli.Execute intentionally
// returns a non-nil error whenever the command reported failure (build
// errors, failing tests, etc.) as well as for usage errors, so a non-nil err
// here is not itself exceptional — it's how "the oct command failed" is
// reported. Every tool handler below treats it as tool_result content, not a
// Go-level failure, so the model sees the actual compiler/test output either way.
func runCLI(args []string) (stdout string, stderr string, execErr error) {
	var outBuf, errBuf bytes.Buffer
	execErr = cli.Execute(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), execErr
}

// toolResult formats a runCLI() outcome as a CallToolResult. Successes and
// tool-reported failures (bad path, failing tests, non-zero build errors)
// both come back as ordinary text content so the model can read and act on
// them; IsError is set only for command failures so the model doesn't need
// to string-match "FAIL" itself, and can still see the raw output either way.
func toolResult(command string, stdout, stderr string, execErr error) *mcp.CallToolResult {
	text := stdout
	if stderr != "" {
		if text != "" {
			text += "\n"
		}
		text += "[stderr]\n" + stderr
	}
	if execErr != nil {
		if text != "" {
			text += "\n"
		}
		text += fmt.Sprintf("[oct %s failed] %v", command, execErr)
	}
	if text == "" {
		text = fmt.Sprintf("oct %s completed with no output", command)
	}
	return &mcp.CallToolResult{
		IsError: execErr != nil,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// ---------------------------------------------------------------------
// oct_test
// ---------------------------------------------------------------------

type testArgs struct {
	Path        string `json:"path" jsonschema:"file or directory path to test (relative to the Oct workspace root, or absolute)"`
	Suite       string `json:"suite,omitempty" jsonschema:"restrict to facts/artifacts tagged with this [Suite(\"...\")] name"`
	Execution   string `json:"execution,omitempty" jsonschema:"auto (default), compiled, or interpreted"`
	AllPackages bool   `json:"all_packages,omitempty" jsonschema:"recurse and run every discovered package under path, not just path itself"`
}

func handleTest(_ context.Context, _ *mcp.CallToolRequest, args testArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "path is required"}}}, nil, nil
	}
	cliArgs := []string{"test"}
	if args.Suite != "" {
		cliArgs = append(cliArgs, "--suite", args.Suite)
	}
	if args.Execution != "" {
		cliArgs = append(cliArgs, "--execution", args.Execution)
	}
	if args.AllPackages {
		cliArgs = append(cliArgs, "--all-packages")
	}
	cliArgs = append(cliArgs, args.Path)

	stdout, stderr, err := runCLI(cliArgs)
	return toolResult("test", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// oct_build
// ---------------------------------------------------------------------

type buildArgs struct {
	Path string `json:"path" jsonschema:"path to the .oct source file or package to compile"`
}

func handleBuild(_ context.Context, _ *mcp.CallToolRequest, args buildArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "path is required"}}}, nil, nil
	}
	stdout, stderr, err := runCLI([]string{"build", args.Path})
	return toolResult("build", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// oct_run
// ---------------------------------------------------------------------

type runArgs struct {
	Path string `json:"path" jsonschema:"path to the .oct program to run"`
}

func handleRun(_ context.Context, _ *mcp.CallToolRequest, args runArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "path is required"}}}, nil, nil
	}
	stdout, stderr, err := runCLI([]string{"run", args.Path})
	return toolResult("run", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// oct_artifact
// ---------------------------------------------------------------------

type artifactArgs struct {
	Path      string `json:"path" jsonschema:"file or directory path whose [Artifact] blocks should be generated"`
	Execution string `json:"execution,omitempty" jsonschema:"compiled or interpreted"`
}

func handleArtifact(_ context.Context, _ *mcp.CallToolRequest, args artifactArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "path is required"}}}, nil, nil
	}
	cliArgs := []string{"artifact"}
	if args.Execution != "" {
		cliArgs = append(cliArgs, "--execution", args.Execution)
	}
	cliArgs = append(cliArgs, args.Path)

	stdout, stderr, err := runCLI(cliArgs)
	return toolResult("artifact", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// oct_new
// ---------------------------------------------------------------------

type newArgs struct {
	Kind string `json:"kind" jsonschema:"experiment, library, wrapper-library, application, or app"`
	Name string `json:"name" jsonschema:"PascalCase package name"`
	Dir  string `json:"dir" jsonschema:"explicit target directory for the new scaffold (required — the server has no implicit working directory to infer Experiments/Libraries/Applications from)"`
}

func handleNew(_ context.Context, _ *mcp.CallToolRequest, args newArgs) (*mcp.CallToolResult, any, error) {
	if args.Kind == "" || args.Name == "" || args.Dir == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "kind, name, and dir are all required"}}}, nil, nil
	}
	// oct new's normal CWD-relative auto-placement (Experiments/<Name>,
	// Libraries/<Name>, ...) depends on the process's working directory,
	// which is shared and mutable process-global state — unsafe to rely on
	// for concurrent MCP tool calls. Always pass the explicit [path]
	// argument instead so this tool has no CWD dependency at all.
	stdout, stderr, err := runCLI([]string{"new", args.Kind, args.Name, args.Dir})
	return toolResult("new", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// oct_fmt_check
// ---------------------------------------------------------------------

type fmtCheckArgs struct {
	Path string `json:"path" jsonschema:"file or directory path to check formatting for"`
}

func handleFmtCheck(_ context.Context, _ *mcp.CallToolRequest, args fmtCheckArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "path is required"}}}, nil, nil
	}
	// --check only: this tool never rewrites files. A separate, explicitly
	// named oct_fmt_write tool would be the place for mutating formatting,
	// kept out of scope here deliberately (read-only tools first).
	stdout, stderr, err := runCLI([]string{"fmt", "--check", args.Path})
	return toolResult("fmt --check", stdout, stderr, err), nil, nil
}

// ---------------------------------------------------------------------
// server wiring
// ---------------------------------------------------------------------

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "oct-mcp",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_test",
		Description: "Run Oct octest suites ([Fact]s) under a file or directory and report pass/fail output.",
	}, handleTest)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_build",
		Description: "Compile an Oct source file or package, reporting typecheck/compile errors if any.",
	}, handleBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_run",
		Description: "Run an Oct program and capture its stdout/stderr.",
	}, handleRun)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_artifact",
		Description: "Run [Artifact] blocks under a file or directory, producing .octagon output files.",
	}, handleArtifact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_new",
		Description: "Scaffold a new Oct experiment, library, wrapper-library, or application at an explicit target directory.",
	}, handleNew)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "oct_fmt_check",
		Description: "Check (without modifying) whether Oct source files under a path are canonically formatted.",
	}, handleFmtCheck)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Println("oct-mcp server exited:", err)
		os.Exit(1)
	}
}
