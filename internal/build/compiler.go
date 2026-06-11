package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type Result struct {
	ArtifactPath string
	MIRDumpPath  string
}

type MIRModule struct {
	EntryPackage  string
	EntryFunc     string
	EntryReturn   string
	EntryFallible bool
	Records       []MIRRecord
	Enums         []MIREnum
	Flows         []MIRFlow
	Functions     []MIRFunction
}

type MIRRecord struct {
	Package string
	Name    string
	Fields  []MIRField
}

type MIREnum struct {
	Package  string
	Name     string
	Variants []MIREnumVariant
}

type MIREnumVariant struct {
	Name        string
	PayloadType string
}

type MIRFlow struct {
	Package    string
	Name       string
	Parameters []MIRField
	Board      []MIRField
	Return     string
	EntryState string
	States     []MIRFlowState
}

type MIRFlowState struct {
	Name       string
	Statements []MIRFlowStmt
}

type MIRFlowStmt interface{ mirFlowStmt() }

type MIRFlowGoto struct{ Target string }

func (MIRFlowGoto) mirFlowStmt() {}

type MIRFlowSuspend struct{}

func (MIRFlowSuspend) mirFlowStmt() {}

type MIRFlowRemember struct{}

func (MIRFlowRemember) mirFlowStmt() {}

type MIRFlowResume struct{}

func (MIRFlowResume) mirFlowStmt() {}

type MIRFlowFieldAssign struct {
	Target string
	Field  string
	Value  MIRFlowExpr
}

func (MIRFlowFieldAssign) mirFlowStmt() {}

type MIRFlowLetStmt struct {
	Name  string
	Type  string
	Value MIRFlowExpr
}

func (MIRFlowLetStmt) mirFlowStmt() {}

type MIRFlowReturn struct {
	Value MIRFlowExpr
}

func (MIRFlowReturn) mirFlowStmt() {}

type MIRFlowIf struct {
	Condition MIRFlowExpr
	Then      []MIRFlowStmt
	Else      []MIRFlowStmt
}

func (MIRFlowIf) mirFlowStmt() {}

type MIRFlowWhen struct {
	Cases []MIRFlowWhenCase
	Else  MIRFlowWhenAction
}

func (MIRFlowWhen) mirFlowStmt() {}

type MIRFlowWhenCase struct {
	Condition MIRFlowExpr
	Action    MIRFlowWhenAction
}

type MIRFlowWhenAction interface{ mirFlowWhenAction() }

type MIRFlowWhenGoto struct{ Target string }

func (MIRFlowWhenGoto) mirFlowWhenAction() {}

type MIRFlowWhenSuspend struct{}

func (MIRFlowWhenSuspend) mirFlowWhenAction() {}

type MIRFlowWhenReturn struct {
	Value MIRFlowExpr
}

func (MIRFlowWhenReturn) mirFlowWhenAction() {}

type MIRFlowWhenBlock struct {
	Statements []MIRFlowStmt
}

func (MIRFlowWhenBlock) mirFlowWhenAction() {}

type MIRFlowExpr interface{ mirFlowExpr() }

type MIRFlowLiteralExpr struct {
	Value string
}

func (MIRFlowLiteralExpr) mirFlowExpr() {}

type MIRFlowIdentifierExpr struct {
	Name    string
	IsLocal bool
}

func (MIRFlowIdentifierExpr) mirFlowExpr() {}

type MIRFlowFieldExpr struct {
	Target string
	Field  string
}

func (MIRFlowFieldExpr) mirFlowExpr() {}

type MIRFlowBinaryExpr struct {
	Left     MIRFlowExpr
	Operator string
	Right    MIRFlowExpr
}

func (MIRFlowBinaryExpr) mirFlowExpr() {}

type MIRFlowUnaryExpr struct {
	Operator string
	Operand  MIRFlowExpr
}

func (MIRFlowUnaryExpr) mirFlowExpr() {}

type MIRFlowCallExpr struct {
	Callee   string
	Args     []MIRFlowExpr
	Builtin  bool
	RetType  string
	Fallible bool
}

func (MIRFlowCallExpr) mirFlowExpr() {}

type MIRFlowIndexExpr struct {
	Target     MIRFlowExpr
	Index      MIRFlowExpr
	ResultType string
}

func (MIRFlowIndexExpr) mirFlowExpr() {}

type MIRFlowRecordLiteralExpr struct {
	TypeName   string
	FieldNames []string
	FieldVals  []MIRFlowExpr
	RetType    string
}

func (MIRFlowRecordLiteralExpr) mirFlowExpr() {}

type MIRFlowUtilityWhenExpr struct {
	SiteID          int
	ControllerBound bool
	ResultType      string
	Hysteresis      MIRFlowExpr
	MinCommit       MIRFlowExpr
	Cases           []MIRFlowUtilityCase
	Else            MIRFlowExpr
}

func (MIRFlowUtilityWhenExpr) mirFlowExpr() {}

type MIRFlowUtilityCase struct {
	Value     MIRFlowExpr
	Condition MIRFlowExpr
	Score     MIRFlowExpr
}

type MIRFlowSwitchExpr struct {
	Subject    MIRFlowExpr
	Cases      []MIRFlowSwitchCase
	Else       MIRFlowExpr
	ResultType string
}

func (MIRFlowSwitchExpr) mirFlowExpr() {}

type MIRFlowSwitchCase struct {
	Match MIRFlowExpr
	Value MIRFlowExpr
}

type MIRFlowMatchExpr struct {
	Subject     MIRFlowExpr
	SubjectType string
	Cases       []MIRFlowMatchCase
	ResultType  string
}

func (MIRFlowMatchExpr) mirFlowExpr() {}

type MIRFlowMatchCase struct {
	Variant     string
	Binding     string
	BindingType string
	Value       MIRFlowExpr
}

type MIRFlowIfExpr struct {
	Condition  MIRFlowExpr
	Then       MIRFlowExpr
	Else       MIRFlowExpr
	ResultType string
}

func (MIRFlowIfExpr) mirFlowExpr() {}

type MIRField struct {
	Name string
	Type string
}

type MIRFunction struct {
	Package         string
	Name            string
	Params          []MIRField
	Return          string
	IsFallible      bool
	ErrorType       string
	Locals          []MIRField
	Blocks          []MIRBlock
	UsesUtilityWhen bool
}

type MIRBlock struct {
	Label      string
	Statements []MIRStmt
	Terminator MIRTerminator
}

type MIRStmt interface{ mirStmt() }

type MIRAssign struct {
	Target string
	Value  string
}

func (MIRAssign) mirStmt() {}

type MIRCall struct {
	Target        string
	Callee        string
	Args          []string
	ArgTypes      []string
	Builtin       bool
	RetType       string
	FunctionValue bool
}

func (MIRCall) mirStmt() {}

type MIRGenericOctxiliaryCall struct {
	Target         string
	PackageName    string
	OctName        string
	Family         string
	WireName       string
	SidecarCommand string
	Args           []string
	ArgTypes       []string
	RetType        string
	Fallible       bool
	TransportTypes []project.TransportTypeMetadata
}

func (MIRGenericOctxiliaryCall) mirStmt() {}

type MIRDestructureCall struct {
	Targets  []string
	Callee   string
	Args     []string
	Builtin  bool
	RetTypes []string
}

func (MIRDestructureCall) mirStmt() {}

type MIRConstructRecord struct {
	Target     string
	TypeName   string
	FieldNames []string
	FieldVals  []string
}

func (MIRConstructRecord) mirStmt() {}

type MIRConstructArray struct {
	Target   string
	ElemType string
	Values   []string
}

func (MIRConstructArray) mirStmt() {}

type MIRBatchMap struct {
	Target     string
	Input      string
	Worker     string
	InputType  string
	ResultType string
	Captures   []string
}

func (MIRBatchMap) mirStmt() {}

type MIRTerminator interface{ mirTerminator() }

type MIRReturn struct{ Value string }

func (MIRReturn) mirTerminator() {}

type MIRJump struct{ Target string }

func (MIRJump) mirTerminator() {}

type MIRBranch struct{ Cond, TrueTarget, FalseTarget string }

func (MIRBranch) mirTerminator() {}

type MIRFail struct{ Value string }

func (MIRFail) mirTerminator() {}

func Compile(path string) (Result, error) {
	program, err := project.Load(path)
	if err != nil {
		return Result{}, err
	}
	return compileProgram(program, compileOptions{})
}

func CompileForTest(path string) (Result, error) {
	program, err := project.LoadForTest(path)
	if err != nil {
		return Result{}, err
	}
	return compileProgram(program, compileOptions{selectedReachableOnly: true})
}

type compileOptions struct {
	selectedReachableOnly bool
}

func compileProgram(program project.Program, options compileOptions) (Result, error) {
	if err := typecheck.CheckProgram(program); err != nil {
		return Result{}, err
	}

	module, err := lowerProgram(program, options)
	if err != nil {
		return Result{}, err
	}

	goSrc, err := emitGo(module)
	if err != nil {
		return Result{}, err
	}

	artifactPath := artifactPathFor(program.EntrySource)
	genDir, err := generatedBuildDir()
	if err != nil {
		return Result{}, err
	}
	if os.Getenv("OCT_KEEP_GEN") == "" {
		defer os.RemoveAll(genDir)
	}
	genPath := filepath.Join(genDir, filepath.Base(artifactPath)+".gen.go")
	if err := os.WriteFile(genPath, []byte(goSrc), 0o644); err != nil {
		return Result{}, fmt.Errorf("write generated go %s: %w", genPath, err)
	}

	cmd := exec.Command("go", "build", "-o", artifactPath, genPath)
	moduleRoot, err := compilerModuleRoot()
	if err != nil {
		return Result{}, err
	}
	cmd.Dir = moduleRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("go build generated program: %v: %s", err, strings.TrimSpace(stderr.String()))
	}

	res := Result{ArtifactPath: artifactPath}
	if os.Getenv("OCT_MIR_DUMP") != "" {
		dumpPath := artifactPath + ".mir"
		if err := os.WriteFile(dumpPath, []byte(dumpMIR(module)), 0o644); err != nil {
			return Result{}, fmt.Errorf("write MIR dump %s: %w", dumpPath, err)
		}
		res.MIRDumpPath = dumpPath
	}
	return res, nil
}

func artifactPathFor(path string) string {
	return filepath.Join(filepath.Dir(path), filepath.Base(path)+".octbin")
}

func generatedBuildDir() (string, error) {
	root, err := compilerModuleRoot()
	if err != nil {
		return "", err
	}
	parent := filepath.Join(root, ".octbuild")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create generated build directory %s: %w", parent, err)
	}
	dir, err := os.MkdirTemp(parent, "gen-")
	if err != nil {
		return "", fmt.Errorf("create generated build temp dir: %w", err)
	}
	return dir, nil
}

func compilerModuleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate compiler module root: runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

type einsteinTermMeta struct {
	Labels []string
	Rank   int
	Type   string
}

type lowerCtx struct {
	expectedTypeStack []string
	pkg               project.Package
	program           project.Program
	locals            map[string]string
	goNames           map[string]string
	blocks            []MIRBlock
	cur               int
	tempID            int
	batchID           int
	retType           string
	fn                ast.FunctionDecl
	extra             []MIRFunction
	lastRet           string
	usesUtilityWhen   bool
	inPrometheus      bool
	einTerms          map[string]einsteinTermMeta
}

func lowerProgram(program project.Program, options compileOptions) (MIRModule, error) {
	module := MIRModule{EntryPackage: program.Entry}
	reachable := map[string]map[string]struct{}{}
	if options.selectedReachableOnly {
		reachable = collectReachableFunctions(program)
	}
	emitted := map[string]map[string]struct{}{}
	pending := [][2]string{}
	enqueueReachable := func(pkgName string, fnName string) {
		if pkgName == "" || fnName == "" {
			return
		}
		if _, ok := emitted[pkgName]; ok {
			if _, done := emitted[pkgName][fnName]; done {
				return
			}
		}
		if _, ok := reachable[pkgName]; !ok {
			reachable[pkgName] = map[string]struct{}{}
		}
		if _, ok := reachable[pkgName][fnName]; ok {
			return
		}
		reachable[pkgName][fnName] = struct{}{}
		pending = append(pending, [2]string{pkgName, fnName})
	}
	enqueueLoweredCalls := func(lowered []MIRFunction, defaultPkg string) {
		for _, mf := range lowered {
			for _, block := range mf.Blocks {
				for _, stmt := range block.Statements {
					call, ok := stmt.(MIRCall)
					if !ok || call.Builtin || call.FunctionValue {
						continue
					}
					targetPkg := defaultPkg
					targetFn := call.Callee
					if dot := strings.Index(call.Callee, "."); dot >= 0 {
						targetPkg = call.Callee[:dot]
						targetFn = call.Callee[dot+1:]
					}
					enqueueReachable(targetPkg, targetFn)
				}
			}
		}
	}
	for pkgName, fns := range reachable {
		for fnName := range fns {
			pending = append(pending, [2]string{pkgName, fnName})
		}
	}
	pkgNames := make([]string, 0, len(program.Packages))
	for name := range program.Packages {
		pkgNames = append(pkgNames, name)
	}
	sort.Strings(pkgNames)
	for _, pkgName := range pkgNames {
		pkg := program.Packages[pkgName]
		if pkgName == program.Entry {
			for _, fn := range pkg.Functions {
				if fn.Name == "main" {
					module.EntryFunc = "main"
					break
				}
			}
			if module.EntryFunc == "" {
				for _, fn := range pkg.Functions {
					if fn.Name == "Main" {
						module.EntryFunc = "Main"
						break
					}
				}
			}
		}
		for _, r := range pkg.Records {
			mr := MIRRecord{Package: pkgName, Name: r.Name}
			for _, f := range r.Fields {
				mr.Fields = append(mr.Fields, MIRField{Name: f.Name, Type: typeRefStringForPackage(pkgName, f.Type)})
			}
			module.Records = append(module.Records, mr)
		}
		for _, e := range pkg.Enums {
			variants := make([]MIREnumVariant, 0, len(e.Variants))
			for _, variant := range e.Variants {
				payloadType := ""
				if variant.Payload != nil {
					payloadType = typeRefStringForPackage(pkgName, *variant.Payload)
				}
				variants = append(variants, MIREnumVariant{Name: variant.Name, PayloadType: payloadType})
			}
			module.Enums = append(module.Enums, MIREnum{Package: pkgName, Name: e.Name, Variants: variants})
		}
		for _, flow := range pkg.Flows {
			if len(flow.Board) > 0 {
				snapshot := MIRRecord{Package: pkgName, Name: flow.Name + "BoardSnapshot"}
				for _, field := range flow.Board {
					snapshot.Fields = append(snapshot.Fields, MIRField{
						Name: field.Name,
						Type: typeRefStringForPackage(pkgName, field.Type),
					})
				}
				module.Records = append(module.Records, snapshot)
			}
			mirFlow, err := lowerFlow(program, pkgName, flow, pkg)
			if err != nil {
				return MIRModule{}, fmt.Errorf("flow %s.%s: %w", pkgName, flow.Name, err)
			}
			module.Flows = append(module.Flows, mirFlow)
		}
		for _, fn := range pkg.Functions {
			if options.selectedReachableOnly && !isReachableFunction(reachable, pkgName, fn.Name) {
				continue
			}
			if fn.IsArtifact {
				continue
			}
			if _, isWrapper := findGenericWrapperFunction(pkg, fn.Name); isWrapper {
				continue
			}
			if fn.IsTestFile && !fn.IsBenchmark {
				if pkgName != program.Entry {
					continue
				}
				if !fn.IsFact && !fn.IsTheory && !options.selectedReachableOnly {
					continue
				}
			}
			if fn.IsTheory || fn.IsFact {
				// compiled octest runner can target these directly
			}
			if fn.IsTestFile && !fn.IsBenchmark && !fn.IsFact && !fn.IsTheory && !options.selectedReachableOnly {
				continue
			}
			lowered, err := lowerFunction(program, pkg, fn)
			if err != nil {
				return MIRModule{}, err
			}
			if options.selectedReachableOnly {
				enqueueLoweredCalls(lowered, pkgName)
			}
			if _, ok := emitted[pkgName]; !ok {
				emitted[pkgName] = map[string]struct{}{}
			}
			emitted[pkgName][fn.Name] = struct{}{}
			module.Functions = append(module.Functions, lowered...)
		}
	}
	if options.selectedReachableOnly {
		indexed := map[string]map[string]ast.FunctionDecl{}
		for pkgName, pkg := range program.Packages {
			indexed[pkgName] = map[string]ast.FunctionDecl{}
			for _, fn := range pkg.Functions {
				indexed[pkgName][fn.Name] = fn
			}
		}
		for len(pending) > 0 {
			item := pending[0]
			pending = pending[1:]
			pkgFns := indexed[item[0]]
			fn, ok := pkgFns[item[1]]
			if !ok {
				continue
			}
			for _, call := range collectFunctionDeclCalls(fn) {
				targetPkg := call[0]
				if targetPkg == "" {
					targetPkg = item[0]
				}
				if pkg, ok := program.Packages[targetPkg]; ok {
					if _, isWrapper := findGenericWrapperFunction(pkg, call[1]); isWrapper {
						continue
					}
				}
				builtinName := targetPkg + "." + call[1]
				if builtin.IsName(builtinName) || builtin.IsName(call[1]) {
					continue
				}
				enqueueReachable(targetPkg, call[1])
			}
		}
		for pkgName, pkg := range program.Packages {
			for _, fn := range pkg.Functions {
				if !isReachableFunction(reachable, pkgName, fn.Name) {
					continue
				}
				if _, isWrapper := findGenericWrapperFunction(pkg, fn.Name); isWrapper {
					continue
				}
				if _, ok := emitted[pkgName]; ok {
					if _, done := emitted[pkgName][fn.Name]; done {
						continue
					}
				}
				lowered, err := lowerFunction(program, pkg, fn)
				if err != nil {
					return MIRModule{}, err
				}
				enqueueLoweredCalls(lowered, pkgName)
				if _, ok := emitted[pkgName]; !ok {
					emitted[pkgName] = map[string]struct{}{}
				}
				emitted[pkgName][fn.Name] = struct{}{}
				module.Functions = append(module.Functions, lowered...)
			}
		}
		if err := validateUserCallSymbols(module); err != nil {
			return MIRModule{}, err
		}
	}
	if module.EntryFunc == "" {
		return MIRModule{}, fmt.Errorf("entry package '%s' is missing main/Main function", program.Entry)
	}
	module.EntryReturn, module.EntryFallible = lookupEntryFunctionShape(program, module.EntryPackage, module.EntryFunc)
	return module, nil
}

func lookupEntryFunctionShape(program project.Program, pkgName string, fnName string) (string, bool) {
	if pkg, ok := program.Packages[pkgName]; ok {
		for _, fn := range pkg.Functions {
			if fn.Name == fnName {
				return typeRefStringForPackage(pkgName, fn.ReturnType), fn.IsFallible
			}
		}
	}
	return "", false
}

func validateUserCallSymbols(module MIRModule) error {
	defs := map[string]struct{}{}
	for _, fn := range module.Functions {
		defs[fn.Package+"."+fn.Name] = struct{}{}
	}
	for _, flow := range module.Flows {
		// Flow constructor calls are represented as user calls in MIR and are
		// emitted from MIR flows during Go generation.
		defs[flow.Package+"."+flow.Name] = struct{}{}
	}
	missing := map[string]struct{}{}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, stmt := range block.Statements {
				call, ok := stmt.(MIRCall)
				if !ok || call.Builtin || call.FunctionValue {
					continue
				}
				if _, ok := defs[call.Callee]; !ok {
					missing[call.Callee] = struct{}{}
				}
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Errorf("compiled selected reachable invariant failed: missing emitted function definitions for %s", strings.Join(names, ", "))
}

func isReachableFunction(reachable map[string]map[string]struct{}, pkgName string, fnName string) bool {
	pkgFns, ok := reachable[pkgName]
	if !ok {
		return false
	}
	_, ok = pkgFns[fnName]
	return ok
}

func collectReachableFunctions(program project.Program) map[string]map[string]struct{} {
	reachable := map[string]map[string]struct{}{}
	functions := map[string]map[string]ast.FunctionDecl{}
	flows := map[string]map[string]ast.FlowDecl{}
	for pkgName, pkg := range program.Packages {
		functions[pkgName] = map[string]ast.FunctionDecl{}
		for _, fn := range pkg.Functions {
			functions[pkgName][fn.Name] = fn
		}
		flows[pkgName] = map[string]ast.FlowDecl{}
		for _, flow := range pkg.Flows {
			flows[pkgName][flow.Name] = flow
		}
	}
	queue := [][2]string{{program.Entry, "main"}, {program.Entry, "Main"}}
	seen := map[string]struct{}{}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		key := item[0] + "." + item[1]
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fn, isFn := functions[item[0]][item[1]]
		flow, isFlow := flows[item[0]][item[1]]
		if !isFn && !isFlow {
			continue
		}
		if _, ok := reachable[item[0]]; !ok {
			reachable[item[0]] = map[string]struct{}{}
		}
		reachable[item[0]][item[1]] = struct{}{}
		calls := make([][2]string, 0)
		if isFn {
			calls = append(calls, collectFunctionDeclCalls(fn)...)
		}
		if isFlow {
			for _, state := range flow.States {
				calls = append(calls, collectFunctionCalls(state.Body)...)
			}
		}
		for _, call := range calls {
			targetPkg := call[0]
			if targetPkg == "" {
				targetPkg = item[0]
			}
			if targetPkg == "" || call[1] == "" {
				continue
			}
			if pkg, ok := program.Packages[targetPkg]; ok {
				if _, isWrapper := findGenericWrapperFunction(pkg, call[1]); isWrapper {
					continue
				}
			}
			builtinName := call[1]
			if targetPkg != "" {
				builtinName = targetPkg + "." + call[1]
			}
			if builtin.IsName(builtinName) {
				continue
			}
			if targetPkg == "Random" {
				switch call[1] {
				case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
					continue
				}
			}
			queue = append(queue, [2]string{targetPkg, call[1]})
		}
	}
	return reachable
}

func collectFunctionCalls(block ast.Block) [][2]string {
	return collectFunctionCallsWithLocals(block, map[string]struct{}{})
}

func collectFunctionDeclCalls(fn ast.FunctionDecl) [][2]string {
	functionValueLocals := map[string]struct{}{}
	for _, param := range fn.Parameters {
		if param.Type.Function != nil {
			functionValueLocals[param.Name] = struct{}{}
		}
	}
	return collectFunctionCallsWithLocals(fn.Body, functionValueLocals)
}

func collectFunctionCallsWithLocals(block ast.Block, functionValueLocals map[string]struct{}) [][2]string {
	calls := make([][2]string, 0)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case ast.LetStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.VarStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.AssignStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.DestructureAssignStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.IndexAssignStmt:
			for _, idx := range s.Indices {
				calls = append(calls, collectExprCallsWithLocals(idx, functionValueLocals)...)
			}
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.FieldAssignStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.ReturnStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.ExprStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Value, functionValueLocals)...)
		case ast.ForStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Range, functionValueLocals)...)
			calls = append(calls, collectFunctionCallsWithLocals(s.Body, functionValueLocals)...)
		case ast.MatchStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Subject, functionValueLocals)...)
			calls = append(calls, collectFunctionCallsWithLocals(s.OkBody, functionValueLocals)...)
			calls = append(calls, collectFunctionCallsWithLocals(s.ErrBody, functionValueLocals)...)
		case ast.IfStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Condition, functionValueLocals)...)
			calls = append(calls, collectFunctionCallsWithLocals(s.ThenBody, functionValueLocals)...)
			if s.ElseBody != nil {
				calls = append(calls, collectFunctionCallsWithLocals(*s.ElseBody, functionValueLocals)...)
			}
		case ast.WhileStmt:
			calls = append(calls, collectExprCallsWithLocals(s.Condition, functionValueLocals)...)
			calls = append(calls, collectFunctionCallsWithLocals(s.Body, functionValueLocals)...)
		case ast.PrometheusStmt:
			calls = append(calls, collectFunctionCallsWithLocals(s.Body, functionValueLocals)...)
		case ast.WhenStmt:
			for _, c := range s.Cases {
				calls = append(calls, collectExprCallsWithLocals(c.Condition, functionValueLocals)...)
				switch a := c.Action.(type) {
				case ast.WhenReturnAction:
					calls = append(calls, collectExprCallsWithLocals(a.Value, functionValueLocals)...)
				case ast.WhenBlockAction:
					calls = append(calls, collectFunctionCallsWithLocals(ast.Block{Statements: a.Statements}, functionValueLocals)...)
				}
			}
			switch a := s.Else.(type) {
			case ast.WhenReturnAction:
				calls = append(calls, collectExprCallsWithLocals(a.Value, functionValueLocals)...)
			case ast.WhenBlockAction:
				calls = append(calls, collectFunctionCallsWithLocals(ast.Block{Statements: a.Statements}, functionValueLocals)...)
			}
		}
	}
	return calls
}

func collectExprCalls(expr ast.Expr) [][2]string {
	return collectExprCallsWithLocals(expr, map[string]struct{}{})
}

func collectExprCallsWithLocals(expr ast.Expr, functionValueLocals map[string]struct{}) [][2]string {
	calls := make([][2]string, 0)
	switch e := expr.(type) {
	case ast.CallExpr:
		if call, ok := resolveStaticCallTarget(e.Callee); ok {
			if _, isFunctionValueLocal := functionValueLocals[call[1]]; !isFunctionValueLocal || call[0] != "" {
				calls = append(calls, call)
			}
		}
		for _, arg := range e.Arguments {
			calls = append(calls, collectExprCallsWithLocals(arg, functionValueLocals)...)
		}
	case ast.IdentifierExpr:
		if _, isFunctionValueLocal := functionValueLocals[e.Name]; !isFunctionValueLocal {
			calls = append(calls, [2]string{"", e.Name})
		}
	case ast.FieldAccessExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Target, functionValueLocals)...)
	case ast.IndexExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Target, functionValueLocals)...)
		for _, idx := range e.Indices {
			calls = append(calls, collectExprCalls(idx)...)
		}
	case ast.BinaryExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Left, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.Right, functionValueLocals)...)
	case ast.UnaryExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Operand, functionValueLocals)...)
	case ast.RangeExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Start, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.End, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.Step, functionValueLocals)...)
	case ast.ParenExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Inner, functionValueLocals)...)
	case ast.PropagateExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Inner, functionValueLocals)...)
	case ast.UnwrapExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Inner, functionValueLocals)...)
	case ast.ArrayLiteralExpr:
		for _, v := range e.Elements {
			calls = append(calls, collectExprCallsWithLocals(v, functionValueLocals)...)
		}
	case ast.VectorLiteralExpr:
		for _, v := range e.Elements {
			calls = append(calls, collectExprCallsWithLocals(v, functionValueLocals)...)
		}
	case ast.MatrixLiteralExpr:
		for _, row := range e.Rows {
			for _, cell := range row {
				calls = append(calls, collectExprCallsWithLocals(cell, functionValueLocals)...)
			}
		}
	case ast.SwitchExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Subject, functionValueLocals)...)
		for _, c := range e.Cases {
			calls = append(calls, collectExprCallsWithLocals(c.Match, functionValueLocals)...)
			calls = append(calls, collectExprCallsWithLocals(c.Value, functionValueLocals)...)
		}
		calls = append(calls, collectExprCallsWithLocals(e.Else, functionValueLocals)...)
	case ast.MatchExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Subject, functionValueLocals)...)
		for _, c := range e.Cases {
			calls = append(calls, collectExprCallsWithLocals(c.Value, functionValueLocals)...)
		}
	case ast.IfExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Condition, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.ThenExpr, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.ElseExpr, functionValueLocals)...)
	case ast.UtilityWhenExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Policy.Hysteresis, functionValueLocals)...)
		calls = append(calls, collectExprCallsWithLocals(e.Policy.MinCommit, functionValueLocals)...)
		for _, c := range e.Cases {
			calls = append(calls, collectExprCallsWithLocals(c.Value, functionValueLocals)...)
			calls = append(calls, collectExprCallsWithLocals(c.Condition, functionValueLocals)...)
			calls = append(calls, collectExprCallsWithLocals(c.Score, functionValueLocals)...)
		}
		calls = append(calls, collectExprCallsWithLocals(e.Else, functionValueLocals)...)
	case ast.BatchExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Input, functionValueLocals)...)
		calls = append(calls, collectFunctionCalls(e.Body)...)
	}
	return calls
}

func resolveStaticCallTarget(callee ast.Expr) ([2]string, bool) {
	switch c := callee.(type) {
	case ast.IdentifierExpr:
		return [2]string{"", c.Name}, true
	case ast.FieldAccessExpr:
		if pkg, ok := c.Target.(ast.IdentifierExpr); ok {
			return [2]string{pkg.Name, c.Field}, true
		}
	}
	return [2]string{}, false
}

func lowerFunction(program project.Program, pkg project.Package, fn ast.FunctionDecl) ([]MIRFunction, error) {
	ctx := &lowerCtx{pkg: pkg, program: program, locals: map[string]string{}, goNames: map[string]string{}, retType: typeRefStringForPackage(pkg.Name, fn.ReturnType), fn: fn, einTerms: map[string]einsteinTermMeta{}}
	mirFn := MIRFunction{Package: pkg.Name, Name: fn.Name, Return: ctx.retType, IsFallible: fn.IsFallible, ErrorType: typeRefStringForPackage(pkg.Name, fn.ErrorType)}
	for _, p := range fn.Parameters {
		t := typeRefStringForPackage(pkg.Name, p.Type)
		mirFn.Params = append(mirFn.Params, MIRField{Name: goIdent(p.Name), Type: t})
		ctx.locals[p.Name] = t
		ctx.goNames[p.Name] = goIdent(p.Name)
	}
	ctx.blocks = append(ctx.blocks, MIRBlock{Label: "entry"})
	ctx.cur = 0
	if err := ctx.lowerBlock(fn.Body); err != nil {
		return nil, fmt.Errorf("function %s.%s: %w", pkg.Name, fn.Name, err)
	}
	if ctx.blocks[ctx.cur].Terminator == nil {
		if mirFn.Return == "Void" {
			if mirFn.IsFallible {
				ctx.blocks[ctx.cur].Terminator = MIRReturn{Value: fallibleOkValue(ctx.retType, "")}
			} else {
				ctx.blocks[ctx.cur].Terminator = MIRReturn{Value: ""}
			}
		} else {
			return nil, fmt.Errorf("missing return")
		}
	}
	mirFn.UsesUtilityWhen = ctx.usesUtilityWhen
	mirFn.Blocks = ctx.blocks
	for n, t := range ctx.locals {
		isParam := false
		for _, p := range mirFn.Params {
			if p.Name == goIdent(n) {
				isParam = true
				break
			}
		}
		if !isParam {
			mirFn.Locals = append(mirFn.Locals, MIRField{Name: ctx.goLocalName(n), Type: t})
		}
	}
	sort.Slice(mirFn.Locals, func(i, j int) bool { return mirFn.Locals[i].Name < mirFn.Locals[j].Name })
	out := []MIRFunction{mirFn}
	out = append(out, ctx.extra...)
	return out, nil
}

func (c *lowerCtx) lowerBlock(block ast.Block) error {
	for _, stmt := range block.Statements {
		if c.blocks[c.cur].Terminator != nil {
			return nil
		}
		switch s := stmt.(type) {
		case ast.LetStmt:
			var v, t string
			var err error
			if s.TypeHint != nil {
				hint := typeRefStringForPackage(c.pkg.Name, *s.TypeHint)
				v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(s.Value) })
			} else {
				v, t, _, err = c.lowerExpr(s.Value)
			}
			if err != nil {
				return err
			}
			if s.TypeHint != nil {
				hint := typeRefStringForPackage(c.pkg.Name, *s.TypeHint)
				v = coerceExprToType(v, t, hint)
				t = hint
			}
			c.declareLocal(s.Name, t)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: v})
		case ast.VarStmt:
			var v, t string
			var err error
			if s.TypeHint != nil {
				hint := typeRefStringForPackage(c.pkg.Name, *s.TypeHint)
				v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(s.Value) })
			} else {
				v, t, _, err = c.lowerExpr(s.Value)
			}
			if err != nil {
				return err
			}
			if s.TypeHint != nil {
				hint := typeRefStringForPackage(c.pkg.Name, *s.TypeHint)
				v = coerceExprToType(v, t, hint)
				t = hint
			}
			c.declareLocal(s.Name, t)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: v})
		case ast.AssignStmt:
			targetType, ok := c.locals[s.Name]
			if !ok {
				return fmt.Errorf("assignment to unknown local '%s'", s.Name)
			}
			v, t, _, err := c.withExpectedType(targetType, func() (string, string, bool, error) { return c.lowerExpr(s.Value) })
			if err != nil {
				return err
			}
			v = coerceExprToType(v, t, targetType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: v})
		case ast.DestructureAssignStmt:
			call, ok := s.Value.(ast.CallExpr)
			if !ok {
				return fmt.Errorf("compiled mode destructuring currently requires a call RHS")
			}
			callee, ret, builtin, _, err := c.resolveCall(call.Callee)
			if err != nil {
				return err
			}
			retTypes, ok := parseTupleTypeString(ret)
			if !ok {
				return fmt.Errorf("compiled mode destructuring requires tuple return, got %s", ret)
			}
			if len(retTypes) != len(s.Names) {
				return fmt.Errorf("compiled mode destructuring expected %d targets, got %d", len(retTypes), len(s.Names))
			}
			for i, name := range s.Names {
				t, ok := c.locals[name]
				if !ok {
					return fmt.Errorf("assignment to unknown local '%s'", name)
				}
				if t != retTypes[i] {
					return fmt.Errorf("compiled mode destructuring target '%s' expects %s, got %s", name, t, retTypes[i])
				}
			}
			args := make([]string, 0, len(call.Arguments))
			for _, a := range call.Arguments {
				v, _, _, err := c.lowerExpr(a)
				if err != nil {
					return err
				}
				args = append(args, v)
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRDestructureCall{
				Targets:  goIdentList(s.Names),
				Callee:   callee,
				Args:     args,
				Builtin:  builtin,
				RetTypes: retTypes,
			})
		case ast.IndexAssignStmt:
			indexExprs := make([]string, 0, len(s.Indices))
			for _, idxNode := range s.Indices {
				idx, _, _, err := c.lowerExpr(idxNode)
				if err != nil {
					return err
				}
				indexExprs = append(indexExprs, idx)
			}
			val, _, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			targetType, ok := c.locals[s.Target]
			if !ok {
				return fmt.Errorf("index assignment to unknown local '%s'", s.Target)
			}
			switch {
			case strings.HasPrefix(targetType, "[][]"), strings.HasPrefix(targetType, "Matrix<"):
				if len(indexExprs) != 2 {
					return fmt.Errorf("matrix index assignment requires exactly 2 indices, got %d", len(indexExprs))
				}
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: fmt.Sprintf("%s[%s][%s]", c.goLocalName(s.Target), indexExprs[0], indexExprs[1]), Value: val})
			case strings.HasPrefix(targetType, "[]"), strings.HasSuffix(targetType, "[]"):
				if len(indexExprs) != 1 {
					return fmt.Errorf("array index assignment requires exactly 1 index, got %d", len(indexExprs))
				}
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: fmt.Sprintf("%s[%s]", c.goLocalName(s.Target), indexExprs[0]), Value: val})
			default:
				return fmt.Errorf("index assignment requires array or matrix local, got %s", targetType)
			}
		case ast.ExprStmt:
			v, t, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			if t != "Void" {
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: v})
			}
		case ast.ReturnStmt:
			if s.Value == nil {
				c.blocks[c.cur].Terminator = MIRReturn{}
				continue
			}
			v, t, _, err := c.withExpectedType(c.retType, func() (string, string, bool, error) { return c.lowerExpr(s.Value) })
			if err != nil {
				return err
			}
			v = coerceExprToType(v, t, c.retType)
			c.lastRet = t
			if c.fn.IsFallible {
				if t == "Error" {
					c.blocks[c.cur].Terminator = MIRReturn{Value: fallibleErrValue(c.retType, v)}
				} else {
					c.blocks[c.cur].Terminator = MIRReturn{Value: fallibleOkValue(c.retType, v)}
				}
			} else {
				c.blocks[c.cur].Terminator = MIRReturn{Value: v}
			}
		case ast.IfStmt:
			if err := c.lowerIfStmt(s); err != nil {
				return err
			}
		case ast.ForStmt:
			if err := c.lowerForStmt(s); err != nil {
				return err
			}
		case ast.WhileStmt:
			if err := c.lowerWhileStmt(s); err != nil {
				return err
			}
		case ast.PrometheusStmt:
			previous := c.inPrometheus
			c.inPrometheus = true
			err := c.lowerBlock(s.Body)
			c.inPrometheus = previous
			if err != nil {
				return err
			}
		case ast.MatchStmt:
			if err := c.lowerMatchStmt(s); err != nil {
				return err
			}
		case ast.GotoStmt:
			return unsupported("goto outside flow state")
		case ast.SuspendStmt:
			return unsupported("suspend outside flow state")
		case ast.RememberStmt:
			return unsupported("remember outside flow state")
		case ast.ResumeStmt:
			return unsupported("resume outside flow state")
		case ast.WhenStmt:
			return unsupported("when outside flow state")
		default:
			return fmt.Errorf("unsupported statement %T", s)
		}
	}
	return nil
}

