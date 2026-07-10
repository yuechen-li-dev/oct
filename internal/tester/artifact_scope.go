package tester

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	testArtifactRootEnv  = "OCT_TEST_TEMP_ROOT"
	keepTestArtifactsEnv = "OCT_KEEP_TEST_ARTIFACTS"
)

type artifactScope struct {
	dir        string
	keep       bool
	diagnostic io.Writer
}

func newArtifactScope(prefix string, diagnostic io.Writer) (*artifactScope, error) {
	base := strings.TrimSpace(os.Getenv(testArtifactRootEnv))
	if base != "" {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return nil, fmt.Errorf("create test artifact root %s: %w", base, err)
		}
	}
	dir, err := os.MkdirTemp(base, prefix+"-")
	if err != nil {
		return nil, fmt.Errorf("create test artifact scope: %w", err)
	}
	return &artifactScope{
		dir:        dir,
		keep:       os.Getenv(keepTestArtifactsEnv) == "1",
		diagnostic: diagnostic,
	}, nil
}

func (s *artifactScope) path(name string) string {
	return filepath.Join(s.dir, name)
}

func (s *artifactScope) close() error {
	if s == nil || s.dir == "" {
		return nil
	}
	if s.keep {
		if s.diagnostic != nil {
			_, _ = fmt.Fprintf(s.diagnostic, "Retained compiled test artifacts: %s\n", s.dir)
		}
		return nil
	}
	if err := os.RemoveAll(s.dir); err != nil {
		return fmt.Errorf("remove test artifact scope %s: %w", s.dir, err)
	}
	return nil
}

func closeArtifactScope(scope *artifactScope, result *error) {
	if err := scope.close(); err != nil {
		*result = errors.Join(*result, err)
	}
}
