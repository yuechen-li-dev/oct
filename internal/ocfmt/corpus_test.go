//go:build integration

package ocfmt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every Oct source in the repository is formatted in memory, in both modes.
// The files are not expected to be formatted already. The test holds the
// formatter to what it promises for any input that parses: it does not fail,
// the result is stable, the two modes describe the same program, and the
// program is unchanged.
func TestRepositorySourcesFormatSafely(t *testing.T) {
	root := filepath.Join("..", "..")
	formatted, refused := 0, 0
	for _, dir := range []string{"Libraries", "Language", "Experiments", "Examples", "Packages"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !isOctFile(path) {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(raw)
			readable, err := formatSourceWithPath(path, src, Options{Mode: ModeReadable})
			if errors.Is(err, errSourceRejected) || (err != nil && strings.HasSuffix(path, ".octfail") && !strings.Contains(err.Error(), "internal error")) {
				refused++
				return nil
			}
			if err != nil {
				t.Errorf("%s: %v", path, err)
				return nil
			}
			formatted++
			again, err := formatSourceWithPath(path, readable, Options{Mode: ModeReadable})
			if err != nil || again != readable {
				t.Errorf("%s: readable output is not stable (err=%v)", path, err)
			}
			compact, err := formatSourceWithPath(path, src, Options{Mode: ModeCompact})
			if err != nil {
				t.Errorf("%s: compact: %v", path, err)
				return nil
			}
			fromCompact, err := formatSourceWithPath(path, compact, Options{Mode: ModeReadable})
			if err != nil || fromCompact != readable {
				t.Errorf("%s: readable(compact(src)) differs from readable(src) (err=%v)", path, err)
			}
			if !strings.HasSuffix(path, ".octfail") {
				assertSameProgram(t, src, readable)
				assertSameProgram(t, src, compact)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if formatted < 1000 {
		t.Fatalf("only %d sources were formatted (%d refused); the walk did not reach the repository", formatted, refused)
	}
	t.Logf("%d sources formatted in both modes, %d refused because they do not parse", formatted, refused)
}