func unsupported(feature string) error {
	return fmt.Errorf("compiled mode does not yet support %s", feature)
}

func unsupportedBuiltin(name string) error {
	return unsupported("builtin " + name)
}

func isOctxiliaryBuiltin(name string) bool {
	switch canonicalCompiledBuiltinName(name) {
	case "FileReadText", "FileReadLines", "FileReadBytes", "FileWriteText", "FileWriteLines", "FileWriteBytes", "FileDelete", "DirectoryList", "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll", "CsvRead", "CsvReadRows", "CsvReadTable", "CsvReadMatrix", "CsvWrite", "CsvWriteRows", "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad", "JsonSave":
		return true
	default:
		return false
	}
}

func isMarkdownCompiledBuiltin(name string) bool {
	switch canonicalCompiledBuiltinName(name) {
	case "MarkdownH1", "MarkdownH2", "MarkdownH3", "MarkdownParagraph", "MarkdownBlank", "MarkdownHorizontalRule", "MarkdownBullets", "MarkdownNumbered", "MarkdownCodeBlock", "MarkdownCallout", "MarkdownImage", "MarkdownFigure", "MarkdownTable", "MarkdownTableWithColumns", "MarkdownKeyValueTable", "MarkdownSection", "MarkdownSubsection", "MarkdownReport", "MarkdownEscapeText", "MarkdownEscapeTableCell":
		return true
	default:
		return false
	}
}

func compiledMarkdownBuiltinReturnType(name string) string {
	switch canonicalCompiledBuiltinName(name) {
	case "MarkdownEscapeText", "MarkdownEscapeTableCell":
		return "String"
	default:
		return "String[]"
	}
}

func usesOctxiliaryBuiltins(usedBuiltins map[string]bool) bool {
	for name := range usedBuiltins {
		if isOctxiliaryBuiltin(name) {
			return true
		}
	}
	return false
}

func usesLinearAlgebraHelpers(usedBuiltins map[string]bool) bool {
	if usedBuiltins["MatMulMV"] || usedBuiltins["MatMulVM"] || usedBuiltins["VecDot"] || usedBuiltins["MatMulMM"] || usedBuiltins["PrometheusMatMulMM"] || usedBuiltins["Trace"] || usedBuiltins["Grad"] || usedBuiltins["Div"] || usedBuiltins["SymGrad"] || usedBuiltins["EinMul"] || usedBuiltins["EinAdd"] || usedBuiltins["EinSub"] || usedBuiltins["EinAddVV"] || usedBuiltins["EinSubVV"] || usedBuiltins["EinDotVV"] || usedBuiltins["EinOuterVV"] || usedBuiltins["EinMulMV"] || usedBuiltins["EinMulVM"] || usedBuiltins["EinDoubleMM"] {
		return true
	}
	for name := range usedBuiltins {
		if strings.HasPrefix(name, "MatBinary") {
			return true
		}
	}
	return false
}

func canonicalCompiledBuiltinName(name string) string {
	switch name {
	case "String.ByteLength":
		return "StringByteLength"
	case "String.RuneCount":
		return "StringRuneCount"
	case "String.Join":
		return "StringJoin"
	case "String.Concat":
		return "StringConcat"
	case "String.From":
		return "StringFrom"
	case "String.ReplaceAll":
		return "StringReplaceAll"
	case "String.Contains":
		return "StringContains"
	case "String.StartsWith":
		return "StringStartsWith"
	case "String.EndsWith":
		return "StringEndsWith"
	case "String.Trim":
		return "StringTrim"
	case "String.SplitLines":
		return "StringSplitLines"
	case "String.EscapeJson", "String.EscapeJSON":
		return "StringEscapeJSON"
	case "String.QuoteJson", "String.QuoteJSON":
		return "StringQuoteJSON"
	case "Random.Gaussian", "Gaussian":
		return "Random.RandNormal"
	case "Markdown.H1", "Markdown.Title":
		return "MarkdownH1"
	case "Markdown.H2", "Markdown.Subtitle":
		return "MarkdownH2"
	case "Markdown.H3":
		return "MarkdownH3"
	case "Markdown.Paragraph":
		return "MarkdownParagraph"
	case "Markdown.Blank":
		return "MarkdownBlank"
	case "Markdown.HorizontalRule":
		return "MarkdownHorizontalRule"
	case "Markdown.Bullets":
		return "MarkdownBullets"
	case "Markdown.Numbered":
		return "MarkdownNumbered"
	case "Markdown.CodeBlock":
		return "MarkdownCodeBlock"
	case "Markdown.Callout":
		return "MarkdownCallout"
	case "Markdown.Image":
		return "MarkdownImage"
	case "Markdown.Figure":
		return "MarkdownFigure"
	case "Markdown.Table":
		return "MarkdownTable"
	case "Markdown.TableWithColumns":
		return "MarkdownTableWithColumns"
	case "Markdown.KeyValueTable":
		return "MarkdownKeyValueTable"
	case "Markdown.Section":
		return "MarkdownSection"
	case "Markdown.Subsection":
		return "MarkdownSubsection"
	case "Markdown.Report":
		return "MarkdownReport"
	case "Markdown.EscapeText":
		return "MarkdownEscapeText"
	case "Markdown.EscapeTableCell":
		return "MarkdownEscapeTableCell"
	default:
		return name
	}
}

func fallibleType(t string) string {
	return "Fallible[" + t + "]"
}

func isFallibleType(t string) bool {
	return strings.HasPrefix(t, "Fallible[") && strings.HasSuffix(t, "]")
}

func fallibleValueType(t string) string {
	return strings.TrimSuffix(strings.TrimPrefix(t, "Fallible["), "]")
}

func fallibleOkValue(retType, v string) string {
	return fmt.Sprintf("__oct_ok(%s,%s)", retType, v)
}

func fallibleErrValue(retType, errVal string) string {
	return fmt.Sprintf("__oct_err(%s,%s)", retType, errVal)
}

func (c *lowerCtx) lowerIfStmt(s ast.IfStmt) error {
	cond, _, _, err := c.lowerExpr(s.Condition)
	if err != nil {
		return err
	}
	thenID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", thenID)})
	elseID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", elseID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: cond, TrueTarget: c.blocks[thenID].Label, FalseTarget: c.blocks[elseID].Label}

	c.cur = thenID
	if err := c.lowerBlock(s.ThenBody); err != nil {
		return err
	}
	thenEndID := c.cur
	thenFallsThrough := c.blocks[thenEndID].Terminator == nil

	c.cur = elseID
	if s.ElseBody != nil {
		if err := c.lowerBlock(*s.ElseBody); err != nil {
			return err
		}
	}
	elseEndID := c.cur
	elseFallsThrough := c.blocks[elseEndID].Terminator == nil

	if !thenFallsThrough && !elseFallsThrough {
		c.cur = thenEndID
		return nil
	}

	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	if thenFallsThrough {
		c.blocks[thenEndID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	if elseFallsThrough {
		c.blocks[elseEndID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	c.cur = mergeID
	return nil
}

func (c *lowerCtx) lowerWhileStmt(s ast.WhileStmt) error {
	condID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", condID)})
	bodyID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", bodyID)})
	exitID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", exitID)})

	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[condID].Label}

	c.cur = condID
	cond, _, _, err := c.lowerExpr(s.Condition)
	if err != nil {
		return err
	}
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        cond,
		TrueTarget:  c.blocks[bodyID].Label,
		FalseTarget: c.blocks[exitID].Label,
	}

	c.cur = bodyID
	if err := c.lowerBlock(s.Body); err != nil {
		return err
	}
	if c.blocks[c.cur].Terminator == nil {
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[condID].Label}
	}
	c.cur = exitID
	return nil
}

func (c *lowerCtx) lowerForStmt(s ast.ForStmt) error {
	rangeExpr, ok := s.Range.(ast.RangeExpr)
	if !ok {
		return unsupported("for range expression")
	}
	start, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.Start) })
	if err != nil {
		return err
	}
	end, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.End) })
	if err != nil {
		return err
	}
	step := "1"
	if rangeExpr.Step != nil {
		step, _, _, err = c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.Step) })
		if err != nil {
			return err
		}
	}

	startLocal := c.temp("Int")
	endLocal := c.temp("Int")
	stepLocal := c.temp("Int")
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements,
		MIRAssign{Target: startLocal, Value: start},
		MIRAssign{Target: endLocal, Value: end},
		MIRAssign{Target: stepLocal, Value: step},
	)

	stepCheckID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", stepCheckID)})
	rangeCheckID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", rangeCheckID)})
	condID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", condID)})
	bodyID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", bodyID)})
	incrID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", incrID)})
	exitID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", exitID)})
	stepFailID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", stepFailID)})
	rangeFailID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", rangeFailID)})

	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[stepCheckID].Label}

	c.cur = stepCheckID
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        fmt.Sprintf("(%s > 0)", stepLocal),
		TrueTarget:  c.blocks[rangeCheckID].Label,
		FalseTarget: c.blocks[stepFailID].Label,
	}

	c.cur = rangeCheckID
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        fmt.Sprintf("(%s <= %s)", startLocal, endLocal),
		TrueTarget:  c.blocks[condID].Label,
		FalseTarget: c.blocks[rangeFailID].Label,
	}

	previousType, hadPrevious := c.locals[s.Name]
	previousGoName := c.goLocalName(s.Name)
	loopGoName := goIdent(s.Name)
	bindLoopName := s.Name != "_"
	if !bindLoopName || hadPrevious {
		loopGoName = c.temp("Int")
	}
	if bindLoopName {
		c.locals[s.Name] = "Int"
		c.goNames[s.Name] = loopGoName
	}

	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: loopGoName, Value: startLocal})
	c.cur = condID
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        fmt.Sprintf("(%s < %s)", loopGoName, endLocal),
		TrueTarget:  c.blocks[bodyID].Label,
		FalseTarget: c.blocks[exitID].Label,
	}

	c.cur = bodyID
	if err := c.lowerBlock(s.Body); err != nil {
		return err
	}
	if c.blocks[c.cur].Terminator == nil {
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[incrID].Label}
	}

	c.cur = incrID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: loopGoName, Value: fmt.Sprintf("(%s + %s)", loopGoName, stepLocal)})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[condID].Label}

	c.cur = stepFailID
	c.blocks[c.cur].Terminator = MIRFail{Value: fmt.Sprintf("%q + fmt.Sprint(%s)", "runtime error: range step must be positive, got ", stepLocal)}

	c.cur = rangeFailID
	c.blocks[c.cur].Terminator = MIRFail{Value: fmt.Sprintf("%q + fmt.Sprint(%s) + %q + fmt.Sprint(%s)", "runtime error: range start must be less than or equal to end, got ", startLocal, "..", endLocal)}

	c.cur = exitID
	if bindLoopName && hadPrevious {
		c.locals[s.Name] = previousType
		c.goNames[s.Name] = previousGoName
	}
	return nil
}

func (c *lowerCtx) lowerMatchStmt(s ast.MatchStmt) error {
	subject, valType, fallible, err := c.lowerExpr(s.Subject)
	if err != nil {
		return err
	}
	if !fallible {
		return fmt.Errorf("match requires fallible expression")
	}
	okID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", okID)})
	errID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", errID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: subject + ".IsErr", TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}

	c.cur = okID
	c.locals[s.OkName] = valType
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.OkName, Value: subject + ".Value"})
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: s.OkName})
	if err := c.lowerBlock(s.OkBody); err != nil {
		return err
	}
	okFallsThrough := c.blocks[c.cur].Terminator == nil

	c.cur = errID
	c.locals[s.ErrName] = "Error"
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.ErrName, Value: subject + ".Err"})
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: s.ErrName})
	if err := c.lowerBlock(s.ErrBody); err != nil {
		return err
	}
	errFallsThrough := c.blocks[c.cur].Terminator == nil

	if !okFallsThrough && !errFallsThrough {
		c.cur = okID
		return nil
	}
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	if okFallsThrough {
		c.blocks[okID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	if errFallsThrough {
		c.blocks[errID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	c.cur = mergeID
	return nil
}

func (c *lowerCtx) lowerPropagateExpr(e ast.PropagateExpr) (string, string, bool, error) {
	inner, valueType, fallible, err := c.lowerExpr(e.Inner)
	if err != nil {
		return "", "", false, err
	}
	if !fallible {
		return "", "", false, fmt.Errorf("operator '?' requires fallible expression")
	}
	out := c.temp(valueType)
	okID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", okID)})
	errID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", errID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})

	c.blocks[c.cur].Terminator = MIRBranch{Cond: inner + ".IsErr", TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}
	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: inner + ".Value"})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = errID
	c.blocks[c.cur].Terminator = MIRReturn{Value: fallibleErrValue(c.retType, inner+".Err")}
	c.cur = mergeID
	return out, valueType, false, nil
}

func (c *lowerCtx) lowerUnwrapExpr(e ast.UnwrapExpr) (string, string, bool, error) {
	inner, valueType, fallible, err := c.lowerExpr(e.Inner)
	if err != nil {
		return "", "", false, err
	}
	if !fallible {
		return "", "", false, fmt.Errorf("operator '!' requires fallible expression")
	}
	out := c.temp(valueType)
	okID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", okID)})
	errID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", errID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})

	c.blocks[c.cur].Terminator = MIRBranch{Cond: inner + ".IsErr", TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}
	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: inner + ".Value"})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = errID
	c.blocks[c.cur].Terminator = MIRFail{Value: fmt.Sprintf("\"unwrap failed: \" + %s.Err", inner)}
	c.cur = mergeID
	return out, valueType, false, nil
}

func (c *lowerCtx) temp(t string) string {
	name := fmt.Sprintf("_t%d", c.tempID)
	c.tempID++
	c.locals[name] = t
	if c.goNames != nil {
		c.goNames[name] = name
	}
	return name
}

func (c *lowerCtx) goLocalName(name string) string {
	if c.goNames != nil {
		if goName, ok := c.goNames[name]; ok {
			return goName
		}
	}
	return goIdent(name)
}

func (c *lowerCtx) declareLocal(name, typ string) {
	previousType, hadPrevious := c.locals[name]
	previousGoName := c.goLocalName(name)
	if c.goNames != nil && hadPrevious && previousType != typ {
		shadowKey := fmt.Sprintf("__shadow_%s_%d", name, c.tempID)
		c.tempID++
		c.locals[shadowKey] = previousType
		c.goNames[shadowKey] = previousGoName
	}
	c.locals[name] = typ
	if c.goNames == nil {
		return
	}
	if hadPrevious && previousType == typ {
		return
	}
	base := goIdent(name)
	candidate := base
	used := map[string]struct{}{}
	for _, goName := range c.goNames {
		used[goName] = struct{}{}
	}
	for i := 1; ; i++ {
		if _, exists := used[candidate]; !exists {
			c.goNames[name] = candidate
			return
		}
		candidate = fmt.Sprintf("%s_%d", base, i)
	}
}

func (c *lowerCtx) currentExpectedType() (string, bool) {
	if len(c.expectedTypeStack) == 0 {
		return "", false
	}
	current := c.expectedTypeStack[len(c.expectedTypeStack)-1]
	if current == "" {
		return "", false
	}
	return current, true
}

func coerceExprToType(value, from, to string) string {
	if from == to {
		return value
	}
	if isIntScalarTypeString(from) && isFloatScalarTypeString(to) {
		return fmt.Sprintf("float64(%s)", value)
	}
	if isIntArrayTypeString(from) && isFloatArrayTypeString(to) {
		return fmt.Sprintf("__octIntArrayToFloat(%s)", value)
	}
	if isComplexScalarTypeString(to) && isNumericTypeString(from) {
		return fmt.Sprintf("complex(float64(%s), 0)", value)
	}
	return value
}

func coerceNumericBinaryOperands(left, leftType, right, rightType, resultType string) (string, string) {
	if isFloatScalarTypeString(resultType) {
		left = coerceExprToType(left, leftType, resultType)
		right = coerceExprToType(right, rightType, resultType)
	}
	return left, right
}

func (c *lowerCtx) expectedArrayElemType() (string, bool) {
	if len(c.expectedTypeStack) == 0 {
		return "", false
	}
	current := c.expectedTypeStack[len(c.expectedTypeStack)-1]
	if !strings.HasSuffix(current, "[]") {
		return "", false
	}
	return strings.TrimSuffix(current, "[]"), true
}

func (c *lowerCtx) expectedMatrixElemType() (string, bool) {
	if len(c.expectedTypeStack) == 0 {
		return "", false
	}
	current := c.expectedTypeStack[len(c.expectedTypeStack)-1]
	return parseMatrixElemType(current)
}

func (c *lowerCtx) withExpectedType(expected string, fn func() (string, string, bool, error)) (string, string, bool, error) {
	c.expectedTypeStack = append(c.expectedTypeStack, expected)
	defer func() { c.expectedTypeStack = c.expectedTypeStack[:len(c.expectedTypeStack)-1] }()
	return fn()
}

func (c *lowerCtx) lowerLogicalBinaryExpr(e ast.BinaryExpr) (string, string, bool, error) {
	left, _, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Left) })
	if err != nil {
		return "", "", false, err
	}

	out := c.temp("Bool")
	shortValue := "false"
	rightOnTrue := true
	if e.Operator == "or" {
		shortValue = "true"
		rightOnTrue = false
	}
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: shortValue})

	rightID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", rightID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})

	trueTarget := c.blocks[rightID].Label
	falseTarget := c.blocks[mergeID].Label
	if !rightOnTrue {
		trueTarget = c.blocks[mergeID].Label
		falseTarget = c.blocks[rightID].Label
	}
	c.blocks[c.cur].Terminator = MIRBranch{Cond: left, TrueTarget: trueTarget, FalseTarget: falseTarget}

	c.cur = rightID
	right, _, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Right) })
	if err != nil {
		return "", "", false, err
	}
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: right})
	if c.blocks[c.cur].Terminator == nil {
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}

	c.cur = mergeID
	return out, "Bool", false, nil
}

func (c *lowerCtx) einTerm(value string) (einsteinTermMeta, bool) {
	if c.einTerms == nil {
		return einsteinTermMeta{}, false
	}
	term, ok := c.einTerms[value]
	return term, ok
}

func (c *lowerCtx) setEinTerm(value string, labels []string) {
	c.setEinTermMeta(value, labels, len(labels), "")
}

func (c *lowerCtx) setEinTermMeta(value string, labels []string, rank int, typ string) {
	if c.einTerms == nil {
		c.einTerms = map[string]einsteinTermMeta{}
	}
	c.einTerms[value] = einsteinTermMeta{Labels: append([]string(nil), labels...), Rank: rank, Type: typ}
}

func einsteinMulFreeLabels(left []string, right []string) ([]string, error) {
	ordered := append(append([]string{}, left...), right...)
	counts := map[string]int{}
	for _, label := range ordered {
		counts[label]++
	}
	free := make([]string, 0, 2)
	seenFree := map[string]struct{}{}
	for _, label := range ordered {
		count := counts[label]
		if count == 1 {
			if _, seen := seenFree[label]; !seen {
				free = append(free, label)
				seenFree[label] = struct{}{}
			}
			continue
		}
		if count > 2 {
			return nil, fmt.Errorf("compiled Einstein multiplication index '%s' appears %d times; only 1 (free) or 2 (contracted) are allowed", label, count)
		}
	}
	if len(free) > 2 {
		return nil, fmt.Errorf("compiled Einstein multiplication result rank %d is not supported in M36; rank-N tensors are deferred", len(free))
	}
	return free, nil
}

