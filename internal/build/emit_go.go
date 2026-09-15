package build

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/project"
	"sort"
	"strings"
)

type goEmitOptions struct {
	packageName string
	includeMain bool
	hostFacade  bool
}

type goSupportFeatures struct {
	NeedsClone         bool
	NeedsRowAssign     bool
	NeedsRange         bool
	NeedsArrayCoercion bool
}

func analyzeGoSupportFeatures(module MIRModule, usedBuiltins map[string]bool) goSupportFeatures {
	features := goSupportFeatures{
		NeedsClone: usedBuiltins["ArrayWhere"] || usedBuiltins["Array.Where"],
		NeedsRange: usedBuiltins["ArrayCrossSection"] || usedBuiltins["Array.CrossSection"],
	}
	for _, record := range module.Records {
		for _, field := range record.Fields {
			features.NeedsRange = features.NeedsRange || compiledTypeNeedsRange(field.Type)
		}
	}
	for _, flow := range module.Flows {
		features.NeedsRange = features.NeedsRange || compiledTypeNeedsRange(flow.Return)
		for _, field := range append(append([]MIRField{}, flow.Parameters...), flow.Board...) {
			features.NeedsRange = features.NeedsRange || compiledTypeNeedsRange(field.Type)
		}
		for _, field := range flow.Board {
			features.NeedsClone = features.NeedsClone || compiledValueNeedsClone(field.Type)
		}
		walkFlowSharedStatements(flow, func(statement MIRStmt) {
			if mirStatementContains(statement, "__octClone(") {
				features.NeedsClone = true
			}
			if mirStatementContains(statement, "__octRange{") {
				features.NeedsRange = true
			}
			if mirStatementContains(statement, "__octIntArrayToFloat(") {
				features.NeedsArrayCoercion = true
			}
			if _, ok := statement.(MIRRowAssign); ok {
				features.NeedsRowAssign, features.NeedsClone = true, true
			}
		})
	}
	for _, function := range module.Functions {
		features.NeedsRange = features.NeedsRange || compiledTypeNeedsRange(function.Return)
		for _, field := range append(append([]MIRField{}, function.Params...), function.Locals...) {
			features.NeedsRange = features.NeedsRange || compiledTypeNeedsRange(field.Type)
		}
		for _, block := range function.Blocks {
			for _, statement := range block.Statements {
				if mirStatementContains(statement, "__octClone(") {
					features.NeedsClone = true
				}
				if mirStatementContains(statement, "__octRange{") {
					features.NeedsRange = true
				}
				if mirStatementContains(statement, "__octIntArrayToFloat(") {
					features.NeedsArrayCoercion = true
				}
				switch statement := statement.(type) {
				case MIRRowAssign:
					features.NeedsRowAssign = true
					features.NeedsClone = true
				case MIRCall:
					if statement.Builtin && canonicalCompiledBuiltinName(statement.Callee) == "Append" && len(statement.ArgTypes) > 1 && compiledValueNeedsClone(statement.ArgTypes[1]) {
						features.NeedsClone = true
					}
				}
			}
			if mirTerminatorContains(block.Terminator, "__octRange{") {
				features.NeedsRange = true
			}
			if mirTerminatorContains(block.Terminator, "__octIntArrayToFloat(") {
				features.NeedsArrayCoercion = true
			}
		}
	}
	return features
}

func compiledTypeNeedsRange(typeName string) bool {
	for strings.HasSuffix(typeName, "[]") {
		typeName = strings.TrimSuffix(typeName, "[]")
	}
	return typeName == "Range"
}

func mirStatementContains(statement MIRStmt, needle string) bool {
	values := []MIRValue{}
	switch statement := statement.(type) {
	case MIRAssign:
		values = append(values, statement.Value)
	case MIRRowAssign:
		values = append(values, statement.Index, statement.Value)
	case MIRIndexAssign:
		values = append(values, statement.Indices...)
		values = append(values, statement.Value)
	case MIRCall:
		values = append(values, statement.Args...)
	case MIRGenericOctxiliaryCall:
		values = append(values, statement.Args...)
	case MIRDestructureCall:
		values = append(values, statement.Args...)
	case MIRConstructRecord:
		values = append(values, statement.FieldVals...)
	case MIRConstructArray:
		values = append(values, statement.Values...)
	case MIRBatchMap:
		values = append(values, statement.Input)
		values = append(values, statement.Captures...)
	}
	for _, value := range values {
		if mirValueContains(value, needle) {
			return true
		}
	}
	return false
}

func mirTerminatorContains(terminator MIRTerminator, needle string) bool {
	switch terminator := terminator.(type) {
	case MIRReturn:
		return mirValueContains(terminator.Value, needle)
	case MIRBranch:
		return mirValueContains(terminator.Cond, needle)
	case MIRFail:
		return mirValueContains(terminator.Value, needle)
	default:
		return false
	}
}

func compiledValueNeedsClone(typeName string) bool {
	return typeName == "Bytes" || strings.HasSuffix(typeName, "[]") || parseMatrixElemTypeOK(typeName) || parseVectorElemTypeOK(typeName)
}

// isSelfAppendAssign reports whether stmt is the self-referential
// accumulation idiom `x = Append(x, v)`, where cloning the append
// result is provably unnecessary: append() either extends the
// existing backing array in place or allocates a fresh one, and in
// neither case does skipping the defensive clone risk aliasing
// another live binding, since x's own binding is what's being
// replaced and no other statement runs in between the read and the
// write. Any value-semantics boundary between x and some other
// variable was already enforced when that other variable was bound
// (LetStmt/VarStmt/AssignStmt all clone on assignment), so by the
// time control reaches `x = Append(x, v)`, x's backing array is not
// shared with anything else that needs protecting.
//
// Without this exemption, every iteration of an accumulation loop
// (`var out = []; for ... { out = Append(out, v) }`) pays a full
// reflection-based deep clone of the whole array on top of the
// append, turning Go's amortized-O(1) append into an O(current
// length) copy per call -- O(n^2) total to build an n-element array.
// This exemption only widens the no-clone case for this one provably
// safe self-append shape; it does not relax cloning anywhere else
// (in particular, the appended value `v` itself is still cloned by
// the Append builtin's own codegen when `v` is itself an array/matrix
// element, independent of this check).
func isSelfAppendAssign(targetName string, value ast.Expr) bool {
	call, ok := value.(ast.CallExpr)
	if !ok {
		return false
	}
	callee, ok := call.Callee.(ast.IdentifierExpr)
	if !ok || callee.Name != "Append" {
		return false
	}
	if len(call.Arguments) == 0 {
		return false
	}
	firstArg, ok := call.Arguments[0].(ast.IdentifierExpr)
	if !ok {
		return false
	}
	return firstArg.Name == targetName
}

func emitGo(m MIRModule) (string, error) {
	return emitGoWithOptions(m, goEmitOptions{packageName: "main", includeMain: true})
}

