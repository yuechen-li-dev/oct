package build

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
	"github.com/yuechen-li-dev/oct/internal/project"
	"sort"
	"strings"
)

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
	userID            int
	batchID           int
	anonymousID       int
	batchDepth        int
	retType           string
	fn                ast.FunctionDecl
	extra             []MIRFunction
	lastRet           string
	usesUtilityWhen   bool
	inPrometheus      bool
	einTerms          map[string]einsteinTermMeta
}

func (c *lowerCtx) eraseRefinementType(t string) string {
	for pkgName, pkg := range c.program.Packages {
		for _, conceptDecl := range pkg.Concepts {
			if len(conceptDecl.Requirements) == 0 {
				continue
			}
			qualified := pkgName + "." + conceptDecl.Name
			if t == qualified || (pkgName == c.pkg.Name && t == conceptDecl.Name) {
				return typeRefStringForPackage("", conceptDecl.Target)
			}
		}
	}
	return t
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
		concepts := append([]ast.ConceptDecl(nil), pkg.Concepts...)
		sort.SliceStable(concepts, func(i, j int) bool { return concepts[i].Name < concepts[j].Name })
		records := append([]ast.RecordDecl(nil), pkg.Records...)
		sort.SliceStable(records, func(i, j int) bool { return records[i].Name < records[j].Name })
		enums := append([]ast.EnumDecl(nil), pkg.Enums...)
		sort.SliceStable(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
		flows := append([]ast.FlowDecl(nil), pkg.Flows...)
		sort.SliceStable(flows, func(i, j int) bool { return flows[i].Name < flows[j].Name })
		functions := append([]ast.FunctionDecl(nil), pkg.Functions...)
		sort.SliceStable(functions, func(i, j int) bool { return functions[i].Name < functions[j].Name })
		for _, conceptDecl := range concepts {
			if len(conceptDecl.Requirements) == 0 {
				continue
			}
			module.Refinements = append(module.Refinements, MIRRefinement{Package: pkgName, Name: conceptDecl.Name, Base: typeRefStringForPackage("", conceptDecl.Target)})
		}
		if pkgName == program.Entry {
			for _, fn := range functions {
				if fn.Name == "main" {
					module.EntryFunc = "main"
					break
				}
			}
			if module.EntryFunc == "" {
				for _, fn := range functions {
					if fn.Name == "Main" {
						module.EntryFunc = "Main"
						break
					}
				}
			}
		}
		for _, r := range records {
			if r.TemplateOrigin != nil {
				module.Templates = append(module.Templates, mirTemplateSpecialization("record", pkgName, r.Name, r.TemplateOrigin, pkgName))
			}
			mr := MIRRecord{Package: pkgName, Name: r.Name, Kind: MIRRecordOrdinary}
			if r.IsTable {
				identity := pkgName + "." + r.Name
				mr.Kind = MIRRecordTableKind
				mr.Subject = layoutcontract.DataSubjectRef{Kind: layoutcontract.MIRRecordTable, Identity: identity}
				module.LayoutContracts = append(module.LayoutContracts, layoutcontract.Contract{
					Subject:    mr.Subject,
					Invariants: layoutcontract.Invariants{NominalIdentity: &layoutcontract.NominalIdentity{Provenance: layoutcontract.Provenance{Phase: layoutcontract.MIRLowering, Source: identity}}},
				})
			}
			for _, f := range r.Fields {
				fieldType := typeRefStringForPackage(pkgName, f.Type)
				if r.IsTable {
					fieldType += "[]"
				}
				mr.Fields = append(mr.Fields, MIRField{Name: f.Name, Type: fieldType})
			}
			module.Records = append(module.Records, mr)
			if r.IsTable {
				row := MIRRecord{Package: pkgName, Name: "__oct_table_row_" + r.Name, Kind: MIRRecordTableRow, Subject: layoutcontract.DataSubjectRef{Kind: layoutcontract.MIRTableRow, Identity: pkgName + "." + r.Name}}
				for _, f := range r.Fields {
					row.Fields = append(row.Fields, MIRField{Name: f.Name, Type: typeRefStringForPackage(pkgName, f.Type)})
				}
				module.Records = append(module.Records, row)
			}
		}
		for _, e := range enums {
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
		for _, flow := range flows {
			if flow.TemplateOrigin != nil {
				module.Templates = append(module.Templates, mirTemplateSpecialization("flow/query", pkgName, flow.Name, flow.TemplateOrigin, pkgName))
			}
			if len(flow.Board) > 0 {
				snapshot := MIRRecord{Package: pkgName, Name: flow.Name + "BoardSnapshot", Kind: MIRRecordOrdinary}
				for _, field := range flow.Board {
					snapshot.Fields = append(snapshot.Fields, MIRField{
						Name: field.Name,
						Type: typeRefStringForPackage(pkgName, field.Type),
					})
				}
				module.Records = append(module.Records, snapshot)
			}
			mirFlow, expressionFunctions, err := lowerFlow(program, pkgName, flow, pkg)
			if err != nil {
				return MIRModule{}, fmt.Errorf("flow %s.%s: %w", pkgName, flow.Name, err)
			}
			module.Flows = append(module.Flows, mirFlow)
			module.Functions = append(module.Functions, expressionFunctions...)
			if options.selectedReachableOnly {
				for _, call := range collectFlowUserCalls(mirFlow) {
					targetPkg := pkgName
					targetFn := call
					if dot := strings.Index(call, "."); dot >= 0 {
						targetPkg, targetFn = call[:dot], call[dot+1:]
					}
					enqueueReachable(targetPkg, targetFn)
				}
			}
		}
		for _, fn := range functions {
			if fn.TemplateOrigin != nil {
				module.Templates = append(module.Templates, mirTemplateSpecialization("function", pkgName, fn.Name, fn.TemplateOrigin, pkgName))
			}
			if fn.SelectorOwner != nil {
				ownerPkg := fn.SelectorOwner.Package
				if ownerPkg == "" {
					ownerPkg = pkgName
				}
				ownerName := fn.SelectorOwner.Name
				if ownerPkgDecl, ok := program.Packages[ownerPkg]; ok {
					for _, ownerRecord := range ownerPkgDecl.Records {
						if ownerRecord.Name != ownerName {
							continue
						}
						for ordinal, field := range ownerRecord.Fields {
							if field.Name != fn.SelectorField {
								continue
							}
							subject := layoutcontract.DataSubjectRef{Kind: layoutcontract.NominalRecord, Identity: ownerPkg + "." + ownerName}
							module.Selectors = append(module.Selectors, MIRSelector{
								Package: pkgName,
								Name:    fn.Name,
								Owner:   subject.Identity,
								Result:  typeRefStringForPackage(pkgName, fn.ReturnType),
								Field:   layoutcontract.FieldRef{Subject: subject, Ordinal: ordinal, Name: field.Name},
							})
						}
					}
				}
			}
			if fn.IsGoImport {
				continue
			}
			if options.selectedReachableOnly && !isReachableFunction(reachable, pkgName, fn.Name) && !fn.IsRefinementConstructor {
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
				if fn.IsGoImport {
					continue
				}
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
	if module.EntryFunc == "" && !options.allowNoEntry {
		return MIRModule{}, fmt.Errorf("entry package '%s' is missing main/Main function", program.Entry)
	}
	if module.EntryFunc != "" {
		module.EntryReturn, module.EntryFallible = lookupEntryFunctionShape(program, module.EntryPackage, module.EntryFunc)
	}
	return module, nil
}

func mirTemplateSpecialization(kind, pkgName, concreteName string, origin *ast.TemplateOrigin, consumerPkg string) MIRTemplateSpecialization {
	args := make([]string, len(origin.TypeArguments))
	for i := range origin.TypeArguments {
		args[i] = typeRefStringForPackage(consumerPkg, origin.TypeArguments[i])
	}
	return MIRTemplateSpecialization{
		Kind:          kind,
		Package:       pkgName,
		ConcreteName:  concreteName,
		OriginPackage: origin.Package,
		OriginName:    origin.Declaration,
		TypeArguments: args,
	}
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
			calls = append(calls, collectExprCallsWithLocals(s.DescendStep, functionValueLocals)...)
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
	case ast.FunctionExpr:
		for _, capture := range e.Captures {
			calls = append(calls, collectExprCallsWithLocals(capture.Value, functionValueLocals)...)
		}
		bodyLocals := make(map[string]struct{}, len(functionValueLocals)+len(e.Parameters)+len(e.Captures))
		for name := range functionValueLocals {
			bodyLocals[name] = struct{}{}
		}
		for _, parameter := range e.Parameters {
			bodyLocals[parameter.Name] = struct{}{}
		}
		for _, capture := range e.Captures {
			bodyLocals[capture.Name] = struct{}{}
		}
		calls = append(calls, collectFunctionCallsWithLocals(e.Body, bodyLocals)...)
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
		if e.Start != nil {
			calls = append(calls, collectExprCallsWithLocals(e.Start, functionValueLocals)...)
		}
		if e.End != nil {
			calls = append(calls, collectExprCallsWithLocals(e.End, functionValueLocals)...)
		}
		if e.Step != nil {
			calls = append(calls, collectExprCallsWithLocals(e.Step, functionValueLocals)...)
		}
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
	case ast.RecordLiteralExpr:
		for _, field := range e.Fields {
			calls = append(calls, collectExprCallsWithLocals(field.Value, functionValueLocals)...)
		}
	case ast.RecordUpdateExpr:
		calls = append(calls, collectExprCallsWithLocals(e.Source, functionValueLocals)...)
		for _, field := range e.Fields {
			calls = append(calls, collectExprCallsWithLocals(field.Value, functionValueLocals)...)
		}
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
		goName := ctx.sourceName(p.Name)
		mirFn.Params = append(mirFn.Params, MIRField{Name: goName, SourceName: p.Name, Type: t})
		ctx.locals[p.Name] = t
		ctx.goNames[p.Name] = goName
	}
	ctx.blocks = append(ctx.blocks, MIRBlock{Label: "entry"})
	ctx.cur = 0
	if err := ctx.lowerBlock(fn.Body); err != nil {
		return nil, fmt.Errorf("function %s.%s: %w", pkg.Name, fn.Name, err)
	}
	if ctx.blocks[ctx.cur].Terminator == nil {
		if mirFn.Return == "Void" {
			if mirFn.IsFallible {
				ctx.blocks[ctx.cur].Terminator = MIRReturn{Value: lowerMIRValue(fallibleOkValue(ctx.retType, ""), fallibleType(ctx.retType))}
			} else {
				ctx.blocks[ctx.cur].Terminator = MIRReturn{}
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
			if p.Name == ctx.goLocalName(n) {
				isParam = true
				break
			}
		}
		if !isParam {
			mirFn.Locals = append(mirFn.Locals, MIRField{Name: ctx.goLocalName(n), SourceName: n, Type: t})
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
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: lowerMIRValueWithClone(v, t)})
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
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: lowerMIRValueWithClone(v, t)})
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
			if !isSelfAppendAssign(s.Name, s.Value) {
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: lowerMIRValueWithClone(v, targetType)})
			} else {
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: c.goLocalName(s.Name), Value: lowerMIRValue(v, targetType)})
			}
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
				Args:     lowerMIRValues(args, nil),
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
			case isTwoDimensionalArrayType(targetType):
				switch len(indexExprs) {
				case 1:
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRRowAssign{Target: c.goLocalName(s.Target), Index: lowerMIRValue(indexExprs[0], "Int"), Value: lowerMIRValue(val, "")})
				case 2:
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRIndexAssign{Target: c.goLocalName(s.Target), Indices: lowerMIRValues(indexExprs, []string{"Int", "Int"}), Value: lowerMIRValue(val, "")})
				default:
					return fmt.Errorf("nested array index assignment requires one row index or two element indices, got %d", len(indexExprs))
				}
			case strings.HasSuffix(targetType, "[]") && !strings.HasSuffix(targetType, "[][]"):
				if len(indexExprs) != 1 {
					return fmt.Errorf("array index assignment requires exactly 1 index, got %d", len(indexExprs))
				}
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRIndexAssign{Target: c.goLocalName(s.Target), Indices: lowerMIRValues(indexExprs, []string{"Int"}), Value: lowerMIRValue(val, "")})
			case strings.HasSuffix(targetType, "[][]") || strings.HasPrefix(targetType, "[][]") || strings.HasPrefix(targetType, "Matrix<"):
				if len(indexExprs) != 2 {
					return fmt.Errorf("matrix index assignment requires exactly 2 indices, got %d", len(indexExprs))
				}
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRIndexAssign{Target: c.goLocalName(s.Target), Indices: lowerMIRValues(indexExprs, []string{"Int", "Int"}), Value: lowerMIRValue(val, "")})
			case strings.HasPrefix(targetType, "[]"):
				if len(indexExprs) != 1 {
					return fmt.Errorf("array index assignment requires exactly 1 index, got %d", len(indexExprs))
				}
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRIndexAssign{Target: c.goLocalName(s.Target), Indices: lowerMIRValues(indexExprs, []string{"Int"}), Value: lowerMIRValue(val, "")})
			default:
				return fmt.Errorf("index assignment requires array or matrix local, got %s", targetType)
			}
		case ast.ExprStmt:
			if call, ok := s.Value.(ast.CallExpr); ok {
				if callee, direct := flattenDirectCallName(call.Callee); direct && callee == "Require" {
					continue
				}
			}
			v, t, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			if t != "Void" {
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: lowerMIRValue(v, t)})
			}
		case ast.ReturnStmt:
			if s.Value == nil {
				if c.fn.IsFallible {
					c.blocks[c.cur].Terminator = MIRReturn{Value: lowerMIRValue(fallibleOkValue(c.retType, ""), fallibleType(c.retType))}
				} else {
					c.blocks[c.cur].Terminator = MIRReturn{}
				}
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
					c.blocks[c.cur].Terminator = MIRReturn{Value: lowerMIRValue(fallibleErrValue(c.retType, v), fallibleType(c.retType))}
				} else {
					c.blocks[c.cur].Terminator = MIRReturn{Value: lowerMIRValue(fallibleOkValue(c.retType, v), fallibleType(c.retType))}
				}
			} else {
				c.blocks[c.cur].Terminator = MIRReturn{Value: lowerMIRValue(v, t)}
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
		if strings.HasPrefix(name, "MatBinary") || strings.HasPrefix(name, "ArrayBinary") {
			return true
		}
	}
	return false
}