func einsteinLabelsMatch(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (c *lowerCtx) lowerExpr(expr ast.Expr) (string, string, bool, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		if expected, ok := c.currentExpectedType(); ok && isFloatScalarTypeString(expected) {
			return fmt.Sprintf("float64(%s)", e.Value), expected, false, nil
		}
		if e.HasUnit {
			return e.Value, fmt.Sprintf("Int<%s>", e.Dimension.String()), false, nil
		}
		return e.Value, "Int", false, nil
	case ast.FloatLiteral:
		if e.HasUnit {
			return e.Value, fmt.Sprintf("Float<%s>", e.Dimension.String()), false, nil
		}
		return e.Value, "Float", false, nil
	case ast.BoolLiteral:
		if e.Value {
			return "true", "Bool", false, nil
		}
		return "false", "Bool", false, nil
	case ast.StringLiteralExpr:
		return fmt.Sprintf("%q", e.Value), "String", false, nil
	case ast.IdentifierExpr:
		t, ok := c.locals[e.Name]
		if ok {
			return c.goLocalName(e.Name), t, false, nil
		}
		if symbol, signature, ok := c.resolveNamedFunctionValue(e); ok {
			return symbol, signature, false, nil
		}
		return "", "", false, fmt.Errorf("unknown identifier '%s'", e.Name)
	case ast.BinaryExpr:
		if e.Operator == "and" || e.Operator == "or" {
			return c.lowerLogicalBinaryExpr(e)
		}
		l, lt, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Left) })
		if err != nil {
			return "", "", false, err
		}
		r, rt, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Right) })
		if err != nil {
			return "", "", false, err
		}
		if leftTerm, leftIndexed := c.einTerm(l); leftIndexed {
			rightTerm, rightIndexed := c.einTerm(r)
			if !rightIndexed {
				return "", "", false, fmt.Errorf("compiled indexed tensor expressions must appear on both sides of '%s'", e.Operator)
			}
			leftElem, leftVector := parseVectorElemType(lt)
			if !leftVector {
				leftElem, _ = parseMatrixElemType(lt)
			}
			rightElem, rightVector := parseVectorElemType(rt)
			if !rightVector {
				rightElem, _ = parseMatrixElemType(rt)
			}
			retElem := unifyLinearElemType(leftElem, rightElem)
			switch e.Operator {
			case "+", "-":
				if leftTerm.Rank != rightTerm.Rank {
					return "", "", false, fmt.Errorf("compiled Einstein addition/subtraction requires matching indexed term ranks (left rank=%d, right rank=%d)", leftTerm.Rank, rightTerm.Rank)
				}
				if leftTerm.Rank != 1 && leftTerm.Rank != 2 {
					return "", "", false, fmt.Errorf("compiled Einstein addition/subtraction supports only rank-1 vectors and rank-2 matrices, got rank %d", leftTerm.Rank)
				}
				if !einsteinLabelsMatch(leftTerm.Labels, rightTerm.Labels) {
					return "", "", false, fmt.Errorf("compiled Einstein '%s' requires matching free-index order", e.Operator)
				}
				callee := "EinAdd"
				if e.Operator == "-" {
					callee = "EinSub"
				}
				if leftTerm.Rank == 1 {
					if !leftVector || !rightVector {
						return "", "", false, fmt.Errorf("compiled rank-1 Einstein addition/subtraction requires vector operands")
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee + "VV", Args: []string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, leftTerm.Labels, 1, ret)
					return tmp, ret, false, nil
				}
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: []string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, Builtin: true, RetType: ret})
				c.setEinTermMeta(tmp, leftTerm.Labels, 2, ret)
				return tmp, ret, false, nil
			case "*":
				free, err := einsteinMulFreeLabels(leftTerm.Labels, rightTerm.Labels)
				if err != nil {
					return "", "", false, err
				}
				if leftTerm.Rank == 2 && rightTerm.Rank == 2 {
					if len(free) == 0 {
						tmp := c.temp(retElem)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinDoubleMM", Args: []string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, Builtin: true, RetType: retElem})
						return tmp, retElem, false, nil
					}
					if len(free) != 2 {
						return "", "", false, fmt.Errorf("compiled matrix/matrix indexed contraction requires either 0 or 2 free indices, got %d", len(free))
					}
					ret := "Matrix<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMul", Args: []string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 2, ret)
					return tmp, ret, false, nil
				}
				if leftTerm.Rank == 1 && rightTerm.Rank == 1 {
					if !leftVector || !rightVector {
						return "", "", false, fmt.Errorf("compiled rank-1 Einstein multiplication requires vector operands")
					}
					if len(free) == 0 {
						tmp := c.temp(retElem)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinDotVV", Args: []string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, Builtin: true, RetType: retElem})
						return tmp, retElem, false, nil
					}
					if len(free) == 2 {
						ret := "Matrix<" + retElem + ">"
						tmp := c.temp(ret)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinOuterVV", Args: []string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, Builtin: true, RetType: ret})
						c.setEinTermMeta(tmp, free, 2, ret)
						return tmp, ret, false, nil
					}
				}
				if leftTerm.Rank == 2 && rightTerm.Rank == 1 {
					if len(free) != 1 {
						return "", "", false, fmt.Errorf("compiled matrix-vector indexed contraction requires exactly 1 free index, got %d", len(free))
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMulMV", Args: []string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], free[0]}, Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 1, ret)
					return tmp, ret, false, nil
				}
				if leftTerm.Rank == 1 && rightTerm.Rank == 2 {
					if len(free) != 1 {
						return "", "", false, fmt.Errorf("compiled vector-matrix indexed contraction requires exactly 1 free index, got %d", len(free))
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMulVM", Args: []string{l, leftTerm.Labels[0], r, rightTerm.Labels[0], rightTerm.Labels[1], free[0]}, Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 1, ret)
					return tmp, ret, false, nil
				}
				return "", "", false, fmt.Errorf("compiled indexed multiplication supports rank-1 vectors and rank-2 matrices only (left rank=%d, right rank=%d)", leftTerm.Rank, rightTerm.Rank)
			default:
				return "", "", false, fmt.Errorf("compiled indexed tensor expressions only support '+', '-', and '*'")
			}
		} else if _, rightIndexed := c.einTerm(r); rightIndexed {
			return "", "", false, fmt.Errorf("compiled indexed tensor expressions must appear on both sides of '%s'", e.Operator)
		}
		if e.Operator == "@" {
			if leftElem, ok := parseMatrixElemType(lt); ok {
				if rightElem, ok := parseVectorElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Vector<" + elemType + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "MatMulMV",
						Args:    []string{l, r},
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
				if rightElem, ok := parseMatrixElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Matrix<" + elemType + ">"
					tmp := c.temp(ret)
					callee := "MatMulMM"
					if c.inPrometheus && leftElem == "Float" && rightElem == "Float" {
						callee = "PrometheusMatMulMM"
					}
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  callee,
						Args:    []string{l, r},
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
			}
			if leftElem, ok := parseVectorElemType(lt); ok {
				if rightElem, ok := parseMatrixElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Vector<" + elemType + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "MatMulVM",
						Args:    []string{l, r},
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
				if rightElem, ok := parseVectorElemType(rt); ok {
					ret := unifyLinearElemType(leftElem, rightElem)
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "VecDot",
						Args:    []string{l, r},
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
			}
		}
		if leftElem, ok := parseVectorElemType(lt); ok {
			if rightElem, ok := parseVectorElemType(rt); ok {
				ret := "Vector<" + unifyLinearElemType(leftElem, rightElem) + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "VecBinaryVV:" + e.Operator,
					Args:    []string{l, r},
					Builtin: true,
					RetType: ret,
				})
				return tmp, ret, false, nil
			}
			if isNumericTypeString(rt) {
				tmp := c.temp(lt)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "VecBinaryVS:" + e.Operator,
					Args:    []string{l, r},
					Builtin: true,
					RetType: lt,
				})
				return tmp, lt, false, nil
			}
		}
		if rightElem, ok := parseVectorElemType(rt); ok && isNumericTypeString(lt) {
			tmp := c.temp(rt)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
				Target:  tmp,
				Callee:  "VecBinarySV:" + e.Operator,
				Args:    []string{l, r},
				Builtin: true,
				RetType: "Vector<" + rightElem + ">",
			})
			return tmp, rt, false, nil
		}
		if leftElem, ok := parseMatrixElemType(lt); ok && isLinearElementwiseOperatorString(e.Operator) {
			if rightElem, ok := parseMatrixElemType(rt); ok {
				retElem := scalarBinaryResultTypeString(e.Operator, leftElem, rightElem)
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:   tmp,
					Callee:   "MatBinaryMM:" + e.Operator,
					Args:     []string{l, r},
					ArgTypes: []string{lt, rt},
					Builtin:  true,
					RetType:  ret,
				})
				return tmp, ret, false, nil
			}
			if isNumericTypeString(rt) {
				retElem := scalarBinaryResultTypeString(e.Operator, leftElem, rt)
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:   tmp,
					Callee:   "MatBinaryMS:" + e.Operator,
					Args:     []string{l, r},
					ArgTypes: []string{lt, rt},
					Builtin:  true,
					RetType:  ret,
				})
				return tmp, ret, false, nil
			}
		}
		if rightElem, ok := parseMatrixElemType(rt); ok && isNumericTypeString(lt) && isLinearElementwiseOperatorString(e.Operator) {
			retElem := scalarBinaryResultTypeString(e.Operator, lt, rightElem)
			ret := "Matrix<" + retElem + ">"
			tmp := c.temp(ret)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
				Target:   tmp,
				Callee:   "MatBinarySM:" + e.Operator,
				Args:     []string{l, r},
				ArgTypes: []string{lt, rt},
				Builtin:  true,
				RetType:  ret,
			})
			return tmp, ret, false, nil
		}
		ret := lt
		switch e.Operator {
		case "==", "!=", "<", "<=", ">", ">=", "and", "or":
			ret = "Bool"
		case "+", "-", "*", "/":
			if isComplexScalarTypeString(lt) || isComplexScalarTypeString(rt) {
				ret = "Complex"
			} else if isFloatScalarTypeString(lt) || isFloatScalarTypeString(rt) {
				if strings.HasPrefix(lt, "Float<") && strings.HasSuffix(lt, ">") {
					ret = lt
				} else if strings.HasPrefix(rt, "Float<") && strings.HasSuffix(rt, ">") {
					ret = rt
				} else {
					ret = "Float"
				}
			}
		case "%":
			ret = "Int"
		}
		tmp := c.temp(ret)
		op := e.Operator
		if op == "and" {
			op = "&&"
		}
		if op == "or" {
			op = "||"
		}
		if ret == "Complex" && isComplexCompatibleScalarTypeString(lt) && isComplexCompatibleScalarTypeString(rt) {
			l = coerceExprToType(l, lt, "Complex")
			r = coerceExprToType(r, rt, "Complex")
		} else if isNumericTypeString(lt) && isNumericTypeString(rt) {
			l, r = coerceNumericBinaryOperands(l, lt, r, rt, ret)
			if ret == "Bool" && (isFloatScalarTypeString(lt) || isFloatScalarTypeString(rt)) {
				l, r = coerceNumericBinaryOperands(l, lt, r, rt, "Float")
			}
		}
		if op == "%" {
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{
				Target: tmp,
				Value:  fmt.Sprintf("func(__a int, __b int) int { if __b == 0 { panic(\"runtime error: modulo by zero\") }; __r := __a %% __b; if __r < 0 { if __b > 0 { __r += __b } else { __r -= __b } }; return __r }(%s, %s)", l, r),
			})
			return tmp, ret, false, nil
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("(%s %s %s)", l, op, r)})
		return tmp, ret, false, nil
	case ast.UnaryExpr:
		v, t, _, err := c.lowerExpr(e.Operand)
		if err != nil {
			return "", "", false, err
		}
		tmp := c.temp(t)
		op := e.Operator
		if op == "not" {
			op = "!"
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("(%s%s)", op, v)})
		return tmp, t, false, nil
	case ast.CallExpr:
		if calleeField, ok := e.Callee.(ast.FieldAccessExpr); ok {
			if enumType, variant, ok := c.flattenEnumVariantExpr(calleeField); ok {
				enumValue, resolvedEnumType, enumFound, err := c.resolveEnumVariantConstructor(enumType, variant, e.Arguments)
				if err != nil {
					return "", "", false, err
				}
				if enumFound {
					return enumValue, resolvedEnumType, false, nil
				}
			}
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "error" {
			if len(e.Arguments) != 1 {
				return "", "", false, fmt.Errorf("error() expects one argument")
			}
			v, _, _, err := c.lowerExpr(e.Arguments[0])
			if err != nil {
				return "", "", false, err
			}
			return v, "Error", false, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "WriteOctagon" {
			args := make([]string, 0, len(e.Arguments))
			for _, a := range e.Arguments {
				v, _, _, err := c.lowerExpr(a)
				if err != nil {
					return "", "", false, err
				}
				args = append(args, v)
			}
			tmp := c.temp("Int")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "WriteOctagon", Args: args, Builtin: true, RetType: "Int"})
			return tmp, "Int", false, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "LoadOctagon" {
			args := make([]string, 0, len(e.Arguments))
			for _, a := range e.Arguments {
				v, _, _, err := c.lowerExpr(a)
				if err != nil {
					return "", "", false, err
				}
				args = append(args, v)
			}
			ret := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
			tmp := c.temp(fallibleType(ret))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "LoadOctagon", Args: args, Builtin: true, RetType: ret})
			return tmp, ret, true, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "Result" {
			if len(e.Arguments) != 1 {
				return "", "", false, fmt.Errorf("Result expects one argument")
			}
			flowArg, flowType, _, err := c.lowerExpr(e.Arguments[0])
			if err != nil {
				return "", "", false, err
			}
			resultType, ok := parseFlowInstanceType(flowType)
			if !ok {
				return "", "", false, fmt.Errorf("Result expects FlowInstance argument")
			}
			tmp := c.temp(fallibleType(resultType))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Result", Args: []string{flowArg}, Builtin: true, RetType: resultType})
			return tmp, resultType, true, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok {
			switch ident.Name {
			case "PrometheusMatMul":
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("PrometheusMatMul expects 2 arguments")
				}
				leftArg, leftType, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				rightArg, rightType, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				if leftType != "Matrix<Float>" || rightType != "Matrix<Float>" {
					return "", "", false, fmt.Errorf("PrometheusMatMul expects Matrix<Float> arguments")
				}
				tmp := c.temp("Matrix<Float>")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "PrometheusMatMulMM",
					Args:    []string{leftArg, rightArg},
					Builtin: true,
					RetType: "Matrix<Float>",
				})
				return tmp, "Matrix<Float>", false, nil
			case "Abs", "Pi", "E", "I", "Complex", "ComplexPolar", "Real", "Imag", "Arg", "Conj", "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "Exp", "Ln", "Pow", "Log10", "Sinh", "Cosh", "Tanh", "Trace", "Grad", "Div", "SymGrad":
				args := make([]string, 0, len(e.Arguments))
				argTypes := make([]string, 0, len(e.Arguments))
				for _, a := range e.Arguments {
					v, t, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
					argTypes = append(argTypes, t)
				}
				ret, err := compiledBuiltinReturnType(ident.Name, argTypes)
				if err != nil {
					return "", "", false, err
				}
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: ident.Name, Args: args, ArgTypes: argTypes, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			}
		}
		if calleeName, ok := flattenDirectCallName(e.Callee); ok {
			if strings.HasPrefix(calleeName, "Assert.") {
				args := make([]string, 0, len(e.Arguments))
				for _, a := range e.Arguments {
					v, _, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
				}
				switch calleeName {
				case "Assert.True":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.True", Args: args, Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.False":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.False", Args: args, Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Equal":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Equal", Args: args, Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Near":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Near", Args: args, Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Error":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Error", Args: args, Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				}
			}
			switch calleeName {
			case "Matrix.tabulate":
				if len(e.Arguments) != 3 {
					return "", "", false, fmt.Errorf("Matrix.tabulate expects 3 arguments")
				}
				rowsArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				colsArg, _, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				callbackName, callbackRet, err := c.resolveMatrixTabulateCallback(e.Arguments[2])
				if err != nil {
					return "", "", false, err
				}
				ret := "Matrix<" + callbackRet + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.tabulate", Args: []string{rowsArg, colsArg, callbackName}, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.fill":
				if len(e.Arguments) != 3 {
					return "", "", false, fmt.Errorf("Matrix.fill expects 3 arguments")
				}
				args := make([]string, 0, 3)
				elemType := "Int"
				for idx, a := range e.Arguments {
					v, t, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
					if idx == 2 {
						elemType = t
					}
				}
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.fill", Args: args, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.zeros":
				if len(e.TypeArguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.zeros expects 1 type argument")
				}
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("Matrix.zeros expects 2 arguments")
				}
				rowsArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				colsArg, _, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				elemType := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.zeros", Args: []string{rowsArg, colsArg}, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.identity":
				if len(e.TypeArguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.identity expects 1 type argument")
				}
				if len(e.Arguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.identity expects 1 argument")
				}
				sizeArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				elemType := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.identity", Args: []string{sizeArg}, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			}
		}
		if callTarget, signature, ok, err := c.resolveFunctionValueCall(e.Callee); err != nil {
			return "", "", false, err
		} else if ok {
			args := make([]string, 0, len(e.Arguments))
			argTypes := make([]string, 0, len(e.Arguments))
			for i, a := range e.Arguments {
				var expected string
				if i < len(signature.Parameters) {
					expected = signature.Parameters[i]
				}
				v, at, _, err := c.withExpectedType(expected, func() (string, string, bool, error) { return c.lowerExpr(a) })
				if err != nil {
					return "", "", false, err
				}
				if expected != "" {
					v = goCoerceArg(v, at, expected)
				}
				args = append(args, v)
				argTypes = append(argTypes, at)
			}
			localType := signature.ReturnType
			if signature.Fallible {
				localType = fallibleType(signature.ReturnType)
			}
			tmp := c.temp(localType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callTarget, Args: args, ArgTypes: argTypes, RetType: signature.ReturnType, FunctionValue: true})
			return tmp, signature.ReturnType, signature.Fallible, nil
		}
		callee, ret, builtin, fallible, err := c.resolveCall(e.Callee)
		if err != nil {
			return "", "", false, err
		}
		expectedArgTypes := c.resolveCallArgTypes(e.Callee)
		args := make([]string, 0, len(e.Arguments))
		argTypes := make([]string, 0, len(e.Arguments))
		for i, a := range e.Arguments {
			var expected string
			if i < len(expectedArgTypes) {
				expected = expectedArgTypes[i]
			}
			v, at, _, err := c.withExpectedType(expected, func() (string, string, bool, error) { return c.lowerExpr(a) })
			if err != nil {
				return "", "", false, err
			}
			if expected != "" {
				v = goCoerceArg(v, at, expected)
			}
			args = append(args, v)
			argTypes = append(argTypes, at)
		}
		if builtin && callee == "Append" && len(argTypes) > 0 {
			ret = argTypes[0]
		}
		if builtin && isMarkdownCompiledBuiltin(callee) {
			checkedRet, err := compiledBuiltinReturnType(callee, argTypes)
			if err != nil {
				return "", "", false, err
			}
			ret = checkedRet
		}
		if builtin && (callee == "Idx" || callee == "EinMul" || callee == "EinAdd" || callee == "EinSub") {
			checkedRet, err := compiledBuiltinReturnType(callee, argTypes)
			if err != nil {
				return "", "", false, err
			}
			ret = checkedRet
		}
		if meta, ok, err := c.genericWrapperMetadataForCallee(e.Callee); err != nil {
			return "", "", false, err
		} else if ok {
			if ret != meta.Return {
				return "", "", false, fmt.Errorf("wrapper function %s.%s manifest return %s does not match Oct stub return %s", meta.PackageName, meta.OctName, meta.Return, ret)
			}
			if fallible != meta.Fallible {
				return "", "", false, fmt.Errorf("wrapper function %s.%s manifest fallible %t does not match Oct stub fallible %t", meta.PackageName, meta.OctName, meta.Fallible, fallible)
			}
			if len(argTypes) != len(meta.Args) {
				return "", "", false, fmt.Errorf("wrapper function %s.%s expects %d arguments, got %d", meta.PackageName, meta.OctName, len(meta.Args), len(argTypes))
			}
			effectiveArgTypes := append([]string(nil), argTypes...)
			for i := range argTypes {
				if !wrapperArgTypeMatches(meta.Args[i], argTypes[i]) {
					return "", "", false, fmt.Errorf("wrapper function %s.%s argument %d expects %s, got %s", meta.PackageName, meta.OctName, i+1, meta.Args[i], argTypes[i])
				}
				effectiveArgTypes[i] = meta.Args[i]
				if !isOctxiliaryTransportType(meta.Args[i]) && !findTransportRecord(meta.TransportTypes, meta.Args[i]).ok {
					return "", "", false, fmt.Errorf("wrapper function %s.%s argument %d uses unsupported transport type %s", meta.PackageName, meta.OctName, i+1, meta.Args[i])
				}
			}
			if transport := findTransportRecord(meta.TransportTypes, meta.Return); transport.ok {
				if transport.typ.Kind != "handle" {
					return "", "", false, fmt.Errorf("wrapper function %s.%s return uses declared record transport type %s; record returns are not supported", meta.PackageName, meta.OctName, meta.Return)
				}
			} else if !isOctxiliaryTransportType(meta.Return) {
				return "", "", false, fmt.Errorf("wrapper function %s.%s return uses unsupported transport type %s", meta.PackageName, meta.OctName, meta.Return)
			}
			localType := meta.Return
			if meta.Fallible {
				localType = fallibleType(meta.Return)
			}
			tmp := c.temp(localType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRGenericOctxiliaryCall{Target: tmp, PackageName: meta.PackageName, OctName: meta.OctName, Family: meta.Family, WireName: meta.WireName, SidecarCommand: meta.SidecarCommand, Args: args, ArgTypes: effectiveArgTypes, RetType: meta.Return, Fallible: meta.Fallible, TransportTypes: meta.TransportTypes})
			return tmp, meta.Return, meta.Fallible, nil
		}
		if builtin && callee == "BoardSnapshot" {
			if len(argTypes) != 1 {
				return "", "", false, fmt.Errorf("BoardSnapshot expects 1 argument")
			}
			flowRet, ok := parseFlowInstanceType(argTypes[0])
			if !ok {
				return "", "", false, fmt.Errorf("BoardSnapshot expects FlowInstance argument")
			}
			snapshotType := ""
			for _, flowDecl := range c.pkg.Flows {
				if typeRefStringForPackage(c.pkg.Name, flowDecl.ReturnType) == flowRet && len(flowDecl.Board) > 0 {
					if snapshotType != "" {
						return "", "", false, fmt.Errorf("compiled BoardSnapshot requires unambiguous flow identity for return type %s", flowRet)
					}
					snapshotType = c.pkg.Name + "." + flowDecl.Name + "BoardSnapshot"
				}
			}
			if snapshotType == "" {
				return "", "", false, fmt.Errorf("compiled mode does not yet support builtin BoardSnapshot")
			}
			ret = snapshotType
		}
		localType := ret
		if fallible {
			localType = fallibleType(ret)
		}
		if !fallible && localType == "Void" {
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: callee, Args: args, ArgTypes: argTypes, Builtin: builtin, RetType: ret})
			return "", ret, false, nil
		}
		tmp := c.temp(localType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: args, ArgTypes: argTypes, Builtin: builtin, RetType: ret})
		return tmp, ret, fallible, nil
	case ast.ArrayLiteralExpr:
		vals := []string{}
		typeName := "Int"
		hint, hasHint := c.expectedArrayElemType()
		if hasHint {
			typeName = hint
		}
		for i, el := range e.Elements {
			var v, t string
			var err error
			if hasHint {
				v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(el) })
			} else {
				v, t, _, err = c.lowerExpr(el)
			}
			if err != nil {
				return "", "", false, err
			}
			if hasHint {
				v = coerceExprToType(v, t, hint)
			} else if i == 0 {
				typeName = t
			}
			vals = append(vals, v)
		}
		tmp := c.temp(typeName + "[]")
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: typeName, Values: vals})
		return tmp, typeName + "[]", false, nil
	case ast.VectorLiteralExpr:
		vals := make([]string, 0, len(e.Elements))
		elemType := "Int"
		for i, el := range e.Elements {
			v, t, _, err := c.lowerExpr(el)
			if err != nil {
				return "", "", false, err
			}
			vals = append(vals, v)
			if i == 0 {
				elemType = t
			}
		}
		vectorType := "Vector<" + elemType + ">"
		tmp := c.temp(vectorType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: elemType, Values: vals})
		return tmp, vectorType, false, nil
	case ast.MatrixLiteralExpr:
		rows := make([]string, 0, len(e.Rows))
		elemType := "Int"
		hint, hasHint := c.expectedMatrixElemType()
		if hasHint {
			elemType = hint
		}
		for _, row := range e.Rows {
			rowVals := make([]string, 0, len(row))
			for j, cell := range row {
				var v, t string
				var err error
				if hasHint {
					v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(cell) })
				} else {
					v, t, _, err = c.lowerExpr(cell)
				}
				if err != nil {
					return "", "", false, err
				}
				if hasHint {
					v = coerceExprToType(v, t, hint)
				} else if len(rows) == 0 && j == 0 {
					elemType = t
				}
				rowVals = append(rowVals, v)
			}
			rowType := "Vector<" + elemType + ">"
			rowTmp := c.temp(rowType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: rowTmp, ElemType: elemType, Values: rowVals})
			rows = append(rows, rowTmp)
		}
		matrixType := "Matrix<" + elemType + ">"
		tmp := c.temp(matrixType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: "Vector<" + elemType + ">", Values: rows})
		return tmp, matrixType, false, nil
	case ast.FieldAccessExpr:
		if enumType, variant, ok := c.flattenEnumVariantExpr(e); ok {
			enumValue, resolvedEnumType, enumFound, err := c.resolveEnumVariantValue(enumType, variant)
			if err != nil {
				return "", "", false, err
			}
			if enumFound {
				return enumValue, resolvedEnumType, false, nil
			}
		}
		t, targetType, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", false, err
		}
		if _, ok := parseMatrixElemType(targetType); ok {
			switch e.Field {
			case "rows":
				tmp := c.temp("Int")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("len(%s)", t)})
				return tmp, "Int", false, nil
			case "cols":
				tmp := c.temp("Int")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("func() int { if len(%s) == 0 { return 0 }; return len(%s[0]) }()", t, t)})
				return tmp, "Int", false, nil
			}
		}
		fieldType := "Int"
		if resolvedType, ok := c.lookupRecordFieldType(targetType, e.Field); ok {
			fieldType = resolvedType
		}
		tmp := c.temp(fieldType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("%s.%s", t, e.Field)})
		return tmp, fieldType, false, nil
	case ast.IndexExpr:
		target, targetType, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", false, err
		}
		if matrixElem, ok := parseMatrixElemType(targetType); ok {
			if len(e.Indices) != 2 {
				return "", "", false, fmt.Errorf("compiled mode matrix indexing requires exactly 2 indices")
			}
			first, firstType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[0]) })
			if err != nil {
				return "", "", false, err
			}
			second, secondType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[1]) })
			if err != nil {
				return "", "", false, err
			}
			if firstType == "Int" && secondType == "Int" {
				tmp := c.temp(matrixElem)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("%s[%s][%s]", target, first, second)})
				return tmp, matrixElem, false, nil
			}
			if firstType == "Index" && secondType == "Index" {
				tmp := c.temp(targetType)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: target})
				c.setEinTermMeta(tmp, []string{first, second}, 2, targetType)
				return tmp, targetType, false, nil
			}
			return "", "", false, fmt.Errorf("compiled mode matrix indexing expects either [Int, Int] element access or [Index, Index] Einstein term access, got [%s, %s]", firstType, secondType)
		}
		if len(e.Indices) != 1 {
			return "", "", false, fmt.Errorf("compiled mode only supports single-dimension indexing")
		}
		idx, idxType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[0]) })
		if err != nil {
			return "", "", false, err
		}
		if idxType == "Index" {
			if _, ok := parseVectorElemType(targetType); ok {
				tmp := c.temp(targetType)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: target})
				c.setEinTermMeta(tmp, []string{idx}, 1, targetType)
				return tmp, targetType, false, nil
			}
			return "", "", false, fmt.Errorf("compiled mode single-dimension indexing requires Int index, got %s", idxType)
		}
		if idxType != "Int" {
			return "", "", false, fmt.Errorf("compiled mode single-dimension indexing requires Int index, got %s", idxType)
		}
		elemType := strings.TrimSuffix(targetType, "[]")
		valueExpr := fmt.Sprintf("%s[%s]", target, idx)
		if targetType == "Bytes" {
			elemType = "Int"
			valueExpr = fmt.Sprintf("int(%s[%s])", target, idx)
		}
		if vectorElem, ok := parseVectorElemType(targetType); ok {
			elemType = vectorElem
		}
		tmp := c.temp(elemType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: valueExpr})
		return tmp, elemType, false, nil
	case ast.RecordLiteralExpr:
		vals := []string{}
		names := []string{}
		typeName := e.TypeName
		if !strings.Contains(typeName, ".") {
			typeName = c.pkg.Name + "." + typeName
		}
		for _, f := range e.Fields {
			fieldType, hasFieldType := c.lookupRecordFieldType(typeName, f.Name)
			var v, t string
			var err error
			if hasFieldType {
				v, t, _, err = c.withExpectedType(fieldType, func() (string, string, bool, error) { return c.lowerExpr(f.Value) })
			} else {
				v, t, _, err = c.lowerExpr(f.Value)
			}
			if err != nil {
				return "", "", false, err
			}
			if hasFieldType {
				v = coerceExprToType(v, t, fieldType)
			}
			vals = append(vals, v)
			names = append(names, f.Name)
		}
		tmp := c.temp(typeName)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructRecord{Target: tmp, TypeName: typeName, FieldNames: names, FieldVals: vals})
		return tmp, typeName, false, nil
	case ast.RecordUpdateExpr:
		source, sourceType, _, err := c.lowerExpr(e.Source)
		if err != nil {
			return "", "", false, err
		}
		fieldTypes, ok := c.lookupRecordFields(sourceType)
		if !ok {
			return "", "", false, fmt.Errorf("record update requires record source")
		}
		overrides := make(map[string]string, len(e.Fields))
		for _, field := range e.Fields {
			value, _, _, err := c.lowerExpr(field.Value)
			if err != nil {
				return "", "", false, err
			}
			overrides[field.Name] = value
		}
		names := make([]string, 0, len(fieldTypes))
		values := make([]string, 0, len(fieldTypes))
		for _, field := range fieldTypes {
			names = append(names, field.Name)
			if override, exists := overrides[field.Name]; exists {
				values = append(values, override)
				continue
			}
			values = append(values, fmt.Sprintf("%s.%s", source, field.Name))
		}
		tmp := c.temp(sourceType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructRecord{Target: tmp, TypeName: sourceType, FieldNames: names, FieldVals: values})
		return tmp, sourceType, false, nil
	case ast.EnumValueExpr:
		enumType := e.EnumName
		if !strings.Contains(enumType, ".") {
			enumType = c.pkg.Name + "." + enumType
		}
		return fmt.Sprintf("%s_%s", e.EnumName, e.Variant), enumType, false, nil
	case ast.IfExpr:
		return c.lowerIfExpr(e)
	case ast.SwitchExpr:
		return c.lowerSwitchExpr(e)
	case ast.MatchExpr:
		return c.lowerMatchExpr(e)
	case ast.RangeExpr:
		return "", "", false, unsupported("range")
	case ast.PropagateExpr:
		return c.lowerPropagateExpr(e)
	case ast.UnwrapExpr:
		return c.lowerUnwrapExpr(e)
	case ast.BatchExpr:
		return c.lowerBatchExpr(e)
	case ast.UtilityWhenExpr:
		if e.EnumTarget != nil && utilityWhenHasPayloadCandidate(e) {
			return "", "", false, unsupported("compiled enum-targeted utility payload candidates require delayed payload lowering")
		}
		h, _, _, err := c.lowerExpr(e.Policy.Hysteresis)
		if err != nil {
			return "", "", false, err
		}
		m, _, _, err := c.lowerExpr(e.Policy.MinCommit)
		if err != nil {
			return "", "", false, err
		}
		elseExpr, resultType, _, err := c.lowerExpr(e.Else)
		if err != nil {
			return "", "", false, err
		}
		cases := make([]string, 0, len(e.Cases))
		valueType := goType(resultType)
		for _, wc := range e.Cases {
			v, _, _, err := c.lowerExpr(wc.Value)
			if err != nil {
				return "", "", false, err
			}
			cond, _, _, err := c.lowerExpr(wc.Condition)
			if err != nil {
				return "", "", false, err
			}
			score, _, _, err := c.lowerExpr(wc.Score)
			if err != nil {
				return "", "", false, err
			}
			cases = append(cases, fmt.Sprintf("{Valid: %s, Value: %s, Score: %s}", cond, v, score))
		}
		c.usesUtilityWhen = true
		return fmt.Sprintf("__octUtilSelect[%s](map[int]__octUtilitySiteState{}, %d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
			valueType, e.SiteID, h, m, valueType, strings.Join(cases, ", "), elseExpr), resultType, false, nil
	case ast.ParenExpr:
		return c.lowerExpr(e.Inner)
	default:
		return "", "", false, fmt.Errorf("unsupported expression %T", e)
	}
}

func utilityWhenHasPayloadCandidate(e ast.UtilityWhenExpr) bool {
	if _, ok := e.Else.(ast.CallExpr); ok {
		return true
	}
	for _, c := range e.Cases {
		if _, ok := c.Value.(ast.CallExpr); ok {
			return true
		}
	}
	return false
}

func (c *lowerCtx) lowerBatchExpr(e ast.BatchExpr) (string, string, bool, error) {
	input, inputType, _, err := c.lowerExpr(e.Input)
	if err != nil {
		return "", "", false, err
	}
	if !strings.HasSuffix(inputType, "[]") {
		return "", "", false, fmt.Errorf("batch input must be an array")
	}
	itemType := strings.TrimSuffix(inputType, "[]")
	worker, resultType, captureNames, err := c.lowerBatchWorker(e, itemType)
	if err != nil {
		return "", "", false, err
	}
	c.extra = append(c.extra, worker)

	raw := c.temp(fallibleType(resultType + "[]"))
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRBatchMap{
		Target:     raw,
		Input:      input,
		Worker:     worker.Package + "." + worker.Name,
		InputType:  itemType,
		ResultType: resultType,
		Captures:   captureNames,
	})
	value := c.temp(resultType + "[]")
	okID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", okID)})
	errID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", errID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: raw + ".IsErr", TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}

	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: value, Value: raw + ".Value"})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}

	c.cur = errID
	if c.fn.IsFallible {
		c.blocks[c.cur].Terminator = MIRReturn{Value: fallibleErrValue(c.retType, raw+".Err")}
	} else {
		c.blocks[c.cur].Terminator = MIRFail{Value: fmt.Sprintf("%q + %s.Err", "oct error: ", raw)}
	}
	c.cur = mergeID
	return value, resultType + "[]", false, nil
}

func (c *lowerCtx) lowerBatchWorker(e ast.BatchExpr, itemType string) (MIRFunction, string, []string, error) {
	name := fmt.Sprintf("__batch_%s_%d", c.fn.Name, c.batchID)
	c.batchID++
	const retPlaceholder = "__oct_batch_ret__"
	workerDecl := ast.FunctionDecl{Name: name, IsFallible: true, ErrorType: ast.TypeRef{Name: "Error"}}
	captureNames := make([]string, 0, len(c.locals))
	for localName := range c.locals {
		if localName == e.ItemName {
			continue
		}
		if strings.HasPrefix(localName, "_t") || strings.HasPrefix(localName, "__") {
			continue
		}
		captureNames = append(captureNames, localName)
	}
	sort.Strings(captureNames)
	workerLocals := make(map[string]string, len(captureNames)+1)
	workerGoNames := make(map[string]string, len(captureNames)+1)
	workerLocals[e.ItemName] = itemType
	workerGoNames[e.ItemName] = goIdent(e.ItemName)
	for _, captureName := range captureNames {
		workerLocals[captureName] = c.locals[captureName]
		workerGoNames[captureName] = c.goLocalName(captureName)
	}
	wctx := &lowerCtx{
		pkg:      c.pkg,
		program:  c.program,
		locals:   workerLocals,
		goNames:  workerGoNames,
		blocks:   []MIRBlock{{Label: "entry"}},
		cur:      0,
		retType:  retPlaceholder,
		fn:       workerDecl,
		einTerms: map[string]einsteinTermMeta{},
	}
	if err := wctx.lowerBlock(e.Body); err != nil {
		return MIRFunction{}, "", nil, err
	}
	if wctx.blocks[wctx.cur].Terminator == nil {
		return MIRFunction{}, "", nil, fmt.Errorf("batch body missing return")
	}
	if wctx.lastRet == "" {
		return MIRFunction{}, "", nil, fmt.Errorf("batch body return type could not be inferred")
	}
	params := []MIRField{{Name: goIdent(e.ItemName), Type: itemType}}
	for _, captureName := range captureNames {
		params = append(params, MIRField{Name: c.goLocalName(captureName), Type: c.locals[captureName]})
	}
	worker := MIRFunction{
		Package:    c.pkg.Name,
		Name:       name,
		Params:     params,
		Return:     wctx.lastRet,
		IsFallible: true,
		ErrorType:  "Error",
		Blocks:     patchBatchReturnType(wctx.blocks, retPlaceholder, wctx.lastRet),
	}
	paramNames := map[string]struct{}{}
	for _, param := range worker.Params {
		paramNames[param.Name] = struct{}{}
	}
	for n, t := range wctx.locals {
		if _, isParam := paramNames[wctx.goLocalName(n)]; isParam {
			continue
		}
		worker.Locals = append(worker.Locals, MIRField{Name: wctx.goLocalName(n), Type: t})
	}
	sort.Slice(worker.Locals, func(i, j int) bool { return worker.Locals[i].Name < worker.Locals[j].Name })
	return worker, wctx.lastRet, captureNames, nil
}

func patchBatchReturnType(blocks []MIRBlock, from, to string) []MIRBlock {
	out := make([]MIRBlock, len(blocks))
	for i, block := range blocks {
		out[i] = block
		if ret, ok := block.Terminator.(MIRReturn); ok {
			ret.Value = strings.ReplaceAll(ret.Value, from, to)
			out[i].Terminator = ret
		}
	}
	return out
}

func (c *lowerCtx) lookupRecordFieldType(recordType, fieldName string) (string, bool) {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName = parts[0]
		typeName = parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return "", false
	}
	for _, record := range pkg.Records {
		if record.Name != typeName {
			continue
		}
		for _, field := range record.Fields {
			if field.Name == fieldName {
				return typeRefStringForPackage(pkgName, field.Type), true
			}
		}
	}
	for _, flow := range pkg.Flows {
		if flow.Name+"BoardSnapshot" != typeName {
			continue
		}
		for _, field := range flow.Board {
			if field.Name == fieldName {
				return typeRefStringForPackage(pkgName, field.Type), true
			}
		}
		return "", false
	}
	return "", false
}

func (c *lowerCtx) lookupRecordFields(recordType string) ([]MIRField, bool) {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName = parts[0]
		typeName = parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return nil, false
	}
	for _, record := range pkg.Records {
		if record.Name != typeName {
			continue
		}
		fields := make([]MIRField, 0, len(record.Fields))
		for _, field := range record.Fields {
			fields = append(fields, MIRField{Name: field.Name, Type: typeRefStringForPackage(pkgName, field.Type)})
		}
		return fields, true
	}
	return nil, false
}

func (c *lowerCtx) lowerIfExpr(e ast.IfExpr) (string, string, bool, error) {
	cond, _, _, err := c.lowerExpr(e.Condition)
	if err != nil {
		return "", "", false, err
	}
	thenVal, thenType, _, err := c.lowerExpr(e.ThenExpr)
	if err != nil {
		return "", "", false, err
	}
	elseVal, _, _, err := c.lowerExpr(e.ElseExpr)
	if err != nil {
		return "", "", false, err
	}
	out := c.temp(thenType)
	thenID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", thenID)})
	elseID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", elseID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: cond, TrueTarget: c.blocks[thenID].Label, FalseTarget: c.blocks[elseID].Label}
	c.cur = thenID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: thenVal})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = elseID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: elseVal})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = mergeID
	return out, thenType, false, nil
}

func (c *lowerCtx) lowerSwitchExpr(e ast.SwitchExpr) (string, string, bool, error) {
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})

	var (
		subject    string
		out        string
		resultType string
	)
	if e.Subject != nil {
		var err error
		subject, _, _, err = c.lowerExpr(e.Subject)
		if err != nil {
			return "", "", false, err
		}
	}

	assignResult := func(valueExpr ast.Expr) error {
		value, valueType, _, err := c.lowerExpr(valueExpr)
		if err != nil {
			return err
		}
		if out == "" {
			out = c.temp(valueType)
			resultType = valueType
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: value})
		return nil
	}

	for _, switchCase := range e.Cases {
		var cond string
		if e.Subject == nil {
			condValue, _, _, err := c.lowerExpr(switchCase.Match)
			if err != nil {
				return "", "", false, err
			}
			cond = condValue
		} else {
			matchValue, _, _, err := c.lowerExpr(switchCase.Match)
			if err != nil {
				return "", "", false, err
			}
			cond = c.temp("Bool")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{
				Target: cond,
				Value:  fmt.Sprintf("(%s == %s)", subject, matchValue),
			})
		}

		matchID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", matchID)})
		nextID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", nextID)})
		c.blocks[c.cur].Terminator = MIRBranch{
			Cond:        cond,
			TrueTarget:  c.blocks[matchID].Label,
			FalseTarget: c.blocks[nextID].Label,
		}

		c.cur = matchID
		if err := assignResult(switchCase.Value); err != nil {
			return "", "", false, err
		}
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
		c.cur = nextID
	}

	if e.Else != nil {
		if err := assignResult(e.Else); err != nil {
			return "", "", false, err
		}
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	} else {
		c.blocks[c.cur].Terminator = MIRFail{Value: fmt.Sprintf("%q", "non-exhaustive switch reached in compiled mode")}
	}

	c.cur = mergeID
	return out, resultType, false, nil
}

func (c *lowerCtx) lowerMatchExpr(e ast.MatchExpr) (string, string, bool, error) {
	subject, subjectType, _, err := c.lowerExpr(e.Subject)
	if err != nil {
		return "", "", false, err
	}
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	var out, resultType string
	nextID := c.cur
	for i, matchCase := range e.Cases {
		matchID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", matchID)})
		fallthroughID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", fallthroughID)})
		cond := c.temp("Bool")
		c.blocks[nextID].Statements = append(c.blocks[nextID].Statements, MIRAssign{Target: cond, Value: fmt.Sprintf("(%s.Tag == %s_%s_tag)", subject, enumShortName(subjectType), matchCase.Variant)})
		c.blocks[nextID].Terminator = MIRBranch{Cond: cond, TrueTarget: c.blocks[matchID].Label, FalseTarget: c.blocks[fallthroughID].Label}
		c.cur = matchID
		if matchCase.Binding != "" {
			bindingType, ok := c.lookupEnumVariantPayloadType(subjectType, matchCase.Variant)
			if !ok {
				bindingType = "any"
			}
			c.locals[matchCase.Binding] = bindingType
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: matchCase.Binding, Value: fmt.Sprintf("%s.Payload.(%s)", subject, goType(bindingType))})
		}
		value, valueType, _, err := c.lowerExpr(matchCase.Value)
		if err != nil {
			return "", "", false, err
		}
		if out == "" {
			out = c.temp(valueType)
			resultType = valueType
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: value})
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
		nextID = fallthroughID
		if i == len(e.Cases)-1 {
			c.blocks[nextID].Terminator = MIRFail{Value: fmt.Sprintf("%q", "non-exhaustive match reached in compiled mode")}
		}
	}
	c.cur = mergeID
	return out, resultType, false, nil
}

func (c *lowerCtx) genericWrapperMetadataForCallee(callee ast.Expr) (genericWrapperCallMetadata, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		meta, ok := findGenericWrapperFunction(c.pkg, x.Name)
		return meta, ok, nil
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return genericWrapperCallMetadata{}, false, nil
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return genericWrapperCallMetadata{}, false, nil
		}
		meta, found := findGenericWrapperFunction(importPkg, x.Field)
		return meta, found, nil
	default:
		return genericWrapperCallMetadata{}, false, nil
	}
}

type compiledFunctionSignature struct {
	Parameters []string
	ReturnType string
	Fallible   bool
}

func (c *lowerCtx) resolveNamedFunctionValue(expr ast.Expr) (string, string, bool) {
	switch fn := expr.(type) {
	case ast.IdentifierExpr:
		for _, declared := range c.pkg.Functions {
			if declared.Name == fn.Name {
				return "fn_" + strings.ReplaceAll(c.pkg.Name+"."+fn.Name, ".", "_"), functionTypeStringForDecl(c.pkg.Name, declared), true
			}
		}
	case ast.FieldAccessExpr:
		pkgIdent, ok := fn.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", false
		}
		for _, declared := range importPkg.Functions {
			if declared.Name == fn.Field {
				return "fn_" + strings.ReplaceAll(pkgIdent.Name+"."+fn.Field, ".", "_"), functionTypeStringForDecl(pkgIdent.Name, declared), true
			}
		}
	}
	return "", "", false
}

func functionTypeStringForDecl(pkgName string, fn ast.FunctionDecl) string {
	parts := make([]string, 0, len(fn.Parameters))
	for _, param := range fn.Parameters {
		parts = append(parts, typeRefStringForPackage(pkgName, param.Type))
	}
	result := "fn(" + strings.Join(parts, ", ") + ") -> " + typeRefStringForPackage(pkgName, fn.ReturnType)
	if fn.IsFallible {
		result += " ! " + typeRefStringForPackage(pkgName, fn.ErrorType)
	}
	return result
}

func (c *lowerCtx) resolveFunctionValueCall(callee ast.Expr) (string, compiledFunctionSignature, bool, error) {
	if ident, ok := callee.(ast.IdentifierExpr); ok {
		if typ, local := c.locals[ident.Name]; local {
			signature, ok := parseCompiledFunctionType(typ)
			if !ok {
				return "", compiledFunctionSignature{}, false, nil
			}
			return c.goLocalName(ident.Name), signature, true, nil
		}
	}
	return "", compiledFunctionSignature{}, false, nil
}

func parseCompiledFunctionType(typ string) (compiledFunctionSignature, bool) {
	if !strings.HasPrefix(typ, "fn(") {
		return compiledFunctionSignature{}, false
	}
	arrowMarker := ") -> "
	arrow := strings.LastIndex(typ, arrowMarker)
	if arrow < 0 {
		return compiledFunctionSignature{}, false
	}
	paramsText := typ[len("fn("):arrow]
	rest := typ[arrow+len(arrowMarker):]
	fallible := false
	returnType := rest
	if bang := strings.Index(rest, " ! "); bang >= 0 {
		fallible = true
		returnType = rest[:bang]
	}
	params := []string{}
	if strings.TrimSpace(paramsText) != "" {
		for _, part := range strings.Split(paramsText, ",") {
			params = append(params, strings.TrimSpace(part))
		}
	}
	return compiledFunctionSignature{Parameters: params, ReturnType: strings.TrimSpace(returnType), Fallible: fallible}, true
}

