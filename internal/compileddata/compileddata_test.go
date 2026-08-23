package compileddata

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
)

func TestEmitGoIsDeterministicStaticData(t *testing.T) {
	dataset := benchmarkDataset(16)
	first, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Source, second.Source) {
		t.Fatal("compiled-data output is not deterministic")
	}
	for _, forbidden := range [][]byte{[]byte("func init("), []byte("reflect."), []byte("append("), []byte("json."), []byte("octagon"), []byte("unsafe")} {
		if bytes.Contains(first.Source, forbidden) {
			t.Fatalf("generated source contains runtime reconstruction marker %q", forbidden)
		}
	}
	for _, required := range [][]byte{[]byte("var Rows = [...]SampleRow"), []byte("RowsLogicalHash"), []byte("RowsSchemaHash"), []byte("RowsRowCount = 16")} {
		if !bytes.Contains(first.Source, required) {
			t.Fatalf("generated source missing %q", required)
		}
	}
}

func TestLayoutContractMaterializesStaticColumnsWithoutChangingSemanticHashes(t *testing.T) {
	dataset := benchmarkDataset(16)
	generic, err := emitGoGeneric(dataset)
	if err != nil {
		t.Fatal(err)
	}
	aware, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if generic.LogicalHash != aware.LogicalHash || generic.SchemaHash != aware.SchemaHash || generic.RowCount != aware.RowCount {
		t.Fatalf("physical materialization changed semantic identity: generic=%+v aware=%+v", generic, aware)
	}
	if bytes.Contains(generic.Source, []byte("RowsStatusColumn")) {
		t.Fatal("generic control unexpectedly contains a column projection")
	}
	for _, want := range []string{"var RowsIDColumn = [...]int", "var RowsStatusColumn = [...]Status", "var RowsPriceColumn = [...]PositivePrice", "var RowsNameColumn = [...]string"} {
		if !strings.Contains(string(aware.Source), want) {
			t.Fatalf("contract-aware source missing %q", want)
		}
	}
	if aware.Materialization != "row-array+static-column-projections" {
		t.Fatalf("materialization = %q", aware.Materialization)
	}
	if aware.Contract.Invariants.ExactExtent == nil || aware.Contract.Invariants.ExactExtent.Value != 16 {
		t.Fatalf("contract exact extent = %#v", aware.Contract.Invariants.ExactExtent)
	}
	t.Logf("generic-source-bytes=%d contract-source-bytes=%d", len(generic.Source), len(aware.Source))
}

func TestNonTableStaticArrayContractIsTruthfulAndConsumed(t *testing.T) {
	intType := Type{Kind: Int}
	dataset := Dataset{Symbol: "PublishedIDs", Type: Type{Kind: Array, Elem: &intType}, Value: Value{Kind: Array, Array: []Value{{Kind: Int, Int: 1}, {Kind: Int, Int: 3}, {Kind: Int, Int: 5}}}}
	result, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if result.Contract.Subject.Kind != layoutcontract.StaticArray || result.Contract.Invariants.ExactExtent.Value != 3 {
		t.Fatalf("array contract = %s", layoutcontract.Format(result.Contract))
	}
	if result.Contract.Invariants.NominalIdentity != nil || len(result.Contract.Metadata.ColumnProjections) != 0 {
		t.Fatalf("array contract invented table identity/metadata: %s", layoutcontract.Format(result.Contract))
	}
	if result.Materialization != "fixed-array" || !bytes.Contains(result.Source, []byte("const PublishedIDsExtent = 3")) || !bytes.Contains(result.Source, []byte("var PublishedIDs = [...]int{1, 3, 5}")) {
		t.Fatalf("array contract was not consumed by fixed-array materialization:\n%s", result.Source)
	}
}

func TestContractAndGenerationAreDeterministic(t *testing.T) {
	dataset := benchmarkDataset(257)
	first, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if layoutcontract.Format(first.Contract) != layoutcontract.Format(second.Contract) || !bytes.Equal(first.Source, second.Source) {
		t.Fatal("contract derivation or materialization is nondeterministic")
	}
}

func TestContractIsRederivedAfterValueExtentChanges(t *testing.T) {
	dataset := benchmarkDataset(8)
	first, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	dataset = benchmarkDataset(3)
	second, err := EmitGo(dataset)
	if err != nil {
		t.Fatal(err)
	}
	if first.Contract.Invariants.ExactExtent.Value != 8 || second.Contract.Invariants.ExactExtent.Value != 3 || second.RowCount != 3 {
		t.Fatalf("stale extent survived rederivation: first=%s second=%s", layoutcontract.Format(first.Contract), layoutcontract.Format(second.Contract))
	}
}

func TestContractAwareGeneratedGoCompilesAndColumnsMatchRows(t *testing.T) {
	result, err := EmitGo(benchmarkDataset(16))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module generated\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.go"), result.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	testSource := `package main
import "testing"
func TestProjectionParity(t *testing.T) {
    if len(Rows) != RowsRowCount || len(RowsIDColumn) != RowsRowCount { t.Fatal("extent mismatch") }
    for index, row := range Rows {
        if row.ID != RowsIDColumn[index] || row.Status != RowsStatusColumn[index] || row.Price != RowsPriceColumn[index] || row.Name != RowsNameColumn[index] { t.Fatalf("row %d differs", index) }
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "data_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated Go did not compile/pass: %v\n%s", err, output)
	}
}