func emitGoWithOptions(m MIRModule, options goEmitOptions) (string, error) {
	var b strings.Builder
	for _, template := range m.Templates {
		fmt.Fprintf(&b, "// Oct template provenance: %s.%s <- %s.%s<%s> [%s]\n", template.Package, template.ConcreteName, template.OriginPackage, template.OriginName, strings.Join(template.TypeArguments, ", "), template.Kind)
	}
	for _, selector := range m.Selectors {
		fmt.Fprintf(&b, "// Oct selector provenance: %s.%s -> %s.%s (ordinal %d)\n", selector.Package, selector.Name, selector.Field.Subject.Identity, selector.Field.Name, selector.Field.Ordinal)
	}
	if len(m.Templates) > 0 || len(m.Selectors) > 0 {
		b.WriteString("\n")
	}
	usedBuiltins := map[string]bool{}
	emittedRecordTypes := map[string]struct{}{}
	loadTypes := map[string]struct{}{}
	resultTypes := map[string]struct{}{}
	flowResultTypes := map[string]struct{}{}
	needsGenericUtilityHelpers := false
	needsScalarUtilityHelpers := false
	usesGenericOctxiliary := false
	for _, flow := range m.Flows {
		flowResultTypes[flow.Return] = struct{}{}
		collectFlowBuiltins(flow, usedBuiltins)
		walkFlowSharedStatements(flow, func(statement MIRStmt) {
			switch node := statement.(type) {
			case MIRCall:
				if node.Builtin && node.Callee == "LoadOctagon" {
					loadTypes[node.RetType], resultTypes[node.RetType] = struct{}{}, struct{}{}
				}
			case MIRBatchMap:
				resultTypes[node.ResultType+"[]"] = struct{}{}
			case MIRGenericOctxiliaryCall:
				usesGenericOctxiliary = true
				if node.Fallible {
					resultTypes[node.RetType] = struct{}{}
				}
			}
		})
	}
	for _, fn := range m.Functions {
		if fn.UsesUtilityWhen {
			needsGenericUtilityHelpers = true
		}
		if fn.IsFallible {
			resultTypes[fn.Return] = struct{}{}
		}
		for _, bb := range fn.Blocks {
			for _, st := range bb.Statements {
				if call, ok := st.(MIRCall); ok && call.Builtin {
					usedBuiltins[call.Callee] = true
					if call.Callee == "LoadOctagon" {
						loadTypes[call.RetType] = struct{}{}
						resultTypes[call.RetType] = struct{}{}
					}
					continue
				}
				if dcall, ok := st.(MIRDestructureCall); ok && dcall.Builtin {
					usedBuiltins[dcall.Callee] = true
					continue
				}
				if batch, ok := st.(MIRBatchMap); ok {
					usedBuiltins["BatchMap"] = true
					resultTypes[batch.ResultType+"[]"] = struct{}{}
				}
				if generic, ok := st.(MIRGenericOctxiliaryCall); ok {
					usesGenericOctxiliary = true
					if generic.Fallible {
						resultTypes[generic.RetType] = struct{}{}
					}
					if transport := findTransportRecord(generic.TransportTypes, generic.RetType); transport.ok && transport.typ.Kind == "handle" {
						resultTypes[generic.RetType] = struct{}{}
					}
				}
			}
		}
		for _, local := range fn.Locals {
			if isFallibleType(local.Type) {
				resultTypes[fallibleValueType(local.Type)] = struct{}{}
			}
			if flowRet, ok := parseFlowInstanceType(local.Type); ok {
				flowResultTypes[flowRet] = struct{}{}
			}
		}
	}
	for _, flow := range m.Flows {
		features := analyzeFlowFeatures(flow, usedBuiltins)
		needsGenericUtilityHelpers = needsGenericUtilityHelpers || features.NeedsGenericUtility
		needsScalarUtilityHelpers = needsScalarUtilityHelpers || features.NeedsScalarUtility
	}
	supportFeatures := analyzeGoSupportFeatures(m, usedBuiltins)
	importSet := map[string]struct{}{"fmt": {}, "os": {}, "reflect": {}}
	if options.hostFacade {
		for _, flow := range m.Flows {
			if flow.YieldType != "" {
				importSet["encoding/json"] = struct{}{}
				break
			}
		}
	}
	if needsGenericUtilityHelpers {
		importSet["reflect"] = struct{}{}
	}
	if usedBuiltins["BatchMap"] {
		for _, pkg := range []string{"runtime", "sync"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["WriteOctagon"] {
		for _, pkg := range []string{"os", "path/filepath", "reflect", "strconv", "strings"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["Assert.True"] || usedBuiltins["Assert.False"] || usedBuiltins["Assert.Equal"] || usedBuiltins["Assert.Near"] || usedBuiltins["Assert.Error"] || usedBuiltins["Assert.LGTM"] {
		importSet["os"] = struct{}{}
	}
	if usedBuiltins["Assert.Equal"] {
		importSet["reflect"] = struct{}{}
	}
	if usedBuiltins["Assert.Near"] {
		importSet["math"] = struct{}{}
	}
	if usedBuiltins["Idx"] {
		importSet["strings"] = struct{}{}
	}
	if usedBuiltins["PrometheusMatMulMM"] {
		for _, pkg := range []string{"github.com/yuechen-li-dev/oct/internal/prometheus", "os", "os/exec", "strings", "sync"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		for _, pkg := range []string{"errors", "io", "os", "os/exec", "path/filepath", "runtime", "strings", "strconv", "sync", "time", "github.com/yuechen-li-dev/oct/internal/octxiliary"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["LoadOctagon"] {
		for _, pkg := range []string{"errors", "os", "reflect", "sort", "strconv", "strings", "unicode", "unicode/utf8"} {
			importSet[pkg] = struct{}{}
		}
	}
	for builtinName := range usedBuiltins {
		for _, pkg := range builtinImportDeps(builtinName) {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["Abs"] || usedBuiltins["Real"] || usedBuiltins["Imag"] || usedBuiltins["Pi"] || usedBuiltins["E"] || usedBuiltins["ComplexPolar"] || usedBuiltins["Arg"] || usedBuiltins["Sqrt"] || usedBuiltins["Sin"] || usedBuiltins["Cos"] || usedBuiltins["Tan"] || usedBuiltins["Asin"] || usedBuiltins["Acos"] || usedBuiltins["Atan"] || usedBuiltins["Atan2"] || usedBuiltins["Exp"] || usedBuiltins["Ln"] || usedBuiltins["Pow"] || usedBuiltins["Log10"] || usedBuiltins["Sinh"] || usedBuiltins["Cosh"] || usedBuiltins["Tanh"] || usedBuiltins["FloorToInt"] || usedBuiltins["CeilToInt"] || usedBuiltins["RoundToInt"] || usedBuiltins["BaseValue"] || usedBuiltins["BaseUnit"] || usedBuiltins["FFT"] {
		importSet["math"] = struct{}{}
	}
	if usedBuiltins["ComplexPolar"] || usedBuiltins["Arg"] || usedBuiltins["Conj"] || usedBuiltins["Exp"] || usedBuiltins["Ln"] {
		importSet["math/cmplx"] = struct{}{}
	}
	if usedBuiltins["Random.RandInt"] || usedBuiltins["Random.RandFloat01"] || usedBuiltins["Random.RandFloatRange"] || usedBuiltins["Random.RandBernoulli"] || usedBuiltins["Random.RandNormal"] {
		importSet["math"] = struct{}{}
		importSet["crypto/rand"] = struct{}{}
		importSet["encoding/binary"] = struct{}{}
		importSet["math/big"] = struct{}{}
	}
	if usedBuiltins["Random.CryptoRandInt"] || usedBuiltins["Random.CryptoRandFloat01"] || usedBuiltins["Random.CryptoRandBytes"] {
		importSet["math"] = struct{}{}
		importSet["crypto/rand"] = struct{}{}
		importSet["encoding/binary"] = struct{}{}
		importSet["math/big"] = struct{}{}
	}
	imports := make([]string, 0, len(importSet))
	for pkg := range importSet {
		imports = append(imports, pkg)
	}
	sort.Strings(imports)
	fmt.Fprintf(&b, "package %s\n\n", options.packageName)
	b.WriteString("import (\n")
	for _, name := range imports {
		fmt.Fprintf(&b, "\t%q\n", name)
	}
	b.WriteString(")\n\n")
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		resultTypes["Bytes"] = struct{}{}
		resultTypes["String"] = struct{}{}
		resultTypes["String[]"] = struct{}{}
		resultTypes["String[][]"] = struct{}{}
		resultTypes["Float[][]"] = struct{}{}
		resultTypes["Csv.Table"] = struct{}{}
	}
	resultTypes["Int"] = struct{}{}
	resultNames := make([]string, 0, len(resultTypes))
	for t := range resultTypes {
		resultNames = append(resultNames, t)
	}
	sort.Strings(resultNames)
	needsVoidType := false
	for _, t := range resultNames {
		if t == "Void" {
			needsVoidType = true
			break
		}
	}
	if !needsVoidType {
		for _, fn := range m.Functions {
			for _, l := range fn.Locals {
				if l.Type == "Void" {
					needsVoidType = true
					break
				}
			}
			if needsVoidType {
				break
			}
		}
	}
	if !needsVoidType {
		for _, flow := range m.Flows {
			if flow.Return == "Void" {
				needsVoidType = true
				break
			}
		}
	}
	if needsVoidType {
		b.WriteString("type __octVoid struct{}\n\n")
	}
	for _, refinement := range m.Refinements {
		fmt.Fprintf(&b, "type %s_%s = %s\n", refinement.Package, refinement.Name, goType(refinement.Base))
		if options.hostFacade {
			publicName := refinement.Name
			if refinement.Package != m.EntryPackage {
				publicName = refinement.Package + refinement.Name
			}
			fmt.Fprintf(&b, "func Admit%s(value %s) (%s_%s, error) { admitted := fn_%s___oct_refine_%s(value); if admitted.IsErr { var zero %s_%s; return zero, fmt.Errorf(\"%%s\", admitted.Err) }; return admitted.Value, nil }\n", publicName, goType(refinement.Base), refinement.Package, refinement.Name, refinement.Package, refinement.Name, refinement.Package, refinement.Name)
		}
	}
	if len(m.Refinements) > 0 {
		b.WriteString("\n")
	}
	if supportFeatures.NeedsRange {
		b.WriteString("type __octRange struct {\n\tStart int\n\tHasStart bool\n\tEnd int\n\tHasEnd bool\n\tStep int\n\tHasStep bool\n}\n\n")
	}
	if supportFeatures.NeedsClone {
		b.WriteString("func __octClone[T any](value T) T {\n\tcloned := __octCloneValue(reflect.ValueOf(value))\n\tif !cloned.IsValid() { return value }\n\treturn cloned.Interface().(T)\n}\n\nfunc __octCloneValue(value reflect.Value) reflect.Value {\n\tif !value.IsValid() { return value }\n\tswitch value.Kind() {\n\tcase reflect.Slice:\n\t\tif value.IsNil() { return reflect.Zero(value.Type()) }\n\t\tout := reflect.MakeSlice(value.Type(), value.Len(), value.Len())\n\t\tfor i := 0; i < value.Len(); i++ { out.Index(i).Set(__octCloneValue(value.Index(i))) }\n\t\treturn out\n\tcase reflect.Array:\n\t\tout := reflect.New(value.Type()).Elem()\n\t\tfor i := 0; i < value.Len(); i++ { out.Index(i).Set(__octCloneValue(value.Index(i))) }\n\t\treturn out\n\tcase reflect.Struct:\n\t\tout := reflect.New(value.Type()).Elem()\n\t\tfor i := 0; i < value.NumField(); i++ {\n\t\t\tif out.Field(i).CanSet() { out.Field(i).Set(__octCloneValue(value.Field(i))) }\n\t\t}\n\t\treturn out\n\tdefault:\n\t\treturn value\n\t}\n}\n\n")
	}
	if supportFeatures.NeedsRowAssign {
		b.WriteString("func __octAssignRow[T any](matrix [][]T, row int, rhs []T) {\n\tif row < 0 || row >= len(matrix) { panic(fmt.Sprintf(\"runtime error: row index %d out of bounds for array with %d rows\", row, len(matrix))) }\n\tif len(rhs) != len(matrix[row]) { panic(fmt.Sprintf(\"runtime error: row length mismatch: expected %d, got %d\", len(matrix[row]), len(rhs))) }\n\tmatrix[row] = __octClone(rhs)\n}\n\n")
	}
	if usedBuiltins["ArrayCrossSection"] || usedBuiltins["Array.CrossSection"] {
		b.WriteString("func __octArrayCrossSection[T any](values []T, r __octRange) []T {\n\tstart := 0\n\tif r.HasStart { start = r.Start }\n\tend := len(values)\n\tif r.HasEnd { end = r.End }\n\tstep := 1\n\tif r.HasStep { step = r.Step }\n\tif step <= 0 { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range step must be positive, got %d\", step)) }\n\tif start < 0 { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range start must be >= 0, got %d\", start)) }\n\tif end < 0 { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range end must be >= 0, got %d\", end)) }\n\tif start > len(values) { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range start %d exceeds array length %d\", start, len(values))) }\n\tif end > len(values) { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range end %d exceeds array length %d\", end, len(values))) }\n\tif start > end { panic(fmt.Sprintf(\"runtime error: Array.CrossSection range start %d must be <= end %d\", start, end)) }\n\tcount := 0\n\tif start < end { count = ((end - start - 1) / step) + 1 }\n\tout := make([]T, 0, count)\n\tfor i := start; i < end; i += step { out = append(out, values[i]) }\n\treturn out\n}\n\n")
	}
	if usedBuiltins["ArrayWhere"] || usedBuiltins["Array.Where"] {
		b.WriteString("func __octArrayWhere[T any](values []T, mask []bool) []T {\n\tif len(values) != len(mask) { panic(\"runtime error: Array.Where mask length must match values length\") }\n\tout := make([]T, 0, len(values))\n\tfor i, keep := range mask { if keep { out = append(out, __octClone(values[i])) } }\n\treturn out\n}\n\n")
	}
	for _, t := range resultNames {
		valueType := goType(t)
		if t == "Void" {
			valueType = "__octVoid"
		}
		fmt.Fprintf(&b, "type %s struct {\n\tValue %s\n\tErr string\n\tIsErr bool\n}\n\n", goResultTypeName(t), valueType)
	}
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		if _, ok := resultTypes["String[]"]; !ok {
			b.WriteString("type octResult_StringSlice struct {\n\tValue []string\n\tErr string\n\tIsErr bool\n}\n\n")
		}
	}
	for _, r := range m.Records {
		emittedRecordTypes[r.Package+"."+r.Name] = struct{}{}
		fmt.Fprintf(&b, "type %s_%s struct {\n", r.Package, r.Name)
		for _, f := range r.Fields {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, goType(f.Type))
		}
		b.WriteString("}\n\n")
	}
	needsRandomHelpers := usedBuiltins["Random.RandInt"] || usedBuiltins["Random.RandFloat01"] || usedBuiltins["Random.RandFloatRange"] || usedBuiltins["Random.RandBernoulli"] || usedBuiltins["Random.RandNormal"] || usedBuiltins["Random.CryptoRandInt"] || usedBuiltins["Random.CryptoRandFloat01"] || usedBuiltins["Random.CryptoRandBytes"]
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		if _, ok := emittedRecordTypes["Csv.Table"]; !ok {
			b.WriteString("type Csv_Table struct{}\n\n")
			emittedRecordTypes["Csv.Table"] = struct{}{}
		}
	}
	if needsRandomHelpers {
		if _, ok := emittedRecordTypes["Random.Rng"]; !ok {
			b.WriteString("type Random_Rng struct {\n\t_State0 int\n\t_State1 int\n\t_State2 int\n\t_State3 int\n}\n\n")
		}
		if _, ok := emittedRecordTypes["Random.RandIntResult"]; !ok {
			b.WriteString("type Random_RandIntResult struct {\n\tNext Random_Rng\n\tValue int\n}\n\n")
		}
		if _, ok := emittedRecordTypes["Random.RandFloatResult"]; !ok {
			b.WriteString("type Random_RandFloatResult struct {\n\tNext Random_Rng\n\tValue float64\n}\n\n")
		}
		if _, ok := emittedRecordTypes["Random.RandBoolResult"]; !ok {
			b.WriteString("type Random_RandBoolResult struct {\n\tNext Random_Rng\n\tValue bool\n}\n\n")
		}
	}
	for _, e := range m.Enums {
		fmt.Fprintf(&b, "type %s_%s struct {\n\tTag int\n\tPayload any\n}\nconst (\n", e.Package, e.Name)
		for i, v := range e.Variants {
			fmt.Fprintf(&b, "\t%s_%s_tag = %d\n", e.Name, v.Name, i)
		}
		b.WriteString(")\n\n")
		if options.hostFacade {
			publicEnum := e.Name
			if e.Package != m.EntryPackage {
				publicEnum = e.Package + e.Name
			}
			for _, variant := range e.Variants {
				fmt.Fprintf(&b, "func New%s%s(", publicEnum, variant.Name)
				if variant.PayloadType != "" {
					fmt.Fprintf(&b, "payload %s", goType(variant.PayloadType))
				}
				fmt.Fprintf(&b, ") %s_%s { return %s_%s{Tag: %s_%s_tag", e.Package, e.Name, e.Package, e.Name, e.Name, variant.Name)
				if variant.PayloadType != "" {
					b.WriteString(", Payload: payload")
				}
				b.WriteString("} }\n")
			}
			b.WriteString("\n")
		}
	}
	flowTypeNames := make([]string, 0, len(flowResultTypes))
	for t := range flowResultTypes {
		flowTypeNames = append(flowTypeNames, t)
	}
	sort.Strings(flowTypeNames)
	for _, t := range flowTypeNames {
		fmt.Fprintf(&b, "type __octFlowInstance_%s interface {\n", goSafeName(t))
		b.WriteString("\t__octStep(any)\n\t__octActive() string\n\t__octComplete() bool\n\t__octDidYield() bool\n\t__octYielded() (any, bool)\n")
		fmt.Fprintf(&b, "\t__octResult() (%s, bool)\n", goFlowResultType(t))
		b.WriteString("\t__octStateHistory() []string\n\t__octResumeTarget() string\n\t__octBoardSnapshot() (any, bool)\n}\n\n")
		fmt.Fprintf(&b, "type __octResultFlow_%s struct {\n\tValue %s\n\tErr string\n\tIsErr bool\n}\n\n", goSafeName(t), goFlowResultType(t))
	}
	for _, flow := range m.Flows {
		if err := emitGoFlow(&b, flow, analyzeFlowFeatures(flow, usedBuiltins)); err != nil {
			return "", err
		}
	}
	if options.hostFacade {
		for _, flow := range m.Flows {
			features := analyzeFlowFeatures(flow, usedBuiltins)
			publicName := flow.Name
			if flow.Package != m.EntryPackage {
				publicName = flow.Package + flow.Name
			}
			if err := emitGoFlowHostFacade(&b, flow, features, publicName, m.Refinements); err != nil {
				return "", err
			}
		}
	}
	if supportFeatures.NeedsArrayCoercion {
		b.WriteString(__octArrayCoercionHelpers)
	}
	if usedBuiltins["Idx"] {
		b.WriteString(__octIndexHelpers)
	}
	if needsGenericUtilityHelpers || needsScalarUtilityHelpers {
		b.WriteString(__octUtilityCandidate)
	}
	if needsGenericUtilityHelpers {
		b.WriteString(__octGenericUtilityHelpers)
	}
	if needsScalarUtilityHelpers {
		b.WriteString(__octScalarUtilityHelpers)
	}
	if usesLinearAlgebraHelpers(usedBuiltins) {
		b.WriteString(__octLinearAlgebraHelpers)
	}
	if usedBuiltins["Abs"] || usedBuiltins["Real"] || usedBuiltins["Imag"] {
		b.WriteString(__octComplexHelpers)
	}
	if usedBuiltins["FFT"] {
		b.WriteString(__octFFTHelpers)
	}
	if needsRandomHelpers {
		b.WriteString(__octRandomHelpers)
	}
	if usedBuiltins["PrometheusMatMulMM"] {
		b.WriteString(__octPrometheusHelpers)
	}
	if usedBuiltins["StringSplitLines"] || usedBuiltins["StringEscapeJSON"] {
		b.WriteString(__octStringHelpers)
	}
	needsMarkdownHelpers := false
	for builtinName := range usedBuiltins {
		if isMarkdownCompiledBuiltin(builtinName) {
			needsMarkdownHelpers = true
			break
		}
	}
	if needsMarkdownHelpers {
		b.WriteString(__octMarkdownHelpers)
	}
	if usedBuiltins["WriteOctagon"] || usedBuiltins["LoadOctagon"] {
		b.WriteString("type __octParsedKind int\n\n")
		b.WriteString("const (\n")
		b.WriteString("\t__octParsedInt __octParsedKind = iota\n\t__octParsedFloat\n\t__octParsedBool\n\t__octParsedString\n\t__octParsedArray\n\t__octParsedRecord\n\t__octParsedEnum\n)\n\n")
		b.WriteString("type __octParsedValue struct {\n\tKind __octParsedKind\n\tInt int\n\tFloat float64\n\tBool bool\n\tText string\n\tArray []__octParsedValue\n\tRecordType string\n\tRecordFields map[string]__octParsedValue\n\tEnumType string\n\tEnumVariant string\n}\n\n")
		b.WriteString("type __octRecordMeta struct {\n\tFullName string\n\tShortName string\n\tFields []string\n\tFieldTypes map[string]string\n}\n\n")
		b.WriteString("type __octEnumMeta struct {\n\tFullName string\n\tShortName string\n\tVariants []string\n}\n\n")
		b.WriteString("var __octRecordMetaByGoType = map[string]__octRecordMeta{\n")
		for _, r := range m.Records {
			fmt.Fprintf(&b, "\t%q: {FullName: %q, ShortName: %q, Fields: []string{", "main."+r.Package+"_"+r.Name, r.Package+"."+r.Name, r.Name)
			for i, f := range r.Fields {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%q", f.Name)
			}
			b.WriteString("}, FieldTypes: map[string]string{")
			for i, f := range r.Fields {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%q: %q", f.Name, f.Type)
			}
			b.WriteString("}},\n")
		}
		b.WriteString("}\n\n")
		b.WriteString("var __octEnumMetaByGoType = map[string]__octEnumMeta{\n")
		for _, e := range m.Enums {
			fmt.Fprintf(&b, "\t%q: {FullName: %q, ShortName: %q, Variants: []string{", "main."+e.Package+"_"+e.Name, e.Package+"."+e.Name, e.Name)
			for i, v := range e.Variants {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%q", v.Name)
			}
			b.WriteString("}},\n")
		}
		b.WriteString("}\n\n")
		b.WriteString(__octSharedOctagonHelpers)
		if usedBuiltins["WriteOctagon"] {
			b.WriteString(__octWriteHelpers)
		}
		if usesOctxiliaryBuiltins(usedBuiltins) {
			for _, pkg := range []string{"errors", "io", "os", "os/exec", "path/filepath", "runtime", "strings", "sync", "time", "github.com/yuechen-li-dev/oct/internal/octxiliary"} {
				importSet[pkg] = struct{}{}
			}
		}
		if usedBuiltins["LoadOctagon"] {
			b.WriteString("func __octValidateRefinement(expectedType string, value reflect.Value) error {\n\tswitch expectedType {\n")
			for _, refinement := range m.Refinements {
				fmt.Fprintf(&b, "\tcase %q, %q:\n", refinement.Package+"."+refinement.Name, refinement.Name)
				fmt.Fprintf(&b, "\t\tchecked := fn_%s___oct_refine_%s(value.Interface().(%s))\n", refinement.Package, refinement.Name, goType(refinement.Base))
				b.WriteString("\t\tif checked.IsErr { return errors.New(checked.Err) }\n")
			}
			b.WriteString("\t}\n\treturn nil\n}\n\n")
			b.WriteString(__octLoadHelpers)
			loadTypeNames := make([]string, 0, len(loadTypes))
			for t := range loadTypes {
				loadTypeNames = append(loadTypeNames, t)
			}
			sort.Strings(loadTypeNames)
			for _, t := range loadTypeNames {
				fmt.Fprintf(&b, "func __octLoadOctagon_%s(path string) %s {\n", goSafeName(t), goResultTypeName(t))
				fmt.Fprintf(&b, "\tv, err := __octLoadOctagonTyped(path, reflect.TypeOf((*%s)(nil)).Elem(), %q)\n", goType(t), t)
				b.WriteString("\tif err != nil {\n")
				fmt.Fprintf(&b, "\t\treturn %s{Err: err.Error(), IsErr: true}\n", goResultTypeName(t))
				b.WriteString("\t}\n")
				fmt.Fprintf(&b, "\treturn %s{Value: v.(%s)}\n", goResultTypeName(t), goType(t))
				b.WriteString("}\n\n")
			}
		}
	}
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		b.WriteString(__octOctxiliaryHelpers)
	}
	if usedBuiltins["BatchMap"] {
		b.WriteString(octBatchHelpers())
	}
	for _, fn := range m.Functions {
		fmt.Fprintf(&b, "func fn_%s_%s(", fn.Package, fn.Name)
		for i, p := range fn.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s %s", p.Name, goType(p.Type))
		}
		returnType := goType(fn.Return)
		if fn.IsFallible {
			returnType = goResultTypeName(fn.Return)
		}
		if returnType == "" {
			b.WriteString(") {\n")
		} else {
			fmt.Fprintf(&b, ") %s {\n", returnType)
		}
		for _, l := range fn.Locals {
			localType := goType(l.Type)
			if l.Type == "Void" {
				localType = "__octVoid"
			}
			fmt.Fprintf(&b, "\tvar %s %s\n", l.Name, localType)
			if l.Name != "_" {
				fmt.Fprintf(&b, "\t_ = %s\n", l.Name)
			}
		}
		labelToIdx := map[string]int{}
		for i, bb := range fn.Blocks {
			labelToIdx[bb.Label] = i
		}
		pcName := internalName(internalProgramCounter, -1)
		fmt.Fprintf(&b, "\t%s := 0\n\tfor {\n\t\tswitch %s {\n", pcName, pcName)
		for i, bb := range fn.Blocks {
			fmt.Fprintf(&b, "\t\tcase %d:\n", i)
			for _, s := range bb.Statements {
				src, err := goStmt(s)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "\t\t\t%s\n", src)
			}
			terminator := bb.Terminator
			if terminator == nil {
				if i+1 >= len(fn.Blocks) {
					return "", fmt.Errorf("unsupported MIR terminator <nil> in final block %s.%s:%s", fn.Package, fn.Name, bb.Label)
				}
				terminator = MIRJump{Target: fn.Blocks[i+1].Label}
			}
			term, err := goTerminator(terminator, labelToIdx, pcName)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "\t\t\t%s\n", term)
		}
		b.WriteString("\t\t}\n\t}\n}\n\n")
	}
	b.WriteString("var __octAssertionCount int\n\n")
	if !options.includeMain {
		return pruneGeneratedImports(b.String()), nil
	}
	b.WriteString("func main() {\n")
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		b.WriteString("\tdefer __octOctxiliaryClose()\n")
	}
	entryReturn := m.EntryReturn
	entryFallible := m.EntryFallible
	if entryReturn == "Void" && !entryFallible {
		b.WriteString("\tfn_")
		b.WriteString(m.EntryPackage)
		b.WriteString("_")
		b.WriteString(m.EntryFunc)
		b.WriteString("()\n")
	} else {
		b.WriteString("\tresult := fn_")
		b.WriteString(m.EntryPackage)
		b.WriteString("_")
		b.WriteString(m.EntryFunc)
		b.WriteString("()\n")
	}
	if entryFallible {
		b.WriteString("\tif result.IsErr { panic(\"oct error: \" + result.Err) }\n")
		if entryReturn != "Void" {
			b.WriteString("\tfmt.Println(result.Value)\n")
		}
	} else if entryReturn != "Void" {
		b.WriteString("\tfmt.Println(result)\n")
	}
	b.WriteString("	if os.Getenv(\"OCT_ENFORCE_ASSERTIONS\") == \"1\" && __octAssertionCount == 0 {\n")
	b.WriteString("		fmt.Fprintln(os.Stderr, \"test completed with zero assertions\")\n")
	b.WriteString("		os.Exit(1)\n")
	b.WriteString("	}\n")
	b.WriteString("}\n")
	return pruneGeneratedImports(b.String()), nil
}

