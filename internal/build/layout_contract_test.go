package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func TestRecordTableIdentitySurvivesMIRLowering(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.oct")
	source := `package Main
record Plain { Values: Int[] }
record table Rows { ID: Int Value: Float }
fn main() -> Int { return 0 }
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]MIRRecordKind{}
	for _, record := range module.Records {
		kinds[record.Name] = record.Kind
	}
	if kinds["Plain"] != MIRRecordOrdinary || kinds["Rows"] != MIRRecordTableKind || kinds["__oct_table_row_Rows"] != MIRRecordTableRow {
		t.Fatalf("MIR record kinds = %#v", kinds)
	}
	if len(module.LayoutContracts) != 1 || module.LayoutContracts[0].Subject.Kind != layoutcontract.MIRRecordTable {
		t.Fatalf("layout contracts = %#v", module.LayoutContracts)
	}
	dump := dumpMIR(module)
	for _, want := range []string{"record Main.Rows kind=record-table", "record Main.__oct_table_row_Rows kind=synthetic-table-row", "layout-contract subject=mir-record-table:Main.Rows identity=nominal"} {
		if !strings.Contains(dump, want) {
			t.Fatalf("MIR dump missing %q:\n%s", want, dump)
		}
	}
}
