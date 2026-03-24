package build

import (
	"fmt"
	"os"
	"path/filepath"

	"oct/internal/project"
	"oct/internal/typecheck"
)

type Result struct {
	ArtifactPath string
}

func Compile(path string) (Result, error) {
	program, err := project.Load(path)
	if err != nil {
		return Result{}, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return Result{}, err
	}

	artifactPath := artifactPathFor(program.EntrySource)
	artifactBody := []byte("oct m0 placeholder artifact\n")
	if err := os.WriteFile(artifactPath, artifactBody, 0o644); err != nil {
		return Result{}, fmt.Errorf("write artifact %s: %w", artifactPath, err)
	}

	return Result{ArtifactPath: artifactPath}, nil
}

func artifactPathFor(path string) string {
	return filepath.Join(filepath.Dir(path), filepath.Base(path)+".octbin")
}
