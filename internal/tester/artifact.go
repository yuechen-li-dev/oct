package tester

import (
	"fmt"
	"io"
	"sort"

	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type artifactCase struct {
	pkg      string
	filePath string
	name     string
}

func ExecuteArtifacts(path string, stdout io.Writer) error {
	return executeForPathOrExperiment(path, stdout, "artifact", executeArtifactsSingleRoot)
}

func executeArtifactsSingleRoot(path string, stdout io.Writer) error {
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

	for _, artifact := range artifacts {
		qualified := fmt.Sprintf("%s.%s", artifact.pkg, artifact.name)
		_, _ = fmt.Fprintf(stdout, "RUN  %s (%s)\n", qualified, shortPath(path, artifact.filePath))
		if err := interpret.ExecuteFunction(program, artifact.pkg, artifact.name, stdout); err != nil {
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, artifact.filePath), err)
			return fmt.Errorf("1 artifact(s) failed")
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, artifact.filePath))
	}

	_, _ = fmt.Fprintf(stdout, "Result: %d artifact(s) passed, 0 failed\n", len(artifacts))
	return nil
}
