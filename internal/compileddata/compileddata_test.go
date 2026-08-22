package compileddata

import (
	"bytes"
	"fmt"
	"testing"
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
	for _, forbidden := range [][]byte{[]byte("func init("), []byte("reflect."), []byte("append("), []byte("json."), []byte("octagon")} {
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

func BenchmarkEmitGo10000Rows(b *testing.B) {
	dataset := benchmarkDataset(10_000)
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