func (c *lowerCtx) resolveCallArgTypes(callee ast.Expr) []string {
	pkgName := c.pkg.Name
	fnName := ""
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		fnName = x.Name
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return nil
		}
		pkgName = pkgIdent.Name
		fnName = x.Field
	default:
		return nil
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return nil
	}
	for _, fn := range pkg.Functions {
		if fn.Name != fnName {
			continue
		}
		out := make([]string, 0, len(fn.Parameters))
		for _, param := range fn.Parameters {
			out = append(out, typeRefStringForPackage(pkgName, param.Type))
		}
		return out
	}
	return nil
}

func goCoerceArg(expr string, actual string, expected string) string {
	if isIntScalarTypeString(actual) && isFloatLikeType(expected) {
		return fmt.Sprintf("float64(%s)", expr)
	}
	if isIntArrayTypeString(actual) && isFloatArrayTypeString(expected) {
		return fmt.Sprintf("__octIntArrayToFloat(%s)", expr)
	}
	if expected == "Complex" && isNumericTypeString(actual) {
		return fmt.Sprintf("complex(float64(%s), 0)", expr)
	}
	return expr
}

func isFloatLikeType(t string) bool {
	return t == "Float" || (strings.HasPrefix(t, "Float<") && strings.HasSuffix(t, ">"))
}

func (c *lowerCtx) resolveCall(callee ast.Expr) (string, string, bool, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		switch x.Name {
		case "Step", "Active", "Result", "Complete", "StateHistory", "ResumeTarget", "BoardSnapshot":
			switch x.Name {
			case "Step":
				return "Step", "Int", true, false, nil
			case "Active":
				return "Active", "String", true, false, nil
			case "Result":
				return "Result", "", true, true, nil
			case "Complete":
				return "Complete", "Bool", true, false, nil
			case "StateHistory":
				return "StateHistory", "String[]", true, false, nil
			case "ResumeTarget":
				return "ResumeTarget", "String", true, false, nil
			case "BoardSnapshot":
				return "BoardSnapshot", "", true, true, nil
			}
		}
		if x.Name == "Len" {
			return "Len", "Int", true, false, nil
		}
		if x.Name == "Append" {
			return "Append", "", true, false, nil
		}
		if x.Name == "Print" {
			return "Print", "Int", true, false, nil
		}
		if x.Name == "ToString" {
			return "ToString", "String", true, false, nil
		}
		if x.Name == "Float" {
			return "Float", "Float", true, false, nil
		}
		if x.Name == "Idx" {
			return "Idx", "Index", true, false, nil
		}
		if x.Name == "EinMul" || x.Name == "EinAdd" || x.Name == "EinSub" {
			return x.Name, "", true, false, nil
		}
		if x.Name == "Complex" {
			return "Complex", "Complex", true, false, nil
		}
		if x.Name == "fft" {
			return "fft", "Complex[]", true, true, nil
		}
		if x.Name == "Contains" || x.Name == "StartsWith" || x.Name == "EndsWith" {
			return x.Name, "Bool", true, false, nil
		}
		if x.Name == "Trim" || x.Name == "Lower" || x.Name == "Upper" || x.Name == "Join" {
			return x.Name, "String", true, false, nil
		}
		if x.Name == "TupleProbe" {
			return "TupleProbe", "(Int, Int)", true, false, nil
		}
		if x.Name == "BoolIntProbe" {
			return "BoolIntProbe", "(Bool, Int)", true, false, nil
		}
		if c.pkg.Name == "Random" {
			switch x.Name {
			case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
				resolved, ret, _, fallible, err := c.resolveCall(ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "Random"}, Field: x.Name})
				if err == nil {
					return resolved, ret, true, fallible, nil
				}
			}
		}
		for _, fn := range c.pkg.Functions {
			if fn.Name == x.Name {
				return c.pkg.Name + "." + x.Name, typeRefStringForPackage(c.pkg.Name, fn.ReturnType), false, fn.IsFallible, nil
			}
		}
		for _, flow := range c.pkg.Flows {
			if flow.Name == x.Name {
				return c.pkg.Name + "." + x.Name, flowInstanceTypeString(typeRefStringForPackage(c.pkg.Name, flow.ReturnType)), false, false, nil
			}
		}
		if builtin.IsName(x.Name) {
			normalized := x.Name
			if c.pkg.Name == "Random" {
				switch x.Name {
				case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
					normalized = "Random." + x.Name
				}
			}
			switch normalized {
			case "Random.RngSeed":
				return normalized, "Random.Rng", true, false, nil
			case "Random.RandInt":
				return normalized, "Random.RandIntResult", true, false, nil
			case "Random.RandFloat01", "Random.RandFloatRange", "Random.RandNormal", "Random.Gaussian":
				return normalized, "Random.RandFloatResult", true, false, nil
			case "Random.RandBernoulli":
				return normalized, "Random.RandBoolResult", true, false, nil
			case "Random.CryptoRandInt":
				return normalized, "Int", true, true, nil
			case "Random.CryptoRandFloat01":
				return normalized, "Float", true, true, nil
			case "Random.CryptoRandBytes":
				return normalized, "Bytes", true, true, nil
			case "StringByteLength", "StringRuneCount", "StringJoin", "StringConcat", "StringFrom", "StringReplaceAll", "StringContains", "StringStartsWith", "StringEndsWith", "StringTrim", "StringSplitLines", "StringEscapeJSON", "StringQuoteJSON":
				ret := "String"
				switch normalized {
				case "StringByteLength", "StringRuneCount":
					ret = "Int"
				case "StringContains", "StringStartsWith", "StringEndsWith":
					ret = "Bool"
				case "StringSplitLines":
					ret = "String[]"
				}
				return normalized, ret, true, false, nil
			case "MarkdownH1", "MarkdownH2", "MarkdownH3", "MarkdownParagraph", "MarkdownBlank", "MarkdownHorizontalRule", "MarkdownBullets", "MarkdownNumbered", "MarkdownCodeBlock", "MarkdownCallout", "MarkdownImage", "MarkdownFigure", "MarkdownTable", "MarkdownTableWithColumns", "MarkdownKeyValueTable", "MarkdownSection", "MarkdownSubsection", "MarkdownReport", "MarkdownEscapeText", "MarkdownEscapeTableCell":
				return normalized, compiledMarkdownBuiltinReturnType(normalized), true, false, nil
			case "RoundToInt", "FloorToInt", "CeilToInt":
				return normalized, "Int", true, false, nil
			case "Pi", "E", "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "Exp", "Ln", "Pow", "Log10", "Sinh", "Cosh", "Tanh", "BaseValue", "Clamp01":
				return normalized, "Float", true, false, nil
			case "Abs":
				return normalized, "Float", true, false, nil
			case "FormatFloat":
				return normalized, "String", true, false, nil
			case "Require":
				return normalized, "Void", true, false, nil
			case "FileReadText":
				return normalized, "String", true, true, nil
			case "FileReadBytes":
				return normalized, "Bytes", true, true, nil
			case "FileReadLines", "DirectoryList":
				return normalized, "String[]", true, true, nil
			case "FileWriteText", "FileWriteLines", "FileWriteBytes":
				return normalized, "Int", true, true, nil
			case "FileExists":
				return normalized, "Bool", true, false, nil
			case "FileDelete", "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll":
				return normalized, "Int", true, true, nil
			case "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad":
				return normalized, "String", true, true, nil
			case "JsonSave":
				return normalized, "Int", true, true, nil
			case "CsvRead", "CsvReadRows":
				return normalized, "String[][]", true, true, nil
			case "CsvReadTable":
				return normalized, "Csv.Table", true, true, nil
			case "CsvReadMatrix":
				return normalized, "Float[][]", true, true, nil
			case "CsvWrite", "CsvWriteRows":
				return normalized, "Int", true, true, nil
			case "PathJoin", "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
				return normalized, "String", true, false, nil
			default:
				return "", "", false, false, unsupportedBuiltin(x.Name)
			}
		}
		return "", "", false, false, fmt.Errorf("unknown function '%s'", x.Name)
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false, false, fmt.Errorf("unsupported call target")
		}
		builtinName := pkgIdent.Name + "." + x.Field
		if pkgIdent.Name == "Markdown" {
			if aliasName, mapped := builtin.ResolveNamespacedAlias(pkgIdent.Name, x.Field); mapped {
				return aliasName, compiledMarkdownBuiltinReturnType(aliasName), true, false, nil
			}
		}
		if builtin.IsName(builtinName) {
			canonical := canonicalCompiledBuiltinName(builtinName)
			if isMarkdownCompiledBuiltin(canonical) {
				return canonical, compiledMarkdownBuiltinReturnType(canonical), true, false, nil
			}
			switch builtinName {
			case "Random.RngSeed":
				return builtinName, "Random.Rng", true, false, nil
			case "Random.RandInt":
				return builtinName, "Random.RandIntResult", true, false, nil
			case "Random.RandFloat01":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.RandFloatRange":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.RandBernoulli":
				return builtinName, "Random.RandBoolResult", true, false, nil
			case "Random.RandNormal", "Random.Gaussian":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.CryptoRandInt":
				return builtinName, "Int", true, true, nil
			case "Random.CryptoRandFloat01":
				return builtinName, "Float", true, true, nil
			case "Random.CryptoRandBytes":
				return builtinName, "Bytes", true, true, nil
			default:
				ret := "String"
				switch canonical {
				case "StringByteLength", "StringRuneCount":
					ret = "Int"
				case "StringContains", "StringStartsWith", "StringEndsWith":
					ret = "Bool"
				case "StringSplitLines":
					ret = "String[]"
				}
				return canonical, ret, true, false, nil
			}
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", false, false, fmt.Errorf("unknown package '%s'", pkgIdent.Name)
		}
		for _, fn := range importPkg.Functions {
			if fn.Name == x.Field {
				return pkgIdent.Name + "." + x.Field, typeRefStringForPackage(pkgIdent.Name, fn.ReturnType), false, fn.IsFallible, nil
			}
		}
		return "", "", false, false, fmt.Errorf("unknown function '%s.%s'", pkgIdent.Name, x.Field)
	default:
		return "", "", false, false, fmt.Errorf("unsupported callee %T", callee)
	}
}

type genericWrapperCallMetadata struct {
	PackageName    string
	OctName        string
	Family         string
	WireName       string
	SidecarCommand string
	Args           []string
	Return         string
	Fallible       bool
	TransportTypes []project.TransportTypeMetadata
}

func findGenericWrapperFunction(pkg project.Package, fnName string) (genericWrapperCallMetadata, bool) {
	for _, wrapper := range pkg.Wrappers {
		for _, fn := range wrapper.Functions {
			if fn.OctName == fnName {
				return genericWrapperCallMetadata{
					PackageName:    pkg.Name,
					OctName:        fn.OctName,
					Family:         wrapper.Family,
					WireName:       fn.WireName,
					SidecarCommand: wrapper.SidecarCommand,
					Args:           append([]string(nil), fn.Args...),
					Return:         fn.Return,
					Fallible:       fn.Fallible,
					TransportTypes: append([]project.TransportTypeMetadata(nil), wrapper.TransportTypes...),
				}, true
			}
		}
	}
	return genericWrapperCallMetadata{}, false
}

func wrapperArgTypeMatches(manifestType string, actualType string) bool {
	if manifestType == actualType {
		return true
	}
	if strings.HasPrefix(manifestType, "Int<") && strings.HasSuffix(manifestType, ">") && actualType == "Int" {
		return true
	}
	return false
}

func isOctxiliaryTransportType(t string) bool {
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return true
	}
	switch t {
	case "Void", "Int", "Float", "Bool", "String", "String[]", "String[][]", "Float[]", "Bytes":
		return true
	default:
		return false
	}
}

type transportRecordLookup struct {
	typ project.TransportTypeMetadata
	ok  bool
}

func findTransportRecord(types []project.TransportTypeMetadata, name string) transportRecordLookup {
	for _, typ := range types {
		if typ.Name == name {
			return transportRecordLookup{typ: typ, ok: true}
		}
	}
	return transportRecordLookup{}
}

func transportRuntimeBaseType(t string) string {
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return "Int"
	}
	return t
}

func flattenDirectCallName(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		left, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", false
		}
		return left.Name + "." + node.Field, true
	default:
		return "", false
	}
}

func (c *lowerCtx) resolveMatrixTabulateCallback(expr ast.Expr) (string, string, error) {
	switch fn := expr.(type) {
	case ast.IdentifierExpr:
		for _, declared := range c.pkg.Functions {
			if declared.Name == fn.Name {
				return "fn_" + strings.ReplaceAll(c.pkg.Name+"."+fn.Name, ".", "_"), typeRefStringForPackage(c.pkg.Name, declared.ReturnType), nil
			}
		}
		return "", "", fmt.Errorf("Matrix.tabulate callback '%s' must be a named function", fn.Name)
	case ast.FieldAccessExpr:
		pkgIdent, ok := fn.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", fmt.Errorf("Matrix.tabulate callback must be a named function")
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", fmt.Errorf("unknown package '%s'", pkgIdent.Name)
		}
		for _, declared := range importPkg.Functions {
			if declared.Name == fn.Field {
				return "fn_" + strings.ReplaceAll(pkgIdent.Name+"."+fn.Field, ".", "_"), typeRefStringForPackage(pkgIdent.Name, declared.ReturnType), nil
			}
		}
		return "", "", fmt.Errorf("unknown function '%s.%s'", pkgIdent.Name, fn.Field)
	default:
		return "", "", fmt.Errorf("Matrix.tabulate callback must be a named function")
	}
}

func (c *lowerCtx) flattenEnumVariantExpr(expr ast.FieldAccessExpr) (string, string, bool) {
	enumType, ok := flattenEnumTypeExpr(expr.Target)
	if !ok {
		return "", "", false
	}
	return enumType, expr.Field, true
}

func flattenEnumTypeExpr(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		pkgIdent, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", false
		}
		return pkgIdent.Name + "." + node.Field, true
	default:
		return "", false
	}
}

func (c *lowerCtx) resolveEnumVariantValue(enumType string, variant string) (string, string, bool, error) {
	enumPkg := c.pkg.Name
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := c.program.Packages[enumPkg]
	if !ok {
		return "", "", false, nil
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == variant {
				return fmt.Sprintf("%s_%s{Tag: %s_%s_tag}", enumPkg, enumName, enumName, variant), enumPkg + "." + enumName, true, nil
			}
		}
		return "", "", true, fmt.Errorf("enum '%s' has no variant '%s'", enumType, variant)
	}
	return "", "", false, nil
}

func (c *lowerCtx) resolveEnumVariantConstructor(enumType string, variant string, args []ast.Expr) (string, string, bool, error) {
	enumPkg := c.pkg.Name
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := c.program.Packages[enumPkg]
	if !ok {
		return "", "", false, nil
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name != variant {
				continue
			}
			if declaredVariant.Payload == nil {
				if len(args) != 0 {
					return "", "", true, fmt.Errorf("enum '%s' variant '%s' does not accept a payload", enumType, variant)
				}
				return fmt.Sprintf("%s_%s{Tag: %s_%s_tag}", enumPkg, enumName, enumName, variant), enumPkg + "." + enumName, true, nil
			}
			if len(args) != 1 {
				return "", "", true, fmt.Errorf("enum '%s' variant '%s' requires exactly 1 payload argument", enumType, variant)
			}
			payload, _, _, err := c.lowerExpr(args[0])
			if err != nil {
				return "", "", true, err
			}
			return fmt.Sprintf("%s_%s{Tag: %s_%s_tag, Payload: %s}", enumPkg, enumName, enumName, variant, payload), enumPkg + "." + enumName, true, nil
		}
		return "", "", true, fmt.Errorf("enum '%s' has no variant '%s'", enumType, variant)
	}
	return "", "", false, nil
}

func enumShortName(enumType string) string {
	if dot := strings.Index(enumType, "."); dot >= 0 {
		return enumType[dot+1:]
	}
	return enumType
}

func (c *lowerCtx) lookupEnumVariantPayloadType(enumType string, variant string) (string, bool) {
	return lookupEnumVariantPayloadTypeForProgram(c.program, c.pkg.Name, enumType, variant)
}

func lookupEnumVariantPayloadTypeForProgram(program project.Program, currentPkg string, enumType string, variant string) (string, bool) {
	enumPkg := currentPkg
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := program.Packages[enumPkg]
	if !ok {
		return "", false
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == variant && declaredVariant.Payload != nil {
				return typeRefStringForPackage(enumPkg, *declaredVariant.Payload), true
			}
		}
	}
	return "", false
}

func lowerFlowEnumVariantExpr(program project.Program, currentPkg string, enumName string, variant string) (string, string, bool) {
	enumPkg := currentPkg
	localEnumName := enumName
	if dot := strings.Index(enumName, "."); dot >= 0 {
		enumPkg = enumName[:dot]
		localEnumName = enumName[dot+1:]
	}
	pkg, ok := program.Packages[enumPkg]
	if !ok {
		return "", "", false
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != localEnumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == variant {
				return fmt.Sprintf("%s_%s{Tag: %s_%s_tag}", enumPkg, localEnumName, localEnumName, variant), enumPkg + "." + localEnumName, true
			}
		}
	}
	return "", "", false
}

func typeRefString(t ast.TypeRef) string {
	return typeRefStringForPackage("", t)
}

func typeRefStringForPackage(currentPkg string, t ast.TypeRef) string {
	if t.Function != nil {
		parts := make([]string, 0, len(t.Function.Parameters))
		for _, param := range t.Function.Parameters {
			parts = append(parts, typeRefStringForPackage(currentPkg, param))
		}
		result := "fn(" + strings.Join(parts, ", ") + ") -> " + typeRefStringForPackage(currentPkg, t.Function.ReturnType)
		if t.Function.IsFallible {
			errorType := "Error"
			if t.Function.ErrorType != nil {
				errorType = typeRefStringForPackage(currentPkg, *t.Function.ErrorType)
			}
			result += " ! " + errorType
		}
		return result
	}
	if len(t.TupleOf) > 0 {
		parts := make([]string, 0, len(t.TupleOf))
		for _, elem := range t.TupleOf {
			parts = append(parts, typeRefStringForPackage(currentPkg, elem))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	if t.VectorOf != nil {
		return "Vector<" + typeRefStringForPackage(currentPkg, *t.VectorOf) + ">"
	}
	if t.MatrixOf != nil {
		return "Matrix<" + typeRefStringForPackage(currentPkg, *t.MatrixOf) + ">"
	}
	base := t.Name
	if t.Package != "" {
		base = t.Package + "." + base
	} else if currentPkg != "" && base != "" && !isBuiltinTypeName(base) {
		base = currentPkg + "." + base
	}
	if base == "" {
		base = "Void"
	}
	if t.HasUnit {
		base = fmt.Sprintf("%s<%s>", base, t.Dimension.String())
	}
	if t.IsArray || t.ArrayDepth > 0 {
		depth := t.ArrayDepth
		if depth == 0 {
			depth = 1
		}
		return base + strings.Repeat("[]", depth)
	}
	return base
}

func isBuiltinTypeName(name string) bool {
	switch name {
	case "Int", "Float", "Complex", "Bool", "String", "Index", "Bytes", "Error", "Void":
		return true
	default:
		return false
	}
}

func flowInstanceTypeString(resultType string) string {
	return "FlowInstance<" + resultType + ">"
}

func parseGenericType(input, base string) (string, bool) {
	prefix := base + "<"
	if !strings.HasPrefix(input, prefix) || !strings.HasSuffix(input, ">") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(input, prefix), ">"), true
}

func parseVectorElemType(t string) (string, bool) {
	return parseGenericType(t, "Vector")
}

func parseMatrixElemType(t string) (string, bool) {
	return parseGenericType(t, "Matrix")
}

func parseArrayElemType(t string) (string, bool) {
	if !strings.HasSuffix(t, "[]") {
		return "", false
	}
	return strings.TrimSuffix(t, "[]"), true
}

func isFloatScalarTypeString(t string) bool {
	return t == "Float" || (strings.HasPrefix(t, "Float<") && strings.HasSuffix(t, ">"))
}

func isIntScalarTypeString(t string) bool {
	return t == "Int" || (strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">"))
}

func isFloatArrayTypeString(t string) bool {
	elem, ok := parseArrayElemType(t)
	return ok && isFloatScalarTypeString(elem)
}

func isIntArrayTypeString(t string) bool {
	elem, ok := parseArrayElemType(t)
	return ok && isIntScalarTypeString(elem)
}

func isNumericTypeString(t string) bool {
	return isIntScalarTypeString(t) || isFloatScalarTypeString(t)
}

func isLinearElementwiseOperatorString(operator string) bool {
	return operator == "+" || operator == "-" || operator == "*" || operator == "/"
}

func scalarBinaryResultTypeString(operator string, leftType string, rightType string) string {
	switch operator {
	case "+", "-", "*", "/":
		if isFloatScalarTypeString(leftType) || isFloatScalarTypeString(rightType) {
			if strings.HasPrefix(leftType, "Float<") && strings.HasSuffix(leftType, ">") {
				return leftType
			}
			if strings.HasPrefix(rightType, "Float<") && strings.HasSuffix(rightType, ">") {
				return rightType
			}
			return "Float"
		}
	}
	return leftType
}

func isComplexScalarTypeString(t string) bool {
	return t == "Complex"
}

func isComplexCompatibleScalarTypeString(t string) bool {
	return isNumericTypeString(t) || isComplexScalarTypeString(t)
}

func unifyLinearElemType(leftElem, rightElem string) string {
	if strings.HasPrefix(leftElem, "Float<") || strings.HasPrefix(rightElem, "Float<") {
		if strings.HasPrefix(leftElem, "Float<") {
			return leftElem
		}
		return rightElem
	}
	if leftElem == "Float" || rightElem == "Float" {
		return "Float"
	}
	return leftElem
}

func parseFlowInstanceType(t string) (string, bool) {
	if !strings.HasPrefix(t, "FlowInstance<") || !strings.HasSuffix(t, ">") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(t, "FlowInstance<"), ">"), true
}

func parseTupleTypeString(t string) ([]string, bool) {
	if !strings.HasPrefix(t, "(") || !strings.HasSuffix(t, ")") {
		return nil, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "("), ")"))
	if inner == "" {
		return nil, false
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		part := strings.TrimSpace(p)
		if part == "" {
			return nil, false
		}
		out = append(out, part)
	}
	if len(out) < 2 {
		return nil, false
	}
	return out, true
}

func compiledBuiltinReturnType(name string, argTypes []string) (string, error) {
	name = canonicalCompiledBuiltinName(name)
	switch name {
	case "Idx":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function 'Idx' expects 1 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin Idx for type %s", argTypes[0])
		}
		return "Index", nil
	case "EinMul", "EinAdd", "EinSub":
		if len(argTypes) != 6 {
			return "", fmt.Errorf("function '%s' expects 6 arguments, got %d", name, len(argTypes))
		}
		leftElem, leftMatrix := parseMatrixElemType(argTypes[0])
		if !leftMatrix {
			return "", fmt.Errorf("function '%s' argument 1 expects Matrix, got %s", name, argTypes[0])
		}
		rightElem, rightMatrix := parseMatrixElemType(argTypes[3])
		if !rightMatrix {
			return "", fmt.Errorf("function '%s' argument 4 expects Matrix, got %s", name, argTypes[3])
		}
		for _, pos := range []int{1, 2, 4, 5} {
			if argTypes[pos] != "Index" {
				return "", fmt.Errorf("function '%s' argument %d expects Index, got %s", name, pos+1, argTypes[pos])
			}
		}
		return "Matrix<" + unifyLinearElemType(leftElem, rightElem) + ">", nil
	case "fft":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function 'fft' expects 1 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "Complex[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin fft for type %s", argTypes[0])
		}
		return "Complex[]", nil
	case "Require":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'Require' expects 2 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "Bool" {
			return "", fmt.Errorf("compiled mode does not yet support builtin Require for type %s", argTypes[0])
		}
		if argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin Require for type %s", argTypes[1])
		}
		return "Void", nil
	case "FormatFloat":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'FormatFloat' expects 2 arguments, got %d", len(argTypes))
		}
		if !isFloatScalarTypeString(argTypes[0]) {
			return "", fmt.Errorf("compiled mode does not yet support builtin FormatFloat for type %s", argTypes[0])
		}
		if !isIntScalarTypeString(argTypes[1]) {
			return "", fmt.Errorf("compiled mode does not yet support builtin FormatFloat for type %s", argTypes[1])
		}
		return "String", nil
	case "Pi", "E":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "Float", nil
	case "Abs":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return argTypes[0], nil
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Abs for type %s", argTypes[0])
	case "I":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "Complex", nil
	case "Real", "Imag", "Arg":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Conj":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Complex", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Exp", "Ln", "Log10", "Sinh", "Cosh", "Tanh", "FloorToInt", "CeilToInt", "RoundToInt", "Math.FloorToInt", "Math.CeilToInt", "BaseValue":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if name == "FloorToInt" || name == "CeilToInt" || name == "RoundToInt" || name == "Math.FloorToInt" || name == "Math.CeilToInt" || name == "BaseValue" {
			if isFloatScalarTypeString(argTypes[0]) {
				if name == "FloorToInt" || name == "CeilToInt" || name == "RoundToInt" || name == "Math.FloorToInt" || name == "Math.CeilToInt" {
					return "Int", nil
				}
				return "Float", nil
			}
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		if name == "Exp" || name == "Ln" {
			if isComplexScalarTypeString(argTypes[0]) {
				return "Complex", nil
			}
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Atan2":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[idx])
			}
		}
		return "Float", nil
	case "Pow":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[idx])
			}
		}
		return "Float", nil
	case "ComplexPolar":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'ComplexPolar' expects 2 arguments, got %d", len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin ComplexPolar for type %s", argTypes[idx])
			}
		}
		return "Complex", nil
	case "Complex":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'Complex' expects 2 arguments, got %d", len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin Complex for type %s", argTypes[idx])
			}
		}
		return "Complex", nil
	case "Trace":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		elemType, ok := parseMatrixElemType(argTypes[0])
		if !ok {
			return "", fmt.Errorf("compiled mode does not yet support builtin Trace for type %s", argTypes[0])
		}
		return elemType, nil
	case "Grad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return "Matrix<" + elemType + ">", nil
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return "Vector<" + argTypes[0] + ">", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Grad for type %s", argTypes[0])
	case "Div":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseMatrixElemType(argTypes[0]); ok {
			return "Vector<" + elemType + ">", nil
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return elemType, nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Div for type %s", argTypes[0])
	case "SymGrad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return "Matrix<" + elemType + ">", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin SymGrad for type %s", argTypes[0])
	case "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "JsonSave":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "CsvRead", "CsvReadRows":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[][]", nil
	case "CsvReadTable":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Csv.Table", nil
	case "CsvReadMatrix":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Float[][]", nil
	case "CsvWrite", "CsvWriteRows":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "FileReadText":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil

	case "FileReadBytes":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Bytes", nil

	case "FileReadLines", "DirectoryList":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil

	case "FileWriteText", "FileWriteLines", "FileWriteBytes":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || (name == "FileWriteText" && argTypes[1] != "String") || (name == "FileWriteLines" && argTypes[1] != "String[]") || (name == "FileWriteBytes" && argTypes[1] != "Bytes") {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "FileExists":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Bool", nil
	case "FileDelete", "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "PathJoin":
		if len(argTypes) != 1 || argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %v", name, argTypes)
		}
		return "String", nil
	case "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
		if len(argTypes) != 1 || argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %v", name, argTypes)
		}
		return "String", nil
	case "StringByteLength":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "StringRuneCount":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "StringConcat":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "StringFrom":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		switch argTypes[0] {
		case "Int", "Float", "Bool", "String":
			return "String", nil
		default:
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
	case "StringJoin":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String", nil
	case "StringReplaceAll":
		if len(argTypes) != 3 {
			return "", fmt.Errorf("function '%s' expects 3 arguments, got %d", name, len(argTypes))
		}
		for _, t := range argTypes {
			if t != "String" {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, t)
			}
		}
		return "String", nil
	case "StringContains", "StringStartsWith", "StringEndsWith":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || (name == "FileWriteText" && argTypes[1] != "String") || (name == "FileWriteLines" && argTypes[1] != "String[]") {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Bool", nil
	case "StringTrim", "StringEscapeJSON", "StringQuoteJSON":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "StringSplitLines":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownEscapeText", "MarkdownEscapeTableCell":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "MarkdownH1", "MarkdownH2", "MarkdownH3", "MarkdownParagraph":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownBlank", "MarkdownHorizontalRule":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "String[]", nil
	case "MarkdownBullets", "MarkdownNumbered":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownCodeBlock", "MarkdownCallout":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownImage", "MarkdownFigure":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownKeyValueTable":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" || argTypes[1] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownReport":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownSection", "MarkdownSubsection":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownTable":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] == "String[][]" || argTypes[0] == "String[]" || argTypes[0] == "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownTableWithColumns":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[1] != "String[]" || argTypes[0] == "String[][]" || argTypes[0] == "String[]" || argTypes[0] == "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	default:
		return "", fmt.Errorf("compiled mode does not yet support builtin %s", name)
	}
}

var flowLowerProgram project.Program

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func lowerFlow(program project.Program, pkgName string, flow ast.FlowDecl, pkg project.Package) (MIRFlow, error) {
	flowLowerProgram = program
	env := map[string]string{}
	locals := map[string]bool{}
	boardFieldTypes := map[string]string{}
	for _, p := range flow.Parameters {
		env[p.Name] = typeRefStringForPackage(pkgName, p.Type)
	}
	out := MIRFlow{
		Package:    pkgName,
		Name:       flow.Name,
		Return:     typeRefStringForPackage(pkgName, flow.ReturnType),
		EntryState: flow.EntryState,
	}
	for _, p := range flow.Parameters {
		out.Parameters = append(out.Parameters, MIRField{Name: p.Name, Type: typeRefStringForPackage(pkgName, p.Type)})
	}
	for _, field := range flow.Board {
		fieldType := typeRefStringForPackage(pkgName, field.Type)
		out.Board = append(out.Board, MIRField{Name: field.Name, Type: fieldType})
		boardFieldTypes[field.Name] = fieldType
	}
	if len(flow.Board) > 0 {
		env["board"] = "__flow_board_" + flow.Name
	}
	for _, st := range flow.States {
		stateEnv := cloneFlowEnv(env)
		stateLocals := cloneFlowLocals(locals)
		lowered, err := lowerFlowBlock(st.Body, stateEnv, stateLocals, pkg.Name, boardFieldTypes)
		if err != nil {
			return MIRFlow{}, fmt.Errorf("state %s: %w", st.Name, err)
		}
		out.States = append(out.States, MIRFlowState{Name: st.Name, Statements: lowered})
	}
	return out, nil
}

func cloneFlowEnv(env map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	return out
}

func cloneFlowLocals(locals map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range locals {
		out[k] = v
	}
	return out
}