func BenchmarkEmitGo10000Rows(b *testing.B) {
	benchmarkEmitGoRows(b, 10_000)
}

func BenchmarkEmitGo100000Rows(b *testing.B) {
	benchmarkEmitGoRows(b, 100_000)
}

type lookupBenchmarkRow struct {
	ID     int
	Status int
	Price  float64
	Name   string
}

var lookupBenchmarkRows []lookupBenchmarkRow
var lookupBenchmarkStatuses []int
var lookupBenchmarkIDs []int
var lookupBenchmarkSink int

func init() {
	lookupBenchmarkRows = make([]lookupBenchmarkRow, 100_000)
	lookupBenchmarkStatuses = make([]int, len(lookupBenchmarkRows))
	lookupBenchmarkIDs = make([]int, len(lookupBenchmarkRows))
	for index := range lookupBenchmarkRows {
		status := index & 1
		lookupBenchmarkRows[index] = lookupBenchmarkRow{ID: index + 1, Status: status, Price: float64(index) / 100, Name: fmt.Sprintf("row-%05d", index)}
		lookupBenchmarkStatuses[index] = status
		lookupBenchmarkIDs[index] = index + 1
	}
}

func BenchmarkCatalogLookupExistingGenericRows(b *testing.B) {
	benchmarkCatalogLookup(b, false, 99_991)
}

func BenchmarkCatalogLookupExistingLayoutColumn(b *testing.B) {
	benchmarkCatalogLookup(b, true, 99_991)
}

func BenchmarkCatalogLookupMissingGenericRows(b *testing.B) {
	benchmarkCatalogLookup(b, false, 100_001)
}

func BenchmarkCatalogLookupMissingLayoutColumn(b *testing.B) {
	benchmarkCatalogLookup(b, true, 100_001)
}

func benchmarkCatalogLookup(b *testing.B, projected bool, key int) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		found := -1
		if projected {
			for index, id := range lookupBenchmarkIDs {
				if id == key {
					found = lookupBenchmarkRows[index].ID
					break
				}
			}
		} else {
			for _, row := range lookupBenchmarkRows {
				if row.ID == key {
					found = row.ID
					break
				}
			}
		}
		lookupBenchmarkSink = found
	}
}

func BenchmarkCatalogFilterGenericRows(b *testing.B) {
	benchmarkCatalogFilter(b, false)
}

func BenchmarkCatalogFilterLayoutColumn(b *testing.B) {
	benchmarkCatalogFilter(b, true)
}

func benchmarkCatalogFilter(b *testing.B, projected bool) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		count := 0
		if projected {
			for _, status := range lookupBenchmarkStatuses {
				if status == 1 {
					count++
				}
			}
		} else {
			for _, row := range lookupBenchmarkRows {
				if row.Status == 1 {
					count++
				}
			}
		}
		lookupBenchmarkSink = count
	}
}

func benchmarkEmitGoRows(b *testing.B, rows int) {
	dataset := benchmarkDataset(rows)
	var last Result
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := EmitGo(dataset)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(result.Source)))
		last = result
	}
	b.ReportMetric(float64(len(last.Source)), "source-bytes")
	b.ReportMetric(float64(last.Timings.Validation.Microseconds()), "validate-us")
	b.ReportMetric(float64(last.Timings.Emission.Microseconds()), "emit-us")
	b.ReportMetric(float64(last.Timings.Formatting.Microseconds()), "gofmt-us")
}

func benchmarkDataset(rows int) Dataset {
	intType := Type{Kind: Int}
	stringType := Type{Kind: String}
	statusType := Type{Kind: Enum, Name: "Status", Variants: []string{"Draft", "Published"}}
	priceBase := Type{Kind: Float}
	priceType := Type{Kind: Refinement, Name: "PositivePrice", Base: &priceBase}
	tableType := Type{Kind: Record, Name: "Sample", Table: true, Fields: []Field{{Name: "ID", Type: intType}, {Name: "Status", Type: statusType}, {Name: "Price", Type: priceType}, {Name: "Name", Type: stringType}}}
	columns := map[string]Value{}
	for _, name := range []string{"ID", "Status", "Price", "Name"} {
		columns[name] = Value{Kind: Array, Array: make([]Value, rows)}
	}
	for row := 0; row < rows; row++ {
		columns["ID"].Array[row] = Value{Kind: Int, Int: int64(row + 1)}
		variant := "Draft"
		if row%2 == 0 {
			variant = "Published"
		}
		columns["Status"].Array[row] = Value{Kind: Enum, Variant: variant}
		columns["Price"].Array[row] = Value{Kind: Float, Float: float64(row+1) / 100}
		columns["Name"].Array[row] = Value{Kind: String, Text: fmt.Sprintf("row-%05d", row)}
	}
	return Dataset{Symbol: "Rows", Type: tableType, Value: Value{Kind: Record, Fields: columns}}
}
