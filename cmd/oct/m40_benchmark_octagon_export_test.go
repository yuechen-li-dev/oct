package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/ast"
	"oct/internal/cli"
	"oct/internal/octagon"
)

func TestOctBenchOctagonOutEmitsSingleLoadableFile(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "bench-results.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn Slow() -> Void { let x = 1 + 2 Print(x) }",
		"[Benchmark]",
		"fn Fast() -> Void { return }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root, "--octagon-out", output}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	body, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("expected .octagon file to be written: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n") {
		t.Fatalf("expected .octagon output to end with trailing newline, got %q", body)
	}

	expr, err := octagon.Load(output)
	if err != nil {
		t.Fatalf("expected emitted benchmark .octagon to load: %v", err)
	}
	assertBenchmarkRunShape(t, expr, []string{"Main.Fast", "Main.Slow"})
}

func TestOctBenchOctagonOutDeterministicStructureAcrossRuns(t *testing.T) {
	root := t.TempDir()
	outputOne := filepath.Join(root, "bench-one.octagon")
	outputTwo := filepath.Join(root, "bench-two.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn Alpha() -> Void { return }",
		"[Benchmark]",
		"fn Beta() -> Void { return }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--octagon-out", outputOne}, &stdout, &stderr); err != nil {
		t.Fatalf("first benchmark run failed: err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := cli.Execute([]string{"bench", root, "--octagon-out", outputTwo}, &stdout, &stderr); err != nil {
		t.Fatalf("second benchmark run failed: err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	exprOne, err := octagon.Load(outputOne)
	if err != nil {
		t.Fatalf("load first .octagon file: %v", err)
	}
	exprTwo, err := octagon.Load(outputTwo)
	if err != nil {
		t.Fatalf("load second .octagon file: %v", err)
	}
	assertBenchmarkRunShape(t, exprOne, []string{"Main.Alpha", "Main.Beta"})
	assertBenchmarkRunShape(t, exprTwo, []string{"Main.Alpha", "Main.Beta"})
}

func TestOctBenchOctagonOutRejectsNonOctagonPath(t *testing.T) {
	root := t.TempDir()
	badPath := filepath.Join(root, "bench-results.txt")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Fast() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root, "--octagon-out", badPath}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected bench to fail for non-.octagon output path")
	}
	if !strings.Contains(err.Error(), "bench --octagon-out path must end with .octagon") {
		t.Fatalf("expected deterministic suffix error, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if _, statErr := os.Stat(badPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no output file for invalid suffix, stat err=%v", statErr)
	}
}

func TestOctBenchOctagonOutSkipsWriteOnBenchmarkFailure(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "bench-results.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Boom() -> Void { let x = 1 / 0 Print(x) }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"bench", root, "--octagon-out", output}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected bench to fail when benchmark fails")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("expected no .octagon output when benchmark execution fails, stat err=%v", statErr)
	}
}

func TestOctBenchOctagonOutRoundTripLoadShape(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "bench-results.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bench.octest", "package Main\n[Benchmark]\nfn Only() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--octagon-out", output}, &stdout, &stderr); err != nil {
		t.Fatalf("expected benchmark success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	expr, err := octagon.Load(output)
	if err != nil {
		t.Fatalf("expected .octagon roundtrip load success, got %v", err)
	}
	assertBenchmarkRunShape(t, expr, []string{"Main.Only"})
}

func assertBenchmarkRunShape(t *testing.T, expr ast.Expr, wantNames []string) {
	t.Helper()
	run, ok := expr.(ast.RecordLiteralExpr)
	if !ok {
		t.Fatalf("expected top-level record, got %T", expr)
	}
	if run.TypeName != "BenchmarkRun" {
		t.Fatalf("expected BenchmarkRun type, got %q", run.TypeName)
	}
	if len(run.Fields) != 1 || run.Fields[0].Name != "Cases" {
		t.Fatalf("expected BenchmarkRun with only Cases field, got %#v", run.Fields)
	}

	casesExpr := unwrapOctagonParenExpr(run.Fields[0].Value)
	cases, ok := casesExpr.(ast.ArrayLiteralExpr)
	if !ok {
		t.Fatalf("expected Cases array, got %T", run.Fields[0].Value)
	}
	if len(cases.Elements) != len(wantNames) {
		t.Fatalf("expected %d benchmark case(s), got %d", len(wantNames), len(cases.Elements))
	}

	for i, element := range cases.Elements {
		item, ok := element.(ast.RecordLiteralExpr)
		if !ok {
			t.Fatalf("expected case %d record, got %T", i, element)
		}
		if item.TypeName != "BenchmarkCaseResult" {
			t.Fatalf("expected BenchmarkCaseResult record, got %q", item.TypeName)
		}
		wantFields := []string{"Name", "DurationNs", "BackendRequested", "BackendUsed", "Status", "Environment"}
		if len(item.Fields) != len(wantFields) {
			t.Fatalf("expected %d fields, got %#v", len(wantFields), item.Fields)
		}
		for idx, field := range item.Fields {
			if field.Name != wantFields[idx] {
				t.Fatalf("expected field %d to be %q, got %#v", idx, wantFields[idx], item.Fields)
			}
		}
		nameExpr, ok := unwrapOctagonParenExpr(item.Fields[0].Value).(ast.StringLiteralExpr)
		if !ok {
			t.Fatalf("expected Name string field, got %T", item.Fields[0].Value)
		}
		if nameExpr.Value != wantNames[i] {
			t.Fatalf("expected case name %q at index %d, got %q", wantNames[i], i, nameExpr.Value)
		}
		if _, ok := unwrapOctagonParenExpr(item.Fields[1].Value).(ast.IntegerLiteral); !ok {
			t.Fatalf("expected DurationNs int field, got %T", item.Fields[1].Value)
		}
		for _, fieldIndex := range []int{2, 3, 4, 5} {
			if _, ok := unwrapOctagonParenExpr(item.Fields[fieldIndex].Value).(ast.StringLiteralExpr); !ok {
				t.Fatalf("expected field %q string value, got %T", item.Fields[fieldIndex].Name, item.Fields[fieldIndex].Value)
			}
		}
	}
}

func unwrapOctagonParenExpr(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.Inner
	}
}