func lowerFlowBlock(block ast.Block, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) ([]MIRFlowStmt, error) {
	out := make([]MIRFlowStmt, 0, len(block.Statements))
	for _, stmt := range block.Statements {
		s, err := lowerFlowStmt(stmt, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func lowerFlowStmt(stmt ast.Stmt, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowStmt, error) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if _, exists := env[s.Name]; exists {
			return nil, fmt.Errorf("flow local '%s' conflicts with existing binding", s.Name)
		}
		v, t, fallible, err := lowerFlowExprTyped(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		if fallible {
			return nil, fmt.Errorf("fallible calls are not supported in compiled flow let bindings; handle outside the flow or use non-fallible helper")
		}
		if s.TypeHint != nil {
			hint := typeRefStringForPackage(pkg, *s.TypeHint)
			if hint != t {
				return nil, fmt.Errorf("flow let '%s' expected %s, got %s", s.Name, hint, t)
			}
		}
		env[s.Name] = t
		locals[s.Name] = true
		return MIRFlowLetStmt{Name: s.Name, Type: t, Value: v}, nil
	case ast.GotoStmt:
		return MIRFlowGoto{Target: s.Target}, nil
	case ast.SuspendStmt:
		return MIRFlowSuspend{}, nil
	case ast.RememberStmt:
		return MIRFlowRemember{}, nil
	case ast.ResumeStmt:
		return MIRFlowResume{}, nil
	case ast.FieldAssignStmt:
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowFieldAssign{Target: s.Target, Field: s.Field, Value: v}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return MIRFlowReturn{}, nil
		}
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowReturn{Value: v}, nil
	case ast.IfStmt:
		cond, err := lowerFlowExpr(s.Condition, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		thenBody, err := lowerFlowBlock(s.ThenBody, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		var elseBody []MIRFlowStmt
		if s.ElseBody != nil {
			elseBody, err = lowerFlowBlock(*s.ElseBody, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
		}
		return MIRFlowIf{Condition: cond, Then: thenBody, Else: elseBody}, nil
	case ast.WhenStmt:
		cases := make([]MIRFlowWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			cond, err := lowerFlowExpr(c.Condition, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			action, err := lowerFlowWhenAction(c.Action, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowWhenCase{Condition: cond, Action: action})
		}
		elseAction, err := lowerFlowWhenAction(s.Else, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhen{Cases: cases, Else: elseAction}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow statement %T", stmt))
	}
}

func lowerFlowWhenAction(action ast.WhenAction, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowWhenAction, error) {
	switch a := action.(type) {
	case ast.WhenGotoAction:
		return MIRFlowWhenGoto{Target: a.Target}, nil
	case ast.WhenSuspendAction:
		return MIRFlowWhenSuspend{}, nil
	case ast.WhenReturnAction:
		v, err := lowerFlowExpr(a.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhenReturn{Value: v}, nil
	case ast.WhenBlockAction:
		statements := make([]MIRFlowStmt, 0, len(a.Statements))
		for _, statement := range a.Statements {
			lowered, err := lowerFlowStmt(statement, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			statements = append(statements, lowered)
		}
		return MIRFlowWhenBlock{Statements: statements}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow when action %T", action))
	}
}

func lowerFlowExprTyped(expr ast.Expr, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowExpr, string, bool, error) {
	v, err := lowerFlowExpr(expr, env, locals, pkg, boardFieldTypes)
	if err != nil {
		return nil, "", false, err
	}
	t, err := inferFlowExprType(expr, env, pkg, boardFieldTypes)
	if err != nil {
		return nil, "", false, err
	}
	fallible := false
	if call, ok := expr.(ast.CallExpr); ok {
		if calleeField, ok := call.Callee.(ast.FieldAccessExpr); ok {
			if enumType, variant, ok := flattenFlowEnumValueExpr(calleeField); ok {
				if _, _, enumFound := lowerFlowEnumVariantExpr(flowLowerProgram, pkg, enumType, variant); enumFound {
					return v, t, false, nil
				}
			}
		}
		_, _, _, isFallible, err := resolveFlowCall(call.Callee, pkg)
		if err != nil {
			return nil, "", false, err
		}
		fallible = isFallible
	}
	return v, t, fallible, nil
}

func lowerFlowExpr(expr ast.Expr, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowExpr, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		return MIRFlowLiteralExpr{Value: e.Value}, nil
	case ast.FloatLiteral:
		return MIRFlowLiteralExpr{Value: e.Value}, nil
	case ast.BoolLiteral:
		if e.Value {
			return MIRFlowLiteralExpr{Value: "true"}, nil
		}
		return MIRFlowLiteralExpr{Value: "false"}, nil
	case ast.StringLiteralExpr:
		return MIRFlowLiteralExpr{Value: fmt.Sprintf("%q", e.Value)}, nil
	case ast.IdentifierExpr:
		if _, ok := env[e.Name]; !ok {
			return nil, fmt.Errorf("unknown identifier '%s'", e.Name)
		}
		return MIRFlowIdentifierExpr{Name: e.Name, IsLocal: locals[e.Name]}, nil
	case ast.BinaryExpr:
		l, err := lowerFlowExpr(e.Left, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		r, err := lowerFlowExpr(e.Right, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowBinaryExpr{Left: l, Operator: e.Operator, Right: r}, nil
	case ast.UnaryExpr:
		v, err := lowerFlowExpr(e.Operand, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowUnaryExpr{Operator: e.Operator, Operand: v}, nil
	case ast.CallExpr:
		if calleeField, ok := e.Callee.(ast.FieldAccessExpr); ok {
			if enumType, variant, ok := flattenFlowEnumValueExpr(calleeField); ok {
				enumExpr, _, enumFound := lowerFlowEnumVariantExpr(flowLowerProgram, pkg, enumType, variant)
				if enumFound {
					_, hasPayload := lookupEnumVariantPayloadTypeForProgram(flowLowerProgram, pkg, enumType, variant)
					if !hasPayload {
						if len(e.Arguments) != 0 {
							return nil, fmt.Errorf("enum '%s' variant '%s' does not accept a payload", enumType, variant)
						}
						return MIRFlowLiteralExpr{Value: enumExpr}, nil
					}
					if len(e.Arguments) != 1 {
						return nil, fmt.Errorf("enum '%s' variant '%s' requires exactly 1 payload argument", enumType, variant)
					}
					payloadExpr, err := lowerFlowExpr(e.Arguments[0], env, locals, pkg, boardFieldTypes)
					if err != nil {
						return nil, err
					}
					payload, err := emitGoFlowExpr(payloadExpr, pkg)
					if err != nil {
						return nil, err
					}
					return MIRFlowLiteralExpr{Value: fmt.Sprintf("%s_%s{Tag: %s_%s_tag, Payload: %s}", enumPackageName(enumType, pkg), enumShortName(enumType), enumShortName(enumType), variant, payload)}, nil
				}
			}
		}
		callee, ret, builtin, fallible, err := resolveFlowCall(e.Callee, pkg)
		if err != nil {
			return nil, err
		}
		if fallible {
			return nil, fmt.Errorf("fallible calls are not supported in compiled flow expressions; handle outside the flow or use non-fallible helper")
		}
		args := make([]MIRFlowExpr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			v, err := lowerFlowExpr(arg, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		return MIRFlowCallExpr{Callee: callee, Args: args, Builtin: builtin, RetType: ret, Fallible: fallible}, nil
	case ast.IndexExpr:
		if len(e.Indices) != 1 {
			return nil, unsupported("compiled flow expression indexing only supports single-dimension indexing")
		}
		targetType, err := inferFlowExprType(e.Target, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(targetType, "[]") {
			return nil, unsupported(fmt.Sprintf("compiled flow expression indexing target type %q", targetType))
		}
		idxType, err := inferFlowExprType(e.Indices[0], env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		if idxType != "Int" {
			return nil, fmt.Errorf("compiled flow expression index must be Int, got %s", idxType)
		}
		target, err := lowerFlowExpr(e.Target, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		idx, err := lowerFlowExpr(e.Indices[0], env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowIndexExpr{
			Target:     target,
			Index:      idx,
			ResultType: strings.TrimSuffix(targetType, "[]"),
		}, nil
	case ast.RecordLiteralExpr:
		typeName := e.TypeName
		if !strings.Contains(typeName, ".") {
			typeName = pkg + "." + typeName
		}
		fieldValues := make([]MIRFlowExpr, 0, len(e.Fields))
		fieldNames := make([]string, 0, len(e.Fields))
		for _, f := range e.Fields {
			v, err := lowerFlowExpr(f.Value, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			fieldValues = append(fieldValues, v)
			fieldNames = append(fieldNames, f.Name)
		}
		return MIRFlowRecordLiteralExpr{
			TypeName:   typeName,
			FieldNames: fieldNames,
			FieldVals:  fieldValues,
			RetType:    typeName,
		}, nil
	case ast.UtilityWhenExpr:
		if e.EnumTarget != nil && utilityWhenHasPayloadCandidate(e) {
			return nil, unsupported("compiled enum-targeted utility payload candidates require delayed payload lowering")
		}
		resultType, err := inferFlowExprType(e.Else, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		h, err := lowerFlowExpr(e.Policy.Hysteresis, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		m, err := lowerFlowExpr(e.Policy.MinCommit, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		cases := make([]MIRFlowUtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			val, err := lowerFlowExpr(c.Value, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cond, err := lowerFlowExpr(c.Condition, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			score, err := lowerFlowExpr(c.Score, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowUtilityCase{Value: val, Condition: cond, Score: score})
		}
		elseExpr, err := lowerFlowExpr(e.Else, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowUtilityWhenExpr{
			SiteID:          e.SiteID,
			ControllerBound: e.ControllerBound,
			ResultType:      resultType,
			Hysteresis:      h,
			MinCommit:       m,
			Cases:           cases,
			Else:            elseExpr,
		}, nil
	case ast.SwitchExpr:
		resultType, err := inferFlowExprType(expr, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		var subject MIRFlowExpr
		if e.Subject != nil {
			subject, err = lowerFlowExpr(e.Subject, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
		}
		cases := make([]MIRFlowSwitchCase, 0, len(e.Cases))
		for _, switchCase := range e.Cases {
			matchExpr, err := lowerFlowExpr(switchCase.Match, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			valueExpr, err := lowerFlowExpr(switchCase.Value, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowSwitchCase{Match: matchExpr, Value: valueExpr})
		}
		var elseExpr MIRFlowExpr
		if e.Else != nil {
			elseExpr, err = lowerFlowExpr(e.Else, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
		}
		return MIRFlowSwitchExpr{Subject: subject, Cases: cases, Else: elseExpr, ResultType: resultType}, nil
	case ast.MatchExpr:
		resultType, err := inferFlowExprType(expr, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		subjectType, err := inferFlowExprType(e.Subject, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		subject, err := lowerFlowExpr(e.Subject, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		cases := make([]MIRFlowMatchCase, 0, len(e.Cases))
		for _, matchCase := range e.Cases {
			caseLocals := cloneBoolMap(locals)
			caseEnv := cloneStringMap(env)
			bindingType := ""
			if matchCase.Binding != "" {
				if t, ok := lookupEnumVariantPayloadTypeForProgram(flowLowerProgram, pkg, subjectType, matchCase.Variant); ok {
					bindingType = t
				} else {
					bindingType = "any"
				}
				caseLocals[matchCase.Binding] = true
				caseEnv[matchCase.Binding] = bindingType
			}
			valueExpr, err := lowerFlowExpr(matchCase.Value, caseEnv, caseLocals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowMatchCase{
				Variant:     matchCase.Variant,
				Binding:     matchCase.Binding,
				BindingType: bindingType,
				Value:       valueExpr,
			})
		}
		return MIRFlowMatchExpr{Subject: subject, SubjectType: subjectType, Cases: cases, ResultType: resultType}, nil
	case ast.IfExpr:
		resultType, err := inferFlowExprType(expr, env, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		condition, err := lowerFlowExpr(e.Condition, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		thenExpr, err := lowerFlowExpr(e.ThenExpr, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		elseExpr, err := lowerFlowExpr(e.ElseExpr, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowIfExpr{Condition: condition, Then: thenExpr, Else: elseExpr, ResultType: resultType}, nil
	case ast.ParenExpr:
		return lowerFlowExpr(e.Inner, env, locals, pkg, boardFieldTypes)
	case ast.FieldAccessExpr:
		targetIdent, ok := e.Target.(ast.IdentifierExpr)
		if !ok {
			return nil, unsupported(fmt.Sprintf("flow field access target %T", e.Target))
		}
		if targetIdent.Name == "board" {
			if _, ok := boardFieldTypes[e.Field]; !ok {
				return nil, fmt.Errorf("board has no field '%s'", e.Field)
			}
		}
		if _, ok := env[targetIdent.Name]; !ok {
			if enumExpr, _, ok := lowerFlowEnumVariantExpr(flowLowerProgram, pkg, targetIdent.Name, e.Field); ok {
				return MIRFlowLiteralExpr{Value: enumExpr}, nil
			}
		}
		return MIRFlowFieldExpr{Target: targetIdent.Name, Field: e.Field}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow expression %T", expr))
	}
}

func inferFlowExprType(expr ast.Expr, env map[string]string, pkg string, boardFieldTypes map[string]string) (string, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		return "Int", nil
	case ast.FloatLiteral:
		return "Float", nil
	case ast.BoolLiteral:
		return "Bool", nil
	case ast.StringLiteralExpr:
		return "String", nil
	case ast.IdentifierExpr:
		t, ok := env[e.Name]
		if !ok {
			return "", fmt.Errorf("unknown identifier '%s'", e.Name)
		}
		return t, nil
	case ast.UnaryExpr:
		if e.Operator == "not" {
			return "Bool", nil
		}
		return inferFlowExprType(e.Operand, env, pkg, boardFieldTypes)
	case ast.BinaryExpr:
		switch e.Operator {
		case "==", "!=", "<", "<=", ">", ">=", "and", "or":
			return "Bool", nil
		default:
			return inferFlowExprType(e.Left, env, pkg, boardFieldTypes)
		}
	case ast.CallExpr:
		if calleeField, ok := e.Callee.(ast.FieldAccessExpr); ok {
			if enumType, variant, ok := flattenFlowEnumValueExpr(calleeField); ok {
				if _, resolvedType, enumFound := lowerFlowEnumVariantExpr(flowLowerProgram, pkg, enumType, variant); enumFound {
					payloadType, hasPayload := lookupEnumVariantPayloadTypeForProgram(flowLowerProgram, pkg, enumType, variant)
					if !hasPayload {
						if len(e.Arguments) != 0 {
							return "", fmt.Errorf("enum '%s' variant '%s' does not accept a payload", enumType, variant)
						}
						return resolvedType, nil
					}
					if len(e.Arguments) != 1 {
						return "", fmt.Errorf("enum '%s' variant '%s' requires exactly 1 payload argument", enumType, variant)
					}
					argType, err := inferFlowExprType(e.Arguments[0], env, pkg, boardFieldTypes)
					if err != nil {
						return "", err
					}
					if argType != payloadType {
						return "", fmt.Errorf("enum '%s' variant '%s' payload expects %s, got %s", enumType, variant, payloadType, argType)
					}
					return resolvedType, nil
				}
			}
		}
		_, ret, _, fallible, err := resolveFlowCall(e.Callee, pkg)
		if err != nil {
			return "", err
		}
		if fallible {
			return "", fmt.Errorf("fallible calls are not supported in compiled flow expressions; handle outside the flow or use non-fallible helper")
		}
		return ret, nil
	case ast.IndexExpr:
		if len(e.Indices) != 1 {
			return "", unsupported("compiled flow expression indexing only supports single-dimension indexing")
		}
		targetType, err := inferFlowExprType(e.Target, env, pkg, boardFieldTypes)
		if err != nil {
			return "", err
		}
		if !strings.HasSuffix(targetType, "[]") {
			return "", unsupported(fmt.Sprintf("compiled flow expression indexing target type %q", targetType))
		}
		idxType, err := inferFlowExprType(e.Indices[0], env, pkg, boardFieldTypes)
		if err != nil {
			return "", err
		}
		if idxType != "Int" {
			return "", fmt.Errorf("compiled flow expression index must be Int, got %s", idxType)
		}
		return strings.TrimSuffix(targetType, "[]"), nil
	case ast.RecordLiteralExpr:
		if strings.Contains(e.TypeName, ".") {
			return e.TypeName, nil
		}
		return pkg + "." + e.TypeName, nil
	case ast.UtilityWhenExpr:
		return inferFlowExprType(e.Else, env, pkg, boardFieldTypes)
	case ast.SwitchExpr:
		var resultType string
		for _, switchCase := range e.Cases {
			caseType, err := inferFlowExprType(switchCase.Value, env, pkg, boardFieldTypes)
			if err != nil {
				return "", err
			}
			if resultType == "" {
				resultType = caseType
			}
		}
		if e.Else != nil {
			elseType, err := inferFlowExprType(e.Else, env, pkg, boardFieldTypes)
			if err != nil {
				return "", err
			}
			if resultType == "" {
				resultType = elseType
			}
		}
		return resultType, nil
	case ast.MatchExpr:
		subjectType, err := inferFlowExprType(e.Subject, env, pkg, boardFieldTypes)
		if err != nil {
			return "", err
		}
		var resultType string
		for _, matchCase := range e.Cases {
			caseEnv := cloneStringMap(env)
			if matchCase.Binding != "" {
				if t, ok := lookupEnumVariantPayloadTypeForProgram(flowLowerProgram, pkg, subjectType, matchCase.Variant); ok {
					caseEnv[matchCase.Binding] = t
				} else {
					caseEnv[matchCase.Binding] = "any"
				}
			}
			caseType, err := inferFlowExprType(matchCase.Value, caseEnv, pkg, boardFieldTypes)
			if err != nil {
				return "", err
			}
			if resultType == "" {
				resultType = caseType
			}
		}
		return resultType, nil
	case ast.IfExpr:
		thenType, err := inferFlowExprType(e.ThenExpr, env, pkg, boardFieldTypes)
		if err != nil {
			return "", err
		}
		if _, err := inferFlowExprType(e.Condition, env, pkg, boardFieldTypes); err != nil {
			return "", err
		}
		if _, err := inferFlowExprType(e.ElseExpr, env, pkg, boardFieldTypes); err != nil {
			return "", err
		}
		return thenType, nil
	case ast.ParenExpr:
		return inferFlowExprType(e.Inner, env, pkg, boardFieldTypes)
	case ast.FieldAccessExpr:
		targetIdent, ok := e.Target.(ast.IdentifierExpr)
		if !ok {
			return "", unsupported(fmt.Sprintf("flow field access target %T", e.Target))
		}
		if targetIdent.Name == "board" {
			t, exists := boardFieldTypes[e.Field]
			if !exists {
				return "", fmt.Errorf("board has no field '%s'", e.Field)
			}
			return t, nil
		}
		if _, ok := env[targetIdent.Name]; !ok {
			if _, enumType, ok := lowerFlowEnumVariantExpr(flowLowerProgram, pkg, targetIdent.Name, e.Field); ok {
				return enumType, nil
			}
		}
		return "", unsupported(fmt.Sprintf("flow field access %s.%s", targetIdent.Name, e.Field))
	default:
		return "", unsupported(fmt.Sprintf("flow expression %T", expr))
	}
}

func enumPackageName(enumType string, currentPkg string) string {
	if dot := strings.Index(enumType, "."); dot >= 0 {
		return enumType[:dot]
	}
	return currentPkg
}

func flattenFlowEnumValueExpr(expr ast.FieldAccessExpr) (string, string, bool) {
	enumType, ok := flattenEnumTypeExpr(expr.Target)
	if !ok {
		return "", "", false
	}
	return enumType, expr.Field, true
}

func resolveFlowCall(callee ast.Expr, pkg string) (string, string, bool, bool, error) {
	resolveBuiltin := func(name string) (string, string, bool, bool, error) {
		switch name {
		case "Abs", "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "Exp", "Ln", "Pow", "Log10", "Sinh", "Cosh", "Tanh", "Pi", "E", "BaseValue", "Float", "Clamp01":
			return name, "Float", true, false, nil
		case "FloorToInt", "CeilToInt", "RoundToInt":
			return name, "Int", true, false, nil
		case "Len":
			return name, "Int", true, false, nil
		case "FormatFloat":
			return name, "String", true, false, nil
		default:
			return "", "", false, false, unsupportedBuiltin(name)
		}
	}
	resolvePkgFn := func(pkgName, fnName string) (string, string, bool, bool, error) {
		declPkg, ok := flowLowerProgram.Packages[pkgName]
		if !ok {
			return "", "", false, false, fmt.Errorf("unknown package '%s'", pkgName)
		}
		for _, fn := range declPkg.Functions {
			if fn.Name == fnName {
				return pkgName + "." + fnName, typeRefStringForPackage(pkgName, fn.ReturnType), false, fn.IsFallible, nil
			}
		}
		return "", "", false, false, fmt.Errorf("unknown function '%s.%s'", pkgName, fnName)
	}
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		if builtin.IsName(x.Name) {
			return resolveBuiltin(x.Name)
		}
		resolved, ret, builtinCall, fallible, err := resolvePkgFn(pkg, x.Name)
		if err != nil {
			return "", "", false, false, fmt.Errorf("unknown function '%s'", x.Name)
		}
		return resolved, ret, builtinCall, fallible, nil
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false, false, unsupported(fmt.Sprintf("flow expression call target %T", callee))
		}
		qualified := pkgIdent.Name + "." + x.Field
		if builtin.IsName(qualified) {
			return resolveBuiltin(qualified)
		}
		return resolvePkgFn(pkgIdent.Name, x.Field)
	default:
		return "", "", false, false, unsupported(fmt.Sprintf("flow expression call target %T", callee))
	}
}

func dumpMIR(m MIRModule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module entry=%s\n", m.EntryPackage)
	for _, flow := range m.Flows {
		fmt.Fprintf(&b, "flow %s.%s -> %s\n", flow.Package, flow.Name, flow.Return)
		for idx, state := range flow.States {
			fmt.Fprintf(&b, "  state[%d] %s\n", idx, state.Name)
			for _, stmt := range state.Statements {
				fmt.Fprintf(&b, "    %s\n", dumpFlowStmt(stmt))
			}
		}
	}
	for _, fn := range m.Functions {
		fmt.Fprintf(&b, "fn %s.%s", fn.Package, fn.Name)
		fmt.Fprintf(&b, "(")
		for i, p := range fn.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:%s", p.Name, p.Type)
		}
		fmt.Fprintf(&b, ") -> %s", fn.Return)
		if fn.IsFallible {
			fmt.Fprintf(&b, " ! %s", fn.ErrorType)
		}
		b.WriteString("\n")
		for _, bb := range fn.Blocks {
			fmt.Fprintf(&b, "  %s:\n", bb.Label)
			for _, s := range bb.Statements {
				switch st := s.(type) {
				case MIRAssign:
					fmt.Fprintf(&b, "    %s = %s\n", st.Target, st.Value)
				case MIRCall:
					fmt.Fprintf(&b, "    %s = call %s(%s)\n", st.Target, st.Callee, strings.Join(st.Args, ", "))
				case MIRConstructArray:
					fmt.Fprintf(&b, "    %s = [%s]\n", st.Target, strings.Join(st.Values, ", "))
				case MIRConstructRecord:
					fmt.Fprintf(&b, "    %s = %s{...}\n", st.Target, st.TypeName)
				case MIRBatchMap:
					fmt.Fprintf(&b, "    %s = batch_map %s with %s\n", st.Target, st.Input, st.Worker)
				}
			}
			switch t := bb.Terminator.(type) {
			case MIRReturn:
				fmt.Fprintf(&b, "    return %s\n", t.Value)
			case MIRJump:
				fmt.Fprintf(&b, "    jump %s\n", t.Target)
			case MIRBranch:
				fmt.Fprintf(&b, "    branch %s ? %s : %s\n", t.Cond, t.TrueTarget, t.FalseTarget)
			case MIRFail:
				fmt.Fprintf(&b, "    fail %s\n", t.Value)
			}
		}
	}
	return b.String()
}

func dumpFlowStmt(stmt MIRFlowStmt) string {
	switch s := stmt.(type) {
	case MIRFlowGoto:
		return "goto " + s.Target
	case MIRFlowSuspend:
		return "suspend"
	case MIRFlowRemember:
		return "remember"
	case MIRFlowResume:
		return "resume"
	case MIRFlowFieldAssign:
		return fmt.Sprintf("%s.%s = %s", s.Target, s.Field, dumpFlowExpr(s.Value))
	case MIRFlowReturn:
		if s.Value == nil {
			return "return"
		}
		return "return " + dumpFlowExpr(s.Value)
	case MIRFlowIf:
		return fmt.Sprintf("if %s { ... }", dumpFlowExpr(s.Condition))
	case MIRFlowWhen:
		parts := make([]string, 0, len(s.Cases))
		for _, c := range s.Cases {
			parts = append(parts, fmt.Sprintf("case %s -> %s", dumpFlowExpr(c.Condition), dumpFlowWhenAction(c.Action)))
		}
		return "when { " + strings.Join(parts, "; ") + "; else -> " + dumpFlowWhenAction(s.Else) + " }"
	default:
		return fmt.Sprintf("unsupported-flow-stmt(%T)", stmt)
	}
}

func dumpFlowWhenAction(action MIRFlowWhenAction) string {
	switch a := action.(type) {
	case MIRFlowWhenGoto:
		return "goto " + a.Target
	case MIRFlowWhenSuspend:
		return "suspend"
	case MIRFlowWhenReturn:
		return "return " + dumpFlowExpr(a.Value)
	case MIRFlowWhenBlock:
		parts := make([]string, 0, len(a.Statements))
		for _, stmt := range a.Statements {
			parts = append(parts, dumpFlowStmt(stmt))
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	default:
		return fmt.Sprintf("unsupported-flow-action(%T)", action)
	}
}

func dumpFlowExpr(expr MIRFlowExpr) string {
	switch e := expr.(type) {
	case MIRFlowLiteralExpr:
		return e.Value
	case MIRFlowIdentifierExpr:
		return e.Name
	case MIRFlowFieldExpr:
		return e.Target + "." + e.Field
	case MIRFlowBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", dumpFlowExpr(e.Left), e.Operator, dumpFlowExpr(e.Right))
	case MIRFlowUnaryExpr:
		return fmt.Sprintf("(%s%s)", e.Operator, dumpFlowExpr(e.Operand))
	case MIRFlowCallExpr:
		args := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, dumpFlowExpr(arg))
		}
		return fmt.Sprintf("%s(%s)", e.Callee, strings.Join(args, ", "))
	case MIRFlowIndexExpr:
		return fmt.Sprintf("%s[%s]", dumpFlowExpr(e.Target), dumpFlowExpr(e.Index))
	case MIRFlowRecordLiteralExpr:
		return fmt.Sprintf("%s{...}", e.TypeName)
	case MIRFlowUtilityWhenExpr:
		return fmt.Sprintf("utility_when[site=%d,hysteresis=%s,min_commit=%s,cases=%d]", e.SiteID, dumpFlowExpr(e.Hysteresis), dumpFlowExpr(e.MinCommit), len(e.Cases))
	case MIRFlowIfExpr:
		return fmt.Sprintf("if %s { %s } else { %s }", dumpFlowExpr(e.Condition), dumpFlowExpr(e.Then), dumpFlowExpr(e.Else))
	default:
		return fmt.Sprintf("unsupported-flow-expr(%T)", expr)
	}
}

func emitGo(m MIRModule) (string, error) {
	var b strings.Builder
	usedBuiltins := map[string]bool{}
	emittedRecordTypes := map[string]struct{}{}
	loadTypes := map[string]struct{}{}
	resultTypes := map[string]struct{}{}
	flowResultTypes := map[string]struct{}{}
	needsUtilityHelpers := false
	usesGenericOctxiliary := false
	for _, flow := range m.Flows {
		flowResultTypes[flow.Return] = struct{}{}
		if flowHasUtilityWhen(flow) {
			needsUtilityHelpers = true
		}
		collectFlowBuiltins(flow, usedBuiltins)
	}
	for _, fn := range m.Functions {
		if fn.UsesUtilityWhen {
			needsUtilityHelpers = true
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
	importSet := map[string]struct{}{"fmt": {}, "os": {}}
	if needsUtilityHelpers {
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
	if usedBuiltins["Assert.True"] || usedBuiltins["Assert.False"] || usedBuiltins["Assert.Equal"] || usedBuiltins["Assert.Near"] || usedBuiltins["Assert.Error"] {
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
	if usedBuiltins["Abs"] || usedBuiltins["Pi"] || usedBuiltins["E"] || usedBuiltins["ComplexPolar"] || usedBuiltins["Arg"] || usedBuiltins["Sqrt"] || usedBuiltins["Sin"] || usedBuiltins["Cos"] || usedBuiltins["Tan"] || usedBuiltins["Asin"] || usedBuiltins["Acos"] || usedBuiltins["Atan"] || usedBuiltins["Atan2"] || usedBuiltins["Exp"] || usedBuiltins["Ln"] || usedBuiltins["Pow"] || usedBuiltins["Log10"] || usedBuiltins["Sinh"] || usedBuiltins["Cosh"] || usedBuiltins["Tanh"] || usedBuiltins["FloorToInt"] || usedBuiltins["CeilToInt"] || usedBuiltins["RoundToInt"] || usedBuiltins["BaseValue"] || usedBuiltins["fft"] {
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
	b.WriteString("package main\n\n")
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
	if needsVoidType {
		b.WriteString("type __octVoid struct{}\n\n")
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
	}
	flowTypeNames := make([]string, 0, len(flowResultTypes))
	for t := range flowResultTypes {
		flowTypeNames = append(flowTypeNames, t)
	}
	sort.Strings(flowTypeNames)
	for _, t := range flowTypeNames {
		fmt.Fprintf(&b, "type __octFlowInstance_%s interface {\n", goSafeName(t))
		b.WriteString("\t__octStep()\n\t__octActive() string\n\t__octComplete() bool\n")
		fmt.Fprintf(&b, "\t__octResult() (%s, bool)\n", goFlowResultType(t))
		b.WriteString("\t__octStateHistory() []string\n\t__octResumeTarget() string\n\t__octBoardSnapshot() (any, bool)\n}\n\n")
		fmt.Fprintf(&b, "type __octResultFlow_%s struct {\n\tValue %s\n\tErr string\n\tIsErr bool\n}\n\n", goSafeName(t), goFlowResultType(t))
	}
	for _, flow := range m.Flows {
		if err := emitGoFlow(&b, flow); err != nil {
			return "", err
		}
	}
	b.WriteString(__octArrayCoercionHelpers)
	if usedBuiltins["Idx"] {
		b.WriteString(__octIndexHelpers)
	}
	if needsUtilityHelpers {
		b.WriteString(__octUtilityHelpers)
	}
	if usesLinearAlgebraHelpers(usedBuiltins) {
		b.WriteString(__octLinearAlgebraHelpers)
	}
	if usedBuiltins["Abs"] || usedBuiltins["Real"] || usedBuiltins["Imag"] {
		b.WriteString(__octComplexHelpers)
	}
	if usedBuiltins["fft"] {
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
		b.WriteString("type __octRecordMeta struct {\n\tFullName string\n\tShortName string\n\tFields []string\n}\n\n")
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
		b.WriteString(__octBatchHelpers)
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
		b.WriteString("\tpc := 0\n\tfor {\n\t\tswitch pc {\n")
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
			term, err := goTerminator(terminator, labelToIdx)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "\t\t\t%s\n", term)
		}
		b.WriteString("\t\t}\n\t}\n}\n\n")
	}
	b.WriteString("var __octAssertionCount int\n\n")
	b.WriteString("func main() {\n")
	if usesOctxiliaryBuiltins(usedBuiltins) || usesGenericOctxiliary {
		b.WriteString("\tdefer __octOctxiliaryClose()\n")
	}
	entryReturn := m.EntryReturn
	entryFallible := m.EntryFallible
	if entryReturn == "Void" && !entryFallible {
		b.WriteString("\tfn_" + m.EntryPackage + "_" + m.EntryFunc + "()\n")
	} else {
		b.WriteString("\tresult := fn_" + m.EntryPackage + "_" + m.EntryFunc + "()\n")
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
	case MIRFlowBinaryExpr:
		collectFlowBuiltinsExpr(e.Left, usedBuiltins)
		collectFlowBuiltinsExpr(e.Right, usedBuiltins)
	case MIRFlowUnaryExpr:
		collectFlowBuiltinsExpr(e.Operand, usedBuiltins)
	case MIRFlowCallExpr:
		if e.Builtin {
			usedBuiltins[e.Callee] = true
		}
		for _, arg := range e.Args {
			collectFlowBuiltinsExpr(arg, usedBuiltins)
		}
	case MIRFlowIndexExpr:
		collectFlowBuiltinsExpr(e.Target, usedBuiltins)
		collectFlowBuiltinsExpr(e.Index, usedBuiltins)
	case MIRFlowRecordLiteralExpr:
		for _, fieldVal := range e.FieldVals {
			collectFlowBuiltinsExpr(fieldVal, usedBuiltins)
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
	case MIRFlowSwitchExpr:
		if e.Subject != nil {
			collectFlowBuiltinsExpr(e.Subject, usedBuiltins)
		}
		for _, c := range e.Cases {
			collectFlowBuiltinsExpr(c.Match, usedBuiltins)
			collectFlowBuiltinsExpr(c.Value, usedBuiltins)
		}
		if e.Else != nil {
			collectFlowBuiltinsExpr(e.Else, usedBuiltins)
		}
	case MIRFlowMatchExpr:
		collectFlowBuiltinsExpr(e.Subject, usedBuiltins)
		for _, c := range e.Cases {
			collectFlowBuiltinsExpr(c.Value, usedBuiltins)
		}
	case MIRFlowIfExpr:
		collectFlowBuiltinsExpr(e.Condition, usedBuiltins)
		collectFlowBuiltinsExpr(e.Then, usedBuiltins)
		collectFlowBuiltinsExpr(e.Else, usedBuiltins)
	}
}

const __octArrayCoercionHelpers = `
func __octIntArrayToFloat(values []int) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = float64(value)
	}
	return out
}
`

const __octComplexHelpers = `
func __octComplexReal(value complex128) float64 { return real(value) }
func __octComplexImag(value complex128) float64 { return imag(value) }
func __octComplexAbs(value complex128) float64 { return math.Hypot(real(value), imag(value)) }
`

const __octSharedOctagonHelpers = `
func __octTypeKey(t reflect.Type) string {
	if t.Name() == "" {
		return t.String()
	}
	if t.PkgPath() == "" {
		return t.Name()
	}
	return t.PkgPath() + "." + t.Name()
}
`

const __octIndexHelpers = `
func __octIdx(label string) string {
	if strings.TrimSpace(label) == "" {
		panic("runtime error: Idx requires non-empty label")
	}
	return label
}
`

const __octLinearAlgebraHelpers = `
type __octNumber interface {
	~int | ~float64
}

func __octMatMulMV[T __octNumber](left [][]T, right []T) []T {
	if len(left) == 0 {
		return []T{}
	}
	if len(left[0]) != len(right) {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %d", len(left), len(left[0]), len(right)))
	}
	result := make([]T, len(left))
	for r := range left {
		if len(left[r]) != len(right) {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %d", len(left), len(left[r]), len(right)))
		}
		var acc T
		for c := range right {
			acc += left[r][c] * right[c]
		}
		result[r] = acc
	}
	return result
}


func __octMatMulVM[T __octNumber](left []T, right [][]T) []T {
	if len(right) == 0 {
		if len(left) != 0 {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), 0, 0))
		}
		return []T{}
	}
	rightCols := len(right[0])
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), len(right), rightCols))
	}
	for r := range right {
		if len(right[r]) != rightCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), len(right), len(right[r])))
		}
	}
	result := make([]T, rightCols)
	for c := 0; c < rightCols; c++ {
		var acc T
		for r := range left {
			acc += left[r] * right[r][c]
		}
		result[c] = acc
	}
	return result
}

func __octVecDot[T __octNumber](left []T, right []T) T {
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: vector dot product requires matching lengths; got %d and %d", len(left), len(right)))
	}
	if len(left) == 0 {
		panic("runtime error: vector dot product requires non-empty vectors")
	}
	var acc T
	for i := range left {
		acc += left[i] * right[i]
	}
	return acc
}

func __octVecAddVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] + right[i] }; return out }
func __octVecSubVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] - right[i] }; return out }
func __octVecMulVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] * right[i] }; return out }
func __octVecDivVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] / right[i] }; return out }
func __octVecAddVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] + right }; return out }
func __octVecSubVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] - right }; return out }
func __octVecMulVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] * right }; return out }
func __octVecDivVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] / right }; return out }
func __octVecAddSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left + right[i] }; return out }
func __octVecSubSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left - right[i] }; return out }
func __octVecMulSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left * right[i] }; return out }
func __octVecDivSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left / right[i] }; return out }

func __octMatMulMM[T __octNumber](left [][]T, right [][]T) [][]T {
	if len(left) == 0 || len(right) == 0 {
		return [][]T{}
	}
	leftCols := len(left[0])
	rightRows := len(right)
	if leftCols != rightRows {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), leftCols, len(right), len(right[0])))
	}
	rightCols := len(right[0])
	for r := range left {
		if len(left[r]) != leftCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), len(left[r]), len(right), rightCols))
		}
	}
	for r := range right {
		if len(right[r]) != rightCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), leftCols, len(right), len(right[r])))
		}
	}
	result := make([][]T, len(left))
	for r := range left {
		row := make([]T, rightCols)
		for c := 0; c < rightCols; c++ {
			var acc T
			for k := 0; k < leftCols; k++ {
				acc += left[r][k] * right[k][c]
			}
			row[c] = acc
		}
		result[r] = row
	}
	return result
}

func __octMatBinaryValue[L __octNumber, R __octNumber, O __octNumber](left L, right R, op string) O {
	l := O(left)
	r := O(right)
	switch op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		return l / r
	default:
		panic(fmt.Sprintf("runtime invariant violation: unsupported matrix binary operator %q", op))
	}
}

func __octMatBinaryMM[L __octNumber, R __octNumber, O __octNumber](left [][]L, right [][]R, op string) [][]O {
	leftRows, leftCols := __octMatrixDims(left, "matrix elementwise "+op, "left")
	rightRows, rightCols := __octMatrixDims(right, "matrix elementwise "+op, "right")
	if leftRows != rightRows || leftCols != rightCols {
		panic(fmt.Sprintf("runtime error: matrix shapes must match; got %dx%d and %dx%d", leftRows, leftCols, rightRows, rightCols))
	}
	out := make([][]O, leftRows)
	for rowIndex := range left {
		row := make([]O, leftCols)
		for colIndex := range left[rowIndex] {
			row[colIndex] = __octMatBinaryValue[L, R, O](left[rowIndex][colIndex], right[rowIndex][colIndex], op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatBinaryMS[M __octNumber, S __octNumber, O __octNumber](matrix [][]M, scalar S, op string) [][]O {
	rows, cols := __octMatrixDims(matrix, "matrix-scalar "+op, "left")
	out := make([][]O, rows)
	for rowIndex := range matrix {
		row := make([]O, cols)
		for colIndex := range matrix[rowIndex] {
			row[colIndex] = __octMatBinaryValue[M, S, O](matrix[rowIndex][colIndex], scalar, op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatBinarySM[S __octNumber, M __octNumber, O __octNumber](scalar S, matrix [][]M, op string) [][]O {
	rows, cols := __octMatrixDims(matrix, "scalar-matrix "+op, "right")
	out := make([][]O, rows)
	for rowIndex := range matrix {
		row := make([]O, cols)
		for colIndex := range matrix[rowIndex] {
			row[colIndex] = __octMatBinaryValue[S, M, O](scalar, matrix[rowIndex][colIndex], op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatrixDims[T __octNumber](m [][]T, op string, side string) (int, int) {
	if len(m) == 0 {
		return 0, 0
	}
	cols := len(m[0])
	for r := range m {
		if len(m[r]) != cols {
			panic(fmt.Sprintf("runtime error: %s requires rectangular %s matrix", op, side))
		}
	}
	return len(m), cols
}

func __octEinDimByLabel(label string, dim int, dims map[string]int) {
	if label == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if prev, ok := dims[label]; ok && prev != dim {
		panic(fmt.Sprintf("runtime error: index '%s' has inconsistent extents", label))
	}
	dims[label] = dim
}


func __octVectorDimByLabel(label string, length int, dims map[string]int) {
	__octEinDimByLabel(label, length, dims)
}

func __octEinAddVV[T __octNumber](left []T, l0 string, right []T, r0 string) []T {
	return __octEinAddSubVV(left, l0, right, r0, false)
}

func __octEinSubVV[T __octNumber](left []T, l0 string, right []T, r0 string) []T {
	return __octEinAddSubVV(left, l0, right, r0, true)
}

func __octEinAddSubVV[T __octNumber](left []T, l0 string, right []T, r0 string, subtract bool) []T {
	op := "EinAdd"
	if subtract {
		op = "EinSub"
	}
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 != r0 {
		panic(fmt.Sprintf("runtime error: %s requires matching free-index order on both vector terms (left=[%s], right=[%s])", op, l0, r0))
	}
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: %s requires matching vector lengths for index '%s'; got %d and %d", op, l0, len(left), len(right)))
	}
	out := make([]T, len(left))
	for i := range left {
		if subtract {
			out[i] = left[i] - right[i]
		} else {
			out[i] = left[i] + right[i]
		}
	}
	return out
}

func __octEinDotVV[T __octNumber](left []T, l0 string, right []T, r0 string) T {
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 != r0 {
		panic(fmt.Sprintf("runtime error: EinDot requires matching contracted vector indices (left=[%s], right=[%s])", l0, r0))
	}
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: EinDot requires matching vector lengths for index '%s'; got %d and %d", l0, len(left), len(right)))
	}
	var acc T
	for i := range left {
		acc += left[i] * right[i]
	}
	return acc
}

func __octEinOuterVV[T __octNumber](left []T, l0 string, right []T, r0 string) [][]T {
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == r0 {
		panic(fmt.Sprintf("runtime error: EinOuter requires distinct free vector indices (left=[%s], right=[%s])", l0, r0))
	}
	out := make([][]T, len(left))
	for i := range left {
		row := make([]T, len(right))
		for j := range right {
			row[j] = left[i] * right[j]
		}
		out[i] = row
	}
	return out
}

func __octEinMulMV[T __octNumber](left [][]T, l0 string, l1 string, right []T, r0 string, free0 string) []T {
	leftRows, leftCols := __octMatrixDims(left, "EinMulMV", "left")
	if l0 == "" || l1 == "" || r0 == "" || free0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == l1 {
		panic(fmt.Sprintf("runtime error: EinMulMV requires distinct matrix indices (left=[%s,%s])", l0, l1))
	}
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octVectorDimByLabel(r0, len(right), dims)
	if free0 == l0 && r0 == l1 {
		out := make([]T, leftRows)
		for row := 0; row < leftRows; row++ {
			var acc T
			for col := 0; col < leftCols; col++ {
				acc += left[row][col] * right[col]
			}
			out[row] = acc
		}
		return out
	}
	if free0 == l1 && r0 == l0 {
		out := make([]T, leftCols)
		for col := 0; col < leftCols; col++ {
			var acc T
			for row := 0; row < leftRows; row++ {
				acc += left[row][col] * right[row]
			}
			out[col] = acc
		}
		return out
	}
	panic(fmt.Sprintf("runtime error: EinMulMV requires one matrix index to contract with the vector index (left=[%s,%s], right=[%s], free=[%s])", l0, l1, r0, free0))
}

func __octEinMulVM[T __octNumber](left []T, l0 string, right [][]T, r0 string, r1 string, free0 string) []T {
	rightRows, rightCols := __octMatrixDims(right, "EinMulVM", "right")
	if l0 == "" || r0 == "" || r1 == "" || free0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if r0 == r1 {
		panic(fmt.Sprintf("runtime error: EinMulVM requires distinct matrix indices (right=[%s,%s])", r0, r1))
	}
	dims := map[string]int{}
	__octVectorDimByLabel(l0, len(left), dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	if free0 == r1 && l0 == r0 {
		out := make([]T, rightCols)
		for col := 0; col < rightCols; col++ {
			var acc T
			for row := 0; row < rightRows; row++ {
				acc += left[row] * right[row][col]
			}
			out[col] = acc
		}
		return out
	}
	if free0 == r0 && l0 == r1 {
		out := make([]T, rightRows)
		for row := 0; row < rightRows; row++ {
			var acc T
			for col := 0; col < rightCols; col++ {
				acc += left[col] * right[row][col]
			}
			out[row] = acc
		}
		return out
	}
	panic(fmt.Sprintf("runtime error: EinMulVM requires the vector index to contract with one matrix index (left=[%s], right=[%s,%s], free=[%s])", l0, r0, r1, free0))
}

func __octEinFreeAndContracted(l0 string, l1 string, r0 string, r1 string) ([]string, []string) {
	ordered := []string{l0, l1, r0, r1}
	counts := map[string]int{}
	for _, label := range ordered {
		counts[label]++
	}
	free := []string{}
	contracted := []string{}
	seenFree := map[string]bool{}
	seenContracted := map[string]bool{}
	for _, label := range ordered {
		switch counts[label] {
		case 1:
			if !seenFree[label] {
				free = append(free, label)
				seenFree[label] = true
			}
		case 2:
			if !seenContracted[label] {
				contracted = append(contracted, label)
				seenContracted[label] = true
			}
		default:
			panic(fmt.Sprintf("runtime error: index '%s' appears %d times in [%s,%s]*[%s,%s]; only 1 (free) or 2 (contracted) are allowed in M33", label, counts[label], l0, l1, r0, r1))
		}
	}
	return free, contracted
}

func __octEinMulMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	leftRows, leftCols := __octMatrixDims(left, "EinMul", "left")
	rightRows, rightCols := __octMatrixDims(right, "EinMul", "right")
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	free, contracted := __octEinFreeAndContracted(l0, l1, r0, r1)
	if len(free) != 2 {
		panic(fmt.Sprintf("runtime error: EinMul requires exactly 2 free indices in M33, got %d for [%s,%s]*[%s,%s]", len(free), l0, l1, r0, r1))
	}
	rows := dims[free[0]]
	cols := dims[free[1]]
	out := make([][]T, rows)
	assignments := map[string]int{}
	for r := 0; r < rows; r++ {
		row := make([]T, cols)
		for c := 0; c < cols; c++ {
			assignments[free[0]] = r
			assignments[free[1]] = c
			var acc T
			accSet := false
			var loop func(int)
			loop = func(pos int) {
				if pos == len(contracted) {
					product := left[assignments[l0]][assignments[l1]] * right[assignments[r0]][assignments[r1]]
					if !accSet {
						acc = product
						accSet = true
						return
					}
					acc += product
					return
				}
				label := contracted[pos]
				for v := 0; v < dims[label]; v++ {
					assignments[label] = v
					loop(pos + 1)
				}
			}
			loop(0)
			if !accSet {
				panic("runtime error: EinMul failed to accumulate contracted terms")
			}
			row[c] = acc
		}
		out[r] = row
	}
	return out
}

func __octEinDoubleMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) T {
	leftRows, leftCols := __octMatrixDims(left, "EinDoubleMM", "left")
	rightRows, rightCols := __octMatrixDims(right, "EinDoubleMM", "right")
	if l0 == l1 || r0 == r1 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires distinct indices per matrix term (left=[%s,%s], right=[%s,%s]); use Trace(...) for single-matrix trace", l0, l1, r0, r1))
	}
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	free, contracted := __octEinFreeAndContracted(l0, l1, r0, r1)
	if len(free) != 0 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires zero free indices, got %d for [%s,%s]*[%s,%s]", len(free), l0, l1, r0, r1))
	}
	if len(contracted) != 2 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires exactly 2 contracted indices, got %d for [%s,%s]*[%s,%s]", len(contracted), l0, l1, r0, r1))
	}
	assignments := map[string]int{}
	var acc T
	accSet := false
	var loop func(int)
	loop = func(pos int) {
		if pos == len(contracted) {
			product := left[assignments[l0]][assignments[l1]] * right[assignments[r0]][assignments[r1]]
			if !accSet {
				acc = product
				accSet = true
				return
			}
			acc += product
			return
		}
		label := contracted[pos]
		for v := 0; v < dims[label]; v++ {
			assignments[label] = v
			loop(pos + 1)
		}
	}
	loop(0)
	if !accSet {
		panic("runtime error: EinDoubleMM failed to accumulate contracted terms")
	}
	return acc
}

func __octEinAddMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	return __octEinAddSubMM(left, l0, l1, right, r0, r1, false)
}

func __octEinSubMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	return __octEinAddSubMM(left, l0, l1, right, r0, r1, true)
}

func __octEinAddSubMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string, subtract bool) [][]T {
	op := "EinAdd"
	if subtract {
		op = "EinSub"
	}
	leftRows, leftCols := __octMatrixDims(left, op, "left")
	rightRows, rightCols := __octMatrixDims(right, op, "right")
	if l0 == "" || l1 == "" || r0 == "" || r1 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == l1 || r0 == r1 {
		panic(fmt.Sprintf("runtime error: %s requires distinct free indices per matrix term (left=[%s,%s], right=[%s,%s])", op, l0, l1, r0, r1))
	}
	if l0 != r0 || l1 != r1 {
		panic(fmt.Sprintf("runtime error: %s requires matching free-index order on both terms (left=[%s,%s], right=[%s,%s])", op, l0, l1, r0, r1))
	}
	if leftRows != rightRows || leftCols != rightCols {
		panic(fmt.Sprintf("runtime error: %s requires matching matrix shapes", op))
	}
	out := make([][]T, leftRows)
	for r := range left {
		row := make([]T, leftCols)
		for c := range left[r] {
			if subtract {
				row[c] = left[r][c] - right[r][c]
			} else {
				row[c] = left[r][c] + right[r][c]
			}
		}
		out[r] = row
	}
	return out
}

func __octTrace[T __octNumber](m [][]T) T {
	if len(m) == 0 {
		panic("runtime error: Trace requires non-empty matrix")
	}
	if len(m[0]) != len(m) {
		panic(fmt.Sprintf("runtime error: Trace requires square matrix, got %dx%d", len(m), len(m[0])))
	}
	var out T
	for i := 0; i < len(m); i++ {
		if len(m[i]) != len(m) {
			panic(fmt.Sprintf("runtime error: Trace requires square matrix, got %dx%d", len(m), len(m[i])))
		}
		out += m[i][i]
	}
	return out
}

func __octGrad[T __octNumber](v []T) [][]T {
	out := make([][]T, len(v))
	for i := range v {
		row := make([]T, len(v))
		row[i] = v[i]
		out[i] = row
	}
	return out
}

func __octGradScalar[T __octNumber](v T) []T {
	return []T{v}
}

func __octDivVector[T __octNumber](v []T) T {
	var out T
	for _, cell := range v {
		out += cell
	}
	return out
}

func __octDiv[T __octNumber](m [][]T) []T {
	out := make([]T, len(m))
	for r := range m {
		out[r] = __octDivVector(m[r])
	}
	return out
}

func __octSymGrad[T __octNumber](v []T) [][]T {
	return __octGrad(v)
}
`

const __octFFTHelpers = `
func __octFFT(values []complex128) octResult_ComplexSlice {
	n := len(values)
	if n == 0 {
		return octResult_ComplexSlice{Err: "runtime error: fft requires non-empty input", IsErr: true}
	}
	if n&(n-1) != 0 {
		return octResult_ComplexSlice{Err: "runtime error: fft requires power-of-two input length", IsErr: true}
	}

	out := make([]complex128, n)
	copy(out, values)

	for i := 0; i < n; i++ {
		j := __octReverseBits(i, n)
		if j > i {
			out[i], out[j] = out[j], out[i]
		}
	}

	for span := 2; span <= n; span *= 2 {
		halfSpan := span / 2
		phaseStep := -2.0 * math.Pi / float64(span)
		for blockStart := 0; blockStart < n; blockStart += span {
			for j := 0; j < halfSpan; j++ {
				angle := phaseStep * float64(j)
				twiddle := complex(math.Cos(angle), math.Sin(angle))
				odd := out[blockStart+j+halfSpan]
				t := twiddle * odd
				even := out[blockStart+j]
				out[blockStart+j] = even + t
				out[blockStart+j+halfSpan] = even - t
			}
		}
	}

	return octResult_ComplexSlice{Value: out}
}

func __octReverseBits(index int, width int) int {
	value := index
	reversed := 0
	n := width
	for n > 1 {
		lowBit := value & 1
		reversed = reversed*2 + lowBit
		value /= 2
		n /= 2
	}
	return reversed
}
`

const __octRandomHelpers = `
func __octRandomRotl(x uint64, k int) uint64 { return (x << k) | (x >> (64 - k)) }
func __octRandomSplitMix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
func __octRandomSeedState(seed int) Random_Rng {
	x := uint64(seed)
	return Random_Rng{_State0: int(__octRandomSplitMix64(x)), _State1: int(__octRandomSplitMix64(x + 1)), _State2: int(__octRandomSplitMix64(x + 2)), _State3: int(__octRandomSplitMix64(x + 3))}
}
func __octRandomNext(s Random_Rng) (Random_Rng, uint64) {
	s0 := uint64(s._State0)
	s1 := uint64(s._State1)
	s2 := uint64(s._State2)
	s3 := uint64(s._State3)
	result := __octRandomRotl(s1*5, 7) * 9
	t := s1 << 17
	s2 ^= s0
	s3 ^= s1
	s1 ^= s2
	s0 ^= s3
	s2 ^= t
	s3 = __octRandomRotl(s3, 45)
	return Random_Rng{_State0: int(s0), _State1: int(s1), _State2: int(s2), _State3: int(s3)}, result
}
func __octRandomFloat01(x uint64) float64 { return float64(x>>11) * (1.0 / (1 << 53)) }
func __octRandomRngSeed(seed int) Random_Rng { return __octRandomSeedState(seed) }
func __octRandomRandInt(rng Random_Rng, min int, max int) Random_RandIntResult {
	if min > max { panic("runtime error: min must be <= max") }
	span := uint64(max - min + 1)
	threshold := ^uint64(0) - (^uint64(0) % span)
	var x uint64
	for {
		rng, x = __octRandomNext(rng)
		if x < threshold { break }
	}
	return Random_RandIntResult{Next: rng, Value: min + int(x%span)}
}
func __octRandomRandFloat01(rng Random_Rng) Random_RandFloatResult {
	rng, x := __octRandomNext(rng)
	return Random_RandFloatResult{Next: rng, Value: __octRandomFloat01(x)}
}
func __octRandomRandFloatRange(rng Random_Rng, min float64, max float64) Random_RandFloatResult {
	if min > max { panic("runtime error: min must be <= max") }
	if min == max { return Random_RandFloatResult{Next: rng, Value: min} }
	rng, x := __octRandomNext(rng)
	return Random_RandFloatResult{Next: rng, Value: min + (max-min)*__octRandomFloat01(x)}
}
func __octRandomRandBernoulli(rng Random_Rng, p float64) Random_RandBoolResult {
	if p < 0 || p > 1 { panic("runtime error: p must be in [0,1]") }
	if p == 0 || p == 1 { return Random_RandBoolResult{Next: rng, Value: p == 1} }
	rng, x := __octRandomNext(rng)
	return Random_RandBoolResult{Next: rng, Value: __octRandomFloat01(x) < p}
}
func __octRandomRandNormal(rng Random_Rng, mean float64, stddev float64) Random_RandFloatResult {
	if stddev < 0 { panic("runtime error: stddev must be >= 0") }
	if stddev == 0 { return Random_RandFloatResult{Next: rng, Value: mean} }
	rng, u1 := __octRandomNext(rng)
	rng, u2 := __octRandomNext(rng)
	z := math.Sqrt(-2*math.Log(math.Max(__octRandomFloat01(u1), 1e-12))) * math.Cos(2*math.Pi*__octRandomFloat01(u2))
	return Random_RandFloatResult{Next: rng, Value: mean + stddev*z}
}
func __octCryptoRandBytes(count int) ([]byte, error) {
	if count < 0 { return nil, fmt.Errorf("count must be >= 0") }
	out := make([]byte, count)
	_, err := rand.Read(out)
	return out, err
}
func __octCryptoRandInt(min int, max int) (int, error) {
	if min > max { return 0, fmt.Errorf("runtime error: min must be <= max") }
	span := max - min + 1
	n, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
	if err != nil { return 0, err }
	return min + int(n.Int64()), nil
}
func __octCryptoRandFloat01() (float64, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil { return 0, err }
	return __octRandomFloat01(binary.LittleEndian.Uint64(b[:])), nil
}
`

func builtinImportDeps(name string) []string {
	name = canonicalCompiledBuiltinName(name)
	if isMarkdownCompiledBuiltin(name) {
		if name == "MarkdownTable" || name == "MarkdownTableWithColumns" {
			return []string{"reflect", "strings"}
		}
		return []string{"strings"}
	}
	switch name {
	case "Contains", "StartsWith", "EndsWith", "Trim", "Lower", "Upper", "Join":
		return []string{"strings"}
	case "StringRuneCount":
		return []string{"unicode/utf8"}
	case "StringContains", "StringStartsWith", "StringEndsWith", "StringTrim", "StringJoin", "StringReplaceAll", "StringSplitLines":
		return []string{"strings"}
	case "StringEscapeJSON", "StringQuoteJSON":
		return []string{"strconv"}
	case "FormatFloat":
		return []string{"strconv"}
	case "CsvReadMatrix":
		return []string{"strconv"}
	case "PathJoin", "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
		if name == "PathStem" {
			return []string{"path/filepath", "strings"}
		}
		return []string{"path/filepath"}
	}
	return nil
}

const __octPrometheusHelpers = `
var __octPrometheusVulkanEnvOnce sync.Once
var __octPrometheusVulkanEnvValue string

func __octPrometheusVulkanEnv() string {
	__octPrometheusVulkanEnvOnce.Do(func() {
		__octPrometheusVulkanEnvValue = __octDetectPrometheusVulkanEnv()
	})
	return __octPrometheusVulkanEnvValue
}

func __octDetectPrometheusVulkanEnv() string {
	if os.Getenv("WSL_DISTRO_NAME") == "" {
		return "not_applicable"
	}
	output, err := exec.Command("vulkaninfo", "--summary").CombinedOutput()
	if err != nil {
		return "wsl_vulkan_unknown"
	}
	text := strings.ToLower(string(output))
	if strings.Contains(text, "driver_id_mesa_dozen") || strings.Contains(text, "drivername         = dozen") {
		return "vulkan_wsl_dzn"
	}
	if strings.Contains(text, "llvmpipe") || strings.Contains(text, "physical_device_type_cpu") {
		return "software_vulkan_llvmpipe_or_cpu"
	}
	if strings.Contains(text, "vulkan") {
		return "vulkan_wsl_other"
	}
	return "wsl_vulkan_unknown"
}

func __octPrometheusMatMulMM(left [][]float64, right [][]float64) [][]float64 {
	out, run, err := prometheus.RunCompiledMatMulMM(left, right)
	env := __octPrometheusVulkanEnv()
	if env == "not_applicable" && run.VulkanEnv != "" {
		env = run.VulkanEnv
	}
	fmt.Printf("backend_requested=%s backend_used=%s status=%s correctness=%t detail_code=%d detail_name=%s vulkan_env=%s wall=%dns\n",
		run.RequestedBackend, run.UsedBackend, run.Status.String(), run.Correctness.Pass, run.DetailCode, run.DetailName, env, run.WallTimeNs)
	if err != nil {
		panic(fmt.Sprintf("PrometheusMatMulMM failed: %v", err))
	}
	return out
}
`

const __octStringHelpers = `
func __octStringSplitLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if normalized == "" {
		return []string{}
	}
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func __octStringEscapeJSON(text string) string {
	quoted := strconv.Quote(text)
	return quoted[1 : len(quoted)-1]
}
`

const __octMarkdownHelpers = `
func __octMarkdownNormalizeInline(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\n", " ")
	return strings.TrimSpace(normalized)
}

func __octMarkdownEscapeTableCell(text string) string {
	return strings.ReplaceAll(__octMarkdownNormalizeInline(text), "|", "\\|")
}

func __octMarkdownLongestBacktickRun(text string) int {
	longest := 0
	current := 0
	for _, r := range text {
		if r == 96 {
			current++
			if current > longest { longest = current }
			continue
		}
		current = 0
	}
	return longest
}

func __octMarkdownCodeFence(language string, body []string) string {
	longest := __octMarkdownLongestBacktickRun(strings.TrimSpace(language))
	for _, line := range body {
		if run := __octMarkdownLongestBacktickRun(line); run > longest { longest = run }
	}
	fenceLength := 3
	if longest + 1 > fenceLength { fenceLength = longest + 1 }
	return strings.Repeat(string(rune(96)), fenceLength)
}

func __octMarkdownList(items []string, numbered bool) []string {
	out := make([]string, 0, len(items))
	for idx, item := range items {
		prefix := "- "
		if numbered { prefix = fmt.Sprintf("%d. ", idx+1) }
		out = append(out, prefix + __octMarkdownNormalizeInline(item))
	}
	return out
}

func __octMarkdownCodeBlock(language string, body []string) []string {
	fence := __octMarkdownCodeFence(language, body)
	out := []string{fence + strings.TrimSpace(language)}
	out = append(out, body...)
	out = append(out, fence)
	return out
}

func __octMarkdownCallout(kind string, lines []string) []string {
	labelMap := map[string]string{"note": "Note", "info": "Info", "warning": "Warning", "danger": "Danger", "success": "Success"}
	label, ok := labelMap[kind]
	if !ok { panic(fmt.Sprintf("runtime error: MarkdownCallout unsupported kind '%s' (supported: note, info, warning, danger, success)", kind)) }
	if len(lines) == 0 { return []string{"> **" + label + ":**"} }
	out := make([]string, 0, len(lines))
	for idx, line := range lines {
		normalized := __octMarkdownNormalizeInline(line)
		if idx == 0 {
			if normalized == "" { out = append(out, "> **" + label + ":**") } else { out = append(out, "> **" + label + ":** " + normalized) }
			continue
		}
		if normalized == "" { out = append(out, ">") } else { out = append(out, "> " + normalized) }
	}
	return out
}

func __octMarkdownImage(path string, caption string) []string {
	alt := strings.ReplaceAll(__octMarkdownNormalizeInline(caption), "]", "\\]")
	return []string{"![" + alt + "](" + path + ")"}
}

func __octMarkdownFigure(path string, caption string) []string {
	image := __octMarkdownImage(path, caption)[0]
	return []string{image, "", "*Figure: " + __octMarkdownNormalizeInline(caption) + "*"}
}

func __octMarkdownFlattenBlocks(blocks [][]string) []string {
	out := []string{}
	for _, block := range blocks {
		if len(block) == 0 { continue }
		if len(out) > 0 { out = append(out, "") }
		out = append(out, block...)
	}
	return out
}

func __octMarkdownSection(title string, blocks [][]string, subsection bool) []string {
	prefix := "## "
	if subsection { prefix = "### " }
	out := []string{prefix + __octMarkdownNormalizeInline(title)}
	lines := __octMarkdownFlattenBlocks(blocks)
	if len(lines) == 0 { return out }
	out = append(out, "")
	out = append(out, lines...)
	return out
}

func __octMarkdownKeyValueTable(keys []string, values []string) []string {
	if len(keys) != len(values) { panic("runtime error: MarkdownKeyValueTable keys and values must have equal lengths") }
	out := []string{"| key | value |", "| --- | --- |"}
	for idx := range keys {
		out = append(out, "| " + __octMarkdownEscapeTableCell(keys[idx]) + " | " + __octMarkdownEscapeTableCell(values[idx]) + " |")
	}
	return out
}

func __octMarkdownRecordColumns(table any, requested []string, explicit bool) ([]string, [][]string) {
	v := reflect.ValueOf(table)
	if v.Kind() == reflect.Pointer { v = v.Elem() }
	if v.Kind() != reflect.Struct { panic("runtime error: MarkdownTable expects record-of-string-columns") }
	t := v.Type()
	columns := make([]string, 0, t.NumField())
	fieldByName := map[string]reflect.Value{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		columns = append(columns, field.Name)
		fieldByName[field.Name] = v.Field(i)
	}
	if len(columns) == 0 { panic("runtime error: MarkdownTable requires at least one column") }
	if explicit {
		seen := map[string]struct{}{}
		columns = []string{}
		for _, name := range requested {
			if strings.TrimSpace(name) == "" { panic("runtime error: MarkdownTableWithColumns column names must be non-empty") }
			if _, ok := fieldByName[name]; !ok { panic(fmt.Sprintf("runtime error: MarkdownTableWithColumns unknown column '%s'", name)) }
			if _, dup := seen[name]; dup { panic(fmt.Sprintf("runtime error: MarkdownTableWithColumns duplicate requested column '%s'", name)) }
			seen[name] = struct{}{}
			columns = append(columns, name)
		}
		if len(columns) == 0 { panic("runtime error: MarkdownTableWithColumns requires at least one requested column") }
	}
	rowCount := -1
	colArrays := make([][]string, 0, len(columns))
	for _, name := range columns {
		field := fieldByName[name]
		if field.Kind() != reflect.Slice || field.Type().Elem().Kind() != reflect.String { panic(fmt.Sprintf("runtime error: Markdown table column '%s' must be String[]", name)) }
		arr := make([]string, field.Len())
		for i := 0; i < field.Len(); i++ { arr[i] = field.Index(i).String() }
		if rowCount == -1 { rowCount = len(arr) } else if rowCount != len(arr) { panic("runtime error: Markdown table columns must have equal lengths") }
		colArrays = append(colArrays, arr)
	}
	return columns, colArrays
}

func __octMarkdownTable(table any) []string {
	columns, colArrays := __octMarkdownRecordColumns(table, nil, false)
	return __octMarkdownRenderTable(columns, colArrays)
}

func __octMarkdownTableWithColumns(table any, requested []string) []string {
	columns, colArrays := __octMarkdownRecordColumns(table, requested, true)
	return __octMarkdownRenderTable(columns, colArrays)
}

func __octMarkdownRenderTable(columns []string, colArrays [][]string) []string {
	header := "|"
	sep := "|"
	for _, name := range columns {
		header += " " + __octMarkdownEscapeTableCell(name) + " |"
		sep += " --- |"
	}
	out := []string{header, sep}
	rowCount := 0
	if len(colArrays) > 0 { rowCount = len(colArrays[0]) }
	for r := 0; r < rowCount; r++ {
		line := "|"
		for c := 0; c < len(columns); c++ { line += " " + __octMarkdownEscapeTableCell(colArrays[c][r]) + " |" }
		out = append(out, line)
	}
	return out
}
`

const __octWriteHelpers = `
func __octWriteOctagon(path string, value any) {
	path = __octAttributedOutputPath(path)
	if !strings.HasSuffix(path, ".octagon") {
		panic("WriteOctagon path must end with .octagon")
	}
	rendered, err := __octSerialize(reflect.ValueOf(value), 0)
	if err != nil {
		panic(fmt.Sprintf("WriteOctagon cannot serialize value: %v", err))
	}
	if err := os.WriteFile(path, []byte(rendered+"\n"), 0o644); err != nil {
		panic(fmt.Sprintf("WriteOctagon write %s: %v", path, err))
	}
}

func __octAttributedOutputPath(path string) string {
	prefix := os.Getenv("OCT_OUTPUT_PATH_PREFIX")
	if prefix == "" {
		return path
	}
	return filepath.Join(filepath.Dir(path), prefix+"."+filepath.Base(path))
}

func __octSerialize(v reflect.Value, depth int) (string, error) {
	if !v.IsValid() {
		return "", fmt.Errorf("invalid value")
	}
	if v.Kind() == reflect.Interface {
		return __octSerialize(v.Elem(), depth)
	}
	if meta, ok := __octEnumMetaByGoType[__octTypeKey(v.Type())]; ok {
		idx := int(v.Int())
		if idx < 0 || idx >= len(meta.Variants) {
			return "", fmt.Errorf("enum %s variant index %d out of range", meta.ShortName, idx)
		}
		return meta.ShortName + "." + meta.Variants[idx], nil
	}
	switch v.Kind() {
	case reflect.Int:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.String:
		return strconv.Quote(v.String()), nil
	case reflect.Slice:
		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			part, err := __octSerialize(v.Index(i), depth)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case reflect.Struct:
		meta, ok := __octRecordMetaByGoType[__octTypeKey(v.Type())]
		if !ok {
			return "", fmt.Errorf("record type %q is not representable in .octagon output", v.Type().String())
		}
		fields := make([]string, 0, len(meta.Fields))
		for _, field := range meta.Fields {
			fieldValue := v.FieldByName(field)
			value, err := __octSerialize(fieldValue, depth+1)
			if err != nil {
				return "", err
			}
			if __octNeedsFieldParens(fieldValue) {
				value = "(" + value + ")"
			}
			fields = append(fields, fmt.Sprintf("%s%s: %s", __octIndent(depth+1), field, value))
		}
		return fmt.Sprintf("%s {\n%s\n%s}", meta.ShortName, strings.Join(fields, "\n"), __octIndent(depth)), nil
	default:
		return "", fmt.Errorf("value kind %s is not representable in .octagon output", v.Kind().String())
	}
}

func __octIndent(depth int) string {
	return strings.Repeat("    ", depth)
}

func __octNeedsFieldParens(v reflect.Value) bool {
	for v.Kind() == reflect.Interface {
		if !v.IsValid() || v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	return v.Kind() == reflect.Int || v.Kind() == reflect.Float64
}
`

const __octLoadHelpers = `
type __octParser struct {
	input string
	pos int
}

func __octLoadOctagonTyped(path string, target reflect.Type, expectedType string) (any, error) {
	if !strings.HasSuffix(path, ".octagon") {
		return nil, errors.New("LoadOctagon path must end with .octagon")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	value, err := (__octParser{input: string(data)}).parse()
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	out, err := __octMaterialize(value, target, expectedType)
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	return out.Interface(), nil
}

func (p __octParser) parse() (__octParsedValue, error) {
	p.skipWS()
	v, err := p.parseValue()
	if err != nil {
		return __octParsedValue{}, err
	}
	p.skipWS()
	if p.pos != len(p.input) {
		return __octParsedValue{}, fmt.Errorf("expected end of file after top-level value")
	}
	return v, nil
}

func (p *__octParser) parseValue() (__octParsedValue, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return __octParsedValue{}, fmt.Errorf("expected expression")
	}
	switch p.input[p.pos] {
	case '"':
		s, err := p.parseString()
		if err != nil {
			return __octParsedValue{}, err
		}
		return __octParsedValue{Kind: __octParsedString, Text: s}, nil
	case '[':
		return p.parseArray()
	}
	r, _ := utf8.DecodeRuneInString(p.input[p.pos:])
	if r == '-' || unicode.IsDigit(r) {
		return p.parseNumber()
	}
	id, err := p.parseIdentifier()
	if err != nil {
		return __octParsedValue{}, err
	}
	switch id {
	case "true":
		return __octParsedValue{Kind: __octParsedBool, Bool: true}, nil
	case "false":
		return __octParsedValue{Kind: __octParsedBool, Bool: false}, nil
	}
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		return p.parseRecord(id)
	}
	if dot := strings.LastIndex(id, "."); dot > 0 && dot < len(id)-1 {
		return __octParsedValue{Kind: __octParsedEnum, EnumType: id[:dot], EnumVariant: id[dot+1:]}, nil
	}
	return __octParsedValue{}, fmt.Errorf("expected expression")
}

func (p *__octParser) parseArray() (__octParsedValue, error) {
	p.pos++
	items := []__octParsedValue{}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			return __octParsedValue{Kind: __octParsedArray, Array: items}, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return __octParsedValue{}, err
		}
		items = append(items, v)
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			return __octParsedValue{Kind: __octParsedArray, Array: items}, nil
		}
		return __octParsedValue{}, fmt.Errorf("expected ',' or ']'")
	}
}

func (p *__octParser) parseRecord(name string) (__octParsedValue, error) {
	p.pos++
	fields := map[string]__octParsedValue{}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '}' {
			p.pos++
			return __octParsedValue{Kind: __octParsedRecord, RecordType: name, RecordFields: fields}, nil
		}
		field, err := p.parseIdentifier()
		if err != nil {
			return __octParsedValue{}, err
		}
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ':' {
			return __octParsedValue{}, fmt.Errorf("expected ':'")
		}
		p.pos++
		value, err := p.parseValue()
		if err != nil {
			return __octParsedValue{}, err
		}
		fields[field] = value
	}
}

func (p *__octParser) parseNumber() (__octParsedValue, error) {
	start := p.pos
	if p.input[p.pos] == '-' {
		p.pos++
	}
	dot := false
	exp := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch >= '0' && ch <= '9' {
			p.pos++
			continue
		}
		if ch == '.' {
			dot = true
			p.pos++
			continue
		}
		if ch == 'e' || ch == 'E' {
			exp = true
			p.pos++
			if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
				p.pos++
			}
			continue
		}
		break
	}
	num := p.input[start:p.pos]
	if dot || exp {
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return __octParsedValue{}, fmt.Errorf("invalid Float literal %q", num)
		}
		return __octParsedValue{Kind: __octParsedFloat, Float: v}, nil
	}
	v, err := strconv.Atoi(num)
	if err != nil {
		return __octParsedValue{}, fmt.Errorf("invalid Int literal %q", num)
	}
	return __octParsedValue{Kind: __octParsedInt, Int: v}, nil
}

func (p *__octParser) parseIdentifier() (string, error) {
	p.skipWS()
	start := p.pos
	for p.pos < len(p.input) {
		r, width := utf8.DecodeRuneInString(p.input[p.pos:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' {
			p.pos += width
			continue
		}
		break
	}
	if start == p.pos {
		return "", fmt.Errorf("expected identifier")
	}
	return p.input[start:p.pos], nil
}

func (p *__octParser) parseString() (string, error) {
	start := p.pos
	p.pos++
	escape := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if escape {
			escape = false
			p.pos++
			continue
		}
		if ch == '\\' {
			escape = true
			p.pos++
			continue
		}
		if ch == '"' {
			p.pos++
			return strconv.Unquote(p.input[start:p.pos])
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (p *__octParser) skipWS() {
	for p.pos < len(p.input) {
		r, width := utf8.DecodeRuneInString(p.input[p.pos:])
		if !unicode.IsSpace(r) {
			break
		}
		p.pos += width
	}
}

func __octMaterialize(value __octParsedValue, target reflect.Type, expectedType string) (reflect.Value, error) {
	if meta, ok := __octEnumMetaByGoType[__octTypeKey(target)]; ok {
		if value.Kind != __octParsedEnum {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-enum value", expectedType)
		}
		if value.EnumType != meta.ShortName && value.EnumType != meta.FullName {
			return reflect.Value{}, fmt.Errorf("expected enum %s, got %s", expectedType, value.EnumType)
		}
		for i, v := range meta.Variants {
			if v == value.EnumVariant {
				out := reflect.New(target).Elem()
				out.SetInt(int64(i))
				return out, nil
			}
		}
		return reflect.Value{}, fmt.Errorf("enum %s has no variant %s", expectedType, value.EnumVariant)
	}
	switch target.Kind() {
	case reflect.Int:
		if value.Kind != __octParsedInt {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-int value", expectedType)
		}
		out := reflect.New(target).Elem()
		out.SetInt(int64(value.Int))
		return out, nil
	case reflect.Float64:
		if value.Kind != __octParsedFloat {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-float value", expectedType)
		}
		out := reflect.New(target).Elem()
		out.SetFloat(value.Float)
		return out, nil
	case reflect.Bool:
		if value.Kind != __octParsedBool {
			return reflect.Value{}, fmt.Errorf("expected Bool, got non-bool value")
		}
		out := reflect.New(target).Elem()
		out.SetBool(value.Bool)
		return out, nil
	case reflect.String:
		if value.Kind != __octParsedString {
			return reflect.Value{}, fmt.Errorf("expected String, got non-string value")
		}
		out := reflect.New(target).Elem()
		out.SetString(value.Text)
		return out, nil
	case reflect.Slice:
		if value.Kind != __octParsedArray {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-array value", expectedType)
		}
		out := reflect.MakeSlice(target, 0, len(value.Array))
		for i, item := range value.Array {
			element, err := __octMaterialize(item, target.Elem(), target.Elem().String())
			if err != nil {
				return reflect.Value{}, fmt.Errorf("array element %d mismatch: %w", i, err)
			}
			out = reflect.Append(out, element)
		}
		return out, nil
	case reflect.Struct:
		meta, ok := __octRecordMetaByGoType[__octTypeKey(target)]
		if !ok {
			return reflect.Value{}, fmt.Errorf("unsupported expected type %s", expectedType)
		}
		if value.Kind != __octParsedRecord {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-record value", expectedType)
		}
		if value.RecordType != meta.ShortName && value.RecordType != meta.FullName {
			return reflect.Value{}, fmt.Errorf("expected record %s, got %s", expectedType, value.RecordType)
		}
		out := reflect.New(target).Elem()
		for _, field := range meta.Fields {
			fieldValue, ok := value.RecordFields[field]
			if !ok {
				return reflect.Value{}, fmt.Errorf("record %s missing field %s", expectedType, field)
			}
			materialized, err := __octMaterialize(fieldValue, out.FieldByName(field).Type(), out.FieldByName(field).Type().String())
			if err != nil {
				return reflect.Value{}, fmt.Errorf("record field %s mismatch: %w", field, err)
			}
			out.FieldByName(field).Set(materialized)
		}
		keys := make([]string, 0, len(value.RecordFields))
		for k := range value.RecordFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			found := false
			for _, field := range meta.Fields {
				if k == field {
					found = true
					break
				}
			}
			if !found {
				return reflect.Value{}, fmt.Errorf("record %s has unexpected field %s", expectedType, k)
			}
		}
		return out, nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported expected type %s", expectedType)
	}
}
`

const __octBatchHelpers = `
func __octBatchRun[T any, U any, R any](items []T, worker func(T) R, isErr func(R) bool, errMsg func(R) string, value func(R) U) ([]U, string, bool) {
	if len(items) == 0 {
		return []U{}, "", false
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(items) {
		workerCount = len(items)
	}
	type __octBatchItemResult[V any] struct {
		index int
		value V
		err string
		isErr bool
	}
	jobs := make(chan int, len(items))
	results := make(chan __octBatchItemResult[U], len(items))
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				out := worker(items[idx])
				if isErr(out) {
					results <- __octBatchItemResult[U]{index: idx, err: errMsg(out), isErr: true}
					continue
				}
				results <- __octBatchItemResult[U]{index: idx, value: value(out)}
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)
	ordered := make([]U, len(items))
	var firstErr string
	for result := range results {
		if firstErr == "" && result.isErr {
			firstErr = result.err
			continue
		}
		ordered[result.index] = result.value
	}
	if firstErr != "" {
		return nil, firstErr, true
	}
	return ordered, "", false
}
`

const __octUtilityHelpers = `
type __octUtilitySiteState struct {
	HasCurrent bool
	Current any
	Score int
	CommitAge int
}

type __octUtilCandidate[T any] struct {
	Valid bool
	Value T
	Score int
}

func __octUtilSelect[T any](sites map[int]__octUtilitySiteState, siteID int, hysteresis int, minCommit int, candidates []__octUtilCandidate[T], elseValue T) T {
	valid := make([]__octUtilCandidate[T], 0, len(candidates))
	for _, c := range candidates {
		if c.Valid {
			valid = append(valid, c)
		}
	}
	next := __octUtilCandidate[T]{Valid: true, Value: elseValue, Score: 0}
	if len(valid) > 0 {
		next = valid[0]
		for _, c := range valid[1:] {
			if c.Score > next.Score {
				next = c
			}
		}
	}
	site := sites[siteID]
	if site.HasCurrent {
		currentStillValid := false
		for _, c := range valid {
			if reflect.DeepEqual(c.Value, site.Current) {
				currentStillValid = true
				break
			}
		}
		if currentStillValid {
			commitActive := site.CommitAge < minCommit
			hysteresisBlocks := next.Score <= site.Score+hysteresis
			if commitActive || hysteresisBlocks {
				next = __octUtilCandidate[T]{Valid: true, Value: site.Current.(T), Score: site.Score}
			}
		}
	}
	if !site.HasCurrent || !reflect.DeepEqual(site.Current, next.Value) {
		sites[siteID] = __octUtilitySiteState{HasCurrent: true, Current: next.Value, Score: next.Score, CommitAge: 1}
	} else {
		site.Score = next.Score
		site.CommitAge++
		sites[siteID] = site
	}
	return next.Value
}
`

func flowHasUtilityWhen(flow MIRFlow) bool {
	for _, state := range flow.States {
		for _, stmt := range state.Statements {
			if flowStmtHasUtility(stmt) {
				return true
			}
		}
	}
	return false
}

func flowStmtHasUtility(stmt MIRFlowStmt) bool {
	switch s := stmt.(type) {
	case MIRFlowFieldAssign:
		return flowExprHasUtility(s.Value)
	case MIRFlowLetStmt:
		return flowExprHasUtility(s.Value)
	case MIRFlowReturn:
		return flowExprHasUtility(s.Value)
	case MIRFlowIf:
		if flowExprHasUtility(s.Condition) {
			return true
		}
		for _, st := range s.Then {
			if flowStmtHasUtility(st) {
				return true
			}
		}
		for _, st := range s.Else {
			if flowStmtHasUtility(st) {
				return true
			}
		}
	case MIRFlowWhen:
		for _, c := range s.Cases {
			if flowExprHasUtility(c.Condition) || flowWhenActionHasUtility(c.Action) {
				return true
			}
		}
		return flowWhenActionHasUtility(s.Else)
	}
	return false
}

func flowWhenActionHasUtility(action MIRFlowWhenAction) bool {
	switch a := action.(type) {
	case MIRFlowWhenReturn:
		return flowExprHasUtility(a.Value)
	case MIRFlowWhenBlock:
		for _, statement := range a.Statements {
			if flowStmtHasUtility(statement) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func flowExprHasUtility(expr MIRFlowExpr) bool {
	switch e := expr.(type) {
	case MIRFlowBinaryExpr:
		return flowExprHasUtility(e.Left) || flowExprHasUtility(e.Right)
	case MIRFlowUnaryExpr:
		return flowExprHasUtility(e.Operand)
	case MIRFlowCallExpr:
		for _, arg := range e.Args {
			if flowExprHasUtility(arg) {
				return true
			}
		}
		return false
	case MIRFlowIndexExpr:
		return flowExprHasUtility(e.Target) || flowExprHasUtility(e.Index)
	case MIRFlowRecordLiteralExpr:
		for _, field := range e.FieldVals {
			if flowExprHasUtility(field) {
				return true
			}
		}
		return false
	case MIRFlowUtilityWhenExpr:
		return true
	case MIRFlowIfExpr:
		return flowExprHasUtility(e.Condition) || flowExprHasUtility(e.Then) || flowExprHasUtility(e.Else)
	default:
		return false
	}
}

func emitGoFlow(b *strings.Builder, flow MIRFlow) error {
	structName := "__octFlow_" + flow.Package + "_" + flow.Name
	resultType := flow.Return
	fmt.Fprintf(b, "type %s struct {\n", structName)
	b.WriteString("\tstarted bool\n\tcompleted bool\n\tcurrentState int\n\tinstruction int\n")
	fmt.Fprintf(b, "\tresult %s\n\thasResult bool\n\thistory []string\n", goFlowResultType(resultType))
	b.WriteString("\thasResumeTarget bool\n\tresumeTarget int\n")
	if flowHasUtilityWhen(flow) {
		b.WriteString("\tutilitySites map[int]__octUtilitySiteState\n")
	}
	for _, p := range flow.Parameters {
		fmt.Fprintf(b, "\t%s %s\n", p.Name, goType(p.Type))
	}
	if len(flow.Board) > 0 {
		b.WriteString("\tboard struct {\n")
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\t\t%s %s\n", field.Name, goType(field.Type))
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(b, "func fn_%s_%s(", flow.Package, flow.Name)
	for i, p := range flow.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s %s", p.Name, goType(p.Type))
	}
	fmt.Fprintf(b, ") %s {\n", goType(flowInstanceTypeString(resultType)))
	fmt.Fprintf(b, "\tf := &%s{}\n", structName)
	if flowHasUtilityWhen(flow) {
		b.WriteString("\tf.utilitySites = map[int]__octUtilitySiteState{}\n")
	}
	for _, p := range flow.Parameters {
		fmt.Fprintf(b, "\tf.%s = %s\n", p.Name, p.Name)
	}
	b.WriteString("\treturn f\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octActive() string {\n", structName)
	b.WriteString("\tif !f.started || f.completed { return \"\" }\n\tswitch f.currentState {\n")
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\tcase %d: return %q\n", idx, state.Name)
	}
	b.WriteString("\tdefault: return \"\"\n\t}\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octComplete() bool { return f.completed }\n\n", structName)
	fmt.Fprintf(b, "func (f *%s) __octResult() (%s, bool) {\n", structName, goFlowResultType(resultType))
	b.WriteString("\treturn f.result, f.hasResult\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octStateHistory() []string {\n", structName)
	b.WriteString("\tout := make([]string, len(f.history))\n\tcopy(out, f.history)\n\treturn out\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octStateName(id int) string {\n", structName)
	b.WriteString("\tswitch id {\n")
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\tcase %d: return %q\n", idx, state.Name)
	}
	b.WriteString("\tdefault: return \"\"\n\t}\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octResumeTarget() string {\n", structName)
	b.WriteString("\tif !f.hasResumeTarget { return \"\" }\n\treturn f.__octStateName(f.resumeTarget)\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octBoardSnapshot() (any, bool) {\n", structName)
	if len(flow.Board) == 0 {
		b.WriteString("\treturn nil, false\n}\n\n")
	} else {
		fmt.Fprintf(b, "\treturn %s_%sBoardSnapshot{\n", flow.Package, flow.Name)
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\t\t%s: f.board.%s,\n", field.Name, field.Name)
		}
		b.WriteString("\t}, true\n}\n\n")
	}
	fmt.Fprintf(b, "func (f *%s) __octStep() {\n", structName)
	for _, local := range collectFlowLetLocals(flow) {
		fmt.Fprintf(b, "\tvar %s %s\n", local.Name, goType(local.Type))
	}
	b.WriteString("\tif f.completed { return }\n")
	entryID := 0
	for idx, st := range flow.States {
		if st.Name == flow.EntryState {
			entryID = idx
			break
		}
	}
	fmt.Fprintf(b, "\tif !f.started { f.started = true; f.currentState = %d; f.instruction = 0; f.history = append(f.history, %q) }\n", entryID, flow.EntryState)
	b.WriteString("\tfor {\n\t\tswitch f.currentState {\n")
	stateIDs := map[string]int{}
	for idx, state := range flow.States {
		stateIDs[state.Name] = idx
	}
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\t\tcase %d:\n\t\t\tswitch f.instruction {\n", idx)
		for stmtIdx, stmt := range state.Statements {
			fmt.Fprintf(b, "\t\t\tcase %d:\n", stmtIdx)
			src, err := emitGoFlowStmt(stmt, flow.Package, stateIDs, resultType)
			if err != nil {
				return fmt.Errorf("flow %s.%s state %s: %w", flow.Package, flow.Name, state.Name, err)
			}
			for _, line := range strings.Split(src, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				fmt.Fprintf(b, "\t\t\t\t%s\n", line)
			}
		}
		fmt.Fprintf(b, "\t\t\tdefault:\n\t\t\t\tpanic(\"runtime invariant violation: flow state %s exited without suspend or return\")\n", state.Name)
		b.WriteString("\t\t\t}\n")
	}
	b.WriteString("\t\tdefault:\n\t\t\tpanic(\"runtime invariant violation: unknown flow state\")\n\t\t}\n\t}\n}\n\n")
	return nil
}

func collectFlowLetLocals(flow MIRFlow) []MIRField {
	seen := map[string]struct{}{}
	locals := []MIRField{}
	var visitStmt func(MIRFlowStmt)
	var visitWhenAction func(MIRFlowWhenAction)
	visitWhenAction = func(action MIRFlowWhenAction) {
		switch a := action.(type) {
		case MIRFlowWhenBlock:
			for _, st := range a.Statements {
				visitStmt(st)
			}
		}
	}
	visitStmt = func(stmt MIRFlowStmt) {
		switch s := stmt.(type) {
		case MIRFlowLetStmt:
			if _, ok := seen[s.Name]; ok {
				return
			}
			seen[s.Name] = struct{}{}
			locals = append(locals, MIRField{Name: s.Name, Type: s.Type})
		case MIRFlowIf:
			for _, st := range s.Then {
				visitStmt(st)
			}
			for _, st := range s.Else {
				visitStmt(st)
			}
		case MIRFlowWhen:
			for _, c := range s.Cases {
				visitWhenAction(c.Action)
			}
			visitWhenAction(s.Else)
		}
	}
	for _, state := range flow.States {
		for _, st := range state.Statements {
			visitStmt(st)
		}
	}
	return locals
}

func emitGoFlowStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowGoto:
		target, ok := stateIDs[s.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", s.Target)
		}
		return fmt.Sprintf("f.currentState = %d; f.instruction = 0; f.history = append(f.history, %q); continue", target, s.Target), nil
	case MIRFlowSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowRemember:
		return "f.hasResumeTarget = true\nf.resumeTarget = f.currentState\nf.instruction++\ncontinue", nil
	case MIRFlowResume:
		return "if !f.hasResumeTarget { panic(\"runtime error: resume called with empty resume slot\") }\n__resumeTarget := f.resumeTarget\nf.hasResumeTarget = false\nf.resumeTarget = -1\nf.currentState = __resumeTarget\nf.instruction = 0\nf.history = append(f.history, f.__octStateName(__resumeTarget))\ncontinue", nil
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s\nf.instruction++\ncontinue", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s\nf.instruction++\ncontinue", s.Target, s.Field, v), nil
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s\nf.instruction++\ncontinue", s.Name, v), nil
	case MIRFlowReturn:
		if s.Value == nil {
			if resultType == "Void" {
				return "f.result = __octVoid{}\nf.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
			}
			return "f.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
		}
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	case MIRFlowIf:
		cond, err := emitGoFlowExpr(s.Condition, pkg)
		if err != nil {
			return "", err
		}
		thenSrc, err := emitGoFlowInlineBlock(s.Then, pkg, stateIDs, resultType)
		if err != nil {
			return "", err
		}
		out := "if " + cond + " {\n" + thenSrc + "\n}"
		if len(s.Else) > 0 {
			elseSrc, err := emitGoFlowInlineBlock(s.Else, pkg, stateIDs, resultType)
			if err != nil {
				return "", err
			}
			out += " else {\n" + elseSrc + "\n}"
		}
		out += "\nf.instruction++\ncontinue"
		return out, nil
	case MIRFlowWhen:
		lines := []string{}
		for _, c := range s.Cases {
			cond, err := emitGoFlowExpr(c.Condition, pkg)
			if err != nil {
				return "", err
			}
			action, err := emitGoFlowWhenAction(c.Action, pkg, stateIDs, resultType)
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("if %s {\n%s\n}", cond, action))
		}
		elseAction, err := emitGoFlowWhenAction(s.Else, pkg, stateIDs, resultType)
		if err != nil {
			return "", err
		}
		lines = append(lines, elseAction)
		return strings.Join(lines, "\n"), nil
	default:
		return "", unsupported(fmt.Sprintf("flow statement %T", stmt))
	}
}

func emitGoFlowInlineBlock(stmts []MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	lines := make([]string, 0, len(stmts))
	for _, stmt := range stmts {
		src, err := emitGoFlowInlineStmt(stmt, pkg, stateIDs, resultType)
		if err != nil {
			return "", err
		}
		lines = append(lines, src)
	}
	return strings.Join(lines, "\n"), nil
}

func emitGoFlowInlineStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s", s.Target, s.Field, v), nil
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name, v), nil
	case MIRFlowIf:
		return emitGoFlowStmt(s, pkg, stateIDs, resultType)
	default:
		return emitGoFlowStmt(s, pkg, stateIDs, resultType)
	}
}

func emitGoFlowWhenAction(action MIRFlowWhenAction, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	switch a := action.(type) {
	case MIRFlowWhenGoto:
		target, ok := stateIDs[a.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", a.Target)
		}
		return fmt.Sprintf("f.currentState = %d\nf.instruction = 0\nf.history = append(f.history, %q)\ncontinue", target, a.Target), nil
	case MIRFlowWhenSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowWhenReturn:
		v, err := emitGoFlowExpr(a.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	case MIRFlowWhenBlock:
		lines := make([]string, 0, len(a.Statements))
		for _, statement := range a.Statements {
			src, err := emitGoFlowWhenBlockStmt(statement, pkg, stateIDs, resultType)
			if err != nil {
				return "", err
			}
			lines = append(lines, src)
		}
		return strings.Join(lines, "\n"), nil
	default:
		return "", unsupported(fmt.Sprintf("flow when action %T", action))
	}
}

func emitGoFlowWhenBlockStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowRemember:
		return "f.hasResumeTarget = true\nf.resumeTarget = f.currentState", nil
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name, v), nil
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s", s.Target, s.Field, v), nil
	case MIRFlowGoto:
		target, ok := stateIDs[s.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", s.Target)
		}
		return fmt.Sprintf("f.currentState = %d\nf.instruction = 0\nf.history = append(f.history, %q)\ncontinue", target, s.Target), nil
	case MIRFlowSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowResume:
		return "if !f.hasResumeTarget { panic(\"runtime error: resume called with empty resume slot\") }\n__resumeTarget := f.resumeTarget\nf.hasResumeTarget = false\nf.resumeTarget = -1\nf.currentState = __resumeTarget\nf.instruction = 0\nf.history = append(f.history, f.__octStateName(__resumeTarget))\ncontinue", nil
	case MIRFlowReturn:
		if s.Value == nil {
			if resultType == "Void" {
				return "f.result = __octVoid{}\nf.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
			}
			return "f.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
		}
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	default:
		return "", unsupported(fmt.Sprintf("flow when block statement %T", stmt))
	}
}

func emitGoFlowBlock(block []MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string) (string, error) {
	parts := make([]string, 0, len(block))
	for _, st := range block {
		src, err := emitGoFlowStmt(st, pkg, stateIDs, resultType)
		if err != nil {
			return "", err
		}
		parts = append(parts, src)
	}
	return strings.Join(parts, "\n"), nil
}

func emitGoFlowExpr(expr MIRFlowExpr, pkg string) (string, error) {
	switch e := expr.(type) {
	case MIRFlowLiteralExpr:
		return e.Value, nil
	case MIRFlowIdentifierExpr:
		if e.IsLocal {
			return e.Name, nil
		}
		return "f." + e.Name, nil
	case MIRFlowFieldExpr:
		if e.Target == "board" {
			return "f.board." + e.Field, nil
		}
		return "f." + e.Target + "." + e.Field, nil
	case MIRFlowBinaryExpr:
		l, err := emitGoFlowExpr(e.Left, pkg)
		if err != nil {
			return "", err
		}
		r, err := emitGoFlowExpr(e.Right, pkg)
		if err != nil {
			return "", err
		}
		op := e.Operator
		if op == "and" {
			op = "&&"
		}
		if op == "or" {
			op = "||"
		}
		return fmt.Sprintf("(%s %s %s)", l, op, r), nil
	case MIRFlowUnaryExpr:
		v, err := emitGoFlowExpr(e.Operand, pkg)
		if err != nil {
			return "", err
		}
		op := e.Operator
		if op == "not" {
			op = "!"
		}
		return fmt.Sprintf("(%s%s)", op, v), nil
	case MIRFlowCallExpr:
		args := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			v, err := emitGoFlowExpr(arg, pkg)
			if err != nil {
				return "", err
			}
			args = append(args, v)
		}
		if !e.Builtin {
			return fmt.Sprintf("fn_%s(%s)", strings.ReplaceAll(e.Callee, ".", "_"), strings.Join(args, ", ")), nil
		}
		return emitGoBuiltinCallExpr(e.Callee, args)
	case MIRFlowIndexExpr:
		target, err := emitGoFlowExpr(e.Target, pkg)
		if err != nil {
			return "", err
		}
		idx, err := emitGoFlowExpr(e.Index, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s[%s]", target, idx), nil
	case MIRFlowRecordLiteralExpr:
		parts := make([]string, 0, len(e.FieldNames))
		for i := range e.FieldNames {
			v, err := emitGoFlowExpr(e.FieldVals[i], pkg)
			if err != nil {
				return "", err
			}
			parts = append(parts, fmt.Sprintf("%s: %s", e.FieldNames[i], v))
		}
		return fmt.Sprintf("%s{%s}", goType(e.TypeName), strings.Join(parts, ", ")), nil
	case MIRFlowUtilityWhenExpr:
		h, err := emitGoFlowExpr(e.Hysteresis, pkg)
		if err != nil {
			return "", err
		}
		m, err := emitGoFlowExpr(e.MinCommit, pkg)
		if err != nil {
			return "", err
		}
		elseExpr, err := emitGoFlowExpr(e.Else, pkg)
		if err != nil {
			return "", err
		}
		cases := make([]string, 0, len(e.Cases))
		valueType := goType(e.ResultType)
		for _, c := range e.Cases {
			v, err := emitGoFlowExpr(c.Value, pkg)
			if err != nil {
				return "", err
			}
			cond, err := emitGoFlowExpr(c.Condition, pkg)
			if err != nil {
				return "", err
			}
			score, err := emitGoFlowExpr(c.Score, pkg)
			if err != nil {
				return "", err
			}
			cases = append(cases, fmt.Sprintf("{Valid: %s, Value: %s, Score: %s}", cond, v, score))
		}
		sites := "map[int]__octUtilitySiteState{}"
		if e.ControllerBound {
			sites = "f.utilitySites"
		}
		return fmt.Sprintf("__octUtilSelect[%s](%s, %d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
			valueType, sites, e.SiteID, h, m, valueType, strings.Join(cases, ", "), elseExpr), nil
	case MIRFlowSwitchExpr:
		ifChain := make([]string, 0, len(e.Cases))
		subject := ""
		if e.Subject != nil {
			s, err := emitGoFlowExpr(e.Subject, pkg)
			if err != nil {
				return "", err
			}
			subject = s
		}
		for idx, c := range e.Cases {
			match, err := emitGoFlowExpr(c.Match, pkg)
			if err != nil {
				return "", err
			}
			value, err := emitGoFlowExpr(c.Value, pkg)
			if err != nil {
				return "", err
			}
			cond := match
			if subject != "" {
				cond = fmt.Sprintf("(%s == %s)", subject, match)
			}
			keyword := "if"
			if idx > 0 {
				keyword = "else if"
			}
			ifChain = append(ifChain, fmt.Sprintf("%s %s { return %s }", keyword, cond, value))
		}
		body := strings.Join(ifChain, " ")
		if e.Else != nil {
			elseExpr, err := emitGoFlowExpr(e.Else, pkg)
			if err != nil {
				return "", err
			}
			if body != "" {
				body += "; "
			}
			body += fmt.Sprintf("return %s", elseExpr)
		}
		return fmt.Sprintf("func() %s { %s }()", goType(e.ResultType), body), nil
	case MIRFlowMatchExpr:
		subject, err := emitGoFlowExpr(e.Subject, pkg)
		if err != nil {
			return "", err
		}
		subjectVar := "__oct_match_subject"
		switchCases := make([]string, 0, len(e.Cases))
		for _, c := range e.Cases {
			body := ""
			if c.Binding != "" {
				body = fmt.Sprintf("%s := %s.Payload.(%s); _ = %s; ", c.Binding, subjectVar, goType(c.BindingType), c.Binding)
			}
			value, err := emitGoFlowExpr(c.Value, pkg)
			if err != nil {
				return "", err
			}
			body += fmt.Sprintf("return %s", value)
			switchCases = append(switchCases, fmt.Sprintf("case %s_%s_tag: %s", enumShortName(e.SubjectType), c.Variant, body))
		}
		switchCases = append(switchCases, `default: panic("non-exhaustive match reached in compiled flow mode")`)
		return fmt.Sprintf("func() %s { %s := %s; switch %s.Tag {\n%s\n} }()", goType(e.ResultType), subjectVar, subject, subjectVar, strings.Join(switchCases, "\n")), nil
	case MIRFlowIfExpr:
		condition, err := emitGoFlowExpr(e.Condition, pkg)
		if err != nil {
			return "", err
		}
		thenExpr, err := emitGoFlowExpr(e.Then, pkg)
		if err != nil {
			return "", err
		}
		elseExpr, err := emitGoFlowExpr(e.Else, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", goType(e.ResultType), condition, thenExpr, elseExpr), nil
	default:
		return "", unsupported(fmt.Sprintf("flow expression %T", expr))
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

func octxiliaryKindExpr(t string) string {
	return octxiliaryKindExprWithTransport(t, nil)
}

func octxiliaryKindExprWithTransport(t string, transportTypes []project.TransportTypeMetadata) string {
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "handle" {
		return "octxiliary.ValueHandle"
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

func octxiliaryValueExpr(t string, expr string) (string, error) {
	return octxiliaryValueExprWithTransport(t, expr, nil)
}

func octxiliaryValueExprWithTransport(t string, expr string, transportTypes []project.TransportTypeMetadata) (string, error) {
	return octxiliaryValueExprWithTransportFamily(t, expr, transportTypes, "")
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

func octxiliaryValueExtractExpr(t string, value string) string {
	return octxiliaryValueExtractExprWithTransport(t, value, nil)
}

func octxiliaryValueExtractExprWithTransport(t string, value string, transportTypes []project.TransportTypeMetadata) string {
	if transport := findTransportRecord(transportTypes, t); transport.ok && transport.typ.Kind == "handle" {
		return fmt.Sprintf("%s{Handle: %s.HandleID}", goType(t), value)
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
		return fmt.Sprintf("%s = %s", st.Target, st.Value), nil
	case MIRConstructArray:
		return fmt.Sprintf("%s = []%s{%s}", st.Target, goType(st.ElemType), strings.Join(st.Values, ", ")), nil
	case MIRConstructRecord:
		parts := make([]string, 0, len(st.FieldNames))
		for i := range st.FieldNames {
			parts = append(parts, fmt.Sprintf("%s: %s", st.FieldNames[i], st.FieldVals[i]))
		}
		return fmt.Sprintf("%s = %s{%s}", st.Target, goType(st.TypeName), strings.Join(parts, ", ")), nil
	case MIRGenericOctxiliaryCall:
		valueArgs := make([]string, 0, len(st.Args))
		for i, arg := range st.Args {
			valueExpr, err := octxiliaryValueExprWithTransportFamily(st.ArgTypes[i], arg, st.TransportTypes, st.Family)
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
		if st.Builtin {
			switch canonicalCompiledBuiltinName(st.Callee) {
			case "Idx":
				return fmt.Sprintf("%s = __octIdx(%s)", st.Target, st.Args[0]), nil
			case "EinMul":
				return fmt.Sprintf("%s = __octEinMulMM(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "EinAdd":
				return fmt.Sprintf("%s = __octEinAddMM(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "EinSub":
				return fmt.Sprintf("%s = __octEinSubMM(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "EinAddVV":
				return fmt.Sprintf("%s = __octEinAddVV(%s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3]), nil
			case "EinSubVV":
				return fmt.Sprintf("%s = __octEinSubVV(%s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3]), nil
			case "EinDotVV":
				return fmt.Sprintf("%s = __octEinDotVV(%s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3]), nil
			case "EinOuterVV":
				return fmt.Sprintf("%s = __octEinOuterVV(%s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3]), nil
			case "EinMulMV":
				return fmt.Sprintf("%s = __octEinMulMV(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "EinMulVM":
				return fmt.Sprintf("%s = __octEinMulVM(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "EinDoubleMM":
				return fmt.Sprintf("%s = __octEinDoubleMM(%s, %s, %s, %s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Args[4], st.Args[5]), nil
			case "Len":
				return fmt.Sprintf("%s = len(%s)", st.Target, st.Args[0]), nil
			case "Append":
				return fmt.Sprintf("%s = append(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Print":
				return fmt.Sprintf("fmt.Println(%s); %s = 0", st.Args[0], st.Target), nil
			case "Require":
				return fmt.Sprintf("if !%s { panic(\"runtime error: \" + %s) }; %s = 0", st.Args[0], st.Args[1], st.Target), nil
			case "FormatFloat":
				return fmt.Sprintf("%s = strconv.FormatFloat(%s, 'f', int(%s), 64)", st.Target, st.Args[0], st.Args[1]), nil
			case "Assert.True":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !%s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", st.Args[0], st.Args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !%s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", st.Args[0], st.Args[1], st.Target), nil
			case "Assert.False":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if %s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", st.Args[0], st.Args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if %s { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", st.Args[0], st.Args[1], st.Target), nil
			case "Assert.Equal":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !reflect.DeepEqual(%s, %s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", st.Args[0], st.Args[1], st.Args[2]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !reflect.DeepEqual(%s, %s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", st.Args[0], st.Args[1], st.Args[2], st.Target), nil
			case "Assert.Near":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if math.Abs((%s)-(%s)) > (%s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", st.Args[0], st.Args[1], st.Args[2], st.Args[3]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if math.Abs((%s)-(%s)) > (%s) { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", st.Args[0], st.Args[1], st.Args[2], st.Args[3], st.Target), nil
			case "Assert.Error":
				if st.Target == "_" {
					return fmt.Sprintf("__octAssertionCount++; if !%s.IsErr { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }", st.Args[0], st.Args[1]), nil
				}
				return fmt.Sprintf("__octAssertionCount++; if !%s.IsErr { fmt.Fprintf(os.Stderr, \"assertion failed: %%s\\n\", %s); os.Exit(1) }; %s = __octVoid{}", st.Args[0], st.Args[1], st.Target), nil
			case "ToString":
				return fmt.Sprintf("%s = fmt.Sprint(%s)", st.Target, st.Args[0]), nil
			case "Float":
				return fmt.Sprintf("%s = float64(%s)", st.Target, st.Args[0]), nil
			case "Clamp01":
				return fmt.Sprintf("%s = func(__v float64) float64 { if __v < 0.0 { return 0.0 }; if __v > 1.0 { return 1.0 }; return __v }(%s)", st.Target, st.Args[0]), nil
			case "Complex":
				return fmt.Sprintf("%s = complex(float64(%s), float64(%s))", st.Target, st.Args[0], st.Args[1]), nil
			case "ComplexPolar":
				return fmt.Sprintf("%s = cmplx.Rect(float64(%s), float64(%s))", st.Target, st.Args[0], st.Args[1]), nil
			case "I":
				return fmt.Sprintf("%s = complex(0, 1)", st.Target), nil
			case "Real":
				return fmt.Sprintf("%s = __octComplexReal(%s)", st.Target, st.Args[0]), nil
			case "Imag":
				return fmt.Sprintf("%s = __octComplexImag(%s)", st.Target, st.Args[0]), nil
			case "Arg":
				return fmt.Sprintf("%s = cmplx.Phase(%s)", st.Target, st.Args[0]), nil
			case "Conj":
				return fmt.Sprintf("%s = cmplx.Conj(%s)", st.Target, st.Args[0]), nil
			case "Pi":
				return fmt.Sprintf("%s = math.Pi", st.Target), nil
			case "E":
				return fmt.Sprintf("%s = math.E", st.Target), nil
			case "Abs":
				if len(st.ArgTypes) == 1 && isComplexScalarTypeString(st.ArgTypes[0]) {
					return fmt.Sprintf("%s = __octComplexAbs(%s)", st.Target, st.Args[0]), nil
				}
				if isIntScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = func(__v int) int { if __v < 0 { return -__v }; return __v }(%s)", st.Target, st.Args[0]), nil
				}
				if isFloatScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = math.Abs(%s)", st.Target, st.Args[0]), nil
				}
				return "", fmt.Errorf("compiled mode does not yet support builtin Abs for type %s", st.RetType)
			case "Sqrt":
				return fmt.Sprintf("%s = math.Sqrt(float64(%s))", st.Target, st.Args[0]), nil
			case "Sin":
				return fmt.Sprintf("%s = math.Sin(float64(%s))", st.Target, st.Args[0]), nil
			case "Cos":
				return fmt.Sprintf("%s = math.Cos(float64(%s))", st.Target, st.Args[0]), nil
			case "Tan":
				return fmt.Sprintf("%s = math.Tan(float64(%s))", st.Target, st.Args[0]), nil
			case "Asin":
				return fmt.Sprintf("%s = math.Asin(float64(%s))", st.Target, st.Args[0]), nil
			case "Acos":
				return fmt.Sprintf("%s = math.Acos(float64(%s))", st.Target, st.Args[0]), nil
			case "Atan":
				return fmt.Sprintf("%s = math.Atan(float64(%s))", st.Target, st.Args[0]), nil
			case "Atan2":
				return fmt.Sprintf("%s = math.Atan2(float64(%s), float64(%s))", st.Target, st.Args[0], st.Args[1]), nil
			case "Exp":
				if isComplexScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = cmplx.Exp(%s)", st.Target, st.Args[0]), nil
				}
				return fmt.Sprintf("%s = math.Exp(float64(%s))", st.Target, st.Args[0]), nil
			case "Ln":
				if isComplexScalarTypeString(st.RetType) {
					return fmt.Sprintf("%s = cmplx.Log(%s)", st.Target, st.Args[0]), nil
				}
				return fmt.Sprintf("%s = math.Log(float64(%s))", st.Target, st.Args[0]), nil
			case "Pow":
				return fmt.Sprintf("%s = math.Pow(float64(%s), float64(%s))", st.Target, st.Args[0], st.Args[1]), nil
			case "Log10":
				return fmt.Sprintf("%s = math.Log10(float64(%s))", st.Target, st.Args[0]), nil
			case "Sinh":
				return fmt.Sprintf("%s = math.Sinh(float64(%s))", st.Target, st.Args[0]), nil
			case "Cosh":
				return fmt.Sprintf("%s = math.Cosh(float64(%s))", st.Target, st.Args[0]), nil
			case "Tanh":
				return fmt.Sprintf("%s = math.Tanh(float64(%s))", st.Target, st.Args[0]), nil
			case "FloorToInt", "Math.FloorToInt":
				return fmt.Sprintf("%s = int(math.Floor(float64(%s)))", st.Target, st.Args[0]), nil
			case "CeilToInt", "Math.CeilToInt":
				return fmt.Sprintf("%s = int(math.Ceil(float64(%s)))", st.Target, st.Args[0]), nil
			case "RoundToInt":
				return fmt.Sprintf("%s = int(math.Round(float64(%s)))", st.Target, st.Args[0]), nil
			case "BaseValue":
				return fmt.Sprintf("%s = float64(%s)", st.Target, st.Args[0]), nil
			case "Contains":
				return fmt.Sprintf("%s = strings.Contains(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "StartsWith":
				return fmt.Sprintf("%s = strings.HasPrefix(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "EndsWith":
				return fmt.Sprintf("%s = strings.HasSuffix(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Trim":
				return fmt.Sprintf("%s = strings.TrimSpace(%s)", st.Target, st.Args[0]), nil
			case "Lower":
				return fmt.Sprintf("%s = strings.ToLower(%s)", st.Target, st.Args[0]), nil
			case "Upper":
				return fmt.Sprintf("%s = strings.ToUpper(%s)", st.Target, st.Args[0]), nil
			case "Join":
				return fmt.Sprintf("%s = strings.Join(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil

			case "StringByteLength":
				return fmt.Sprintf("%s = len(%s)", st.Target, st.Args[0]), nil
			case "StringRuneCount":
				return fmt.Sprintf("%s = utf8.RuneCountInString(%s)", st.Target, st.Args[0]), nil
			case "StringJoin":
				return fmt.Sprintf("%s = strings.Join(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "StringConcat":
				return fmt.Sprintf("%s = strings.Join(%s, \"\")", st.Target, st.Args[0]), nil
			case "StringFrom":
				return fmt.Sprintf("%s = fmt.Sprint(%s)", st.Target, st.Args[0]), nil
			case "StringReplaceAll":
				return fmt.Sprintf("%s = strings.ReplaceAll(%s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2]), nil
			case "StringContains":
				return fmt.Sprintf("%s = strings.Contains(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "StringStartsWith":
				return fmt.Sprintf("%s = strings.HasPrefix(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "StringEndsWith":
				return fmt.Sprintf("%s = strings.HasSuffix(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "StringTrim":
				return fmt.Sprintf("%s = strings.TrimSpace(%s)", st.Target, st.Args[0]), nil
			case "StringSplitLines":
				return fmt.Sprintf("%s = __octStringSplitLines(%s)", st.Target, st.Args[0]), nil
			case "StringEscapeJSON":
				return fmt.Sprintf("%s = __octStringEscapeJSON(%s)", st.Target, st.Args[0]), nil
			case "StringQuoteJSON":
				return fmt.Sprintf("%s = strconv.Quote(%s)", st.Target, st.Args[0]), nil
			case "MarkdownEscapeText":
				return fmt.Sprintf("%s = __octMarkdownNormalizeInline(%s)", st.Target, st.Args[0]), nil
			case "MarkdownEscapeTableCell":
				return fmt.Sprintf("%s = __octMarkdownEscapeTableCell(%s)", st.Target, st.Args[0]), nil
			case "MarkdownH1":
				return fmt.Sprintf("%s = []string{\"# \" + __octMarkdownNormalizeInline(%s)}", st.Target, st.Args[0]), nil
			case "MarkdownH2":
				return fmt.Sprintf("%s = []string{\"## \" + __octMarkdownNormalizeInline(%s)}", st.Target, st.Args[0]), nil
			case "MarkdownH3":
				return fmt.Sprintf("%s = []string{\"### \" + __octMarkdownNormalizeInline(%s)}", st.Target, st.Args[0]), nil
			case "MarkdownParagraph":
				return fmt.Sprintf("%s = []string{__octMarkdownNormalizeInline(%s)}", st.Target, st.Args[0]), nil
			case "MarkdownBlank":
				return fmt.Sprintf("%s = []string{\"\"}", st.Target), nil
			case "MarkdownHorizontalRule":
				return fmt.Sprintf("%s = []string{\"---\"}", st.Target), nil
			case "MarkdownBullets":
				return fmt.Sprintf("%s = __octMarkdownList(%s, false)", st.Target, st.Args[0]), nil
			case "MarkdownNumbered":
				return fmt.Sprintf("%s = __octMarkdownList(%s, true)", st.Target, st.Args[0]), nil
			case "MarkdownCodeBlock":
				return fmt.Sprintf("%s = __octMarkdownCodeBlock(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownCallout":
				return fmt.Sprintf("%s = __octMarkdownCallout(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownImage":
				return fmt.Sprintf("%s = __octMarkdownImage(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownFigure":
				return fmt.Sprintf("%s = __octMarkdownFigure(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownKeyValueTable":
				return fmt.Sprintf("%s = __octMarkdownKeyValueTable(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownReport":
				return fmt.Sprintf("%s = __octMarkdownFlattenBlocks(%s)", st.Target, st.Args[0]), nil
			case "MarkdownSection":
				return fmt.Sprintf("%s = __octMarkdownSection(%s, %s, false)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownSubsection":
				return fmt.Sprintf("%s = __octMarkdownSection(%s, %s, true)", st.Target, st.Args[0], st.Args[1]), nil
			case "MarkdownTable":
				return fmt.Sprintf("%s = __octMarkdownTable(%s)", st.Target, st.Args[0]), nil
			case "MarkdownTableWithColumns":
				return fmt.Sprintf("%s = __octMarkdownTableWithColumns(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "fft":
				return fmt.Sprintf("%s = __octFFT(%s)", st.Target, st.Args[0]), nil
			case "WriteOctagon":
				return fmt.Sprintf("__octWriteOctagon(%s, %s); %s = 0", st.Args[0], st.Args[1], st.Target), nil
			case "LoadOctagon":
				return fmt.Sprintf("%s = __octLoadOctagon_%s(%s)", st.Target, goSafeName(st.RetType), st.Args[0]), nil
			case "JsonNormalize", "JsonParse", "JsonStringify":
				return fmt.Sprintf("%s = __octJsonString(%q, %s)", st.Target, canonicalCompiledBuiltinName(st.Callee), st.Args[0]), nil
			case "JsonLoad":
				return fmt.Sprintf("%s = __octJsonString(%q, %s)", st.Target, "JsonLoad", st.Args[0]), nil
			case "JsonSave":
				return fmt.Sprintf("%s = __octJsonSave(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "CsvRead", "CsvReadRows":
				return fmt.Sprintf("%s = __octCsvReadRows(%s)", st.Target, st.Args[0]), nil
			case "CsvWrite", "CsvWriteRows":
				return fmt.Sprintf("%s = __octCsvWriteRows(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "CsvReadMatrix":
				return fmt.Sprintf("%s = __octCsvReadMatrix(%s)", st.Target, st.Args[0]), nil
			case "CsvReadTable":
				return fmt.Sprintf("%s = __octCsvReadTable(%s)", st.Target, st.Args[0]), nil
			case "FileReadText":
				return fmt.Sprintf("%s = __octFileReadText(%s)", st.Target, st.Args[0]), nil
			case "FileWriteText":
				return fmt.Sprintf("%s = __octFileWriteText(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "FileReadBytes":
				return fmt.Sprintf("%s = __octFileReadBytes(%s)", st.Target, st.Args[0]), nil
			case "FileWriteBytes":
				return fmt.Sprintf("%s = __octFileWriteBytes(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "FileReadLines":
				return fmt.Sprintf("%s = __octFileReadLines(%s)", st.Target, st.Args[0]), nil
			case "FileWriteLines":
				return fmt.Sprintf("%s = __octFileWriteLines(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "FileExists":
				return fmt.Sprintf("%s = func() bool { _, __err := os.Stat(%s); return __err == nil }()", st.Target, st.Args[0]), nil
			case "PathJoin":
				return fmt.Sprintf("%s = filepath.Join(%s...)", st.Target, st.Args[0]), nil
			case "PathBaseName":
				return fmt.Sprintf("%s = filepath.Base(%s)", st.Target, st.Args[0]), nil
			case "PathExtension":
				return fmt.Sprintf("%s = filepath.Ext(%s)", st.Target, st.Args[0]), nil
			case "PathStem":
				return fmt.Sprintf("%s = strings.TrimSuffix(filepath.Base(%s), filepath.Ext(filepath.Base(%s)))", st.Target, st.Args[0], st.Args[0]), nil
			case "PathParent":
				return fmt.Sprintf("%s = filepath.Dir(%s)", st.Target, st.Args[0]), nil
			case "PathClean":
				return fmt.Sprintf("%s = filepath.Clean(%s)", st.Target, st.Args[0]), nil
			case "FileDelete":
				return fmt.Sprintf("%s = __octFileDelete(%s)", st.Target, st.Args[0]), nil
			case "DirectoryList":
				return fmt.Sprintf("%s = __octDirectoryList(%s)", st.Target, st.Args[0]), nil
			case "DirectoryMake":
				return fmt.Sprintf("%s = __octDirectoryMake(%s)", st.Target, st.Args[0]), nil
			case "DirectoryMakeAll":
				return fmt.Sprintf("%s = __octDirectoryMakeAll(%s)", st.Target, st.Args[0]), nil
			case "DirectoryRemoveAll":
				return fmt.Sprintf("%s = __octDirectoryRemoveAll(%s)", st.Target, st.Args[0]), nil
			case "Step":
				return fmt.Sprintf("%s.__octStep(); %s = 0", st.Args[0], st.Target), nil
			case "Active":
				return fmt.Sprintf("%s = %s.__octActive()", st.Target, st.Args[0]), nil
			case "Result":
				return fmt.Sprintf("%s = func() %s { __value, __ok := %s.__octResult(); if !__ok { return %s{Err: \"Result() called before flow completion\", IsErr: true} }; return %s{Value: __value} }()",
					st.Target, goResultTypeName(st.RetType), st.Args[0], goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "Complete":
				return fmt.Sprintf("%s = %s.__octComplete()", st.Target, st.Args[0]), nil
			case "StateHistory":
				return fmt.Sprintf("%s = %s.__octStateHistory()", st.Target, st.Args[0]), nil
			case "ResumeTarget":
				return fmt.Sprintf("%s = %s.__octResumeTarget()", st.Target, st.Args[0]), nil
			case "BoardSnapshot":
				return fmt.Sprintf("%s = func() %s { __snap, __ok := %s.__octBoardSnapshot(); if !__ok { return %s{Err: \"BoardSnapshot() requires a flow with a declared board\", IsErr: true} }; __typed, __typedOk := __snap.(%s); if !__typedOk { return %s{Err: \"BoardSnapshot() flow snapshot type mismatch\", IsErr: true} }; return %s{Value: __typed} }()",
					st.Target, goResultTypeName(st.RetType), st.Args[0], goResultTypeName(st.RetType), goType(st.RetType), goResultTypeName(st.RetType), goResultTypeName(st.RetType)), nil
			case "MatMulMV":
				return fmt.Sprintf("%s = __octMatMulMV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MatMulVM":
				return fmt.Sprintf("%s = __octMatMulVM(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecDot":
				return fmt.Sprintf("%s = __octVecDot(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVV:+":
				return fmt.Sprintf("%s = __octVecAddVV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVV:-":
				return fmt.Sprintf("%s = __octVecSubVV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVV:*":
				return fmt.Sprintf("%s = __octVecMulVV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVV:/":
				return fmt.Sprintf("%s = __octVecDivVV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVS:+":
				return fmt.Sprintf("%s = __octVecAddVS(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVS:-":
				return fmt.Sprintf("%s = __octVecSubVS(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVS:*":
				return fmt.Sprintf("%s = __octVecMulVS(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinaryVS:/":
				return fmt.Sprintf("%s = __octVecDivVS(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinarySV:+":
				return fmt.Sprintf("%s = __octVecAddSV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinarySV:-":
				return fmt.Sprintf("%s = __octVecSubSV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinarySV:*":
				return fmt.Sprintf("%s = __octVecMulSV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "VecBinarySV:/":
				return fmt.Sprintf("%s = __octVecDivSV(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "MatMulMM":
				return fmt.Sprintf("%s = __octMatMulMM(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
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
				return fmt.Sprintf("%s = __octMatBinaryMM[%s, %s, %s](%s, %s, %q)", st.Target, goType(leftElem), goType(rightElem), goType(retElem), st.Args[0], st.Args[1], op), nil
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
				return fmt.Sprintf("%s = __octMatBinaryMS[%s, %s, %s](%s, %s, %q)", st.Target, goType(leftElem), goType(st.ArgTypes[1]), goType(retElem), st.Args[0], st.Args[1], op), nil
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
				return fmt.Sprintf("%s = __octMatBinarySM[%s, %s, %s](%s, %s, %q)", st.Target, goType(st.ArgTypes[0]), goType(rightElem), goType(retElem), st.Args[0], st.Args[1], op), nil
			case "PrometheusMatMulMM":
				return fmt.Sprintf("%s = __octPrometheusMatMulMM(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Trace":
				return fmt.Sprintf("%s = __octTrace(%s)", st.Target, st.Args[0]), nil
			case "Grad":
				if _, ok := parseMatrixElemType(st.RetType); ok {
					return fmt.Sprintf("%s = __octGrad(%s)", st.Target, st.Args[0]), nil
				}
				return fmt.Sprintf("%s = __octGradScalar(%s)", st.Target, st.Args[0]), nil
			case "Div":
				if _, ok := parseVectorElemType(st.RetType); ok {
					return fmt.Sprintf("%s = __octDiv(%s)", st.Target, st.Args[0]), nil
				}
				return fmt.Sprintf("%s = __octDivVector(%s)", st.Target, st.Args[0]), nil
			case "SymGrad":
				return fmt.Sprintf("%s = __octSymGrad(%s)", st.Target, st.Args[0]), nil
			case "Matrix.fill":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.fill return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __row := make([]%s, __cols); for __c := 0; __c < __cols; __c++ { __row[__c] = %s }; __m[__r] = __row }; return __m }()",
					st.Target, goElemType, st.Args[0], st.Args[1], goElemType, goElemType, st.Args[2]), nil
			case "Matrix.zeros":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.zeros return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __m[__r] = make([]%s, __cols) }; return __m }()",
					st.Target, goElemType, st.Args[0], st.Args[1], goElemType, goElemType), nil
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
					st.Target, goElemType, st.Args[0], goElemType, goElemType, one), nil
			case "Matrix.tabulate":
				elemType, ok := parseMatrixElemType(st.RetType)
				if !ok {
					return "", fmt.Errorf("invalid Matrix.tabulate return type %s", st.RetType)
				}
				goElemType := goType(elemType)
				return fmt.Sprintf("%s = func() [][]%s { __rows := int(%s); __cols := int(%s); __m := make([][]%s, __rows); for __r := 0; __r < __rows; __r++ { __row := make([]%s, __cols); for __c := 0; __c < __cols; __c++ { __row[__c] = %s(__r, __c) }; __m[__r] = __row }; return __m }()",
					st.Target, goElemType, st.Args[0], st.Args[1], goElemType, goElemType, st.Args[2]), nil
			case "Random.RngSeed":
				return fmt.Sprintf("%s = __octRandomRngSeed(%s)", st.Target, st.Args[0]), nil
			case "Random.RandInt":
				return fmt.Sprintf("%s = __octRandomRandInt(%s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2]), nil
			case "Random.RandFloat01":
				return fmt.Sprintf("%s = __octRandomRandFloat01(%s)", st.Target, st.Args[0]), nil
			case "Random.RandFloatRange":
				return fmt.Sprintf("%s = __octRandomRandFloatRange(%s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2]), nil
			case "Random.RandBernoulli":
				return fmt.Sprintf("%s = __octRandomRandBernoulli(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Random.RandNormal":
				return fmt.Sprintf("%s = __octRandomRandNormal(%s, %s, %s)", st.Target, st.Args[0], st.Args[1], st.Args[2]), nil
			case "Random.CryptoRandBytes":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandBytes(%s); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Bytes"), st.Args[0], goResultTypeName("Bytes"), goResultTypeName("Bytes")), nil
			case "Random.CryptoRandInt":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandInt(%s, %s); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Int"), st.Args[0], st.Args[1], goResultTypeName("Int"), goResultTypeName("Int")), nil
			case "Random.CryptoRandFloat01":
				return fmt.Sprintf("%s = func() %s { __v, __err := __octCryptoRandFloat01(); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
					st.Target, goResultTypeName("Float"), goResultTypeName("Float"), goResultTypeName("Float")), nil
			default:
				return "", fmt.Errorf("compiled mode does not yet support builtin %s", st.Callee)
			}
		}
		if st.FunctionValue {
			if st.Target == "_" && st.RetType == "Void" {
				return fmt.Sprintf("%s(%s)", st.Callee, strings.Join(st.Args, ", ")), nil
			}
			return fmt.Sprintf("%s = %s(%s)", st.Target, st.Callee, strings.Join(st.Args, ", ")), nil
		}
		if st.Target == "_" && st.RetType == "Void" {
			return fmt.Sprintf("fn_%s(%s)", strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(st.Args, ", ")), nil
		}
		return fmt.Sprintf("%s = fn_%s(%s)", st.Target, strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(st.Args, ", ")), nil
	case MIRDestructureCall:
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
		return fmt.Sprintf("%s = fn_%s(%s)", strings.Join(st.Targets, ", "), strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(st.Args, ", ")), nil
	case MIRBatchMap:
		workerName := "fn_" + strings.ReplaceAll(st.Worker, ".", "_")
		forwarderArgs := []string{"__item"}
		forwarderParams := []string{fmt.Sprintf("__item %s", goType(st.InputType))}
		for _, capture := range st.Captures {
			forwarderArgs = append(forwarderArgs, capture)
		}
		workerExpr := workerName
		if len(st.Captures) > 0 {
			workerExpr = fmt.Sprintf("func(%s) %s { return %s(%s) }", strings.Join(forwarderParams, ", "), goResultTypeName(st.ResultType), workerName, strings.Join(forwarderArgs, ", "))
		}
		return fmt.Sprintf("%s = func() %s { __vals, __err, __isErr := __octBatchRun(%s, %s, func(r %s) bool { return r.IsErr }, func(r %s) string { return r.Err }, func(r %s) %s { return r.Value }); if __isErr { return %s{Err: __err, IsErr: true} }; return %s{Value: __vals} }()",
			st.Target,
			goType(fallibleType(st.ResultType+"[]")),
			st.Input,
			workerExpr,
			goResultTypeName(st.ResultType),
			goResultTypeName(st.ResultType),
			goResultTypeName(st.ResultType),
			goType(st.ResultType),
			goResultTypeName(st.ResultType+"[]"),
			goResultTypeName(st.ResultType+"[]")), nil
	default:
		return "", fmt.Errorf("unsupported MIR stmt %T", s)
	}
}

func goTerminator(t MIRTerminator, labels map[string]int) (string, error) {
	switch term := t.(type) {
	case MIRReturn:
		if term.Value == "" {
			return "return", nil
		}
		return "return " + goReturnExpr(term.Value), nil
	case MIRJump:
		return fmt.Sprintf("pc = %d; continue", labels[term.Target]), nil
	case MIRBranch:
		return fmt.Sprintf("if %s { pc = %d } else { pc = %d }; continue", term.Cond, labels[term.TrueTarget], labels[term.FalseTarget]), nil
	case MIRFail:
		return "panic(" + term.Value + ")", nil
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

func CompileForTestWithSelectedFiles(path string, selectedFiles []string) (Result, error) {
	program, err := project.LoadForTestWithSelectedFiles(path, selectedFiles)
	if err != nil {
		return Result{}, err
	}
	return compileProgram(program, compileOptions{selectedReachableOnly: true})
}

const __octOctxiliaryHelpers = `
var __octOctxiliaryOnce sync.Once
var __octOctxiliaryCmd *exec.Cmd
var __octOctxiliaryIn io.WriteCloser
var __octOctxiliaryOut io.ReadCloser
var __octOctxiliaryErr error
var __octOctxiliaryMu sync.Mutex
var __octOctxiliaryReqID int
type __octOctxiliaryClient struct { cmd *exec.Cmd; in io.WriteCloser; out io.ReadCloser; mu sync.Mutex; reqID int; err error; closed bool }
var __octOctxiliaryGenericMu sync.Mutex
var __octOctxiliaryGenericClients = map[string]*__octOctxiliaryClient{}

func __octOctxiliaryWaitOrKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil { return }
	waitDone := make(chan error, 1)
	go func(){ waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}
}

func __octOctxiliaryCloseClient(client *__octOctxiliaryClient) {
	if client == nil { return }
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed { return }
	client.closed = true
	if client.in != nil { _ = client.in.Close(); client.in = nil }
	__octOctxiliaryWaitOrKill(client.cmd)
	client.cmd = nil
	if client.out != nil { _ = client.out.Close(); client.out = nil }
}

func __octOctxiliaryClose() {
	__octOctxiliaryMu.Lock()
	cmd, in, out := __octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut
	__octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut = nil, nil, nil
	__octOctxiliaryMu.Unlock()
	if in != nil { _ = in.Close() }
	__octOctxiliaryWaitOrKill(cmd)
	if out != nil { _ = out.Close() }
	__octOctxiliaryGenericMu.Lock()
	clients := make([]*__octOctxiliaryClient, 0, len(__octOctxiliaryGenericClients))
	for _, client := range __octOctxiliaryGenericClients { clients = append(clients, client) }
	__octOctxiliaryGenericClients = map[string]*__octOctxiliaryClient{}
	__octOctxiliaryGenericMu.Unlock()
	for _, client := range clients { __octOctxiliaryCloseClient(client) }
}

func __octGenericFallible(sidecarCommand string, family string, function string, args []octxiliary.Value, expected octxiliary.ValueKind) (octxiliary.Value, error) {
	return __octOctxiliaryGenericCall(sidecarCommand, family, function, args, expected)
}

func __octJsonString(function string, input string) octResult_String {
	value, err := __octGenericFallible("octxiliary-json", "Json", function, []octxiliary.Value{{Kind: octxiliary.ValueString, String: input}}, octxiliary.ValueString)
	if err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	return octResult_String{Value: value.String}
}

func __octJsonSave(path string, input string) octResult_Int {
	value, err := __octGenericFallible("octxiliary-json", "Json", "JsonSave", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}, {Kind: octxiliary.ValueString, String: input}}, octxiliary.ValueInt)
	if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	return octResult_Int{Value: value.Int}
}

func __octCsvReadRows(path string) octResult_StringSliceSlice {
	value, err := __octGenericFallible("octxiliary-csv", "Csv", "CsvReadRows", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}}, octxiliary.ValueStringMatrix)
	if err != nil { return octResult_StringSliceSlice{Err: err.Error(), IsErr: true} }
	return octResult_StringSliceSlice{Value: value.Strings2}
}

func __octCsvWriteRows(path string, rows [][]string) octResult_Int {
	value, err := __octGenericFallible("octxiliary-csv", "Csv", "CsvWriteRows", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}, {Kind: octxiliary.ValueStringMatrix, Strings2: rows}}, octxiliary.ValueInt)
	if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	return octResult_Int{Value: value.Int}
}

func __octCsvReadMatrix(path string) octResult_FloatSliceSlice {
	rows := __octCsvReadRows(path)
	if rows.IsErr { return octResult_FloatSliceSlice{Err: rows.Err, IsErr: true} }
	if len(rows.Value) == 0 { return octResult_FloatSliceSlice{Err: "InvalidData: csv matrix requires at least one row", IsErr: true} }
	matrix := make([][]float64, 0, len(rows.Value))
	for rowIndex, row := range rows.Value {
		floatRow := make([]float64, 0, len(row))
		for colIndex, cell := range row {
			parsed, err := strconv.ParseFloat(cell, 64)
			if err != nil { return octResult_FloatSliceSlice{Err: fmt.Sprintf("InvalidData: non-numeric cell at row %d column %d: %q", rowIndex+1, colIndex+1, cell), IsErr: true} }
			floatRow = append(floatRow, parsed)
		}
		matrix = append(matrix, floatRow)
	}
	return octResult_FloatSliceSlice{Value: matrix}
}

func __octCsvReadTable(path string) octResult_Csv_Table {
	rows := __octCsvReadRows(path)
	if rows.IsErr { return octResult_Csv_Table{Err: rows.Err, IsErr: true} }
	if len(rows.Value) == 0 { return octResult_Csv_Table{Err: "InvalidData: csv table requires at least one header row", IsErr: true} }
	headerRow := rows.Value[0]
	if len(headerRow) == 0 { return octResult_Csv_Table{Err: "InvalidData: csv table header row cannot be empty", IsErr: true} }
	seen := map[string]int{}
	for idx, header := range headerRow {
		if header == "" { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: csv table header %d is empty", idx+1), IsErr: true} }
		if prior, ok := seen[header]; ok { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: duplicate csv table header %q at columns %d and %d", header, prior+1, idx+1), IsErr: true} }
		seen[header] = idx
	}
	for rowIndex := 1; rowIndex < len(rows.Value); rowIndex++ {
		if len(rows.Value[rowIndex]) != len(headerRow) { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: inconsistent column count at row %d", rowIndex+1), IsErr: true} }
	}
	return octResult_Csv_Table{Value: Csv_Table{}}
}

func __octFileReadText(path string) octResult_String {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadText", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_String{Err: resp.Error, IsErr: true} }
	return octResult_String{Value: resp.Text}
}

func __octFileWriteText(path string, text string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteText", Path: path, Text: text}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileReadBytes(path string) octResult_Bytes {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadBytes", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Bytes{Err: resp.Error, IsErr: true} }
	return octResult_Bytes{Value: resp.Bytes}
}

func __octFileWriteBytes(path string, data []byte) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteBytes", Path: path, Bytes: data, HasBytes: true}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileReadLines(path string) octResult_StringSlice {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadLines", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_StringSlice{Err: resp.Error, IsErr: true} }
	return octResult_StringSlice{Value: resp.Lines}
}

func __octFileWriteLines(path string, lines []string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteLines", Path: path, Lines: lines, HasLines: true}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileDelete(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileDelete", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryList(path string) octResult_StringSlice {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryList", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_StringSlice{Err: resp.Error, IsErr: true} }
	return octResult_StringSlice{Value: resp.Lines}
}

func __octDirectoryMake(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryMake", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryMakeAll(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryMakeAll", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryRemoveAll(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryRemoveAll", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}


func __octOctxiliaryGenericCall(sidecarCommand string, family string, function string, args []octxiliary.Value, expected octxiliary.ValueKind) (octxiliary.Value, error) {
	client := __octOctxiliaryGenericClient(sidecarCommand)
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err != nil { return octxiliary.Value{}, client.err }
	client.reqID++
	req := octxiliary.Request{ID: client.reqID, Family: family, Function: function, Args: args, HasArgs: true}
	if err := octxiliary.ValidateRequest(req); err != nil { return octxiliary.Value{}, err }
	if err := octxiliary.WriteFrame(client.in, octxiliary.EncodeRequest(req)); err != nil { return octxiliary.Value{}, err }
	frame, err := octxiliary.ReadFrame(client.out); if err != nil { return octxiliary.Value{}, err }
	resp, err := octxiliary.ParseResponse(frame); if err != nil { return octxiliary.Value{}, err }
	if err := octxiliary.ValidateResponse(resp); err != nil { return octxiliary.Value{}, err }
	if !resp.OK { return octxiliary.Value{}, errors.New(resp.Error) }
	if !resp.HasValue { return octxiliary.Value{}, errors.New("Octxiliary generic response missing typed value") }
	if resp.Value.Kind != expected { return octxiliary.Value{}, fmt.Errorf("Octxiliary generic response kind mismatch: expected %s, got %s", expected, resp.Value.Kind) }
	return resp.Value, nil
}

func __octOctxiliaryValidateHandle(value octxiliary.Value, family string, handleType string) error {
	if value.Kind != octxiliary.ValueHandle { return fmt.Errorf("Octxiliary handle response kind mismatch: expected Handle, got %s", value.Kind) }
	if value.HandleFamily != family { return fmt.Errorf("Octxiliary handle response family mismatch: expected %s, got %s", family, value.HandleFamily) }
	if value.HandleType != handleType { return fmt.Errorf("Octxiliary handle response type mismatch: expected %s, got %s", handleType, value.HandleType) }
	if value.HandleID <= 0 { return fmt.Errorf("Octxiliary handle response ID must be positive") }
	return nil
}

func __octOctxiliaryGenericClient(sidecarCommand string) *__octOctxiliaryClient {
	__octOctxiliaryGenericMu.Lock()
	defer __octOctxiliaryGenericMu.Unlock()
	if client, ok := __octOctxiliaryGenericClients[sidecarCommand]; ok { return client }
	client := &__octOctxiliaryClient{}
	path, err := __octOctxiliarySidecarPath(sidecarCommand)
	if err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	cmd := exec.Command(path)
	in, _ := cmd.StdinPipe(); out, _ := cmd.StdoutPipe(); if err := cmd.Start(); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	client.cmd, client.in, client.out = cmd, in, out
	if err := octxiliary.WriteHandshake(in); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	if err := octxiliary.ReadHandshake(out); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	__octOctxiliaryGenericClients[sidecarCommand] = client
	return client
}

func __octOctxiliarySidecarPath(sidecarCommand string) (string, error) {
	if sidecarCommand == "" { return "", errors.New("Octxiliary sidecar command is empty") }
	if path, ok := __octOctxiliaryResolveSidecarInDir(filepath.Dir(os.Args[0]), sidecarCommand); ok { return path, nil }
	wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
	if wrapperPath != "" {
		if path, ok := __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath, sidecarCommand); ok { return path, nil }
	}
	return "", fmt.Errorf("Octxiliary sidecar %q not found; set OCT_WRAPPER_PATH or place it beside .octbin", sidecarCommand)
}

func __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath string, sidecarCommand string) (string, bool) {
	info, err := os.Stat(wrapperPath)
	if err != nil { return "", false }
	if info.IsDir() { return __octOctxiliaryResolveSidecarInDir(wrapperPath, sidecarCommand) }
	if __octOctxiliarySidecarBasenameMatches(filepath.Base(wrapperPath), sidecarCommand) { return wrapperPath, true }
	return "", false
}

func __octOctxiliaryResolveSidecarInDir(dir string, sidecarCommand string) (string, bool) {
	for _, name := range __octOctxiliarySidecarCommandCandidates(sidecarCommand) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() { return candidate, true }
	}
	return "", false
}

func __octOctxiliarySidecarCommandCandidates(sidecarCommand string) []string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(sidecarCommand), ".exe") { return []string{sidecarCommand, sidecarCommand + ".exe"} }
	return []string{sidecarCommand}
}

func __octOctxiliarySidecarBasenameMatches(base string, sidecarCommand string) bool {
	for _, candidate := range __octOctxiliarySidecarCommandCandidates(sidecarCommand) {
		if base == candidate { return true }
	}
	return false
}

func __octOctxiliaryEnsure() error {
	__octOctxiliaryOnce.Do(func(){
		path, ok := __octOctxiliaryResolveSidecarInDir(filepath.Dir(os.Args[0]), "octxiliary-io")
		if !ok {
			wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
			if wrapperPath != "" { path, ok = __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath, "octxiliary-io") }
		}
		if !ok { __octOctxiliaryErr = errors.New("Octxiliary sidecar not found; set OCT_WRAPPER_PATH or place octxiliary-io beside .octbin") ; return }
		cmd := exec.Command(path)
		in, _ := cmd.StdinPipe(); out, _ := cmd.StdoutPipe(); if err := cmd.Start(); err != nil { __octOctxiliaryErr = err; return }
		__octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut = cmd, in, out
		if err := octxiliary.WriteHandshake(in); err != nil { __octOctxiliaryErr = err; return }
		if err := octxiliary.ReadHandshake(out); err != nil { __octOctxiliaryErr = err; return }
	})
	return __octOctxiliaryErr
}
`