func pruneGeneratedImports(src string) string {
	start := strings.Index(src, "import (\n")
	if start < 0 {
		return src
	}
	bodyStart := start + len("import (\n")
	endRel := strings.Index(src[bodyStart:], ")\n\n")
	if endRel < 0 {
		return src
	}
	end := bodyStart + endRel
	importBlock := src[bodyStart:end]
	body := src[end+len(")\n\n"):]
	kept := make([]string, 0)
	for _, line := range strings.Split(importBlock, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		pkg := strings.Trim(trimmed, "\"")
		ident := generatedImportIdent(pkg)
		if ident == "" || strings.Contains(body, ident+".") {
			kept = append(kept, line)
		}
	}
	return src[:bodyStart] + strings.Join(kept, "\n") + "\n" + src[end:]
}

func generatedImportIdent(pkg string) string {
	switch pkg {
	case "github.com/yuechen-li-dev/oct/internal/octxiliary":
		return "octxiliary"
	case "github.com/yuechen-li-dev/oct/internal/prometheus":
		return "prometheus"
	}
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		return pkg[idx+1:]
	}
	return pkg
}

func collectFlowUserCalls(flow MIRFlow) []string {
	seen := map[string]struct{}{}
	var visitExpr func(MIRFlowExpr)
	var visitStmt func(MIRFlowStmt)
	var visitAction func(MIRFlowWhenAction)
	visitExpr = func(expr MIRFlowExpr) {
		switch value := expr.(type) {
		case MIRFlowSharedExpr:
			for _, block := range value.Blocks {
				for _, statement := range block.Statements {
					if call, ok := statement.(MIRCall); ok && !call.Builtin && !call.FunctionValue {
						seen[call.Callee] = struct{}{}
					}
				}
			}
		case MIRFlowUtilityWhenExpr:
			visitExpr(value.Hysteresis)
			visitExpr(value.MinCommit)
			visitExpr(value.Else)
			for _, candidate := range value.Cases {
				visitExpr(candidate.Value)
				visitExpr(candidate.Condition)
				visitExpr(candidate.Score)
			}
		}
	}
	visitAction = func(action MIRFlowWhenAction) {
		switch a := action.(type) {
		case MIRFlowWhenReturn:
			if a.Value != nil {
				visitExpr(a.Value)
			}
		case MIRFlowWhenBlock:
			for _, statement := range a.Statements {
				visitStmt(statement)
			}
		}
	}
	visitStmt = func(stmt MIRFlowStmt) {
		switch s := stmt.(type) {
		case MIRFlowLetStmt:
			visitExpr(s.Value)
		case MIRFlowLocalAssign:
			visitExpr(s.Value)
		case MIRFlowWhile:
			visitExpr(s.Condition)
			for _, nested := range s.Body {
				visitStmt(nested)
			}
		case MIRFlowFor:
			visitExpr(s.Start)
			visitExpr(s.End)
			visitExpr(s.Step)
			for _, nested := range s.Body {
				visitStmt(nested)
			}
		case MIRFlowFallibleMatch:
			visitExpr(s.Subject)
			for _, nested := range s.OkBody {
				visitStmt(nested)
			}
			for _, nested := range s.ErrBody {
				visitStmt(nested)
			}
		case MIRFlowFieldAssign:
			visitExpr(s.Value)
		case MIRFlowFieldIndexAssign:
			for _, index := range s.Indices {
				visitExpr(index)
			}
			visitExpr(s.Value)
		case MIRFlowReturn:
			if s.Value != nil {
				visitExpr(s.Value)
			}
		case MIRFlowYield:
			visitExpr(s.Value)
		case MIRFlowIf:
			visitExpr(s.Condition)
			for _, nested := range s.Then {
				visitStmt(nested)
			}
			for _, nested := range s.Else {
				visitStmt(nested)
			}
		case MIRFlowWhen:
			for _, c := range s.Cases {
				visitExpr(c.Condition)
				visitAction(c.Action)
			}
			visitAction(s.Else)
		}
	}
	for _, state := range flow.States {
		for _, statement := range state.Statements {
			visitStmt(statement)
		}
	}
	result := make([]string, 0, len(seen))
	for call := range seen {
		result = append(result, call)
	}
	sort.Strings(result)
	return result
}