func canonicalCompiledBuiltinName(name string) string {
	name = builtin.CanonicalName(name)
	switch name {
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
	c.blocks[c.cur].Terminator = MIRBranch{Cond: lowerMIRValue(cond, "Bool"), TrueTarget: c.blocks[thenID].Label, FalseTarget: c.blocks[elseID].Label}

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
		Cond:        lowerMIRValue(cond, "Bool"),
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

func (c *lowerCtx) lowerRangeExpr(e ast.RangeExpr) (string, string, bool, error) {
	parts := []string{}
	if e.Start != nil {
		start, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(e.Start) })
		if err != nil {
			return "", "", false, err
		}
		parts = append(parts, "Start: "+start, "HasStart: true")
	}
	if e.End != nil {
		end, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(e.End) })
		if err != nil {
			return "", "", false, err
		}
		parts = append(parts, "End: "+end, "HasEnd: true")
	}
	if e.Step != nil {
		if e.Start == nil || e.End == nil {
			return "", "", false, fmt.Errorf("open-ended stepped ranges are not supported in M0")
		}
		step, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(e.Step) })
		if err != nil {
			return "", "", false, err
		}
		parts = append(parts, "Step: "+step, "HasStep: true")
	} else {
		parts = append(parts, "Step: 1")
	}
	return "__octRange{" + strings.Join(parts, ", ") + "}", "Range", false, nil
}

