package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/yuechen-li-dev/oct/internal/tester"
)

// structuredSchemaVersion is the stable machine-readable CLI contract for the
// routine agent workflow. Human-oriented output remains the default.
const structuredSchemaVersion = "oct.cli.result.v1"

type structuredCommandResult struct {
	SchemaVersion            string                     `json:"schemaVersion"`
	Command                  string                     `json:"command"`
	OctVersion               string                     `json:"octVersion"`
	ExecutionIdentity        string                     `json:"executionIdentity"`
	Target                   string                     `json:"target"`
	OK                       bool                       `json:"ok"`
	ExitStatus               int                        `json:"exitStatus"`
	TimingMilliseconds       int64                      `json:"timingMilliseconds"`
	Diagnostics              []structuredDiagnostic     `json:"diagnostics"`
	Stdout                   string                     `json:"stdout"`
	DiscoveredTestFiles      []string                   `json:"discoveredTestFiles,omitempty"`
	Execution                structuredExecution        `json:"execution"`
	Summary                  structuredSummary          `json:"summary"`
	Artifacts                []tester.GeneratedArtifact `json:"artifacts,omitempty"`
	ArtifactMetadataComplete bool                       `json:"artifactMetadataComplete,omitempty"`
}

type structuredDiagnostic struct {
	Severity string `json:"severity"`
	Phase    string `json:"phase"`
	Message  string `json:"message"`
}

type structuredExecution struct {
	Requested            string `json:"requested"`
	Actual               string `json:"actual,omitempty"`
	CompiledCases        int    `json:"compiledCases"`
	InterpretedFallbacks int    `json:"interpretedFallbacks"`
}

type structuredSummary struct {
	Discovered int `json:"discovered"`
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

var (
	structuredOutcomeRE   = regexp.MustCompile(`(?m)^(?:\[[^]\r\n]+\] )?(PASS|FAIL|SKIP) `)
	structuredExecutionRE = regexp.MustCompile(`(?m)^(?:\[[^]\r\n]+\] )?Execution summary: compiled: (\d+) interpreted fallback: (\d+)`)
)

func executeTestJSON(path string, stdout io.Writer, options tester.TestOptions) error {
	options.JSON = false
	started := time.Now()
	var log bytes.Buffer
	err := tester.ExecuteWithOptions(path, &log, options)
	result := baseStructuredResult("oct test", path, "test", options.Execution, started, log.String(), err)
	result.DiscoveredTestFiles = tester.DiscoverTestFilesForTarget(path)
	return writeStructuredResult(stdout, result, err)
}

func executeArtifactJSON(path string, stdout io.Writer, options tester.ArtifactOptions) error {
	options.JSON = false
	report := &tester.ArtifactReport{}
	options.Report = report
	started := time.Now()
	var log bytes.Buffer
	err := tester.ExecuteArtifactsWithOptions(path, &log, options)
	result := baseStructuredResult("oct artifact", path, "artifact", options.Execution, started, log.String(), err)
	result.Artifacts = report.Artifacts
	result.ArtifactMetadataComplete = report.MetadataComplete
	result.Execution.Actual = report.Execution
	return writeStructuredResult(stdout, result, err)
}

func baseStructuredResult(command string, path string, phase string, requested string, started time.Time, output string, commandErr error) structuredCommandResult {
	if requested == "" {
		if command == "oct test" {
			requested = "auto"
		} else {
			requested = "interpreted"
		}
	}
	result := structuredCommandResult{
		SchemaVersion:      structuredSchemaVersion,
		Command:            command,
		OctVersion:         version,
		ExecutionIdentity:  "gooct-cli",
		Target:             filepath.ToSlash(filepath.Clean(path)),
		OK:                 commandErr == nil,
		TimingMilliseconds: time.Since(started).Milliseconds(),
		Diagnostics:        []structuredDiagnostic{},
		Stdout:             output,
		Execution:          structuredExecution{Requested: requested},
	}
	if commandErr != nil {
		result.ExitStatus = 1
		result.Diagnostics = append(result.Diagnostics, structuredDiagnostic{Severity: "error", Phase: phase, Message: commandErr.Error()})
	}
	for _, match := range structuredOutcomeRE.FindAllStringSubmatch(output, -1) {
		switch match[1] {
		case "PASS":
			result.Summary.Passed++
		case "FAIL":
			result.Summary.Failed++
		case "SKIP":
			result.Summary.Skipped++
		}
	}
	result.Summary.Discovered = result.Summary.Passed + result.Summary.Failed + result.Summary.Skipped
	if match := structuredExecutionRE.FindStringSubmatch(output); len(match) == 3 {
		result.Execution.CompiledCases, _ = strconv.Atoi(match[1])
		result.Execution.InterpretedFallbacks, _ = strconv.Atoi(match[2])
	}
	return result
}

func writeStructuredResult(stdout io.Writer, result structuredCommandResult, commandErr error) error {
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return err
	}
	return commandErr
}
