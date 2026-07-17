package tester

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type artifactCase struct {
	pkg      string
	filePath string
	name     string
}

type ArtifactOptions struct {
	Execution   string
	AllPackages bool
	JSON        bool
	Report      *ArtifactReport
}

// ArtifactReport is an observation of the artifact lane for CLI and agent
// callers. It does not change file creation or artifact execution semantics.
type ArtifactReport struct {
	Execution        string
	MetadataComplete bool
	Artifacts        []GeneratedArtifact
}

type GeneratedArtifact struct {
	Function string `json:"function"`
	Path     string `json:"path"`
	MIMEType string `json:"mimeType"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}

func ExecuteArtifacts(path string, stdout io.Writer) error {
	return ExecuteArtifactsWithOptions(path, stdout, ArtifactOptions{})
}

func ExecuteArtifactsWithOptions(path string, stdout io.Writer, options ArtifactOptions) error {
	executionMode := strings.TrimSpace(options.Execution)
	if executionMode == "" {
		executionMode = "interpreted"
	}
	if executionMode != "compiled" && executionMode != "interpreted" {
		return fmt.Errorf("invalid artifact execution mode %q (expected compiled|interpreted)", executionMode)
	}
	if options.Report != nil {
		options.Report.Execution = executionMode
		options.Report.MetadataComplete = executionMode == "interpreted"
		options.Report.Artifacts = nil
	}
	return executeForPathOrExperiment(path, stdout, "artifact", func(singlePath string, singleStdout io.Writer) error {
		return executeArtifactsSingleRoot(singlePath, singleStdout, executionMode, options.AllPackages, options.Report)
	})
}

func executeArtifactsSingleRoot(path string, stdout io.Writer, executionMode string, allPackages bool, report *ArtifactReport) error {
	program, err := project.LoadForTest(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	selectedSources, err := selectedTestSources(path)
	if err != nil {
		return err
	}
	var artifacts []artifactCase
	for pkgName, pkg := range program.Packages {
		if !allPackages && pkgName != program.Entry {
			continue
		}
		for _, fn := range pkg.Functions {
			if !fn.IsArtifact || !isSelectedSource(selectedSources, fn.SourcePath) {
				continue
			}
			artifacts = append(artifacts, artifactCase{pkg: pkgName, filePath: fn.SourcePath, name: fn.Name})
		}
	}

	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].pkg != artifacts[j].pkg {
			return artifacts[i].pkg < artifacts[j].pkg
		}
		if artifacts[i].filePath != artifacts[j].filePath {
			return artifacts[i].filePath < artifacts[j].filePath
		}
		return artifacts[i].name < artifacts[j].name
	})

	if len(artifacts) == 0 {
		return fmt.Errorf("no [Artifact] functions found")
	}

	_, _ = fmt.Fprintf(stdout, "Execution: %s\n", executionMode)
	for _, artifact := range artifacts {
		qualified := fmt.Sprintf("%s.%s", artifact.pkg, artifact.name)
		_, _ = fmt.Fprintf(stdout, "RUN  %s (%s)\n", qualified, shortPath(path, artifact.filePath))
		err := executeArtifactCase(program, artifact, stdout, executionMode, report)
		if err != nil {
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, artifact.filePath), err)
			return fmt.Errorf("1 artifact(s) failed")
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, artifact.filePath))
	}

	_, _ = fmt.Fprintf(stdout, "Result: %d artifact(s) passed, 0 failed\n", len(artifacts))
	return nil
}

func executeArtifactCase(program project.Program, artifact artifactCase, stdout io.Writer, executionMode string, report *ArtifactReport) error {
	if executionMode == "compiled" {
		return executeCompiledArtifactCase(program, artifact, stdout)
	}
	qualified := fmt.Sprintf("%s.%s", artifact.pkg, artifact.name)
	return interpret.ExecuteFunctionWithArgsAndOptions(program, artifact.pkg, artifact.name, nil, stdout, interpret.ExecuteOptions{
		ArtifactProgressRecorder: func(event interpret.ArtifactProgressEvent) {
			if event.Kind == "checkpoint" {
				_, _ = fmt.Fprintf(stdout, "CHECKPOINT %s: %s\n", qualified, event.Label)
				return
			}
			if event.Kind == "progress" {
				_, _ = fmt.Fprintf(stdout, "PROGRESS %s: %s %d/%d\n", qualified, event.Label, event.Current, event.Total)
			}
		},
		ArtifactWriteRecorder: func(event interpret.ArtifactWriteEvent) {
			if report != nil {
				reportGeneratedArtifact(report, event)
			}
		},
	})
}

func reportGeneratedArtifact(report *ArtifactReport, event interpret.ArtifactWriteEvent) {
	info, err := os.Stat(event.Path)
	if err != nil || info.IsDir() {
		return
	}
	contents, err := os.ReadFile(event.Path)
	if err != nil {
		return
	}
	sum := sha256.Sum256(contents)
	record := GeneratedArtifact{
		Function: event.Function,
		Path:     filepath.ToSlash(filepath.Clean(event.Path)),
		MIMEType: artifactMIMEType(filepath.Ext(event.Path)),
		Bytes:    info.Size(),
		SHA256:   hex.EncodeToString(sum[:]),
	}
	for index, existing := range report.Artifacts {
		if existing.Path == record.Path {
			report.Artifacts[index] = record
			return
		}
	}
	report.Artifacts = append(report.Artifacts, record)
	sort.Slice(report.Artifacts, func(i, j int) bool { return report.Artifacts[i].Path < report.Artifacts[j].Path })
}

func artifactMIMEType(ext string) string {
	switch strings.ToLower(ext) {
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".md":
		return "text/markdown"
	case ".octagon":
		return "application/octet-stream"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func executeCompiledArtifactCase(program project.Program, artifact artifactCase, stdout io.Writer) (retErr error) {
	pkg, ok := program.Packages[artifact.pkg]
	if !ok {
		return fmt.Errorf("unknown package %q", artifact.pkg)
	}
	scope, err := newArtifactScope("oct-artifact-run", stdout)
	if err != nil {
		return err
	}
	defer closeArtifactScope(scope, &retErr)
	runnerPath, err := writeCompiledArtifactRunner(scope.path("runner.octest"), artifact.pkg, artifact.name)
	if err != nil {
		return err
	}
	result, err := build.CompileForTestWithSelectedFilesInPackage(runnerPath, pkg.Directory, []string{runnerPath, artifact.filePath})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestCycleTime)
	defer cancel()
	cmd := exec.CommandContext(ctx, result.ArtifactPath)
	output, runErr := cmd.CombinedOutput()
	if len(output) > 0 {
		_, _ = stdout.Write(output)
	}
	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("compiled artifact run failed: %w", runErr)
		}
		return fmt.Errorf("compiled artifact run failed: %w: %s", runErr, msg)
	}
	return nil
}

func writeCompiledArtifactRunner(runnerPath string, pkgName string, fnName string) (string, error) {
	source := strings.Join([]string{
		"package " + pkgName,
		"fn main() -> Int {",
		"    " + fnName + "()",
		"    return 0",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(runnerPath, []byte(source), 0o644); err != nil {
		return "", err
	}
	return runnerPath, nil
}
