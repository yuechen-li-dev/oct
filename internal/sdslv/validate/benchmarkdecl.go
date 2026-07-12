package validate

// The benchmark declaration projection is intentionally distinct from M29's
// test projection.  It is the compiler-owned handoff for .sdslvbench tooling.

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const DefaultBenchmarkWarmup uint32 = 10
const DefaultBenchmarkIterations uint32 = 100

type ValidatedBenchmark struct {
	StableID, Name string
	Function       ast.FunctionDecl
	SourceSpan     source.Span
	WarmupCount    uint32
	IterationCount uint32
	Launch         ValidatedLaunchMetadata
	Shader         string
	Resources      []ast.ResourceDecl
}

func ValidatedBenchmarks(module ast.Module, identity string) ([]ValidatedBenchmark, []diagnostic.Diagnostic) {
	issues := Diagnostics(module)
	if len(issues) != 0 {
		return nil, issues
	}
	var out []ValidatedBenchmark
	seen := map[string]source.Span{}
	for _, decl := range module.Decls {
		functions := []ast.FunctionDecl{}
		shaderName := ""
		resources := []ast.ResourceDecl(nil)
		switch d := decl.(type) {
		case ast.FunctionDecl:
			functions = []ast.FunctionDecl{d}
		case ast.ShaderDecl:
			functions = d.Methods
			shaderName = d.Name
			resources = d.Resources
		}
		for _, fn := range functions {
			var benchmark bool
			for _, a := range fn.Attributes {
				if a.Name == "Benchmark" {
					benchmark = true
				}
			}
			if !benchmark {
				continue
			}
			if first, exists := seen[fn.Name]; exists {
				issues = append(issues, diagnostic.Diagnostic{Path: module.Source.Path, Code: "SDSL-V3608", Message: "duplicate benchmark name " + fn.Name, Span: fn.Span, Related: []diagnostic.Related{{Message: "first benchmark is here", Span: first}}})
				continue
			}
			seen[fn.Name] = fn.Span
			b := ValidatedBenchmark{Name: fn.Name, Function: fn, SourceSpan: fn.Span, Shader: shaderName, Resources: append([]ast.ResourceDecl(nil), resources...), WarmupCount: DefaultBenchmarkWarmup, IterationCount: DefaultBenchmarkIterations, Launch: ValidatedLaunchMetadata{WorkgroupSize: [3]uint32{1, 1, 1}, DispatchGroups: [3]uint32{1, 1, 1}}}
			if fn.NumThreads != nil {
				b.Launch.WorkgroupSize = launchValues([]ast.Expr{fn.NumThreads.X, fn.NumThreads.Y, fn.NumThreads.Z})
			}
			for _, a := range fn.Attributes {
				switch a.Name {
				case "Warmup":
					b.WarmupCount = benchmarkCount(a.Arguments)
				case "Iterations":
					b.IterationCount = benchmarkCount(a.Arguments)
				case "WorkgroupSize":
					b.Launch.WorkgroupSize = launchValues(a.Arguments)
					b.Launch.WorkgroupSpan = a.Span
					b.Launch.WorkgroupArgSpans = argumentSpans(a.Arguments)
				case "DispatchGroups":
					b.Launch.DispatchGroups = launchValues(a.Arguments)
					b.Launch.DispatchSpan = a.Span
					b.Launch.DispatchArgSpans = argumentSpans(a.Arguments)
				}
			}
			sum := sha256.Sum256([]byte(identity + "\x00" + fn.Name + "\x00" + strconv.FormatUint(uint64(b.Launch.DispatchGroups[0]), 10) + "," + strconv.FormatUint(uint64(b.Launch.DispatchGroups[1]), 10) + "," + strconv.FormatUint(uint64(b.Launch.DispatchGroups[2]), 10)))
			b.StableID = "sdslvbench-" + hex.EncodeToString(sum[:12])
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		issues = append(issues, diagnostic.Diagnostic{Path: module.Source.Path, Code: "SDSL-V3601", Message: ".sdslvbench requires at least one [Benchmark] declaration", Span: module.Span})
	}
	diagnostic.Sort(issues)
	return out, issues
}

func benchmarkCount(args []ast.Expr) uint32 {
	lit := args[0].(ast.IntegerLiteral)
	n, _ := strconv.ParseUint(strings.TrimRight(lit.Value, "uU"), 10, 32)
	return uint32(n)
}