func (c *lowerCtx) lowerForStmt(s ast.ForStmt) error {
	rangeExpr, ok := s.Range.(ast.RangeExpr)
	if !ok {
		return unsupported("for range expression")
	}
	if rangeExpr.Start == nil || rangeExpr.End == nil {
		return fmt.Errorf("for loop range requires start and end")
	}
	start, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.Start) })
	if err != nil {
		return err
	}
	end, _, _, err := c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.End) })
	if err != nil {
		return err
	}
	if s.Direction == ast.ForDirectionDesc && rangeExpr.Step != nil {
		return fmt.Errorf("for loop cannot use both step and descend")
	}
	step := "1"
	hasExplicitStep := rangeExpr.Step != nil || s.DescendStep != nil
	if rangeExpr.Step != nil {
		step, _, _, err = c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(rangeExpr.Step) })
		if err != nil {
			return err
		}
	} else if s.DescendStep != nil {
		step, _, _, err = c.withExpectedType("Int", func() (string, string, bool, error) { return c.lowerExpr(s.DescendStep) })
		if err != nil {
			return err
		}
	}

	startLocal := c.temp("Int")
	endLocal := c.temp("Int")
	stepLocal := c.temp("Int")
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements,
		MIRAssign{Target: startLocal, Value: lowerMIRValue(start, "Int")},
		MIRAssign{Target: endLocal, Value: lowerMIRValue(end, "Int")},
		MIRAssign{Target: stepLocal, Value: lowerMIRValue(step, "Int")},
	)

	stepCheckID := -1
	if hasExplicitStep {
		stepCheckID = len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", stepCheckID)})
	}
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
	stepFailID := -1
	if hasExplicitStep {
		stepFailID = len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", stepFailID)})
	}
	rangeFailID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", rangeFailID)})

	if hasExplicitStep {
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[stepCheckID].Label}
		c.cur = stepCheckID
		c.blocks[c.cur].Terminator = MIRBranch{
			Cond:        MIRBinary{Op: ">", Left: mirLocal(stepLocal, "Int"), Right: mirInt("0"), Type: "Bool"},
			TrueTarget:  c.blocks[rangeCheckID].Label,
			FalseTarget: c.blocks[stepFailID].Label,
		}
	} else {
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[rangeCheckID].Label}
	}

	c.cur = rangeCheckID
	rangeCond := fmt.Sprintf("(%s <= %s)", startLocal, endLocal)
	if s.Direction == ast.ForDirectionDesc {
		rangeCond = fmt.Sprintf("(%s >= %s)", startLocal, endLocal)
	}
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        lowerMIRValue(rangeCond, "Bool"),
		TrueTarget:  c.blocks[condID].Label,
		FalseTarget: c.blocks[rangeFailID].Label,
	}

	previousType, hadPrevious := c.locals[s.Name]
	previousGoName := c.goLocalName(s.Name)
	loopGoName := c.sourceName(s.Name)
	bindLoopName := s.Name != "_"
	if !bindLoopName || hadPrevious {
		loopGoName = c.temp("Int")
	}
	if bindLoopName {
		c.locals[s.Name] = "Int"
		c.goNames[s.Name] = loopGoName
	}

	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: loopGoName, Value: mirLocal(startLocal, "Int")})
	c.cur = condID
	loopCond := fmt.Sprintf("(%s < %s)", loopGoName, endLocal)
	if s.Direction == ast.ForDirectionDesc {
		loopCond = fmt.Sprintf("(%s > %s)", loopGoName, endLocal)
	}
	c.blocks[c.cur].Terminator = MIRBranch{
		Cond:        lowerMIRValue(loopCond, "Bool"),
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
	updateExpr := fmt.Sprintf("(%s + %s)", loopGoName, stepLocal)
	if s.Direction == ast.ForDirectionDesc {
		updateExpr = fmt.Sprintf("(%s - %s)", loopGoName, stepLocal)
	}
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: loopGoName, Value: lowerMIRValue(updateExpr, "Int")})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[condID].Label}

	if hasExplicitStep {
		c.cur = stepFailID
		stepFailMessage := "runtime error: range step must be positive, got "
		if s.Direction == ast.ForDirectionDesc {
			stepFailMessage = "runtime error: descending for loop requires positive descend step, got "
		}
		c.blocks[c.cur].Terminator = MIRFail{Value: MIRBinary{Op: "+", Left: mirString(stepFailMessage), Right: MIRIntrinsicValue{Kind: "stringify", Type: "String", Args: []MIRValue{mirLocal(stepLocal, "Int")}}, Type: "String"}}
	}

	c.cur = rangeFailID
	rangeFailMessage := "runtime error: range start must be less than or equal to end, got "
	if s.Direction == ast.ForDirectionDesc {
		rangeFailMessage = "runtime error: descending range start must be greater than or equal to end, got "
	}
	c.blocks[c.cur].Terminator = MIRFail{Value: MIRBinary{Op: "+", Left: MIRBinary{Op: "+", Left: MIRBinary{Op: "+", Left: mirString(rangeFailMessage), Right: MIRIntrinsicValue{Kind: "stringify", Type: "String", Args: []MIRValue{mirLocal(startLocal, "Int")}}, Type: "String"}, Right: mirString(".."), Type: "String"}, Right: MIRIntrinsicValue{Kind: "stringify", Type: "String", Args: []MIRValue{mirLocal(endLocal, "Int")}}, Type: "String"}}

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
	c.blocks[c.cur].Terminator = MIRBranch{Cond: MIRFieldAccess{Target: lowerMIRValue(subject, valType), Field: "IsErr", Type: "Bool"}, TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}

	c.cur = okID
	c.locals[s.OkName] = valType
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.OkName, Value: MIRFieldAccess{Target: lowerMIRValue(subject, valType), Field: "Value", Type: valType}})
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: mirLocal(s.OkName, valType)})
	if err := c.lowerBlock(s.OkBody); err != nil {
		return err
	}
	okFallsThrough := c.blocks[c.cur].Terminator == nil

	c.cur = errID
	c.locals[s.ErrName] = "Error"
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.ErrName, Value: MIRFieldAccess{Target: lowerMIRValue(subject, valType), Field: "Err", Type: "Error"}})
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: mirLocal(s.ErrName, "Error")})
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

	c.blocks[c.cur].Terminator = MIRBranch{Cond: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "IsErr", Type: "Bool"}, TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}
	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "Value", Type: valueType}})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = errID
	c.blocks[c.cur].Terminator = MIRReturn{Value: MIRResultValue{ResultType: c.retType, Error: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "Err", Type: "Error"}, IsError: true}}
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

	c.blocks[c.cur].Terminator = MIRBranch{Cond: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "IsErr", Type: "Bool"}, TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}
	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "Value", Type: valueType}})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = errID
	c.blocks[c.cur].Terminator = MIRFail{Value: MIRBinary{Op: "+", Left: mirString("unwrap failed: "), Right: MIRFieldAccess{Target: lowerMIRValue(inner, valueType), Field: "Err", Type: "Error"}, Type: "String"}}
	c.cur = mergeID
	return out, valueType, false, nil
}

