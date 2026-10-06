package interpret

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
)

// The interpreter reads the type argument of `Option.Some(value)` where the
// typechecker wrote it. A program that was not typechecked has none there,
// and the interpreter says so; it does not run the construction untyped.
func TestInterpreterRefusesAnOptionTheTypecheckerDidNotResolve(t *testing.T) {
	dir := t.TempDir()
	source := `package Main

fn Main() -> Int {
    let level: Option<Float> = Option.Some(1)
    return match level {
        case Option.Some(v) => 1
        case Option.None => 0
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.oct"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = ExecuteMain(program, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "`Option.Some` reached the interpreter without the type the typechecker gives it") {
		t.Fatalf("ExecuteMain on an unchecked program: %v", err)
	}
}