func collectFlowBuiltins(flow MIRFlow, usedBuiltins map[string]bool) {
	for _, state := range flow.States {
		for _, stmt := range state.Statements {
			collectFlowBuiltinsStmt(stmt, usedBuiltins)
		}
	}
}

func collectFlowBuiltinsStmt(stmt MIRFlowStmt, usedBuiltins map[string]bool) {
	switch s := stmt.(type) {
	case MIRFlowLetStmt:
		collectFlowBuiltinsExpr(s.Value, usedBuiltins)
	case MIRFlowLocalAssign:
		collectFlowBuiltinsExpr(s.Value, usedBuiltins)
	case MIRFlowWhile:
		collectFlowBuiltinsExpr(s.Condition, usedBuiltins)
		for _, nested := range s.Body {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
	case MIRFlowFor:
		collectFlowBuiltinsExpr(s.Start, usedBuiltins)
		collectFlowBuiltinsExpr(s.End, usedBuiltins)
		collectFlowBuiltinsExpr(s.Step, usedBuiltins)
		for _, nested := range s.Body {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
	case MIRFlowFallibleMatch:
		collectFlowBuiltinsExpr(s.Subject, usedBuiltins)
		for _, nested := range s.OkBody {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
		for _, nested := range s.ErrBody {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
	case MIRFlowFieldAssign:
		collectFlowBuiltinsExpr(s.Value, usedBuiltins)
	case MIRFlowReturn:
		if s.Value != nil {
			collectFlowBuiltinsExpr(s.Value, usedBuiltins)
		}
	case MIRFlowIf:
		collectFlowBuiltinsExpr(s.Condition, usedBuiltins)
		for _, nested := range s.Then {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
		for _, nested := range s.Else {
			collectFlowBuiltinsStmt(nested, usedBuiltins)
		}
	case MIRFlowWhen:
		for _, c := range s.Cases {
			collectFlowBuiltinsExpr(c.Condition, usedBuiltins)
			collectFlowBuiltinsWhenAction(c.Action, usedBuiltins)
		}
		collectFlowBuiltinsWhenAction(s.Else, usedBuiltins)
	}
}

func collectFlowBuiltinsWhenAction(action MIRFlowWhenAction, usedBuiltins map[string]bool) {
	switch a := action.(type) {
	case MIRFlowWhenReturn:
		collectFlowBuiltinsExpr(a.Value, usedBuiltins)
	case MIRFlowWhenBlock:
		for _, stmt := range a.Statements {
			collectFlowBuiltinsStmt(stmt, usedBuiltins)
		}
	}
}

func collectFlowBuiltinsExpr(expr MIRFlowExpr, usedBuiltins map[string]bool) {
	switch e := expr.(type) {
	case MIRFlowSharedExpr:
		for _, block := range e.Blocks {
			for _, statement := range block.Statements {
				switch node := statement.(type) {
				case MIRCall:
					if node.Builtin {
						usedBuiltins[node.Callee] = true
					}
				case MIRBatchMap:
					usedBuiltins["BatchMap"] = true
				}
			}
		}
	case MIRFlowUtilityWhenExpr:
		collectFlowBuiltinsExpr(e.Hysteresis, usedBuiltins)
		collectFlowBuiltinsExpr(e.MinCommit, usedBuiltins)
		for _, c := range e.Cases {
			collectFlowBuiltinsExpr(c.Value, usedBuiltins)
			collectFlowBuiltinsExpr(c.Condition, usedBuiltins)
			collectFlowBuiltinsExpr(c.Score, usedBuiltins)
		}
		collectFlowBuiltinsExpr(e.Else, usedBuiltins)
	}
}

func walkFlowSharedStatements(flow MIRFlow, visit func(MIRStmt)) {
	var walkExpr func(MIRFlowExpr)
	var walkStmt func(MIRFlowStmt)
	var walkAction func(MIRFlowWhenAction)
	walkExpr = func(expr MIRFlowExpr) {
		switch node := expr.(type) {
		case MIRFlowSharedExpr:
			for _, block := range node.Blocks {
				for _, statement := range block.Statements {
					visit(statement)
				}
			}
		case MIRFlowUtilityWhenExpr:
			walkExpr(node.Hysteresis)
			walkExpr(node.MinCommit)
			walkExpr(node.Else)
			for _, candidate := range node.Cases {
				walkExpr(candidate.Value)
				walkExpr(candidate.Condition)
				walkExpr(candidate.Score)
			}
		}
	}
	walkAction = func(action MIRFlowWhenAction) {
		switch node := action.(type) {
		case MIRFlowWhenReturn:
			if node.Value != nil {
				walkExpr(node.Value)
			}
		case MIRFlowWhenBlock:
			for _, statement := range node.Statements {
				walkStmt(statement)
			}
		}
	}
	walkStmt = func(statement MIRFlowStmt) {
		switch node := statement.(type) {
		case MIRFlowLetStmt:
			walkExpr(node.Value)
		case MIRFlowLocalAssign:
			walkExpr(node.Value)
		case MIRFlowFieldAssign:
			walkExpr(node.Value)
		case MIRFlowFieldIndexAssign:
			for _, index := range node.Indices {
				walkExpr(index)
			}
			walkExpr(node.Value)
		case MIRFlowReturn:
			if node.Value != nil {
				walkExpr(node.Value)
			}
		case MIRFlowYield:
			walkExpr(node.Value)
		case MIRFlowIf:
			walkExpr(node.Condition)
			for _, nested := range node.Then {
				walkStmt(nested)
			}
			for _, nested := range node.Else {
				walkStmt(nested)
			}
		case MIRFlowWhile:
			walkExpr(node.Condition)
			for _, nested := range node.Body {
				walkStmt(nested)
			}
		case MIRFlowFor:
			walkExpr(node.Start)
			walkExpr(node.End)
			walkExpr(node.Step)
			for _, nested := range node.Body {
				walkStmt(nested)
			}
		case MIRFlowFallibleMatch:
			walkExpr(node.Subject)
			for _, nested := range node.OkBody {
				walkStmt(nested)
			}
			for _, nested := range node.ErrBody {
				walkStmt(nested)
			}
		case MIRFlowWhen:
			for _, candidate := range node.Cases {
				walkExpr(candidate.Condition)
				walkAction(candidate.Action)
			}
			walkAction(node.Else)
		}
	}
	for _, state := range flow.States {
		for _, statement := range state.Statements {
			walkStmt(statement)
		}
	}
}

func emitGoStringFromAssign(target string, args []string, argTypes []string) (string, error) {
	if len(args) != 1 || len(argTypes) != 1 {
		return "", fmt.Errorf("function 'StringFrom' expects 1 argument, got %d", len(args))
	}
	expr, err := emitGoStringFromExpr(args[0], argTypes[0])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = %s", target, expr), nil
}

func emitGoStringFromExpr(arg string, argType string) (string, error) {
	switch argType {
	case "Int":
		return fmt.Sprintf("strconv.Itoa(%s)", arg), nil
	case "Float":
		return fmt.Sprintf("strconv.FormatFloat(%s, 'g', -1, 64)", arg), nil
	case "Bool":
		return fmt.Sprintf("strconv.FormatBool(%s)", arg), nil
	case "String":
		return arg, nil
	default:
		return "", fmt.Errorf("compiled mode does not yet support builtin StringFrom for type %s", argType)
	}
}

func emitGoBuiltinCallExpr(callee string, args []string) (string, error) {
	switch canonicalCompiledBuiltinName(callee) {
	case "Len":
		return fmt.Sprintf("len(%s)", args[0]), nil
	case "Abs":
		return fmt.Sprintf("math.Abs(%s)", args[0]), nil
	case "Sqrt":
		return fmt.Sprintf("math.Sqrt(%s)", args[0]), nil
	case "Sin":
		return fmt.Sprintf("math.Sin(%s)", args[0]), nil
	case "Cos":
		return fmt.Sprintf("math.Cos(%s)", args[0]), nil
	case "Tan":
		return fmt.Sprintf("math.Tan(%s)", args[0]), nil
	case "Asin":
		return fmt.Sprintf("math.Asin(%s)", args[0]), nil
	case "Acos":
		return fmt.Sprintf("math.Acos(%s)", args[0]), nil
	case "Atan":
		return fmt.Sprintf("math.Atan(%s)", args[0]), nil
	case "Atan2":
		return fmt.Sprintf("math.Atan2(%s, %s)", args[0], args[1]), nil
	case "Exp":
		return fmt.Sprintf("math.Exp(%s)", args[0]), nil
	case "Ln":
		return fmt.Sprintf("math.Log(%s)", args[0]), nil
	case "Pow":
		return fmt.Sprintf("math.Pow(%s, %s)", args[0], args[1]), nil
	case "Log10":
		return fmt.Sprintf("math.Log10(%s)", args[0]), nil
	case "Sinh":
		return fmt.Sprintf("math.Sinh(%s)", args[0]), nil
	case "Cosh":
		return fmt.Sprintf("math.Cosh(%s)", args[0]), nil
	case "Tanh":
		return fmt.Sprintf("math.Tanh(%s)", args[0]), nil
	case "Pi":
		return "math.Pi", nil
	case "E":
		return "math.E", nil
	case "Float":
		return fmt.Sprintf("float64(%s)", args[0]), nil
	case "Clamp01":
		return fmt.Sprintf("func(__v float64) float64 { if __v < 0.0 { return 0.0 }; if __v > 1.0 { return 1.0 }; return __v }(%s)", args[0]), nil
	case "FloorToInt":
		return fmt.Sprintf("int(math.Floor(%s))", args[0]), nil
	case "CeilToInt":
		return fmt.Sprintf("int(math.Ceil(%s))", args[0]), nil
	case "RoundToInt":
		return fmt.Sprintf("int(math.Round(%s))", args[0]), nil
	case "FormatFloat":
		return fmt.Sprintf("strconv.FormatFloat(%s, 'f', int(%s), 64)", args[0], args[1]), nil
	default:
		return "", unsupportedBuiltin(callee)
	}
}

func octxiliaryKindExprWithTransport(t string, transportTypes []project.TransportTypeMetadata) string {
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "handle" {
		return "octxiliary.ValueHandle"
	}
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "record" {
		return "octxiliary.ValueRecord"
	}
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return "octxiliary.ValueInt"
	}
	switch t {
	case "Void":
		return "octxiliary.ValueVoid"
	case "Int":
		return "octxiliary.ValueInt"
	case "Float":
		return "octxiliary.ValueFloat"
	case "Bool":
		return "octxiliary.ValueBool"
	case "String":
		return "octxiliary.ValueString"
	case "String[]":
		return "octxiliary.ValueStringArray"
	case "String[][]":
		return "octxiliary.ValueStringMatrix"
	case "Float[]":
		return "octxiliary.ValueFloatArray"
	case "Bytes":
		return "octxiliary.ValueBytes"
	default:
		return "octxiliary.ValueKind(\"" + t + "\")"
	}
}

func octxiliaryValueExprWithTransportFamily(t string, expr string, transportTypes []project.TransportTypeMetadata, family string) (string, error) {
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueInt, Int: %s}", expr), nil
	}
	switch t {
	case "Void":
		return "octxiliary.Value{Kind: octxiliary.ValueVoid}", nil
	case "Int":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueInt, Int: %s}", expr), nil
	case "Float":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueFloat, Float: %s}", expr), nil
	case "Bool":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueBool, Bool: %s}", expr), nil
	case "String":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueString, String: %s}", expr), nil
	case "String[]":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueStringArray, Strings: %s}", expr), nil
	case "String[][]":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueStringMatrix, Strings2: %s}", expr), nil
	case "Float[]":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueFloatArray, Floats: %s}", expr), nil
	case "Bytes":
		return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueBytes, Bytes: %s}", expr), nil
	default:
		if record := findTransportRecord(transportTypes, t); record.ok {
			if record.typ.Kind == "handle" {
				return fmt.Sprintf("func() octxiliary.Value { __handleID := %s.Handle; return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: %q, HandleType: %q, HandleID: __handleID} }()", expr, family, t), nil
			}
			fields := make([]string, 0, len(record.typ.Fields))
			for _, field := range record.typ.Fields {
				fieldType := transportRuntimeBaseType(field.Type)
				fieldExpr, err := octxiliaryValueExprWithTransportFamily(fieldType, expr+"."+field.Name, transportTypes, family)
				if err != nil {
					return "", err
				}
				fields = append(fields, fmt.Sprintf("{Name: %q, Value: %s}", field.Name, fieldExpr))
			}
			return fmt.Sprintf("octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: %q, Fields: []octxiliary.FieldValue{%s}}", t, strings.Join(fields, ", ")), nil
		}
		return "", fmt.Errorf("unsupported Octxiliary transport type %s", t)
	}
}