func (c *lowerCtx) temp(t string) string {
	name := internalName(internalTemporary, c.tempID)
	c.tempID++
	c.locals[name] = t
	if c.goNames != nil {
		c.goNames[name] = name
	}
	return name
}

type internalSymbolKind string

const (
	internalProgramCounter internalSymbolKind = "pc"
	internalTemporary      internalSymbolKind = "tmp"
	internalBatchItem      internalSymbolKind = "batch_item"
	internalBatchCapture   internalSymbolKind = "batch_capture"
	internalBatchWorker    internalSymbolKind = "batch_worker"
	internalAnonymous      internalSymbolKind = "anonymous"
)

// internalName is the sole emitted namespace for compiler-owned locals. Source
// bindings are separately mangled by sourceName, so valid Oct spelling cannot
// collide with these Go implementation details.
func internalName(kind internalSymbolKind, id int) string {
	if id < 0 {
		return "__oct_internal_" + string(kind)
	}
	return fmt.Sprintf("__oct_internal_%s_%d", kind, id)
}

func (c *lowerCtx) sourceName(name string) string {
	if c.goNames != nil {
		if existing, ok := c.goNames[name]; ok {
			return existing
		}
	}
	generated := fmt.Sprintf("__oct_user_%d", c.userID)
	c.userID++
	return generated
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
	base := c.sourceName(name)
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
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(shortValue, "Bool")})

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
	c.blocks[c.cur].Terminator = MIRBranch{Cond: lowerMIRValue(left, "Bool"), TrueTarget: trueTarget, FalseTarget: falseTarget}

	c.cur = rightID
	right, _, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Right) })
	if err != nil {
		return "", "", false, err
	}
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(right, "Bool")})
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
