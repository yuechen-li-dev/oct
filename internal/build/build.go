package build

import (
	"fmt"
	"os"
	"path/filepath"

	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
)

type Result struct {
	ArtifactPath string
}

func Compile(path string) (Result, error) {
	file, err := source.Load(path)
	if err != nil {
		return Result{}, err
	}

	tokens := lex.Analyze(file)
	program := parse.BuildProgram(tokens)

	artifactPath := artifactPathFor(program.SourcePath)
	artifactBody := []byte("oct m0 placeholder artifact\n")
	if err := os.WriteFile(artifactPath, artifactBody, 0o644); err != nil {
		return Result{}, fmt.Errorf("write artifact %s: %w", artifactPath, err)
	}

	return Result{ArtifactPath: artifactPath}, nil
}

func artifactPathFor(path string) string {
	return filepath.Join(filepath.Dir(path), filepath.Base(path)+".octbin")
}