func octxiliaryValueExtractExprWithTransport(t string, value string, transportTypes []project.TransportTypeMetadata) string {
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "handle" {
		return fmt.Sprintf("%s{Handle: %s.HandleID}", goType(t), value)
	}
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "record" {
		parts := make([]string, 0, len(transport.typ.Fields))
		for idx, field := range transport.typ.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", field.Name, octxiliaryValueExtractExprWithTransport(field.Type, fmt.Sprintf("%s.Fields[%d].Value", value, idx), transportTypes)))
		}
		return fmt.Sprintf("%s{%s}", goType(t), strings.Join(parts, ", "))
	}
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return value + ".Int"
	}
	switch t {
	case "Void":
		return "__octVoid{}"
	case "Int":
		return value + ".Int"
	case "Float":
		return value + ".Float"
	case "Bool":
		return value + ".Bool"
	case "String":
		return value + ".String"
	case "String[]":
		return value + ".Strings"
	case "String[][]":
		return value + ".Strings2"
	case "Float[]":
		return value + ".Floats"
	case "Bytes":
		return value + ".Bytes"
	default:
		return value
	}
}

func goStmt(s MIRStmt) (string, error) {
	switch st := s.(type) {
	case MIRAssign:
		value, err := emitGoValue(st.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", st.Target, value), nil
	case MIRRowAssign:
		index, err := emitGoValue(st.Index)
		if err != nil {
			return "", err
		}
		value, err := emitGoValue(st.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("__octAssignRow(%s, %s, %s)", st.Target, index, value), nil
	case MIRIndexAssign:
		indices, err := emitGoValues(st.Indices)
		if err != nil {
			return "", err
		}
		value, err := emitGoValue(st.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s[%s] = %s", st.Target, strings.Join(indices, "]["), value), nil
	case MIRConstructArray:
		values, err := emitGoValues(st.Values)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = []%s{%s}", st.Target, goType(st.ElemType), strings.Join(values, ", ")), nil
	case MIRConstructRecord:
		values, err := emitGoValues(st.FieldVals)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(st.FieldNames))
		for i := range st.FieldNames {
			parts = append(parts, fmt.Sprintf("%s: %s", st.FieldNames[i], values[i]))
		}
		statement := fmt.Sprintf("%s = %s{%s}", st.Target, goType(st.TypeName), strings.Join(parts, ", "))
		if st.TemplateOrigin != "" {
			return fmt.Sprintf("// Oct template override: %s fields=%s\n%s", st.TemplateOrigin, strings.Join(st.TemplateOverrideSet, ","), statement), nil
		}
		return statement, nil
	case MIRGenericOctxiliaryCall:
		valueArgs := make([]string, 0, len(st.Args))
		for i, arg := range st.Args {
			emittedArg, err := emitGoValue(arg)
			if err != nil {
				return "", err
			}
			valueExpr, err := octxiliaryValueExprWithTransportFamily(st.ArgTypes[i], emittedArg, st.TransportTypes, st.Family)
			if err != nil {
				return "", err
			}
			valueArgs = append(valueArgs, valueExpr)
		}
		call := fmt.Sprintf("__octOctxiliaryGenericCall(%q, %q, %q, []octxiliary.Value{%s}, %s)", st.SidecarCommand, st.Family, st.WireName, strings.Join(valueArgs, ", "), octxiliaryKindExprWithTransport(st.RetType, st.TransportTypes))
		retValidation := ""
		if transport := findTransportRecord(st.TransportTypes, st.RetType); transport.ok && transport.typ.Kind == "handle" {
			retValidation = fmt.Sprintf("if __err := __octOctxiliaryValidateHandle(__value, %q, %q); __err != nil { ", st.Family, st.RetType)
		}
		extractExpr := octxiliaryValueExtractExprWithTransport(st.RetType, "__value", st.TransportTypes)
		if st.Fallible {
			if st.RetType == "Void" {
				return fmt.Sprintf("%s = func() %s { __value, __err := %s; _ = __value; if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __octVoid{}} }()", st.Target, goResultTypeName(st.RetType), call, goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			}
			if retValidation != "" {
				return fmt.Sprintf("%s = func() %s { __value, __err := %s; if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; %sreturn %s{Err: __err.Error(), IsErr: true} }; return %s{Value: %s} }()", st.Target, goResultTypeName(st.RetType), call, goResultTypeName(st.RetType), retValidation, goResultTypeName(st.RetType), goResultTypeName(st.RetType), extractExpr), nil
			}
			return fmt.Sprintf("%s = func() %s { __value, __err := %s; if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: %s} }()", st.Target, goResultTypeName(st.RetType), call, goResultTypeName(st.RetType), goResultTypeName(st.RetType), extractExpr), nil
		}
		if st.RetType == "Void" {
			return fmt.Sprintf("%s = func() __octVoid { __value, __err := %s; _ = __value; if __err != nil { panic(\"runtime error: \" + __err.Error()) }; return __octVoid{} }()", st.Target, call), nil
		}
		if retValidation != "" {
			return fmt.Sprintf("%s = func() %s { __value, __err := %s; if __err != nil { panic(\"runtime error: \" + __err.Error()) }; %spanic(\"runtime error: \" + __err.Error()) }; return %s }()", st.Target, goType(st.RetType), call, retValidation, extractExpr), nil
		}
		return fmt.Sprintf("%s = func() %s { __value, __err := %s; if __err != nil { panic(\"runtime error: \" + __err.Error()) }; return %s }()", st.Target, goType(st.RetType), call, extractExpr), nil
	case MIRCall:
		args, err := emitGoValues(st.Args)
		if err != nil {
			return "", err
		}
		if st.Builtin {
			switch canonicalCompiledBuiltinName(st.Callee) {
			case "Idx":
				return fmt.Sprintf("%s = __octIdx(%s)", st.Target, args[0]), nil
			case "EinMul":
				return fmt.Sprintf("%s = __octEinMulMM(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "EinAdd":
				return fmt.Sprintf("%s = __octEinAddMM(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "EinSub":
				return fmt.Sprintf("%s = __octEinSubMM(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "EinAddVV":
				return fmt.Sprintf("%s = __octEinAddVV(%s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3]), nil
			case "EinSubVV":
				return fmt.Sprintf("%s = __octEinSubVV(%s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3]), nil
			case "EinDotVV":
				return fmt.Sprintf("%s = __octEinDotVV(%s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3]), nil
			case "EinOuterVV":
				return fmt.Sprintf("%s = __octEinOuterVV(%s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3]), nil
			case "EinMulMV":
				return fmt.Sprintf("%s = __octEinMulMV(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "EinMulVM":
				return fmt.Sprintf("%s = __octEinMulVM(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "EinDoubleMM":
				return fmt.Sprintf("%s = __octEinDoubleMM(%s, %s, %s, %s, %s, %s)", st.Target, args[0], args[1], args[2], args[3], args[4], args[5]), nil
			case "Len":
				return fmt.Sprintf("%s = len(%s)", st.Target, args[0]), nil
			case "Append":
				value := args[1]
				if len(st.ArgTypes) > 1 {
					value = cloneCompiledValueExpr(value, st.ArgTypes[1])
				}
				return fmt.Sprintf("%s = append(%s, %s)", st.Target, args[0], value), nil
			case "ArrayCrossSection", "Array.CrossSection":
				return fmt.Sprintf("%s = __octArrayCrossSection(%s, %s)", st.Target, args[0], args[1]), nil
			case "ArrayWhere", "Array.Where":
				return fmt.Sprintf("%s = __octArrayWhere(%s, %s)", st.Target, args[0], args[1]), nil
			case "Print":
				return fmt.Sprintf("fmt.Println(%s); %s = 0", args[0], st.Target), nil
			case "FormatFloat":
				return fmt.Sprintf("%s = strconv.FormatFloat(%s, 'f', int(%s), 64)", st.Target, args[0], args[1]), nil
			case "Assert.True":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !%s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", args[0], args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !%s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", args[0], args[1], st.Target), nil
			case "Assert.False":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if %s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", args[0], args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if %s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", args[0], args[1], st.Target), nil
			case "Assert.Equal":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !reflect.DeepEqual(%s, %s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", args[0], args[1], args[2]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !reflect.DeepEqual(%s, %s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", args[0], args[1], args[2], st.Target), nil
			case "Assert.Near":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if math.Abs((%s)-(%s)) > (%s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", args[0], args[1], args[2], args[3]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if math.Abs((%s)-(%s)) > (%s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", args[0], args[1], args[2], args[3], st.Target), nil
			case "Assert.Error":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !%s.IsErr { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", args[0], args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !%s.IsErr { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", args[0], args[1], st.Target), nil
			case "Assert.LGTM":
				return fmt.Sprintf("__octAssertionCount++; if %s.IsErr { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\nunderlying error: %%s\\n\", %s, %s.Err); os.Exit(1) }; %s = %s.Value", args[0], args[1], args[0], st.Target, args[0]), nil
			case "ToString":
				return fmt.Sprintf("%s = fmt.Sprint(%s)", st.Target, args[0]), nil
			case "Float":
				return fmt.Sprintf("%s = float64(%s)", st.Target, args[0]), nil
			case "Clamp01":
				return fmt.Sprintf("%s = func(__v float64) float64 { if __v < 0.0 { return 0.0 }; if __v > 1.0 { return 1.0 }; return __v }(%s)", st.Target, args[0]), nil
			case "Complex":
				return fmt.Sprintf("%s = complex(float64(%s), float64(%s))", st.Target, args[0], args[1]), nil
			case "ComplexPolar":
				return fmt.Sprintf("%s = cmplx.Rect(float64(%s), float64(%s))", st.Target, args[0], args[1]), nil
			case "I":
				return fmt.Sprintf("%s = complex(0, 1)", st.Target), nil
			case "Real":
				return fmt.Sprintf("%s = __octComplexReal(%s)", st.Target, args[0]), nil
			case "Imag":
				return fmt.Sprintf("%s = __octComplexImag(%s)", st.Target, args[0]), nil
			case "Arg":
				return fmt.Sprintf("%s = cmplx.Phase(%s)", st.Target, args[0]), nil
			case "Conj":
				return fmt.Sprintf("%s = cmplx.Conj(%s)", st.Target, args[0]), nil
			case "Pi":
				return fmt.Sprintf("%s = math.Pi", st.Target), nil
			case "E":
				return fmt.Sprintf("%s = math.E", st.Target), nil
			case "Abs":
				if len(st.ArgTypes) == 1 && isComplexScalarTypeString(st.ArgTypes[0]) {
					return fmt.Sprintf("%s = __octComplexAbs(%s)", st.Target, args[0]), nil
				}
				if isIntScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = func(__v int) int { if __v < 0 { return -__v }; return __v }(%s)", st.Target, args[0]), nil
				}
				if isFloatScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = math.Abs(%s)", st.Target, args[0]), nil
				}
				return "", fmt.Errorf("compiled mode does not yet support builtin Abs for type %s", st.RetType)
			case "Sqrt":
				return fmt.Sprintf("%s = math.Sqrt(float64(%s))", st.Target, args[0]), nil
			case "Sin":
				return fmt.Sprintf("%s = math.Sin(float64(%s))", st.Target, args[0]), nil
			case "Cos":
				return fmt.Sprintf("%s = math.Cos(float64(%s))", st.Target, args[0]), nil
			case "Tan":
				return fmt.Sprintf("%s = math.Tan(float64(%s))", st.Target, args[0]), nil
			case "Asin":
				return fmt.Sprintf("%s = math.Asin(float64(%s))", st.Target, args[0]), nil
			case "Acos":
				return fmt.Sprintf("%s = math.Acos(float64(%s))", st.Target, args[0]), nil
			case "Atan":
				return fmt.Sprintf("%s = math.Atan(float64(%s))", st.Target, args[0]), nil
			case "Atan2":
				return fmt.Sprintf("%s = math.Atan2(float64(%s), float64(%s))", st.Target, args[0], args[1]), nil
			case "Exp":
				if isComplexScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = cmplx.Exp(%s)", st.Target, args[0]), nil
				}
				return fmt.Sprintf("%s = math.Exp(float64(%s))", st.Target, args[0]), nil
			case "Ln":
				if isComplexScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = cmplx.Log(%s)", st.Target, args[0]), nil
				}
				return fmt.Sprintf("%s = math.Log(float64(%s))", st.Target, args[0]), nil
			case "Pow":
				return fmt.Sprintf("%s = math.Pow(float64(%s), float64(%s))", st.Target, args[0], args[1]), nil
			case "Log10":
				return fmt.Sprintf("%s = math.Log10(float64(%s))", st.Target, args[0]), nil
			case "Sinh":
				return fmt.Sprintf("%s = math.Sinh(float64(%s))", st.Target, args[0]), nil
			case "Cosh":
				return fmt.Sprintf("%s = math.Cosh(float64(%s))", st.Target, args[0]), nil
			case "Tanh":
				return fmt.Sprintf("%s = math.Tanh(float64(%s))", st.Target, args[0]), nil
			case "FloorToInt", "Math.FloorToInt":
				return fmt.Sprintf("%s = int(math.Floor(float64(%s)))", st.Target, args[0]), nil
			case "CeilToInt", "Math.CeilToInt":
				return fmt.Sprintf("%s = int(math.Ceil(float64(%s)))", st.Target, args[0]), nil
			case "RoundToInt":
				return fmt.Sprintf("%s = int(math.Round(float64(%s)))", st.Target, args[0]), nil
			case "BaseValue", "BaseUnit":
				return fmt.Sprintf("%s = float64(%s)", st.Target, args[0]), nil
			case "Contains":
				return fmt.Sprintf("%s = strings.Contains(%s, %s)", st.Target, args[0], args[1]), nil
			case "StartsWith":
				return fmt.Sprintf("%s = strings.HasPrefix(%s, %s)", st.Target, args[0], args[1]), nil
			case "EndsWith":
				return fmt.Sprintf("%s = strings.HasSuffix(%s, %s)", st.Target, args[0], args[1]), nil
			case "Trim":
				return fmt.Sprintf("%s = strings.TrimSpace(%s)", st.Target, args[0]), nil
			case "Lower":
				return fmt.Sprintf("%s = strings.ToLower(%s)", st.Target, args[0]), nil
			case "Upper":
				return fmt.Sprintf("%s = strings.ToUpper(%s)", st.Target, args[0]), nil
			case "Join":
				return fmt.Sprintf("%s = strings.Join(%s, %s)", st.Target, args[0], args[1]), nil

			case "StringByteLength":
				return fmt.Sprintf("%s = len(%s)", st.Target, args[0]), nil
			case "StringRuneCount":
				return fmt.Sprintf("%s = utf8.RuneCountInString(%s)", st.Target, args[0]), nil
			case "StringJoin":
				return fmt.Sprintf("%s = strings.Join(%s, %s)", st.Target, args[0], args[1]), nil
			case "StringConcat":
				return fmt.Sprintf("%s = strings.Join(%s, \"\")", st.Target, args[0]), nil
			case "StringFrom":
				return emitGoStringFromAssign(st.Target, args, st.ArgTypes)
			case "StringReplaceAll":
				return fmt.Sprintf("%s = strings.ReplaceAll(%s, %s, %s)", st.Target, args[0], args[1], args[2]), nil
			case "StringContains":
				return fmt.Sprintf("%s = strings.Contains(%s, %s)", st.Target, args[0], args[1]), nil
			case "StringStartsWith":
				return fmt.Sprintf("%s = strings.HasPrefix(%s, %s)", st.Target, args[0], args[1]), nil
			case "StringEndsWith":
				return fmt.Sprintf("%s = strings.HasSuffix(%s, %s)", st.Target, args[0], args[1]), nil
			case "StringTrim":
				return fmt.Sprintf("%s = strings.TrimSpace(%s)", st.Target, args[0]), nil
			case "StringSplitLines":
				return fmt.Sprintf("%s = __octStringSplitLines(%s)", st.Target, args[0]), nil
			case "StringEscapeJSON":
				return fmt.Sprintf("%s = __octStringEscapeJSON(%s)", st.Target, args[0]), nil
			case "StringQuoteJSON":
				return fmt.Sprintf("%s = strconv.Quote(%s)", st.Target, args[0]), nil
			case "MarkdownEscapeText":
				return fmt.Sprintf("%s = __octMarkdownNormalizeInline(%s)", st.Target, args[0]), nil
			case "MarkdownEscapeTableCell":
				return fmt.Sprintf("%s = __octMarkdownEscapeTableCell(%s)", st.Target, args[0]), nil
			case "MarkdownH1":
				return fmt.Sprintf("%s = []string{\"# \" + __octMarkdownNormalizeInline(%s)}", st.Target, args[0]), nil
			case "MarkdownH2":
				return fmt.Sprintf("%s = []string{\"## \" + __octMarkdownNormalizeInline(%s)}", st.Target, args[0]), nil
			case "MarkdownH3":
				return fmt.Sprintf("%s = []string{\"### \" + __octMarkdownNormalizeInline(%s)}", st.Target, args[0]), nil
			case "MarkdownParagraph":
				return fmt.Sprintf("%s = []string{__octMarkdownNormalizeInline(%s)}", st.Target, args[0]), nil
			case "MarkdownBlank":
				return fmt.Sprintf("%s = []string{\"\"}", st.Target), nil
			case "MarkdownHorizontalRule":
				return fmt.Sprintf("%s = []string{\"---\"}", st.Target), nil
			case "MarkdownBullets":
				return fmt.Sprintf("%s = __octMarkdownList(%s, false)", st.Target, args[0]), nil
			case "MarkdownNumbered":
				return fmt.Sprintf("%s = __octMarkdownList(%s, true)", st.Target, args[0]), nil
			case "MarkdownCodeBlock":
				return fmt.Sprintf("%s = __octMarkdownCodeBlock(%s, %s)", st.Target, args[0], args[1]), nil
			case "MarkdownCallout":
				return fmt.Sprintf("%s = __octMarkdownCallout(%s, %s)", st.Target, args[0], args[1]), nil
			case "MarkdownImage":
				return fmt.Sprintf("%s = __octMarkdownImage(%s, %s)", st.Target, args[0], args[1]), nil
			case "MarkdownFigure":
				return fmt.Sprintf("%s = __octMarkdownFigure(%s, %s)", st.Target, args[0], args[1]), nil
			case "MarkdownKeyValueTable":
				return fmt.Sprintf("%s = __octMarkdownKeyValueTable(%s, %s)", st.Target, args[0], args[1]), nil
			case "MarkdownReport":
				return fmt.Sprintf("%s = __octMarkdownFlattenBlocks(%s)", st.Target, args[0]), nil
			case "MarkdownSection":
				return fmt.Sprintf("%s = __octMarkdownSection(%s, %s, false)", st.Target, args[0], args[1]), nil
			case "MarkdownSubsection":
				return fmt.Sprintf("%s = __octMarkdownSection(%s, %s, true)", st.Target, args[0], args[1]), nil
			case "MarkdownTable":
				return fmt.Sprintf("%s = __octMarkdownTable(%s)", st.Target, args[0]), nil
			case "MarkdownTableWithColumns":
				return fmt.Sprintf("%s = __octMarkdownTableWithColumns(%s, %s)", st.Target, args[0], args[1]), nil
			case "FFT":
				return fmt.Sprintf("%s = __octFFT(%s)", st.Target, args[0]), nil
			case "WriteOctagon":
				return fmt.Sprintf("__octWriteOctagon(%s, %s); %s = 0", args[0], args[1], st.Target), nil
			case "LoadOctagon":
				return fmt.Sprintf("%s = __octLoadOctagon_%s(%s)", st.Target, goSafeName(st.RetType), args[0]), nil
			case "JsonNormalize", "JsonParse", "JsonStringify":
				return fmt.Sprintf("%s = __octJsonString(%q, %s)", st.Target, canonicalCompiledBuiltinName(st.Callee), args[0]), nil
			case "JsonLoad":
				return fmt.Sprintf("%s = __octJsonString(%q, %s)", st.Target, "JsonLoad", args[0]), nil
			case "JsonSave":
				return fmt.Sprintf("%s = __octJsonSave(%s, %s)", st.Target, args[0], args[1]), nil
			case "CsvRead", "CsvReadRows":
				return fmt.Sprintf("%s = __octCsvReadRows(%s)", st.Target, args[0]), nil
			case "CsvWrite", "CsvWriteRows":
				return fmt.Sprintf("%s = __octCsvWriteRows(%s, %s)", st.Target, args[0], args[1]), nil
			case "CsvReadMatrix":
				return fmt.Sprintf("%s = __octCsvReadMatrix(%s)", st.Target, args[0]), nil
			case "CsvReadTable":
				return fmt.Sprintf("%s = __octCsvReadTable(%s)", st.Target, args[0]), nil
			case "FileReadText":
				return fmt.Sprintf("%s = __octFileReadText(%s)", st.Target, args[0]), nil
			case "FileWriteText":
				return fmt.Sprintf("%s = __octFileWriteText(%s, %s)", st.Target, args[0], args[1]), nil
			case "FileReadBytes":
				return fmt.Sprintf("%s = __octFileReadBytes(%s)", st.Target, args[0]), nil
			case "FileWriteBytes":
				return fmt.Sprintf("%s = __octFileWriteBytes(%s, %s)", st.Target, args[0], args[1]), nil
			case "FileReadLines":
				return fmt.Sprintf("%s = __octFileReadLines(%s)", st.Target, args[0]), nil
			case "FileWriteLines":
				return fmt.Sprintf("%s = __octFileWriteLines(%s, %s)", st.Target, args[0], args[1]), nil
			case "FileExists":
				return fmt.Sprintf("%s = func() bool { _, __err := os.Stat(%s); return __err == nil }()", st.Target, args[0]), nil
			case "PathJoin":
				return fmt.Sprintf("%s = filepath.Join(%s...)", st.Target, args[0]), nil
			case "PathBaseName":
				return fmt.Sprintf("%s = filepath.Base(%s)", st.Target, args[0]), nil
			case "PathExtension":
				return fmt.Sprintf("%s = filepath.Ext(%s)", st.Target, args[0]), nil
			case "PathStem":
				return fmt.Sprintf("%s = strings.TrimSuffix(filepath.Base(%s), filepath.Ext(filepath.Base(%s)))", st.Target, args[0], args[0]), nil
			case "PathParent":
				return fmt.Sprintf("%s = filepath.Dir(%s)", st.Target, args[0]), nil
			case "PathClean":
				return fmt.Sprintf("%s = filepath.Clean(%s)", st.Target, args[0]), nil
			case "FileDelete":
				return fmt.Sprintf("%s = __octFileDelete(%s)", st.Target, args[0]), nil
			case "DirectoryList":
				return fmt.Sprintf("%s = __octDirectoryList(%s)", st.Target, args[0]), nil
			case "DirectoryMake":
				return fmt.Sprintf("%s = __octDirectoryMake(%s)", st.Target, args[0]), nil
			case "DirectoryMakeAll":
				return fmt.Sprintf("%s = __octDirectoryMakeAll(%s)", st.Target, args[0]), nil
			case "DirectoryRemoveAll":
				return fmt.Sprintf("%s = __octDirectoryRemoveAll(%s)", st.Target, args[0]), nil
			case "Step":
				input := "nil"
				if len(args) == 2 {
					input = args[1]
				}
				return fmt.Sprintf("%s.__octStep(%s); %s = 0", args[0], input, st.Target), nil
			case "Active":
				return fmt.Sprintf("%s = %s.__octActive()", st.Target, args[0]), nil
			case "Result":
				return fmt.Sprintf("%s = func() %s { __value, __ok := %s.__octResult(); if !__ok { return %s{Err: \"Result() called before flow completion\", IsErr: true} }; return %s{Value: __value} }()",
					st.Target, goResultTypeName(st.RetType), args[0], goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "Complete":
				return fmt.Sprintf("%s = %s.__octComplete()", st.Target, args[0]), nil
			case "DidYield":
				return fmt.Sprintf("%s = %s.__octDidYield()", st.Target, args[0]), nil
			case "Yielded":
				return fmt.Sprintf("%s = func() %s { __value, __ok := %s.__octYielded(); if !__ok { return %s{Err: \"Yielded() called when the last turn did not yield\", IsErr: true} }; __typed, __typedOk := __value.(%s); if !__typedOk { return %s{Err: \"Yielded() flow yield type mismatch\", IsErr: true} }; return %s{Value: __typed} }()", st.Target, goResultTypeName(st.RetType), args[0], goResultTypeName(st.RetType), goType(st.RetType), goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "Query.First":
				return fmt.Sprintf("%s = func() %s { for !%s.__octComplete() { %s.__octStep(nil); if %s.__octDidYield() { __value, __ok := %s.__octYielded(); if !__ok { continue }; __typed, __typedOk := __value.(%s); if !__typedOk { return %s{Err: \"Query.First flow yield type mismatch\", IsErr: true} }; return %s{Value: __typed} } }; return %s{Err: \"Query.First found no value\", IsErr: true} }()", st.Target, goResultTypeName(st.RetType), args[0], args[0], args[0], args[0], goType(st.RetType), goResultTypeName(st.RetType), goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "Query.Any":
				return fmt.Sprintf("%s = func() bool { for !%s.__octComplete() { %s.__octStep(nil); if %s.__octDidYield() { return true } }; return false }()", st.Target, args[0], args[0], args[0]), nil
			case "Query.Count":
				return fmt.Sprintf("%s = func() int { __count := 0; for !%s.__octComplete() { %s.__octStep(nil); if %s.__octDidYield() { __count++ } }; return __count }()", st.Target, args[0], args[0], args[0]), nil
			case "StateHistory":
				return fmt.Sprintf("%s = %s.__octStateHistory()", st.Target, args[0]), nil
			case "ResumeTarget":
				return fmt.Sprintf("%s = %s.__octResumeTarget()", st.Target, args[0]), nil
			case "BoardSnapshot":
				return fmt.Sprintf("%s = func() %s { __snap, __ok := %s.__octBoardSnapshot(); if !__ok { return %s{Err: \"BoardSnapshot() requires a flow with a declared board\", IsErr: true} }; __typed, __typedOk := __snap.(%s); if !__typedOk { return %s{Err: \"BoardSnapshot() flow snapshot type mismatch\", IsErr: true} }; return %s{Value: __typed} }()",
					st.Target, goResultTypeName(st.RetType), args[0], goResultTypeName(st.RetType), goType(st.RetType), goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "MatMulMV":
				return fmt.Sprintf("%s = __octMatMulMV(%s, %s)", st.Target, args[0], args[1]), nil
			case "MatMulVM":
				return fmt.Sprintf("%s = __octMatMulVM(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecDot":
				return fmt.Sprintf("%s = __octVecDot(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVV:+":
				return fmt.Sprintf("%s = __octVecAddVV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVV:-":
				return fmt.Sprintf("%s = __octVecSubVV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVV:*":
				return fmt.Sprintf("%s = __octVecMulVV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVV:/":
				return fmt.Sprintf("%s = __octVecDivVV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVS:+":
				return fmt.Sprintf("%s = __octVecAddVS(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVS:-":
				return fmt.Sprintf("%s = __octVecSubVS(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVS:*":
				return fmt.Sprintf("%s = __octVecMulVS(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinaryVS:/":
				return fmt.Sprintf("%s = __octVecDivVS(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinarySV:+":
				return fmt.Sprintf("%s = __octVecAddSV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinarySV:-":
				return fmt.Sprintf("%s = __octVecSubSV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinarySV:*":
				return fmt.Sprintf("%s = __octVecMulSV(%s, %s)", st.Target, args[0], args[1]), nil
			case "VecBinarySV:/":
				return fmt.Sprintf("%s = __octVecDivSV(%s, %s)", st.Target, args[0], args[1]), nil
			case "MatMulMM":
				return fmt.Sprintf("%s = __octMatMulMM(%s, %s)", st.Target, args[0], args[1]), nil
			case "MatBinaryMM:+", "MatBinaryMM:-", "MatBinaryMM:*", "MatBinaryMM:/":
				if len(st.ArgTypes) != 2 {
					return "", fmt.Errorf("matrix-matrix binary lowering requires argument types")
				}
				leftElem, leftOK := parseMatrixElemType(st.ArgTypes[0])
				rightElem, rightOK := parseMatrixElemType(st.ArgTypes[1])
				retElem, retOK := parseMatrixElemType(st.RetType)
				if !leftOK || !rightOK || !retOK {
					return "", fmt.Errorf("invalid matrix-matrix binary types %v -> %s", st.ArgTypes, st.RetType)
				}
				op := strings.TrimPrefix(st.Callee, "MatBinaryMM:")
				return fmt.Sprintf("%s = __octMatBinaryMM[%s, %s, %s](%s, %s, %q)", st.Target, goType(leftElem), goType(rightElem), goType(retElem), args[0], args[1], op), nil
			case "MatBinaryMS:+", "MatBinaryMS:-", "MatBinaryMS:*", "MatBinaryMS:/":
				if len(st.ArgTypes) != 2 {
					return "", fmt.Errorf("matrix-scalar binary lowering requires argument types")
				}
				leftElem, leftOK := parseMatrixElemType(st.ArgTypes[0])
				retElem, retOK := parseMatrixElemType(st.RetType)
				if !leftOK || !isNumericTypeString(st.ArgTypes[1]) || !retOK {
					return "", fmt.Errorf("invalid matrix-scalar binary types %v -> %s", st.ArgTypes, st.RetType)
				}
				op := strings.TrimPrefix(st.Callee, "MatBinaryMS:")
				return fmt.Sprintf("%s = __octMatBinaryMS[%s, %s, %s](%s, %s, %q)", st.Target, goType(leftElem), goType(st.ArgTypes[1]), goType(retElem), args[0], args[1], op), nil
			case "MatBinarySM:+", "MatBinarySM:-", "MatBinarySM:*", "MatBinarySM:/":
				if len(st.ArgTypes) != 2 {
					return "", fmt.Errorf("scalar-matrix binary lowering requires argument types")
				}
				rightElem, rightOK := parseMatrixElemType(st.ArgTypes[1])
				retElem, retOK := parseMatrixElemType(st.RetType)
				if !isNumericTypeString(st.ArgTypes[0]) || !rightOK || !retOK {
					return "", fmt.Errorf("invalid scalar-matrix binary types %v -> %s", st.ArgTypes, st.RetType)
				}
				op := strings.TrimPrefix(st.Callee, "MatBinarySM:")
				return fmt.Sprintf("%s = __octMatBinarySM[%s, %s, %s](%s, %s, %q)", st.Target, goType(st.ArgTypes[0]), goType(rightElem), goType(retElem), args[0], args[1], op), nil
			case "ArrayBinaryAS:+", "ArrayBinaryAS:-", "ArrayBinaryAS:*", "ArrayBinaryAS:/", "ArrayBinaryAS:==", "ArrayBinaryAS:!=", "ArrayBinaryAS:<", "ArrayBinaryAS:<=", "ArrayBinaryAS:>", "ArrayBinaryAS:>=":
				leftElem, leftOK := parseArrayElemType(st.ArgTypes[0])
				retElem, retOK := parseArrayElemType(st.RetType)
				if st.RetType == "Bool[]" {
					retElem, retOK = "Bool", true
				}
				if len(st.ArgTypes) != 2 || !leftOK || !isNumericTypeString(st.ArgTypes[1]) || !retOK {
					return "", fmt.Errorf("invalid array-scalar binary types %v -> %s", st.ArgTypes, st.RetType)
				}
				op := strings.TrimPrefix(st.Callee, "ArrayBinaryAS:")
				return fmt.Sprintf("%s = __octArrayBinaryAS[%s, %s, %s](%s, %s, %q)", st.Target, goType(leftElem), goType(st.ArgTypes[1]), goType(retElem), args[0], args[1], op), nil
			case "ArrayBinarySA:+", "ArrayBinarySA:-", "ArrayBinarySA:*", "ArrayBinarySA:/", "ArrayBinarySA:==", "ArrayBinarySA:!=", "ArrayBinarySA:<", "ArrayBinarySA:<=", "ArrayBinarySA:>", "ArrayBinarySA:>=":
				rightElem, rightOK := parseArrayElemType(st.ArgTypes[1])
				retElem, retOK := parseArrayElemType(st.RetType)
				if st.RetType == "Bool[]" {
					retElem, retOK = "Bool", true
				}
				if len(st.ArgTypes) != 2 || !isNumericTypeString(st.ArgTypes[0]) || !rightOK || !retOK {
					return "", fmt.Errorf("invalid scalar-array binary types %v -> %s", st.ArgTypes, st.RetType)
				}
				op := strings.TrimPrefix(st.Callee, "ArrayBinarySA:")
				return fmt.Sprintf("%s = __octArrayBinarySA[%s, %s, %s](%s, %s, %q)", st.Target, goType(st.ArgTypes[0]), goType(rightElem), goType(retElem), args[0], args[1], op), nil
			case "PrometheusMatMulMM":
				return fmt.Sprintf("%s = __octPrometheusMatMulMM(%s, %s)", st.Target, args[0], args[1]), nil
			case "Trace":
				return fmt.Sprintf("%s = __octTrace(%s)", st.Target, args[0]), nil
			case "Grad":
				if _, ok := parseMatrixElemType(st.RetType); ok {
					return fmt.Sprintf("%s = __octGrad(%s)", st.Target, args[0]), nil
				}
				return fmt.Sprintf("%s = __octGradScalar(%s)", st.Target, args[0]), nil
			case "Div":
				if _, ok := parseVectorElemType(st.RetType); ok {
					return fmt.Sprintf("%s = __octDiv(%s)", st.Target, args[0]), nil
				}
				return fmt.Sprintf("%s = __octDivVector(%s)", st.Target, args[0]), nil
			case "SymGrad":
				return fmt.Sprintf("%s = __octSymGrad(%s)", st.Target, args[0]), nil
			case "Vector.tabulate":
				elemType, ok := parseVectorElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Vector.tabulate return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() []%s { __length := int(%s); __v := make([]%s, __length); for __i := 0; __i < __length; __i++ { __v[__i] = %s(__i) }; return __v }()",
					st.Target, goElemType, args[0], goElemType, args[1]), nil
			case "Matrix.fill":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.fill return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __row := make([]%s, __cols); for __c := 0; __c < __cols; __c++ { __row[__c] = %s }; __m[__r] = __row }; return __m }()",
					st.Target, goElemType, args[0], args[1], goElemType, goElemType, args[2]), nil
			case "Matrix.zeros":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.zeros return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __m[__r] = make([]%s, __cols) }; return __m }()",
					st.Target, goElemType, args[0], args[1], goElemType, goElemType), nil
			case "Matrix.identity":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.identity return type %s", st.RetType)
				}
				one := "1"
				if strings.HasPrefix(elemType, "Float") {
					one = "1.0"
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __n := int(%s); __m := make([][]%s, __n); for __r := 0; __r < __n; __r++ { __row := make([]%s, __n); __row[__r] = %s; __m[__r] = __row }; return __m }()",
					st.Target, goElemType, args[0], goElemType, goElemType, one), nil
			case "Matrix.tabulate":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.tabulate return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __row := make([]%s, __cols); for __c := 0; __c < __cols; __c++ { __row[__c] = %s(__r, __c) }; __m[__r] = __row }; return __m }()",
					st.Target, goElemType, args[0], args[1], goElemType, goElemType, args[2]), nil
			case "Random.RngSeed":
				return fmt.Sprintf("%s = __octRandomRngSeed(%s)", st.Target, args[0]), nil
			case "Random.RandInt":
				return fmt.Sprintf("%s = __octRandomRandInt(%s, %s, %s)", st.Target, args[0], args[1], args[2]), nil
			case "Random.RandFloat01":
				return fmt.Sprintf("%s = __octRandomRandFloat01(%s)", st.Target, args[0]), nil
			case "Random.RandFloatRange":
				return fmt.Sprintf("%s = __octRandomRandFloatRange(%s, %s, %s)", st.Target, args[0], args[1], args[2]), nil
			case "Random.RandBernoulli":
				return fmt.Sprintf("%s = __octRandomRandBernoulli(%s, %s)", st.Target, args[0], args[1]), nil
			case "Random.RandNormal":
				return fmt.Sprintf("%s = __octRandomRandNormal(%s, %s, %s)", st.Target, args[0], args[1], args[2]), nil
			case "Random.CryptoRandBytes":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandBytes(%s); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Bytes"), args[0], goResultTypeName("Bytes"), goResultTypeName("Bytes")), nil
			case "Random.CryptoRandInt":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandInt(%s, %s); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Int"), args[0], args[1], goResultTypeName("Int"), goResultTypeName("Int")), nil
			case "Random.CryptoRandFloat01":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandFloat01(); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Float"), goResultTypeName("Float"), goResultTypeName("Float")), nil
			default:
				return "", fmt.Errorf("compiled mode does not yet support builtin %s", st.Callee)
			}
		}
		if st.FunctionValue {
			if st.Target == "_" && st.RetType == "Void" {
				return fmt.Sprintf("%s(%s)", st.Callee, strings.Join(args, ", ")), nil
			}
			return fmt.Sprintf("%s = %s(%s)", st.Target, st.Callee, strings.Join(args, ", ")), nil
		}
		if st.Target == "_" && st.RetType == "Void" {
			return fmt.Sprintf("fn_%s(%s)", strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(args, ", ")), nil
		}
		return fmt.Sprintf("%s = fn_%s(%s)", st.Target, strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(args, ", ")), nil
	case MIRDestructureCall:
		args, err := emitGoValues(st.Args)
		if err != nil {
			return "", err
		}
		if st.Builtin {
			switch st.Callee {
			case "TupleProbe":
				return fmt.Sprintf("%s, %s = 1, 2", st.Targets[0], st.Targets[1]), nil
			case "BoolIntProbe":
				return fmt.Sprintf("%s, %s = true, 7", st.Targets[0], st.Targets[1]), nil
			case "Random.RandInt":
				return "", fmt.Errorf("destructuring Random.RandInt is not supported")
			case "Random.RandFloat01":
				return "", fmt.Errorf("destructuring Random.RandFloat01 is not supported")
			case "Random.RandFloatRange":
				return "", fmt.Errorf("destructuring Random.RandFloatRange is not supported")
			case "Random.RandBernoulli":
				return "", fmt.Errorf("destructuring Random.RandBernoulli is not supported")
			case "Random.RandNormal":
				return "", fmt.Errorf("destructuring Random.RandNormal is not supported")
			default:
				return "", fmt.Errorf("compiled mode does not yet support builtin %s", st.Callee)
			}
		}
		return fmt.Sprintf("%s = fn_%s(%s)", strings.Join(st.Targets, ", "), strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(args, ", ")), nil
	case MIRBatchMap:
		workerName := "fn_" + strings.ReplaceAll(st.Worker, ".", "_")
		input, err := emitGoValue(st.Input)
		if err != nil {
			return "", err
		}
		captures, err := emitGoValues(st.Captures)
		if err != nil {
			return "", err
		}
		forwarderArgs := []string{"__item"}
		forwarderParams := []string{fmt.Sprintf("__item %s", goType(st.InputType))}
		for _, capture := range captures {
			forwarderArgs = append(forwarderArgs, capture)
		}
		workerExpr := workerName
		if len(st.Captures) > 0 {
			workerExpr = fmt.Sprintf("func(%s) %s { return %s(%s) }", strings.Join(forwarderParams, ", "), goResultTypeName(st.ResultType), workerName, strings.Join(forwarderArgs, ", "))
		}
		return fmt.Sprintf("%s = func() %s { __vals, __err, __isErr := __octBatchRun(%s, %s, func(r %s) bool { return r.IsErr }, func(r %s) string { return r.Err }, func(r %s) %s { return r.Value }, %t); if __isErr { return %s{Err: __err, IsErr: true} }; return %s{Value: __vals} }()",
			st.Target,
			goType(fallibleType(st.ResultType+"[]")),
			input,
			workerExpr,
			goResultTypeName(st.ResultType),
			goResultTypeName(st.ResultType),
			goResultTypeName(st.ResultType),
			goType(st.ResultType),
			!st.Nested,
			goResultTypeName(st.ResultType+"[]"),
			goResultTypeName(st.ResultType+"[]")), nil
	default:
		return "", fmt.Errorf("unsupported MIR stmt %T", s)
	}
}

func goTerminator(t MIRTerminator, labels map[string]int, pcName string) (string, error) {
	switch term := t.(type) {
	case MIRReturn:
		if term.Value == nil {
			return "return", nil
		}
		value, err := emitGoValue(term.Value)
		if err != nil {
			return "", err
		}
		return "return " + goReturnExpr(value), nil
	case MIRJump:
		return fmt.Sprintf("%s = %d; continue", pcName, labels[term.Target]), nil
	case MIRBranch:
		cond, err := emitGoValue(term.Cond)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("if %s { %s = %d } else { %s = %d }; continue", cond, pcName, labels[term.TrueTarget], pcName, labels[term.FalseTarget]), nil
	case MIRFail:
		value, err := emitGoValue(term.Value)
		if err != nil {
			return "", err
		}
		return "panic(" + value + ")", nil
	default:
		return "", fmt.Errorf("unsupported MIR terminator %T", t)
	}
}

func goType(t string) string {
	if signature, ok := parseCompiledFunctionType(t); ok {
		params := make([]string, 0, len(signature.Parameters))
		for _, param := range signature.Parameters {
			params = append(params, goType(param))
		}
		ret := goType(signature.ReturnType)
		if signature.Fallible {
			ret = goResultTypeName(signature.ReturnType)
		}
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	}
	if flowRet, ok := parseFlowInstanceType(t); ok {
		return "__octFlowInstance_" + goSafeName(flowRet)
	}
	if vectorElem, ok := parseVectorElemType(t); ok {
		return "[]" + goType(vectorElem)
	}
	if matrixElem, ok := parseMatrixElemType(t); ok {
		return "[][]" + goType(matrixElem)
	}
	switch t {
	case "Int":
		return "int"
	case "Float":
		return "float64"
	case "Complex":
		return "complex128"
	case "Bool":
		return "bool"
	case "Range":
		return "__octRange"
	case "String":
		return "string"
	case "Index":
		return "string"
	case "Bytes":
		return "[]byte"
	case "Error":
		return "string"
	case "Void":
		return ""
	}
	if strings.HasPrefix(t, "Float<") && strings.HasSuffix(t, ">") {
		return "float64"
	}
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return "int"
	}
	if isFallibleType(t) {
		return goResultTypeName(fallibleValueType(t))
	}
	if strings.HasSuffix(t, "[]") {
		return "[]" + goType(strings.TrimSuffix(t, "[]"))
	}
	if strings.Contains(t, ".") {
		return strings.ReplaceAll(t, ".", "_")
	}
	return t
}

func isTwoDimensionalArrayType(t string) bool {
	if !strings.HasSuffix(t, "[][]") {
		return false
	}
	return !strings.HasSuffix(strings.TrimSuffix(t, "[][]"), "[]")
}

func cloneCompiledValueExpr(expr string, typeName string) string {
	if compiledValueNeedsClone(typeName) {
		return "__octClone(" + expr + ")"
	}
	return expr
}

func parseMatrixElemTypeOK(t string) bool {
	_, ok := parseMatrixElemType(t)
	return ok
}

func parseVectorElemTypeOK(t string) bool {
	_, ok := parseVectorElemType(t)
	return ok
}

func goIdentList(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = goIdent(name)
	}
	return out
}

func goIdent(name string) string {
	if name == "_" || strings.HasPrefix(name, "_t") {
		return name
	}
	if goKeywords[name] {
		return "oct_" + name
	}
	return name
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}

func goFlowResultType(t string) string {
	if t == "Void" {
		return "__octVoid"
	}
	return goType(t)
}

func goResultTypeName(valueType string) string {
	return "octResult_" + goSafeName(valueType)
}

func goSafeName(valueType string) string {
	s := strings.NewReplacer("[]", "Slice", ".", "_", "[", "_", "]", "", ",", "_", " ", "", "*", "_ptr_", "<", "_", ">", "", "^", "_pow_", "+", "_plus_", "-", "_minus_", "/", "_per_").Replace(valueType)
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

func goReturnExpr(expr string) string {
	if strings.HasPrefix(expr, "__oct_ok(") {
		payload := strings.TrimSuffix(strings.TrimPrefix(expr, "__oct_ok("), ")")
		parts := strings.SplitN(payload, ",", 2)
		if len(parts) != 2 {
			return expr
		}
		retType := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			return fmt.Sprintf("%s{}", goResultTypeName(retType))
		}
		return fmt.Sprintf("%s{Value: %s}", goResultTypeName(retType), value)
	}
	if strings.HasPrefix(expr, "__oct_err(") {
		payload := strings.TrimSuffix(strings.TrimPrefix(expr, "__oct_err("), ")")
		parts := strings.SplitN(payload, ",", 2)
		if len(parts) != 2 {
			return expr
		}
		retType := strings.TrimSpace(parts[0])
		errExpr := strings.TrimSpace(parts[1])
		return fmt.Sprintf("%s{Err: %s, IsErr: true}", goResultTypeName(retType), errExpr)
	}
	return expr
}
