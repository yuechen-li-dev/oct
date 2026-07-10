package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	Execution string
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
	return executeForPathOrExperiment(path, stdout, "artifact", func(singlePath string, singleStdout io.Writer) error {
		return executeArtifactsSingleRoot(singlePath, singleStdout, executionMode)
	})
}

func executeArtifactsSingleRoot(path string, stdout io.Writer, executionMode string) error {
	program, err := project.LoadForTest(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	var artifacts []artifactCase
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if !fn.IsArtifact {
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
		err := executeArtifactCase(program, artifact, stdout, executionMode)
		if err != nil {
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, artifact.filePath), err)
			return fmt.Errorf("1 artifact(s) failed")
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, artifact.filePath))
	}

	_, _ = fmt.Fprintf(stdout, "Result: %d artifact(s) passed, 0 failed\n", len(artifacts))
	return nil
}

func executeArtifactCase(program project.Program, artifact artifactCase, stdout io.Writer, executionMode string) error {
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
	})
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
