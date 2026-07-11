package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/consteval"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func Module(module ast.Module) (vdmir.Module, error) {
	return moduleWithTests(module, nil)
}

func moduleHasTensorAssign(module ast.Module) bool {
	var blockHas func(ast.Block) bool
	blockHas = func(block ast.Block) bool {
		for _, stmt := range block.Statements {
			switch s := stmt.(type) {
			case ast.TensorAssignStmt:
				return true
			case ast.IfStmt:
				if blockHas(s.ThenBody) || (s.ElseBody != nil && blockHas(*s.ElseBody)) {
					return true
				}
			case ast.GuardWhenStmt:
				for _, c := range s.Cases {
					if blockHas(c.Body) {
						return true
					}
				}
				if s.ElseBody != nil && blockHas(*s.ElseBody) {
					return true
				}
			case ast.ForStmt:
				if blockHas(s.Body) {
					return true
				}
			case ast.FlowStmt:
				for _, state := range s.States {
					if blockHas(state.Body) {
						return true
					}
				}
			}
		}
		return false
	}
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.FunctionDecl:
			if blockHas(d.Body) {
				return true
			}
		case ast.ShaderDecl:
			for _, method := range d.Methods {
				if blockHas(method.Body) {
					return true
				}
			}
		}
	}
	return false
}

func moduleWithTests(module ast.Module, testInputs map[string]validate.ValidatedTestInput) (vdmir.Module, error) {
	// Tensor metadata is produced from the validated source module before
	// specialization rewrites declaration names. Spans survive those rewrites
	// and are the one-way handoff key into lowering.
	validatedTensors := []validate.ValidatedTensorAssign(nil)
	if moduleHasTensorAssign(module) {
		validationModule := module
		if len(testInputs) != 0 {
			validationModule = stripTestFunctionAttributes(module)
		}
		validated, issues := validate.ValidatedTensorAssignments(validationModule)
		if len(issues) != 0 {
			return vdmir.Module{}, fmt.Errorf("SDSL-V M32b requires valid tensor metadata: %s", issues[0].Message)
		}
		validatedTensors = validated
	}
	specialized, err := specializeModule(module)
	if err != nil {
		return vdmir.Module{}, err
	}
	expanded, err := expandComptimeModule(specialized)
	if err != nil {
		return vdmir.Module{}, err
	}
	module = expanded
	l := lowering{
		provenance:       vdmir.ProvenanceFromFile(module.Source),
		types:            map[string]typeInfo{},
		functions:        map[string]functionInfo{},
		testInputs:       testInputs,
		tensorAssigns:    map[source.Span]validate.ValidatedTensorAssign{},
		tensorReductions: map[source.Span]validate.ValidatedTensorReduction{},
	}
	for _, tensor := range validatedTensors {
		l.tensorAssigns[tensor.Span] = tensor
		for _, reduction := range tensor.Reductions {
			l.tensorReductions[reduction.Span] = reduction
		}
	}
	l.seedBuiltins()
	l.collect(module)
	out := vdmir.Module{
		Provenance: l.provenance,
		Namespace:  module.Namespace,
	}
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			out.TypeAliases = append(out.TypeAliases, vdmir.TypeAlias{
				Provenance: l.provenance,
				Name:       d.Name,
				Target:     l.lowerTypeRef(d.Type),
			})
		case ast.RecordDecl:
			out.Records = append(out.Records, l.lowerRecord(d.Name, d.Fields))
		case ast.BoardDecl:
			out.Boards = append(out.Boards, l.lowerBoard(d.Name, d.Fields))
		case ast.StreamDecl:
			out.Streams = append(out.Streams, l.lowerStream(d.Name, d.Fields))
		case ast.EnumDecl:
			out.Enums = append(out.Enums, l.lowerEnum(d))
		case ast.FunctionDecl:
			fn, err := l.lowerFunction("", d, nil, nil)
			if err != nil {
				return vdmir.Module{}, err
			}
			out.Functions = append(out.Functions, fn)
		case ast.ShaderDecl:
			if d.Template != nil {
				continue
			}
			resources, bundleName, err := l.resolveShaderResources(d)
			if err != nil {
				return vdmir.Module{}, err
			}
			for i, resource := range resources {
				binding := resolveResourceBinding(resources, i)
				out.Resources = append(out.Resources, vdmir.Resource{
					Provenance:  l.provenance,
					BundleName:  bundleName,
					Name:        resource.Name,
					ElementType: l.lowerResourceElementType(resource.Type),
					Access:      lowerResourceAccess(resource.Access),
					Binding:     binding,
				})
			}
			for _, workgroup := range d.Workgroups {
				typ := l.lowerTypeRef(workgroup.Type)
				elem := vdmir.Type{}
				if typ.Element != nil {
					elem = *typ.Element
				}
				rows := 0
				cols := 0
				isTile := workgroup.Type.Name == "tile"
				length := 0
				if isTile {
					rows = mustConcreteInt(workgroup.Type.TileRows)
					cols = mustConcreteInt(workgroup.Type.TileCols)
					length = rows * cols
				} else {
					length = mustConcreteInt(workgroup.Type.ArraySize)
				}
				out.Workgroups = append(out.Workgroups, vdmir.WorkgroupMemoryDecl{
					Provenance:  l.provenance,
					ShaderName:  d.Name,
					Name:        workgroup.Name,
					Type:        typ,
					ElementType: elem,
					Length:      length,
					Rows:        rows,
					Cols:        cols,
					IsTile:      isTile,
				})
			}
			for _, method := range d.Methods {
				fn, err := l.lowerFunction(d.Name, method, resources, d.Workgroups)
				if err != nil {
					return vdmir.Module{}, err
				}
				out.Functions = append(out.Functions, fn)
				if method.Stage == "compute" {
					out.EntryPoints = append(out.EntryPoints, l.lowerComputeEntryPoint(d, method, resources))
				}
			}
		case ast.ConceptDecl, ast.ConfigDecl, ast.CompileDecl:
			continue
		case ast.UnsupportedDecl:
			if d.Kind == "flow" {
				return vdmir.Module{}, fmt.Errorf("top-level Octomata flow declarations are not supported in SDSL-V M22; use function-local flow blocks")
			}
			return vdmir.Module{}, fmt.Errorf("%s is not implemented in GoOct SDSL-V M0", d.Kind)
		}
	}
	out.ForeignTargets = collectForeignTargets(module)
	out.Flows = l.flows
	return out, nil
}

// ModuleForTarget is the target gate for present and future backends. HLSL is
// the only registered foreign target in M28; callers for another target get a
// clear rejection instead of accidental raw-source emission.
func ModuleForTarget(module ast.Module, target string) (vdmir.Module, error) {
	out, err := Module(module)
	if err != nil {
		return vdmir.Module{}, err
	}
	for _, foreign := range out.ForeignTargets {
		if foreign != target {
			return vdmir.Module{}, fmt.Errorf("this shader contains inline %s and cannot be lowered to target %s", foreign, target)
		}
	}
	return out, nil
}

// ModuleForTests uses the ordinary target lowering and then converts the
// validator-owned Assert call plan into dedicated VD-MIR operations.  Keeping
// this at the lowering boundary prevents the test emitter from inspecting AST
// calls or manifest metadata.
func ModuleForTests(module ast.Module, tests []validate.ValidatedTestDecl, target string) (vdmir.Module, error) {
	testInputs := make(map[string]validate.ValidatedTestInput, len(tests))
	for _, test := range tests {
		testInputs[test.Function.Name] = test.TestInput
	}
	out, err := moduleWithTests(module, testInputs)
	if err != nil {
		return vdmir.Module{}, err
	}
	plans := make(map[string][]validate.ValidatedAssertCall, len(tests))
	for _, test := range tests {
		plans[test.Function.Name] = test.AssertCalls
	}
	for _, foreign := range out.ForeignTargets {
		if foreign != target {
			return vdmir.Module{}, fmt.Errorf("this shader contains inline %s and cannot be lowered to target %s", foreign, target)
		}
	}
	if len(testInputs) != 0 {
		out.Resources = append(out.Resources, vdmir.Resource{
			Provenance:  out.Provenance,
			Name:        vdmir.TestInputResourceName,
			ElementType: vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"},
			Access:      vdmir.ResourceReadOnly,
			Binding:     vdmir.Binding{Set: 0, Binding: 1, Explicit: true},
		})
	}
	for i := range out.Functions {
		plan, ok := plans[out.Functions[i].Name]
		if !ok {
			continue
		}
		index := 0
		body, err := lowerTestAssertBlock(out.Functions[i].Body, plan, &index)
		if err != nil {
			return vdmir.Module{}, fmt.Errorf("SDSL-V test %s: %w", out.Functions[i].Name, err)
		}
		if index != len(plan) {
			return vdmir.Module{}, fmt.Errorf("SDSL-V2902 malformed canonical assertion projection: lowered %d of %d assertions", index, len(plan))
		}
		out.Functions[i].Body = body
	}
	return out, nil
}

func stripTestFunctionAttributes(module ast.Module) ast.Module {
	out := module
	out.Decls = make([]ast.Decl, 0, len(module.Decls))
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.FunctionDecl:
			d.Attributes = preservedTensorValidationAttrs(d.Attributes)
			d.Body = stripTestOnlyStatements(d.Body)
			out.Decls = append(out.Decls, d)
		case ast.ShaderDecl:
			d.Methods = append([]ast.FunctionDecl(nil), d.Methods...)
			for i := range d.Methods {
				d.Methods[i].Attributes = preservedTensorValidationAttrs(d.Methods[i].Attributes)
				d.Methods[i].Body = stripTestOnlyStatements(d.Methods[i].Body)
			}
			out.Decls = append(out.Decls, d)
		default:
			out.Decls = append(out.Decls, decl)
		}
	}
	return out
}

func preservedTensorValidationAttrs(attrs []ast.Attribute) []ast.Attribute {
	return append([]ast.Attribute(nil), attrs...)
}

func stripTestOnlyStatements(block ast.Block) ast.Block {
	out := block
	out.Statements = make([]ast.Stmt, 0, len(block.Statements))
	for _, stmt := range block.Statements {
		if isTestAssertStmt(stmt) {
			continue
		}
		switch s := stmt.(type) {
		case ast.IfStmt:
			s.ThenBody = stripTestOnlyStatements(s.ThenBody)
			if s.ElseBody != nil {
				body := stripTestOnlyStatements(*s.ElseBody)
				s.ElseBody = &body
			}
			out.Statements = append(out.Statements, s)
		case ast.ForStmt:
			s.Body = stripTestOnlyStatements(s.Body)
			out.Statements = append(out.Statements, s)
		case ast.GuardWhenStmt:
			s.Cases = append([]ast.GuardWhenCase(nil), s.Cases...)
			for i := range s.Cases {
				s.Cases[i].Body = stripTestOnlyStatements(s.Cases[i].Body)
			}
			if s.ElseBody != nil {
				body := stripTestOnlyStatements(*s.ElseBody)
				s.ElseBody = &body
			}
			out.Statements = append(out.Statements, s)
		case ast.FlowStmt:
			s.States = append([]ast.StateBlock(nil), s.States...)
			for i := range s.States {
				s.States[i].Body = stripTestOnlyStatements(s.States[i].Body)
			}
			out.Statements = append(out.Statements, s)
		default:
			out.Statements = append(out.Statements, stmt)
		}
	}
	return out
}

func isTestAssertStmt(stmt ast.Stmt) bool {
	expr, ok := stmt.(ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expr.Value.(ast.CallExpr)
	if !ok {
		return false
	}
	field, ok := call.Callee.(ast.FieldAccessExpr)
	if !ok {
		return false
	}
	target, ok := field.Target.(ast.IdentifierExpr)
	return ok && target.Name == "Assert"
}

func lowerTestAssertBlock(block vdmir.Block, plan []validate.ValidatedAssertCall, index *int) (vdmir.Block, error) {
	out := vdmir.Block{Statements: make([]vdmir.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case vdmir.ExprStmt:
			if call, ok := s.Value.(vdmir.CallExpr); ok && vdmirAssertCall(call) {
				if *index >= len(plan) {
					return vdmir.Block{}, fmt.Errorf("SDSL-V2902 assertion has no validated metadata")
				}
				meta := plan[*index]
				*index++
				op, err := makeVDMIRAssert(call, meta)
				if err != nil {
					return vdmir.Block{}, err
				}
				out.Statements = append(out.Statements, op)
				continue
			}
		case vdmir.IfStmt:
			thenBody, err := lowerTestAssertBlock(s.ThenBody, plan, index)
			if err != nil {
				return vdmir.Block{}, err
			}
			s.ThenBody = thenBody
			if s.ElseBody != nil {
				elseBody, err := lowerTestAssertBlock(*s.ElseBody, plan, index)
				if err != nil {
					return vdmir.Block{}, err
				}
				s.ElseBody = &elseBody
			}
			out.Statements = append(out.Statements, s)
			continue
		case vdmir.ForRangeStmt:
			body, err := lowerTestAssertBlock(s.Body, plan, index)
			if err != nil {
				return vdmir.Block{}, err
			}
			s.Body = body
			out.Statements = append(out.Statements, s)
			continue
		case vdmir.BlockStmt:
			body, err := lowerTestAssertBlock(s.Body, plan, index)
			if err != nil {
				return vdmir.Block{}, err
			}
			s.Body = body
			out.Statements = append(out.Statements, s)
			continue
		}
		out.Statements = append(out.Statements, stmt)
	}
	return out, nil
}

func vdmirAssertCall(call vdmir.CallExpr) bool {
	member, ok := call.Callee.(vdmir.FieldAccessExpr)
	if !ok {
		return false
	}
	root, ok := member.Target.(vdmir.VarRefExpr)
	return ok && root.Name == "Assert"
}

func makeVDMIRAssert(call vdmir.CallExpr, meta validate.ValidatedAssertCall) (vdmir.AssertStmt, error) {
	op := vdmir.AssertStmt{Provenance: call.Provenance, Kind: vdmir.AssertKind(meta.Kind), CallSpan: meta.CallSpan, OperandSpans: append([]source.Span(nil), meta.OperandSpans...), LexicalIndex: meta.LexicalIndex, ComponentCount: 1}
	if len(call.Arguments) == 0 {
		return op, fmt.Errorf("SDSL-V2902 assertion without operands")
	}
	switch op.Kind {
	case vdmir.AssertTrue, vdmir.AssertFalse:
		op.Actual = call.Arguments[0]
	case vdmir.AssertEqual, vdmir.AssertNotEqual:
		if len(call.Arguments) != 2 {
			return op, fmt.Errorf("SDSL-V2902 assertion arity mismatch")
		}
		op.Expected, op.Actual = call.Arguments[0], call.Arguments[1]
	case vdmir.AssertNear:
		if len(call.Arguments) != 3 {
			return op, fmt.Errorf("SDSL-V2902 assertion arity mismatch")
		}
		op.Expected, op.Actual, op.Tolerance = call.Arguments[0], call.Arguments[1], call.Arguments[2]
	default:
		return op, fmt.Errorf("SDSL-V2901 unsupported assertion kind %q", meta.Kind)
	}
	value := op.Actual.Type()
	switch value.Kind {
	case vdmir.TypeBool:
		op.ValueKind = vdmir.AssertValueBool
	case vdmir.TypeI32:
		op.ValueKind = vdmir.AssertValueInt
	case vdmir.TypeU32:
		op.ValueKind = vdmir.AssertValueUInt
	case vdmir.TypeF32:
		op.ValueKind = vdmir.AssertValueFloat
	default:
		return op, fmt.Errorf("SDSL-V2901 unsupported assertion operand type %s", value.Name)
	}
	return op, nil
}

func collectForeignTargets(module ast.Module) []string {
	seen := map[string]struct{}{}
	var walkExpr func(ast.Expr)
	var walkBlock func(ast.Block)
	add := func(target string) { seen[target] = struct{}{} }
	walkExpr = func(expr ast.Expr) {
		switch e := expr.(type) {
		case ast.ForeignShaderExpr:
			add(e.TargetLanguage)
		case ast.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case ast.UnaryExpr:
			walkExpr(e.Operand)
		case ast.ParenExpr:
			walkExpr(e.Inner)
		case ast.CallExpr:
			walkExpr(e.Callee)
			for _, a := range e.Arguments {
				walkExpr(a)
			}
		}
	}
	walkBlock = func(block ast.Block) {
		for _, stmt := range block.Statements {
			switch s := stmt.(type) {
			case ast.ForeignShaderStmt:
				add(s.TargetLanguage)
			case ast.LetStmt:
				walkExpr(s.Value)
			case ast.AssignStmt:
				walkExpr(s.Target)
				walkExpr(s.Value)
			case ast.ReturnStmt:
				walkExpr(s.Value)
			case ast.ExprStmt:
				walkExpr(s.Value)
			case ast.IfStmt:
				walkExpr(s.Condition)
				walkBlock(s.ThenBody)
				if s.ElseBody != nil {
					walkBlock(*s.ElseBody)
				}
			case ast.ForStmt:
				walkBlock(s.Body)
			}
		}
	}
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.FunctionDecl:
			walkBlock(d.Body)
		case ast.ShaderDecl:
			for _, method := range d.Methods {
				walkBlock(method.Body)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for target := range seen {
		out = append(out, target)
	}
	slices.Sort(out)
	return out
}

type specializeValue struct {
	typ     ast.TypeRef
	int32   int64
	boolVal bool
}

type conceptFieldSpec struct {
	Path         string
	Type         ast.TypeRef
	DefaultValue ast.Expr
	ZeroAllowed  bool
}

func specializeModule(module ast.Module) (ast.Module, error) {
	concepts := map[string]ast.ConceptDecl{}
	configDecls := map[string]ast.ConfigDecl{}
	templates := map[string]ast.ShaderDecl{}
	var compileDecls []ast.CompileDecl
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.ConceptDecl:
			concepts[d.Name] = d
		case ast.ConfigDecl:
			configDecls[d.Name] = d
		case ast.ShaderDecl:
			if d.Template != nil {
				templates[d.Name] = d
			}
		case ast.CompileDecl:
			compileDecls = append(compileDecls, d)
		}
	}
	out := module
	out.Decls = append([]ast.Decl(nil), module.Decls...)
	for _, decl := range compileDecls {
		template, ok := templates[decl.ShaderName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown template shader %s", decl.ShaderName)
		}
		configDecl, ok := configDecls[decl.ConfigName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown config %s", decl.ConfigName)
		}
		concept, ok := concepts[configDecl.ConceptName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown concept %s", configDecl.ConceptName)
		}
		config, err := expandConfig(concept, configDecl)
		if err != nil {
			return ast.Module{}, err
		}
		env := map[string]specializeValue{}
		for name, value := range config {
			env[template.Template.Name+"."+name] = value
		}
		instance, err := specializeShader(template, decl.AliasName, env)
		if err != nil {
			return ast.Module{}, err
		}
		out.Decls = append(out.Decls, instance)
	}
	return out, nil
}

func specializeShader(shader ast.ShaderDecl, alias string, env map[string]specializeValue) (ast.ShaderDecl, error) {
	out := shader
	out.Name = alias
	out.Template = nil
	out.SpecializedConfig = lowerSpecializedConfig(env)
	out.Workgroups = make([]ast.WorkgroupDecl, 0, len(shader.Workgroups))
	for _, workgroup := range shader.Workgroups {
		ref, err := specializeTypeRef(workgroup.Type, env)
		if err != nil {
			return ast.ShaderDecl{}, err
		}
		out.Workgroups = append(out.Workgroups, ast.WorkgroupDecl{Name: workgroup.Name, Type: ref})
	}
	out.Methods = make([]ast.FunctionDecl, 0, len(shader.Methods))
	for _, method := range shader.Methods {
		specialized, err := specializeFunction(method, env)
		if err != nil {
			return ast.ShaderDecl{}, err
		}
		out.Methods = append(out.Methods, specialized)
	}
	return out, nil
}

func lowerSpecializedConfig(env map[string]specializeValue) map[string]uint32 {
	if len(env) == 0 {
		return nil
	}
	out := map[string]uint32{}
	for key, value := range env {
		field := key
		if dot := strings.IndexByte(field, '.'); dot >= 0 && dot+1 < len(field) {
			field = field[dot+1:]
		}
		if value.int32 < 0 {
			continue
		}
		out[flattenConfigName(field)] = uint32(value.int32)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func specializeFunction(fn ast.FunctionDecl, env map[string]specializeValue) (ast.FunctionDecl, error) {
	out := fn
	if fn.NumThreads != nil {
		x, err := specializeIntExpr(fn.NumThreads.X, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		y, err := specializeIntExpr(fn.NumThreads.Y, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		z, err := specializeIntExpr(fn.NumThreads.Z, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		out.NumThreads = &ast.NumThreads{X: x, Y: y, Z: z}
	}
	body, err := specializeBlock(fn.Body, env)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	out.Body = body
	return out, nil
}

func specializeBlock(block ast.Block, env map[string]specializeValue) (ast.Block, error) {
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		next, err := specializeStmt(stmt, env)
		if err != nil {
			return ast.Block{}, err
		}
		out.Statements = append(out.Statements, next)
	}
	return out, nil
}

func specializeStmt(stmt ast.Stmt, env map[string]specializeValue) (ast.Stmt, error) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		ref, err := specializeLocalTypeRef(s.Type, env)
		if err != nil {
			return nil, err
		}
		var value ast.Expr
		if s.Value != nil {
			value = specializeExpr(s.Value, env)
		}
		return ast.LetStmt{Name: s.Name, Type: ref, Value: value}, nil
	case ast.ComptimeLetStmt:
		ref, err := specializeLocalTypeRef(s.Type, env)
		if err != nil {
			return nil, err
		}
		return ast.ComptimeLetStmt{Name: s.Name, Type: ref, Value: specializeExpr(s.Value, env)}, nil
	case ast.AssignStmt:
		return ast.AssignStmt{Target: specializeExpr(s.Target, env), Value: specializeExpr(s.Value, env)}, nil
	case ast.GuardedWriteStmt:
		return ast.GuardedWriteStmt{
			Target:    specializeExpr(s.Target, env),
			Value:     specializeExpr(s.Value, env),
			Condition: specializeExpr(s.Condition, env),
		}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return s, nil
		}
		return ast.ReturnStmt{Value: specializeExpr(s.Value, env)}, nil
	case ast.ExprStmt:
		return ast.ExprStmt{Value: specializeExpr(s.Value, env)}, nil
	case ast.IfStmt:
		thenBody, err := specializeBlock(s.ThenBody, env)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.IfStmt{Condition: specializeExpr(s.Condition, env), ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.GuardWhenStmt:
		cases := make([]ast.GuardWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			body, err := specializeBlock(c.Body, env)
			if err != nil {
				return nil, err
			}
			cases = append(cases, ast.GuardWhenCase{Condition: specializeExpr(c.Condition, env), Body: body})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.GuardWhenStmt{Cases: cases, ElseBody: elseBody}, nil
	case ast.FlowStmt:
		boards := make([]ast.FlowBoardDecl, 0, len(s.Boards))
		for _, board := range s.Boards {
			boards = append(boards, ast.FlowBoardDecl{
				Name:        board.Name,
				Type:        board.Type,
				Initializer: specializeExpr(board.Initializer, env),
			})
		}
		states := make([]ast.StateBlock, 0, len(s.States))
		for _, state := range s.States {
			body, err := specializeBlock(state.Body, env)
			if err != nil {
				return nil, err
			}
			states = append(states, ast.StateBlock{Span: state.Span, Name: state.Name, NameSpan: state.NameSpan, Body: body})
		}
		return ast.FlowStmt{Name: s.Name, Boards: boards, States: states}, nil
	case ast.ComptimeIfStmt:
		thenBody, err := specializeBlock(s.ThenBody, env)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.ComptimeIfStmt{Condition: specializeExpr(s.Condition, env), ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.ComptimeMatchStmt:
		arms := make([]ast.ComptimeMatchArm, 0, len(s.Arms))
		for _, arm := range s.Arms {
			body, err := specializeBlock(arm.Body, env)
			if err != nil {
				return nil, err
			}
			var pattern ast.Expr
			if arm.Pattern != nil {
				pattern = specializeExpr(arm.Pattern, env)
			}
			arms = append(arms, ast.ComptimeMatchArm{Pattern: pattern, IsElse: arm.IsElse, Body: body})
		}
		return ast.ComptimeMatchStmt{Subject: specializeExpr(s.Subject, env), Arms: arms}, nil
	case ast.ComptimeWhenUtilityStmt:
		cases := make([]ast.ComptimeWhenUtilityCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			body, err := specializeBlock(c.Body, env)
			if err != nil {
				return nil, err
			}
			var condition ast.Expr
			if c.Condition != nil {
				condition = specializeExpr(c.Condition, env)
			}
			cases = append(cases, ast.ComptimeWhenUtilityCase{
				Label:     c.Label,
				Condition: condition,
				Score:     specializeExpr(c.Score, env),
				Body:      body,
			})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.ComptimeWhenUtilityStmt{Cases: cases, ElseBody: elseBody}, nil
	case ast.ComptimeForStmt:
		body, err := specializeBlock(s.Body, env)
		if err != nil {
			return nil, err
		}
		return ast.ComptimeForStmt{Name: s.Name, Start: specializeExpr(s.Start, env), End: specializeExpr(s.End, env), Body: body}, nil
	case ast.ForStmt:
		body, err := specializeBlock(s.Body, env)
		if err != nil {
			return nil, err
		}
		return ast.ForStmt{Attributes: append([]ast.Attribute(nil), s.Attributes...), Name: s.Name, Start: specializeExpr(s.Start, env), End: specializeExpr(s.End, env), Step: specializeExpr(s.Step, env), Body: body}, nil
	case ast.StaticAssertStmt:
		return ast.StaticAssertStmt{Expr: specializeExpr(s.Expr, env), Text: s.Text}, nil
	default:
		return stmt, nil
	}
}

type comptimeBinding struct {
	typ     ast.TypeRef
	int32   int64
	boolVal bool
}

type runtimeOrigin string

const (
	runtimeParam     runtimeOrigin = "runtime parameter"
	runtimeResource  runtimeOrigin = "resource"
	runtimeWorkgroup runtimeOrigin = "workgroup value"
	runtimeBuiltin   runtimeOrigin = "thread builtin"
	runtimeLocal     runtimeOrigin = "runtime local"
)

type runtimeBinding struct {
	origin runtimeOrigin
}

const maxComptimeForExpandedStatements = 256

func expandComptimeModule(module ast.Module) (ast.Module, error) {
	resourceBundles := map[string][]ast.ResourceDecl{}
	for _, decl := range module.Decls {
		stream, ok := decl.(ast.StreamDecl)
		if !ok {
			continue
		}
		for _, field := range stream.Fields {
			if field.Access == "" {
				continue
			}
			resourceBundles[stream.Name] = append(resourceBundles[stream.Name], ast.ResourceDecl{Name: field.Name, Access: field.Access, Type: field.Type, Attributes: field.Attributes})
		}
	}
	out := module
	out.Decls = make([]ast.Decl, 0, len(module.Decls))
	for _, decl := range module.Decls {
		shader, ok := decl.(ast.ShaderDecl)
		if !ok || shader.Template != nil {
			out.Decls = append(out.Decls, decl)
			continue
		}
		expanded, err := expandComptimeShader(shader, resourceBundles)
		if err != nil {
			return ast.Module{}, err
		}
		out.Decls = append(out.Decls, expanded)
	}
	return out, nil
}

func expandComptimeShader(shader ast.ShaderDecl, resourceBundles map[string][]ast.ResourceDecl) (ast.ShaderDecl, error) {
	out := shader
	out.Methods = make([]ast.FunctionDecl, 0, len(shader.Methods))
	resources := shader.Resources
	if shader.ResourceBundleName != "" {
		resources = append([]ast.ResourceDecl(nil), resourceBundles[shader.ResourceBundleName]...)
	}
	for _, method := range shader.Methods {
		expanded, err := expandComptimeFunction(method, resources, shader.Workgroups)
		if err != nil {
			return ast.ShaderDecl{}, fmt.Errorf("shader %s.%s: %w", shader.Name, method.Name, err)
		}
		out.Methods = append(out.Methods, expanded)
	}
	return out, nil
}

func expandComptimeFunction(fn ast.FunctionDecl, resources []ast.ResourceDecl, workgroups []ast.WorkgroupDecl) (ast.FunctionDecl, error) {
	runtime := map[string]runtimeBinding{
		"DispatchThreadID": {origin: runtimeBuiltin},
		"GroupThreadID":    {origin: runtimeBuiltin},
		"GroupID":          {origin: runtimeBuiltin},
		"GroupIndex":       {origin: runtimeBuiltin},
	}
	for _, resource := range resources {
		runtime[resource.Name] = runtimeBinding{origin: runtimeResource}
	}
	for _, workgroup := range workgroups {
		runtime[workgroup.Name] = runtimeBinding{origin: runtimeWorkgroup}
	}
	for _, param := range fn.Parameters {
		runtime[param.Name] = runtimeBinding{origin: runtimeParam}
	}
	body, err := expandComptimeBlock(fn.Body, nil, runtime)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	out := fn
	out.Body = body
	return out, nil
}

func expandComptimeBlock(block ast.Block, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	ct := cloneComptimeBindings(comptime)
	rt := cloneRuntimeBindings(runtime)
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		expanded, err := expandComptimeStmt(stmt, ct, rt)
		if err != nil {
			return ast.Block{}, err
		}
		out.Statements = append(out.Statements, expanded...)
	}
	return out, nil
}

func expandComptimeStmt(stmt ast.Stmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) ([]ast.Stmt, error) {
	switch s := stmt.(type) {
	case ast.ComptimeLetStmt:
		value, err := evalComptimeExpr(s.Value, comptime, runtime, "comptime let initializer must be compile-time")
		if err != nil {
			return nil, err
		}
		comptime[s.Name] = comptimeBinding{typ: value.Type, int32: value.Int32, boolVal: value.Bool}
		return nil, nil
	case ast.ComptimeIfStmt:
		value, err := evalComptimeExpr(s.Condition, comptime, runtime, "comptime if condition must be compile-time bool")
		if err != nil {
			return nil, err
		}
		if value.Type.Name != "bool" {
			return nil, fmt.Errorf("comptime if condition must be compile-time bool")
		}
		selected := s.ElseBody
		if value.Bool {
			selected = &s.ThenBody
		}
		if selected == nil {
			return nil, nil
		}
		body, err := expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.ComptimeMatchStmt:
		body, err := expandComptimeMatch(s, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.ComptimeWhenUtilityStmt:
		body, err := expandComptimeWhenUtility(s, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.ComptimeForStmt:
		body, err := expandComptimeFor(s, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.LetStmt:
		out := s
		out.Type = replaceComptimeTypeRef(s.Type, comptime)
		if s.Value != nil {
			out.Value = replaceComptimeExpr(s.Value, comptime)
		}
		runtime[s.Name] = runtimeBinding{origin: runtimeLocal}
		return []ast.Stmt{out}, nil
	case ast.AssignStmt:
		return []ast.Stmt{ast.AssignStmt{Target: replaceComptimeExpr(s.Target, comptime), Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.GuardedWriteStmt:
		return []ast.Stmt{ast.GuardedWriteStmt{
			Target:    replaceComptimeExpr(s.Target, comptime),
			Value:     replaceComptimeExpr(s.Value, comptime),
			Condition: replaceComptimeExpr(s.Condition, comptime),
		}}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return []ast.Stmt{s}, nil
		}
		return []ast.Stmt{ast.ReturnStmt{Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.ExprStmt:
		return []ast.Stmt{ast.ExprStmt{Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.PushFlowStateStmt, ast.PopFlowStateStmt, ast.GotoFlowStateStmt, ast.FinishFlowStmt:
		// M31a owns static validation. Preserve the node until lowerFlowStmt can
		// explicitly decline runtime emission rather than silently dropping it.
		return []ast.Stmt{s}, nil
	case ast.IfStmt:
		thenBody, err := expandRuntimeNestedBlock(s.ThenBody, comptime, runtime)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := expandRuntimeNestedBlock(*s.ElseBody, comptime, runtime)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return []ast.Stmt{ast.IfStmt{Condition: replaceComptimeExpr(s.Condition, comptime), ThenBody: thenBody, ElseBody: elseBody}}, nil
	case ast.GuardWhenStmt:
		cases := make([]ast.GuardWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			body, err := expandRuntimeNestedBlock(c.Body, comptime, runtime)
			if err != nil {
				return nil, err
			}
			cases = append(cases, ast.GuardWhenCase{Condition: replaceComptimeExpr(c.Condition, comptime), Body: body})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := expandRuntimeNestedBlock(*s.ElseBody, comptime, runtime)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return []ast.Stmt{ast.GuardWhenStmt{Cases: cases, ElseBody: elseBody}}, nil
	case ast.FlowStmt:
		boards := make([]ast.FlowBoardDecl, 0, len(s.Boards))
		for _, board := range s.Boards {
			boards = append(boards, ast.FlowBoardDecl{
				Name:        board.Name,
				Type:        replaceComptimeTypeRef(board.Type, comptime),
				Initializer: replaceComptimeExpr(board.Initializer, comptime),
			})
		}
		states := make([]ast.StateBlock, 0, len(s.States))
		for _, state := range s.States {
			body, err := expandRuntimeNestedBlock(state.Body, comptime, runtime)
			if err != nil {
				return nil, err
			}
			states = append(states, ast.StateBlock{Span: state.Span, Name: state.Name, NameSpan: state.NameSpan, Body: body})
		}
		return []ast.Stmt{ast.FlowStmt{Name: s.Name, Boards: boards, States: states}}, nil
	case ast.ForStmt:
		body, err := expandRuntimeNestedBlock(s.Body, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return []ast.Stmt{ast.ForStmt{Attributes: append([]ast.Attribute(nil), s.Attributes...), Name: s.Name, Start: replaceComptimeExpr(s.Start, comptime), End: replaceComptimeExpr(s.End, comptime), Step: replaceComptimeExpr(s.Step, comptime), Body: body}}, nil
	case ast.StaticAssertStmt:
		value, err := evalComptimeExpr(s.Expr, comptime, runtime, "static assert must be compile-time")
		if err != nil {
			return nil, err
		}
		if value.Type.Name != "bool" {
			return nil, fmt.Errorf("static assert %s must evaluate to bool", s.Text)
		}
		if !value.Bool {
			return nil, fmt.Errorf("failed static assert %s", s.Text)
		}
		return nil, nil
	default:
		return []ast.Stmt{stmt}, nil
	}
}

func expandRuntimeNestedBlock(block ast.Block, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	return expandComptimeBlock(block, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func expandComptimeMatch(stmt ast.ComptimeMatchStmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	subject, err := evalComptimeExpr(stmt.Subject, comptime, runtime, "comptime match scrutinee must be compile-time")
	if err != nil {
		return ast.Block{}, err
	}
	if !consteval.IsInteger(subject.Type) && subject.Type.Name != "bool" {
		return ast.Block{}, fmt.Errorf("comptime match scrutinee must be compile-time")
	}
	seen := map[string]string{}
	hasTrue := false
	hasFalse := false
	var elseBody *ast.Block
	var selected *ast.Block
	for _, arm := range stmt.Arms {
		if arm.IsElse {
			if elseBody != nil {
				return ast.Block{}, fmt.Errorf("duplicate comptime match else arm")
			}
			body := arm.Body
			elseBody = &body
			continue
		}
		pattern, err := evalComptimeMatchPattern(arm.Pattern, comptime, runtime)
		if err != nil {
			return ast.Block{}, err
		}
		if !comptimePatternMatchesSubjectType(pattern, subject) {
			return ast.Block{}, fmt.Errorf("comptime match arm pattern type %s does not match scrutinee type %s", pattern.Type.Name, subject.Type.Name)
		}
		key := comptimeValueKey(pattern)
		label := comptimeValueLabel(pattern)
		if prior, ok := seen[key]; ok {
			return ast.Block{}, fmt.Errorf("duplicate comptime match arm for %s", prior)
		}
		seen[key] = label
		if pattern.Type.Name == "bool" {
			if pattern.Bool {
				hasTrue = true
			} else {
				hasFalse = true
			}
		}
		if selected == nil && comptimeValuesEqual(subject, pattern) {
			body := arm.Body
			selected = &body
		}
	}
	if subject.Type.Name == "bool" {
		if (!hasTrue || !hasFalse) && elseBody == nil {
			return ast.Block{}, fmt.Errorf("bool comptime match requires else arm unless true and false arms are both present")
		}
	} else if consteval.IsInteger(subject.Type) && elseBody == nil {
		return ast.Block{}, fmt.Errorf("comptime match over integer requires else arm")
	}
	if selected == nil {
		selected = elseBody
	}
	if selected == nil {
		return ast.Block{}, fmt.Errorf("no comptime match arm selected and no else arm provided")
	}
	return expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func expandComptimeWhenUtility(stmt ast.ComptimeWhenUtilityStmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	seenLabels := map[string]struct{}{}
	var selected *ast.Block
	selectedLabel := ""
	selectedScore := int64(0)
	hasSelected := false
	tiedHighestLabel := ""
	for _, c := range stmt.Cases {
		if _, exists := seenLabels[c.Label]; exists {
			return ast.Block{}, fmt.Errorf("duplicate comptime when utility case label %s", c.Label)
		}
		seenLabels[c.Label] = struct{}{}
		eligible := true
		if c.Condition != nil {
			guard, err := evalComptimeExpr(c.Condition, comptime, runtime, "comptime when guard must be compile-time bool")
			if err != nil {
				return ast.Block{}, annotateComptimeWhenExprError(err, "guard")
			}
			if guard.Type.Name != "bool" {
				return ast.Block{}, fmt.Errorf("comptime when guard must be compile-time bool")
			}
			eligible = guard.Bool
		}
		if !eligible {
			continue
		}
		score, err := evalComptimeExpr(c.Score, comptime, runtime, "comptime when score must be compile-time numeric")
		if err != nil {
			return ast.Block{}, annotateComptimeWhenExprError(err, "score")
		}
		if !consteval.IsInteger(score.Type) {
			return ast.Block{}, fmt.Errorf("comptime when score must be compile-time numeric")
		}
		if hasSelected && score.Int32 == selectedScore {
			if tiedHighestLabel == "" {
				tiedHighestLabel = c.Label
			}
			continue
		}
		if !hasSelected || score.Int32 > selectedScore {
			body := c.Body
			selected = &body
			selectedLabel = c.Label
			selectedScore = score.Int32
			hasSelected = true
			tiedHighestLabel = ""
		}
	}
	if hasSelected && tiedHighestLabel != "" {
		return ast.Block{}, fmt.Errorf("ambiguous comptime when utility cases %s and %s have tied score %d", selectedLabel, tiedHighestLabel, selectedScore)
	}
	if !hasSelected {
		selected = stmt.ElseBody
	}
	if selected == nil {
		return ast.Block{}, fmt.Errorf("no comptime when utility case qualified and no else block provided")
	}
	return expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func expandComptimeFor(stmt ast.ComptimeForStmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	start, err := evalComptimeExpr(stmt.Start, comptime, runtime, "comptime for start must be compile-time integer")
	if err != nil {
		return ast.Block{}, err
	}
	end, err := evalComptimeExpr(stmt.End, comptime, runtime, "comptime for end must be compile-time integer")
	if err != nil {
		return ast.Block{}, err
	}
	if !consteval.IsInteger(start.Type) {
		return ast.Block{}, fmt.Errorf("comptime for start must be compile-time integer")
	}
	if !consteval.IsInteger(end.Type) {
		return ast.Block{}, fmt.Errorf("comptime for end must be compile-time integer")
	}
	if start.Int32 < 0 || end.Int32 < 0 {
		return ast.Block{}, fmt.Errorf("comptime for bounds must be non-negative in SDSL-V M16")
	}
	if start.Int32 > end.Int32 {
		return ast.Block{}, fmt.Errorf("comptime for range start must be <= end")
	}
	out := ast.Block{Statements: []ast.Stmt{}}
	totalExpanded := 0
	for i := start.Int32; i < end.Int32; i++ {
		iterComptime := cloneComptimeBindings(comptime)
		iterComptime[stmt.Name] = comptimeBinding{typ: ast.TypeRef{Name: "u32"}, int32: i}
		body, err := expandComptimeBlock(stmt.Body, iterComptime, cloneRuntimeBindings(runtime))
		if err != nil {
			return ast.Block{}, err
		}
		body = renameComptimeIterationLocals(body, fmt.Sprintf("__ct%d", i))
		totalExpanded += blockExpansionCost(body)
		if totalExpanded > maxComptimeForExpandedStatements {
			return ast.Block{}, fmt.Errorf("comptime for expansion exceeds M16 limit of %d iterations", maxComptimeForExpandedStatements)
		}
		out.Statements = append(out.Statements, body.Statements...)
	}
	return out, nil
}

func renameComptimeIterationLocals(block ast.Block, suffix string) ast.Block {
	names := map[string]string{}
	collectLocalLetNames(block, names, suffix)
	if len(names) == 0 {
		return block
	}
	return renameRuntimeLocalsInBlock(block, names)
}

func collectLocalLetNames(block ast.Block, names map[string]string, suffix string) {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case ast.LetStmt:
			if _, exists := names[s.Name]; !exists {
				names[s.Name] = s.Name + suffix
			}
		case ast.IfStmt:
			collectLocalLetNames(s.ThenBody, names, suffix)
			if s.ElseBody != nil {
				collectLocalLetNames(*s.ElseBody, names, suffix)
			}
		case ast.GuardWhenStmt:
			for _, c := range s.Cases {
				collectLocalLetNames(c.Body, names, suffix)
			}
			if s.ElseBody != nil {
				collectLocalLetNames(*s.ElseBody, names, suffix)
			}
		case ast.FlowStmt:
			for _, state := range s.States {
				collectLocalLetNames(state.Body, names, suffix)
			}
		case ast.ForStmt:
			collectLocalLetNames(s.Body, names, suffix)
		}
	}
}

func renameRuntimeLocalsInBlock(block ast.Block, names map[string]string) ast.Block {
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		out.Statements = append(out.Statements, renameRuntimeLocalsInStmt(stmt, names))
	}
	return out
}

func renameRuntimeLocalsInStmt(stmt ast.Stmt, names map[string]string) ast.Stmt {
	switch s := stmt.(type) {
	case ast.LetStmt:
		name := s.Name
		if renamed, ok := names[name]; ok {
			name = renamed
		}
		var value ast.Expr
		if s.Value != nil {
			value = renameRuntimeLocalsInExpr(s.Value, names)
		}
		return ast.LetStmt{Name: name, Type: s.Type, Value: value}
	case ast.AssignStmt:
		return ast.AssignStmt{Target: renameRuntimeLocalsInExpr(s.Target, names), Value: renameRuntimeLocalsInExpr(s.Value, names)}
	case ast.GuardedWriteStmt:
		return ast.GuardedWriteStmt{
			Target:    renameRuntimeLocalsInExpr(s.Target, names),
			Value:     renameRuntimeLocalsInExpr(s.Value, names),
			Condition: renameRuntimeLocalsInExpr(s.Condition, names),
		}
	case ast.ReturnStmt:
		if s.Value == nil {
			return s
		}
		return ast.ReturnStmt{Value: renameRuntimeLocalsInExpr(s.Value, names)}
	case ast.ExprStmt:
		return ast.ExprStmt{Value: renameRuntimeLocalsInExpr(s.Value, names)}
	case ast.IfStmt:
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body := renameRuntimeLocalsInBlock(*s.ElseBody, names)
			elseBody = &body
		}
		return ast.IfStmt{Condition: renameRuntimeLocalsInExpr(s.Condition, names), ThenBody: renameRuntimeLocalsInBlock(s.ThenBody, names), ElseBody: elseBody}
	case ast.GuardWhenStmt:
		cases := make([]ast.GuardWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			cases = append(cases, ast.GuardWhenCase{Condition: renameRuntimeLocalsInExpr(c.Condition, names), Body: renameRuntimeLocalsInBlock(c.Body, names)})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body := renameRuntimeLocalsInBlock(*s.ElseBody, names)
			elseBody = &body
		}
		return ast.GuardWhenStmt{Cases: cases, ElseBody: elseBody}
	case ast.FlowStmt:
		boards := make([]ast.FlowBoardDecl, 0, len(s.Boards))
		for _, board := range s.Boards {
			boards = append(boards, ast.FlowBoardDecl{
				Name:        board.Name,
				Type:        board.Type,
				Initializer: renameRuntimeLocalsInExpr(board.Initializer, names),
			})
		}
		states := make([]ast.StateBlock, 0, len(s.States))
		for _, state := range s.States {
			states = append(states, ast.StateBlock{Span: state.Span, Name: state.Name, NameSpan: state.NameSpan, Body: renameRuntimeLocalsInBlock(state.Body, names)})
		}
		return ast.FlowStmt{Name: s.Name, Boards: boards, States: states}
	case ast.ForStmt:
		return ast.ForStmt{
			Attributes: append([]ast.Attribute(nil), s.Attributes...),
			Name:       s.Name,
			Start:      renameRuntimeLocalsInExpr(s.Start, names),
			End:        renameRuntimeLocalsInExpr(s.End, names),
			Step:       renameRuntimeLocalsInExpr(s.Step, names),
			Body:       renameRuntimeLocalsInBlock(s.Body, names),
		}
	default:
		return stmt
	}
}

func renameRuntimeLocalsInExpr(expr ast.Expr, names map[string]string) ast.Expr {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if renamed, ok := names[e.Name]; ok {
			return ast.IdentifierExpr{Name: renamed}
		}
		return e
	case ast.FieldAccessExpr:
		return ast.FieldAccessExpr{Target: renameRuntimeLocalsInExpr(e.Target, names), Field: e.Field}
	case ast.IndexExpr:
		out := ast.IndexExpr{Span: e.Span, Target: renameRuntimeLocalsInExpr(e.Target, names), Index: renameRuntimeLocalsInExpr(e.Index, names), HasSecond: e.HasSecond}
		for _, index := range ast.IndexExpressions(e) {
			out.Indices = append(out.Indices, renameRuntimeLocalsInExpr(index, names))
		}
		if e.HasSecond {
			out.Index2 = renameRuntimeLocalsInExpr(e.Index2, names)
		}
		return out
	case ast.GuardedReadExpr:
		return ast.GuardedReadExpr{
			Target:    renameRuntimeLocalsInExpr(e.Target, names),
			Condition: renameRuntimeLocalsInExpr(e.Condition, names),
			Fallback:  renameRuntimeLocalsInExpr(e.Fallback, names),
		}
	case ast.CallExpr:
		args := make([]ast.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, renameRuntimeLocalsInExpr(arg, names))
		}
		return ast.CallExpr{Callee: renameRuntimeLocalsInExpr(e.Callee, names), Arguments: args}
	case ast.BinaryExpr:
		return ast.BinaryExpr{Left: renameRuntimeLocalsInExpr(e.Left, names), Operator: e.Operator, Right: renameRuntimeLocalsInExpr(e.Right, names)}
	case ast.UnaryExpr:
		return ast.UnaryExpr{Operator: e.Operator, Operand: renameRuntimeLocalsInExpr(e.Operand, names)}
	case ast.ParenExpr:
		return ast.ParenExpr{Inner: renameRuntimeLocalsInExpr(e.Inner, names)}
	case ast.WhenUtilityExpr:
		cases := make([]ast.UtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			cases = append(cases, ast.UtilityCase{Value: renameRuntimeLocalsInExpr(c.Value, names), Condition: renameRuntimeLocalsInExpr(c.Condition, names), Score: renameRuntimeLocalsInExpr(c.Score, names)})
		}
		return ast.WhenUtilityExpr{Cases: cases, Else: renameRuntimeLocalsInExpr(e.Else, names)}
	case ast.WithExpr:
		updates := make([]ast.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			updates = append(updates, ast.FieldUpdate{Name: update.Name, Value: renameRuntimeLocalsInExpr(update.Value, names)})
		}
		return ast.WithExpr{Base: renameRuntimeLocalsInExpr(e.Base, names), Updates: updates}
	case ast.EnumConstructExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: renameRuntimeLocalsInExpr(field.Value, names)})
		}
		return ast.EnumConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Fields: fields}
	case ast.BoardLiteralExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: renameRuntimeLocalsInExpr(field.Value, names)})
		}
		return ast.BoardLiteralExpr{TypeName: e.TypeName, Fields: fields}
	case ast.DeriveExpr:
		fields := make([]ast.DeriveField, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.DeriveField{Name: field.Name, Value: renameRuntimeLocalsInExpr(field.Value, names)})
		}
		return ast.DeriveExpr{Fields: fields}
	case ast.MatchExpr:
		arms := make([]ast.MatchArm, 0, len(e.Arms))
		for _, arm := range e.Arms {
			arms = append(arms, ast.MatchArm{EnumName: arm.EnumName, VariantName: arm.VariantName, BindingName: arm.BindingName, Value: renameRuntimeLocalsInExpr(arm.Value, names)})
		}
		return ast.MatchExpr{Subject: renameRuntimeLocalsInExpr(e.Subject, names), Arms: arms}
	case ast.ReductionExpr:
		return ast.ReductionExpr{
			Attributes: append([]ast.Attribute(nil), e.Attributes...),
			Op:         e.Op,
			Name:       e.Name,
			Start:      renameRuntimeLocalsInExpr(e.Start, names),
			End:        renameRuntimeLocalsInExpr(e.End, names),
			Step:       renameRuntimeLocalsInExpr(e.Step, names),
			Body:       renameRuntimeLocalsInExpr(e.Body, names),
		}
	default:
		return expr
	}
}

func annotateComptimeWhenExprError(err error, position string) error {
	msg := err.Error()
	const prefix = "comptime expression cannot reference "
	if strings.HasPrefix(msg, prefix) {
		return fmt.Errorf("comptime when %s cannot reference %s", position, strings.TrimPrefix(msg, prefix))
	}
	return err
}

func evalComptimeMatchPattern(expr ast.Expr, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (consteval.Value, error) {
	switch expr.(type) {
	case ast.IntegerLiteral, ast.BoolLiteral:
	default:
		return consteval.Value{}, fmt.Errorf("comptime match arm pattern must be compile-time literal")
	}
	value, err := evalComptimeExpr(expr, comptime, runtime, "comptime match arm pattern must be compile-time literal")
	if err != nil {
		return consteval.Value{}, err
	}
	if !consteval.IsInteger(value.Type) && value.Type.Name != "bool" {
		return consteval.Value{}, fmt.Errorf("comptime match arm pattern must be compile-time literal")
	}
	return value, nil
}

func comptimePatternMatchesSubjectType(pattern, subject consteval.Value) bool {
	if subject.Type.Name == "bool" || pattern.Type.Name == "bool" {
		return subject.Type.Name == pattern.Type.Name
	}
	return consteval.IsInteger(subject.Type) && consteval.IsInteger(pattern.Type)
}

func comptimeValuesEqual(left, right consteval.Value) bool {
	if left.Type.Name == "bool" && right.Type.Name == "bool" {
		return left.Bool == right.Bool
	}
	if consteval.IsInteger(left.Type) && consteval.IsInteger(right.Type) {
		return left.Int32 == right.Int32
	}
	return false
}

func comptimeValueKey(value consteval.Value) string {
	if value.Type.Name == "bool" {
		if value.Bool {
			return "bool:true"
		}
		return "bool:false"
	}
	return "int:" + strconv.FormatInt(value.Int32, 10)
}

func comptimeValueLabel(value consteval.Value) string {
	if value.Type.Name == "bool" {
		if value.Bool {
			return "true"
		}
		return "false"
	}
	text := strconv.FormatInt(value.Int32, 10)
	if value.Type.Name == "u32" {
		text += "u"
	}
	return text
}

func evalComptimeExpr(expr ast.Expr, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding, context string) (consteval.Value, error) {
	if err := rejectRuntimeComptimeRefs(expr, runtime); err != nil {
		return consteval.Value{}, err
	}
	env := map[string]consteval.Value{}
	for name, value := range comptime {
		env[name] = consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true}
	}
	value, err := consteval.Eval(expr, env)
	if err != nil {
		return consteval.Value{}, fmt.Errorf("%s: %w", context, err)
	}
	return value, nil
}

func rejectRuntimeComptimeRefs(expr ast.Expr, runtime map[string]runtimeBinding) error {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if binding, ok := runtime[e.Name]; ok {
			return fmt.Errorf("comptime expression cannot reference %s `%s`", binding.origin, e.Name)
		}
	case ast.FieldAccessExpr:
		if root, ok := rootNameForComptime(e); ok {
			if binding, exists := runtime[root]; exists {
				return fmt.Errorf("comptime expression cannot reference %s `%s`", binding.origin, fieldPathForComptime(e))
			}
		}
		return rejectRuntimeComptimeRefs(e.Target, runtime)
	case ast.IndexExpr:
		if err := rejectRuntimeComptimeRefs(e.Target, runtime); err != nil {
			return err
		}
		if err := rejectRuntimeComptimeRefs(e.Index, runtime); err != nil {
			return err
		}
		if e.HasSecond {
			return rejectRuntimeComptimeRefs(e.Index2, runtime)
		}
	case ast.GuardedReadExpr:
		return fmt.Errorf("comptime expression cannot use guarded read in SDSL-V M16a")
	case ast.CallExpr:
		return fmt.Errorf("comptime expression cannot call functions in SDSL-V M13")
	case ast.BinaryExpr:
		if err := rejectRuntimeComptimeRefs(e.Left, runtime); err != nil {
			return err
		}
		return rejectRuntimeComptimeRefs(e.Right, runtime)
	case ast.UnaryExpr:
		return rejectRuntimeComptimeRefs(e.Operand, runtime)
	case ast.ParenExpr:
		return rejectRuntimeComptimeRefs(e.Inner, runtime)
	case ast.ReductionExpr, ast.MatchExpr, ast.WithExpr, ast.EnumConstructExpr, ast.BoardLiteralExpr, ast.DeriveExpr:
		return fmt.Errorf("comptime expression cannot use runtime expression forms in SDSL-V M13")
	}
	return nil
}

func replaceComptimeExpr(expr ast.Expr, comptime map[string]comptimeBinding) ast.Expr {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if value, ok := comptime[e.Name]; ok {
			return literalExprForValue(specializeValue{typ: value.typ, int32: value.int32, boolVal: value.boolVal})
		}
		return expr
	case ast.FieldAccessExpr:
		return ast.FieldAccessExpr{Target: replaceComptimeExpr(e.Target, comptime), Field: e.Field}
	case ast.IndexExpr:
		out := ast.IndexExpr{Span: e.Span, Target: replaceComptimeExpr(e.Target, comptime), Index: replaceComptimeExpr(e.Index, comptime), HasSecond: e.HasSecond}
		for _, index := range ast.IndexExpressions(e) {
			out.Indices = append(out.Indices, replaceComptimeExpr(index, comptime))
		}
		if e.HasSecond {
			out.Index2 = replaceComptimeExpr(e.Index2, comptime)
		}
		return out
	case ast.GuardedReadExpr:
		return ast.GuardedReadExpr{
			Target:    replaceComptimeExpr(e.Target, comptime),
			Condition: replaceComptimeExpr(e.Condition, comptime),
			Fallback:  replaceComptimeExpr(e.Fallback, comptime),
		}
	case ast.CallExpr:
		args := make([]ast.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, replaceComptimeExpr(arg, comptime))
		}
		return ast.CallExpr{Callee: replaceComptimeExpr(e.Callee, comptime), Arguments: args}
	case ast.BinaryExpr:
		return ast.BinaryExpr{Left: replaceComptimeExpr(e.Left, comptime), Operator: e.Operator, Right: replaceComptimeExpr(e.Right, comptime)}
	case ast.UnaryExpr:
		return ast.UnaryExpr{Operator: e.Operator, Operand: replaceComptimeExpr(e.Operand, comptime)}
	case ast.ParenExpr:
		return ast.ParenExpr{Inner: replaceComptimeExpr(e.Inner, comptime)}
	case ast.WhenUtilityExpr:
		cases := make([]ast.UtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			cases = append(cases, ast.UtilityCase{Value: replaceComptimeExpr(c.Value, comptime), Condition: replaceComptimeExpr(c.Condition, comptime), Score: replaceComptimeExpr(c.Score, comptime)})
		}
		return ast.WhenUtilityExpr{Cases: cases, Else: replaceComptimeExpr(e.Else, comptime)}
	case ast.WithExpr:
		updates := make([]ast.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			updates = append(updates, ast.FieldUpdate{Name: update.Name, Value: replaceComptimeExpr(update.Value, comptime)})
		}
		return ast.WithExpr{Base: replaceComptimeExpr(e.Base, comptime), Updates: updates}
	case ast.EnumConstructExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: replaceComptimeExpr(field.Value, comptime)})
		}
		return ast.EnumConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Fields: fields}
	case ast.BoardLiteralExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: replaceComptimeExpr(field.Value, comptime)})
		}
		return ast.BoardLiteralExpr{TypeName: e.TypeName, Fields: fields}
	case ast.DeriveExpr:
		fields := make([]ast.DeriveField, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.DeriveField{Name: field.Name, Value: replaceComptimeExpr(field.Value, comptime)})
		}
		return ast.DeriveExpr{Fields: fields}
	case ast.MatchExpr:
		arms := make([]ast.MatchArm, 0, len(e.Arms))
		for _, arm := range e.Arms {
			arms = append(arms, ast.MatchArm{EnumName: arm.EnumName, VariantName: arm.VariantName, BindingName: arm.BindingName, Value: replaceComptimeExpr(arm.Value, comptime)})
		}
		return ast.MatchExpr{Subject: replaceComptimeExpr(e.Subject, comptime), Arms: arms}
	case ast.ReductionExpr:
		return ast.ReductionExpr{Attributes: append([]ast.Attribute(nil), e.Attributes...), Op: e.Op, Name: e.Name, Start: replaceComptimeExpr(e.Start, comptime), End: replaceComptimeExpr(e.End, comptime), Step: replaceComptimeExpr(e.Step, comptime), Body: replaceComptimeExpr(e.Body, comptime)}
	default:
		return expr
	}
}

func replaceComptimeStmt(stmt ast.Stmt, comptime map[string]comptimeBinding) ast.Stmt {
	switch s := stmt.(type) {
	case ast.LetStmt:
		out := s
		out.Type = replaceComptimeTypeRef(s.Type, comptime)
		if s.Value != nil {
			out.Value = replaceComptimeExpr(s.Value, comptime)
		}
		return out
	case ast.AssignStmt:
		return ast.AssignStmt{Target: replaceComptimeExpr(s.Target, comptime), Value: replaceComptimeExpr(s.Value, comptime)}
	case ast.GuardedWriteStmt:
		return ast.GuardedWriteStmt{
			Target:    replaceComptimeExpr(s.Target, comptime),
			Value:     replaceComptimeExpr(s.Value, comptime),
			Condition: replaceComptimeExpr(s.Condition, comptime),
		}
	case ast.ReturnStmt:
		if s.Value == nil {
			return s
		}
		return ast.ReturnStmt{Value: replaceComptimeExpr(s.Value, comptime)}
	case ast.ExprStmt:
		return ast.ExprStmt{Value: replaceComptimeExpr(s.Value, comptime)}
	case ast.IfStmt:
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body := replaceComptimeBlock(*s.ElseBody, comptime)
			elseBody = &body
		}
		return ast.IfStmt{Condition: replaceComptimeExpr(s.Condition, comptime), ThenBody: replaceComptimeBlock(s.ThenBody, comptime), ElseBody: elseBody}
	case ast.GuardWhenStmt:
		cases := make([]ast.GuardWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			cases = append(cases, ast.GuardWhenCase{Condition: replaceComptimeExpr(c.Condition, comptime), Body: replaceComptimeBlock(c.Body, comptime)})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body := replaceComptimeBlock(*s.ElseBody, comptime)
			elseBody = &body
		}
		return ast.GuardWhenStmt{Cases: cases, ElseBody: elseBody}
	case ast.FlowStmt:
		boards := make([]ast.FlowBoardDecl, 0, len(s.Boards))
		for _, board := range s.Boards {
			boards = append(boards, ast.FlowBoardDecl{
				Name:        board.Name,
				Type:        replaceComptimeTypeRef(board.Type, comptime),
				Initializer: replaceComptimeExpr(board.Initializer, comptime),
			})
		}
		states := make([]ast.StateBlock, 0, len(s.States))
		for _, state := range s.States {
			states = append(states, ast.StateBlock{Span: state.Span, Name: state.Name, NameSpan: state.NameSpan, Body: replaceComptimeBlock(state.Body, comptime)})
		}
		return ast.FlowStmt{Name: s.Name, Boards: boards, States: states}
	case ast.ForStmt:
		return ast.ForStmt{
			Attributes: append([]ast.Attribute(nil), s.Attributes...),
			Name:       s.Name,
			Start:      replaceComptimeExpr(s.Start, comptime),
			End:        replaceComptimeExpr(s.End, comptime),
			Step:       replaceComptimeExpr(s.Step, comptime),
			Body:       replaceComptimeBlock(s.Body, comptime),
		}
	default:
		return stmt
	}
}

func replaceComptimeBlock(block ast.Block, comptime map[string]comptimeBinding) ast.Block {
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		out.Statements = append(out.Statements, replaceComptimeStmt(stmt, comptime))
	}
	return out
}

func cloneComptimeBindings(in map[string]comptimeBinding) map[string]comptimeBinding {
	out := make(map[string]comptimeBinding, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneRuntimeBindings(in map[string]runtimeBinding) map[string]runtimeBinding {
	out := make(map[string]runtimeBinding, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func blockExpansionCost(block ast.Block) int {
	total := 0
	for _, stmt := range block.Statements {
		total += stmtExpansionCost(stmt)
	}
	return total
}

func stmtExpansionCost(stmt ast.Stmt) int {
	switch s := stmt.(type) {
	case ast.IfStmt:
		total := 1 + blockExpansionCost(s.ThenBody)
		if s.ElseBody != nil {
			total += blockExpansionCost(*s.ElseBody)
		}
		return total
	case ast.GuardWhenStmt:
		total := 1
		for _, c := range s.Cases {
			total += blockExpansionCost(c.Body)
		}
		if s.ElseBody != nil {
			total += blockExpansionCost(*s.ElseBody)
		}
		return total
	case ast.ForStmt:
		return 1 + blockExpansionCost(s.Body)
	default:
		return 1
	}
}

func rootNameForComptime(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name, true
	case ast.FieldAccessExpr:
		return rootNameForComptime(e.Target)
	default:
		return "", false
	}
}

func fieldPathForComptime(expr ast.Expr) string {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name
	case ast.FieldAccessExpr:
		return fieldPathForComptime(e.Target) + "." + e.Field
	default:
		return ""
	}
}

const maxRegTileElements = 64

func specializeTypeRef(ref ast.TypeRef, env map[string]specializeValue) (ast.TypeRef, error) {
	out := ref
	if ref.Name == "array" || ref.Name == "tile" || ref.Name == "reg_tile" || ref.Name == "matrix_view" {
		args := make([]ast.TypeRef, len(ref.Args))
		for i, arg := range ref.Args {
			next, err := specializeTypeRef(arg, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			args[i] = next
		}
		out.Args = args
		if ref.HasArraySize {
			sizeExpr, err := specializeIntExpr(ref.ArraySize, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			out.ArraySize = sizeExpr
		}
		if ref.HasTileShape {
			rows, err := specializeIntExpr(ref.TileRows, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			cols, err := specializeIntExpr(ref.TileCols, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			out.TileRows = rows
			out.TileCols = cols
			if ref.Name == "reg_tile" {
				rowsValue := mustConcreteInt(rows)
				colsValue := mustConcreteInt(cols)
				if rowsValue*colsValue > maxRegTileElements {
					return ast.TypeRef{}, fmt.Errorf("reg_tile has %d elements; M15 limit is %d", rowsValue*colsValue, maxRegTileElements)
				}
			}
		}
	}
	return out, nil
}

func specializeLocalTypeRef(ref ast.TypeRef, env map[string]specializeValue) (ast.TypeRef, error) {
	out := ref
	if ref.Name == "array" || ref.Name == "tile" || ref.Name == "reg_tile" || ref.Name == "matrix_view" {
		args := make([]ast.TypeRef, len(ref.Args))
		for i, arg := range ref.Args {
			next, err := specializeLocalTypeRef(arg, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			args[i] = next
		}
		out.Args = args
		if ref.HasArraySize {
			out.ArraySize = specializeExpr(ref.ArraySize, env)
		}
		if ref.HasTileShape {
			out.TileRows = specializeExpr(ref.TileRows, env)
			out.TileCols = specializeExpr(ref.TileCols, env)
		}
	}
	return out, nil
}

func specializeIntExpr(expr ast.Expr, env map[string]specializeValue) (ast.Expr, error) {
	value, err := evalSpecializeConstExpr(expr, env)
	if err != nil {
		return nil, err
	}
	if value.int32 <= 0 {
		return nil, fmt.Errorf("expected positive integer constant expression")
	}
	return literalExprForValue(value), nil
}

func specializeExpr(expr ast.Expr, env map[string]specializeValue) ast.Expr {
	switch e := expr.(type) {
	case ast.FieldAccessExpr:
		if path, ok := specializeFieldPath(e); ok {
			if value, ok := env[path]; ok {
				return literalExprForValue(value)
			}
		}
		return ast.FieldAccessExpr{Target: specializeExpr(e.Target, env), Field: e.Field}
	case ast.IndexExpr:
		out := ast.IndexExpr{Span: e.Span, Target: specializeExpr(e.Target, env), Index: specializeExpr(e.Index, env), HasSecond: e.HasSecond}
		for _, index := range ast.IndexExpressions(e) {
			out.Indices = append(out.Indices, specializeExpr(index, env))
		}
		if e.HasSecond {
			out.Index2 = specializeExpr(e.Index2, env)
		}
		return out
	case ast.GuardedReadExpr:
		return ast.GuardedReadExpr{
			Target:    specializeExpr(e.Target, env),
			Condition: specializeExpr(e.Condition, env),
			Fallback:  specializeExpr(e.Fallback, env),
		}
	case ast.CallExpr:
		args := make([]ast.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, specializeExpr(arg, env))
		}
		return ast.CallExpr{Callee: specializeExpr(e.Callee, env), Arguments: args}
	case ast.BinaryExpr:
		return ast.BinaryExpr{Left: specializeExpr(e.Left, env), Operator: e.Operator, Right: specializeExpr(e.Right, env)}
	case ast.UnaryExpr:
		return ast.UnaryExpr{Operator: e.Operator, Operand: specializeExpr(e.Operand, env)}
	case ast.ParenExpr:
		return ast.ParenExpr{Inner: specializeExpr(e.Inner, env)}
	case ast.WhenUtilityExpr:
		cases := make([]ast.UtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			cases = append(cases, ast.UtilityCase{Value: specializeExpr(c.Value, env), Condition: specializeExpr(c.Condition, env), Score: specializeExpr(c.Score, env)})
		}
		return ast.WhenUtilityExpr{Cases: cases, Else: specializeExpr(e.Else, env)}
	case ast.WithExpr:
		updates := make([]ast.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			updates = append(updates, ast.FieldUpdate{Name: update.Name, Value: specializeExpr(update.Value, env)})
		}
		return ast.WithExpr{Base: specializeExpr(e.Base, env), Updates: updates}
	case ast.EnumConstructExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: specializeExpr(field.Value, env)})
		}
		return ast.EnumConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Fields: fields}
	case ast.BoardLiteralExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: specializeExpr(field.Value, env)})
		}
		return ast.BoardLiteralExpr{TypeName: e.TypeName, Fields: fields}
	case ast.DeriveExpr:
		fields := make([]ast.DeriveField, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.DeriveField{Name: field.Name, Value: specializeExpr(field.Value, env)})
		}
		return ast.DeriveExpr{Fields: fields}
	case ast.MatchExpr:
		arms := make([]ast.MatchArm, 0, len(e.Arms))
		for _, arm := range e.Arms {
			arms = append(arms, ast.MatchArm{EnumName: arm.EnumName, VariantName: arm.VariantName, BindingName: arm.BindingName, Value: specializeExpr(arm.Value, env)})
		}
		return ast.MatchExpr{Subject: specializeExpr(e.Subject, env), Arms: arms}
	case ast.ReductionExpr:
		return ast.ReductionExpr{
			Attributes: append([]ast.Attribute(nil), e.Attributes...),
			Op:         e.Op,
			Name:       e.Name,
			Start:      specializeExpr(e.Start, env),
			End:        specializeExpr(e.End, env),
			Step:       specializeExpr(e.Step, env),
			Body:       specializeExpr(e.Body, env),
		}
	default:
		return expr
	}
}

func evalSpecializeConstExpr(expr ast.Expr, env map[string]specializeValue) (specializeValue, error) {
	ctEnv := map[string]consteval.Value{}
	for key, value := range env {
		ctEnv[key] = consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true}
	}
	value, err := consteval.Eval(expr, ctEnv)
	if err != nil {
		return specializeValue{}, err
	}
	return specializeValue{typ: value.Type, int32: value.Int32, boolVal: value.Bool}, nil
}

func literalExprForValue(value specializeValue) ast.Expr {
	return consteval.LiteralExpr(consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true})
}

func mustConcreteInt(expr ast.Expr) int {
	value, err := evalSpecializeConstExpr(expr, nil)
	if err != nil {
		panic(err)
	}
	return int(value.int32)
}

type fieldInfo struct {
	access     string
	typ        ast.TypeRef
	attributes []ast.Attribute
}

type enumVariantInfo struct {
	name         string
	fields       map[string]fieldInfo
	fieldOrder   []string
	payloadType  string
	hasPayload   bool
	variantIndex int
}

type typeInfo struct {
	kind         string
	target       ast.TypeRef
	fields       map[string]fieldInfo
	enumVariants map[string]enumVariantInfo
}

type functionInfo struct {
	name        string
	emittedName string
	returnType  ast.TypeRef
	params      []ast.Parameter
	shaderName  string
}

type binding struct {
	name   string
	kind   vdmir.VarKind
	typ    ast.TypeRef
	access string
}

func cloneBindings(in map[string]binding) map[string]binding {
	out := make(map[string]binding, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

type lowering struct {
	provenance       vdmir.Provenance
	types            map[string]typeInfo
	functions        map[string]functionInfo
	deriveTemp       int
	testInputs       map[string]validate.ValidatedTestInput
	currentTestInput *validate.ValidatedTestInput
	currentFunction  string
	currentShader    string
	flows            []vdmir.Flow
	tensorAssigns    map[source.Span]validate.ValidatedTensorAssign
	tensorReductions map[source.Span]validate.ValidatedTensorReduction
}

func (l *lowering) seedBuiltins() {
	for _, name := range []string{"void", "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4"} {
		l.types[name] = typeInfo{kind: "builtin"}
	}
	l.types["uint2"] = builtinUintVectorType(2)
	l.types["uint3"] = builtinUintVectorType(3)
	l.types["uint4"] = builtinUintVectorType(4)
}

func (l *lowering) collect(module ast.Module) {
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			l.types[d.Name] = typeInfo{kind: "alias", target: d.Type}
		case ast.RecordDecl:
			l.types[d.Name] = typeInfo{kind: "record", fields: collectFields(d.Fields)}
		case ast.BoardDecl:
			l.types[d.Name] = typeInfo{kind: "board", fields: collectFields(d.Fields)}
		case ast.StreamDecl:
			l.types[d.Name] = typeInfo{kind: "stream", fields: collectFields(d.Fields)}
		case ast.ConceptDecl:
			l.types[d.Name] = typeInfo{kind: "concept", fields: collectConceptFields(d)}
		case ast.EnumDecl:
			l.collectEnum(d)
		case ast.FunctionDecl:
			l.functions[d.Name] = functionInfo{name: d.Name, emittedName: d.Name, returnType: d.ReturnType, params: d.Parameters}
		case ast.ShaderDecl:
			if d.Template != nil {
				continue
			}
			for _, method := range d.Methods {
				key := d.Name + "_" + method.Name
				l.functions[key] = functionInfo{
					name:        method.Name,
					emittedName: key,
					returnType:  method.ReturnType,
					params:      method.Parameters,
					shaderName:  d.Name,
				}
			}
		case ast.ConfigDecl, ast.CompileDecl:
			continue
		}
	}
}

func (l *lowering) collectEnum(enum ast.EnumDecl) {
	variants := map[string]enumVariantInfo{}
	for i, variant := range enum.Variants {
		fields := collectFields(variant.Fields)
		order := make([]string, 0, len(variant.Fields))
		for _, field := range variant.Fields {
			order = append(order, field.Name)
		}
		info := enumVariantInfo{
			name:         variant.Name,
			fields:       fields,
			fieldOrder:   order,
			hasPayload:   variant.Payload,
			variantIndex: i,
		}
		if variant.Payload {
			info.payloadType = payloadTypeName(enum.Name, variant.Name)
			l.types[info.payloadType] = typeInfo{kind: "record", fields: fields}
		}
		variants[variant.Name] = info
	}
	l.types[enum.Name] = typeInfo{kind: "enum", enumVariants: variants}
}

func (l *lowering) lowerEnum(enum ast.EnumDecl) vdmir.Enum {
	out := vdmir.Enum{Provenance: l.provenance, Name: enum.Name}
	for _, variant := range enum.Variants {
		entry := vdmir.EnumVariant{Name: variant.Name, HasPayload: variant.Payload}
		for _, field := range variant.Fields {
			entry.Payload = append(entry.Payload, vdmir.Field{
				Provenance: l.provenance,
				Name:       field.Name,
				Type:       l.lowerTypeRef(field.Type),
			})
		}
		out.Variants = append(out.Variants, entry)
	}
	return out
}

func (l *lowering) lowerRecord(name string, fields []ast.Field) vdmir.Record {
	record := vdmir.Record{Provenance: l.provenance, Name: name}
	for _, field := range fields {
		record.Fields = append(record.Fields, vdmir.Field{
			Provenance: l.provenance,
			Name:       field.Name,
			Type:       l.lowerTypeRef(field.Type),
		})
	}
	return record
}

func (l *lowering) lowerBoard(name string, fields []ast.Field) vdmir.Board {
	board := vdmir.Board{Provenance: l.provenance, Name: name}
	for _, field := range fields {
		board.Fields = append(board.Fields, vdmir.Field{
			Provenance: l.provenance,
			Name:       field.Name,
			Type:       l.lowerTypeRef(field.Type),
		})
	}
	return board
}

func (l *lowering) lowerStream(name string, fields []ast.Field) vdmir.Stream {
	stream := vdmir.Stream{Provenance: l.provenance, Name: name}
	for _, field := range fields {
		if field.Access != "" {
			continue
		}
		stream.Fields = append(stream.Fields, vdmir.Field{
			Provenance: l.provenance,
			Name:       field.Name,
			Type:       l.lowerTypeRef(field.Type),
		})
	}
	return stream
}

func (l *lowering) resolveShaderResources(shader ast.ShaderDecl) ([]ast.ResourceDecl, string, error) {
	if shader.ResourceBundleName == "" {
		return append([]ast.ResourceDecl(nil), shader.Resources...), "", nil
	}
	info, ok := l.types[shader.ResourceBundleName]
	if !ok || info.kind != "stream" {
		return nil, "", fmt.Errorf("unknown stream resource bundle %s", shader.ResourceBundleName)
	}
	names := make([]string, 0, len(info.fields))
	for name := range info.fields {
		names = append(names, name)
	}
	sortStrings(names)
	resources := make([]ast.ResourceDecl, 0, len(names))
	for _, name := range names {
		field := info.fields[name]
		if field.access == "" {
			continue
		}
		resources = append(resources, ast.ResourceDecl{Name: name, Access: field.access, Type: field.typ})
	}
	return resources, shader.ResourceBundleName, nil
}

func (l *lowering) lowerFunction(shaderName string, fn ast.FunctionDecl, resources []ast.ResourceDecl, workgroups []ast.WorkgroupDecl) (vdmir.Function, error) {
	previousTestInput := l.currentTestInput
	previousFunction := l.currentFunction
	previousShader := l.currentShader
	l.currentFunction = fn.Name
	l.currentShader = shaderName
	if shaderName == "" && l.testInputs != nil {
		if input, ok := l.testInputs[fn.Name]; ok {
			copy := input
			l.currentTestInput = &copy
		} else {
			l.currentTestInput = nil
		}
	}
	defer func() {
		l.currentTestInput = previousTestInput
		l.currentFunction = previousFunction
		l.currentShader = previousShader
	}()
	out := vdmir.Function{
		Provenance: l.provenance,
		Name:       fn.Name,
		ShaderName: shaderName,
		ReturnType: l.lowerTypeRef(fn.ReturnType),
	}
	if shaderName != "" {
		out.EmittedName = shaderName + "_" + fn.Name
	} else {
		out.EmittedName = fn.Name
	}

	scope := map[string]binding{}
	addBuiltinBindings(scope)
	for _, resource := range resources {
		scope[resource.Name] = binding{name: resource.Name, kind: vdmir.VarResource, typ: resource.Type, access: resource.Access}
	}
	for _, workgroup := range workgroups {
		scope[workgroup.Name] = binding{name: workgroup.Name, kind: vdmir.VarLocal, typ: workgroup.Type}
	}
	for _, param := range fn.Parameters {
		scope[param.Name] = binding{name: param.Name, kind: vdmir.VarParam, typ: param.Type}
		out.Params = append(out.Params, vdmir.Parameter{
			Provenance: l.provenance,
			Name:       param.Name,
			Type:       l.lowerTypeRef(param.Type),
		})
	}

	locals := map[string]vdmir.Type{}
	body, err := l.lowerBlock(fn.Body, scope, locals, shaderName, fn.ReturnType)
	if err != nil {
		return vdmir.Function{}, err
	}
	out.Body = body
	out.Locals = collectLocals(locals, l.provenance)
	return out, nil
}

func (l *lowering) lowerBlock(block ast.Block, scope map[string]binding, locals map[string]vdmir.Type, shaderName string, returnType ast.TypeRef) (vdmir.Block, error) {
	out := vdmir.Block{}
	for _, stmt := range block.Statements {
		lowered, err := l.lowerStmt(stmt, scope, locals, shaderName, returnType)
		if err != nil {
			return vdmir.Block{}, err
		}
		out.Statements = append(out.Statements, lowered)
	}
	return out, nil
}

func (l *lowering) lowerStmt(stmt ast.Stmt, scope map[string]binding, locals map[string]vdmir.Type, shaderName string, returnType ast.TypeRef) (vdmir.Stmt, error) {
	switch s := stmt.(type) {
	case ast.ForeignShaderStmt:
		return vdmir.ForeignShaderStmt{Provenance: l.provenance, TargetLanguage: s.TargetLanguage, RawSource: s.RawSource, Captures: s.Captures, SourceLine: s.Line}, nil
	case ast.LetStmt:
		loweredType := l.lowerTypeRef(s.Type)
		var value vdmir.Expr
		var err error
		if isRegTileZeroCall(s.Value) {
			value = vdmir.RegTileZeroExpr{Provenance: l.provenance, ExprType: loweredType}
		} else if s.Value != nil {
			value, err = l.lowerExprWithExpected(s.Value, scope, shaderName, &s.Type)
			if err != nil {
				return nil, err
			}
		}
		access := s.Type.Access
		if rowMajor, ok := value.(vdmir.RowMajorViewExpr); ok {
			loweredType = rowMajor.Type()
			access = string(rowMajor.Access)
		}
		scope[s.Name] = binding{name: s.Name, kind: vdmir.VarLocal, typ: s.Type, access: access}
		locals[s.Name] = loweredType
		return vdmir.LetStmt{Provenance: l.provenance, Name: s.Name, Type: loweredType, Value: value}, nil
	case ast.AssignStmt:
		target, err := l.lowerExpr(s.Target, scope, shaderName)
		if err != nil {
			return nil, err
		}
		expected := astTypeFromVDMIR(target.Type())
		value, err := l.lowerExprWithExpected(s.Value, scope, shaderName, &expected)
		if err != nil {
			return nil, err
		}
		return vdmir.AssignStmt{Provenance: l.provenance, Target: target, Value: value}, nil
	case ast.TensorAssignStmt:
		return l.lowerTensorAssign(s, scope, shaderName)
	case ast.GuardedWriteStmt:
		target, err := l.lowerExpr(s.Target, scope, shaderName)
		if err != nil {
			return nil, err
		}
		value, err := l.lowerExpr(s.Value, scope, shaderName)
		if err != nil {
			return nil, err
		}
		condition, err := l.lowerExpr(s.Condition, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.GuardedWriteStmt{Provenance: l.provenance, Target: target, Value: value, Condition: condition}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return vdmir.ReturnStmt{Provenance: l.provenance}, nil
		}
		value, err := l.lowerExprWithExpected(s.Value, scope, shaderName, &returnType)
		if err != nil {
			return nil, err
		}
		return vdmir.ReturnStmt{Provenance: l.provenance, Value: value}, nil
	case ast.ExprStmt:
		value, err := l.lowerExpr(s.Value, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ExprStmt{Provenance: l.provenance, Value: value}, nil
	case ast.FlowStmt:
		return l.lowerFlowStmt(s, scope, locals, shaderName, returnType)
	case ast.IfStmt:
		cond, err := l.lowerExpr(s.Condition, scope, shaderName)
		if err != nil {
			return nil, err
		}
		thenBody, err := l.lowerBlock(s.ThenBody, cloneScope(scope), locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		var elseBody *vdmir.Block
		if s.ElseBody != nil {
			body, err := l.lowerBlock(*s.ElseBody, cloneScope(scope), locals, shaderName, returnType)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return vdmir.IfStmt{Provenance: l.provenance, Condition: cond, ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.GuardWhenStmt:
		return l.lowerGuardWhenStmt(s, scope, locals, shaderName, returnType)
	case ast.ForStmt:
		start, err := l.lowerExpr(s.Start, scope, shaderName)
		if err != nil {
			return nil, err
		}
		end, err := l.lowerExpr(s.End, scope, shaderName)
		if err != nil {
			return nil, err
		}
		step, err := l.lowerExpr(s.Step, scope, shaderName)
		if err != nil {
			return nil, err
		}
		loopType := start.Type()
		loopScope := cloneScope(scope)
		loopScope[s.Name] = binding{name: s.Name, kind: vdmir.VarLocal, typ: astTypeFromVDMIR(loopType)}
		locals[s.Name] = loopType
		body, err := l.lowerBlock(s.Body, loopScope, locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		return vdmir.ForRangeStmt{
			Provenance: l.provenance,
			LoopHint:   lowerLoopHint(s.Attributes),
			Name:       s.Name,
			Type:       loopType,
			Start:      start,
			End:        end,
			Step:       step,
			Body:       body,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported statement in GoOct SDSL-V M3")
	}
}

func (l *lowering) lowerTensorAssign(stmt ast.TensorAssignStmt, scope map[string]binding, shaderName string) (vdmir.Stmt, error) {
	meta, ok := l.tensorAssigns[stmt.Span]
	if !ok {
		return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: missing validated tensor metadata")
	}
	if len(meta.FreeIndices) == 0 || len(meta.FreeIndices) != len(stmt.FreeIndices) {
		return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: malformed free-index metadata")
	}
	loopScope := cloneScope(scope)
	indices := make([]vdmir.TensorIndex, 0, len(meta.FreeIndices))
	for i, index := range meta.FreeIndices {
		if index.Extent <= 0 {
			return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: missing free-index extent for %s", index.Name)
		}
		id := fmt.Sprintf("__sdslv_tensor_free_%d", i)
		loopScope[index.Name] = binding{name: id, kind: vdmir.VarLocal, typ: ast.TypeRef{Name: "u32"}}
		indices = append(indices, vdmir.TensorIndex{ID: id, Name: index.Name, Extent: uint32(index.Extent), Span: index.Span})
	}
	destination, err := l.lowerExpr(stmt.Destination, loopScope, shaderName)
	if err != nil {
		return nil, err
	}
	value, err := l.lowerTensorExpr(meta.Value, loopScope, shaderName)
	if err != nil {
		return nil, err
	}
	kind := vdmir.TensorAssignSet
	if meta.AssignmentKind == ast.TensorAssignAdd {
		kind = vdmir.TensorAssignAdd
	}
	alias := vdmir.TensorAliasNoDestinationRead
	if meta.AliasPolicy == "identical-index-read" {
		alias = vdmir.TensorAliasIdenticalRead
	} else if meta.AliasPolicy != "no-destination-read" {
		return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: malformed alias policy %q", meta.AliasPolicy)
	}
	return vdmir.TensorAssign{Provenance: l.provenance, Destination: destination, Value: value, ElementType: l.lowerTypeRef(meta.DestinationElementType), AssignmentKind: kind, FreeIndices: indices, AliasPolicy: alias, SourceSpan: meta.Span, LoopOrder: append([]string(nil), meta.LoopOrder...)}, nil
}

func (l *lowering) lowerTensorExpr(expr ast.Expr, scope map[string]binding, shaderName string) (vdmir.Expr, error) {
	if reduction, ok := expr.(ast.TensorReductionExpr); ok {
		meta, exists := l.tensorReductions[reduction.Span]
		if !exists || meta.Kind != "Sum" || len(meta.Indices) == 0 {
			return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: malformed reduction metadata")
		}
		reductionScope := cloneScope(scope)
		indices := make([]vdmir.TensorIndex, 0, len(meta.Indices))
		for i, index := range meta.Indices {
			if index.Extent <= 0 {
				return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: missing reduction extent for %s", index.Name)
			}
			id := fmt.Sprintf("__sdslv_tensor_reduce_%d", i)
			reductionScope[index.Name] = binding{name: id, kind: vdmir.VarLocal, typ: ast.TypeRef{Name: "u32"}}
			indices = append(indices, vdmir.TensorIndex{ID: id, Name: index.Name, Extent: uint32(index.Extent), Span: index.Span})
		}
		body, err := l.lowerTensorExpr(meta.Value, reductionScope, shaderName)
		if err != nil {
			return nil, err
		}
		typ := l.lowerTypeRef(meta.ResultType)
		return vdmir.TensorReductionExpr{Provenance: l.provenance, ExprType: typ, Kind: meta.Kind, Indices: indices, Body: body, Identity: tensorZero(typ), Span: meta.Span}, nil
	}
	switch e := expr.(type) {
	case ast.BinaryExpr:
		left, err := l.lowerTensorExpr(e.Left, scope, shaderName)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerTensorExpr(e.Right, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.BinaryExpr{Provenance: l.provenance, ExprType: binaryResultType(left.Type(), lowerBinaryOperator(e.Operator), right.Type()), Left: left, Operator: lowerBinaryOperator(e.Operator), Right: right}, nil
	case ast.ParenExpr:
		return l.lowerTensorExpr(e.Inner, scope, shaderName)
	default:
		return l.lowerExpr(expr, scope, shaderName)
	}
}

func tensorZero(typ vdmir.Type) vdmir.Expr {
	value := "0"
	if typ.Kind == vdmir.TypeF32 {
		value = "0.0"
	}
	if typ.Kind == vdmir.TypeU32 {
		value = "0u"
	}
	return vdmir.LiteralExpr{ExprType: typ, Kind: vdmir.LiteralInteger, Value: value}
}

func (l *lowering) lowerFlowStmt(stmt ast.FlowStmt, scope map[string]binding, locals map[string]vdmir.Type, shaderName string, returnType ast.TypeRef) (vdmir.Stmt, error) {
	flow, issues := validate.ValidateFlow(stmt)
	if len(issues) != 0 {
		return nil, fmt.Errorf("flow %s has invalid M31a metadata", stmt.Name)
	}
	block := vdmir.Block{}
	flowScope := cloneScope(scope)
	for _, board := range stmt.Boards {
		value, err := l.lowerExprWithExpected(board.Initializer, flowScope, shaderName, &board.Type)
		if err != nil {
			return nil, err
		}
		loweredType := l.lowerTypeRef(board.Type)
		flowScope[board.Name] = binding{name: board.Name, kind: vdmir.VarLocal, typ: board.Type}
		locals[board.Name] = loweredType
		block.Statements = append(block.Statements, vdmir.LetStmt{
			Provenance: l.provenance,
			Name:       board.Name,
			Type:       loweredType,
			Value:      value,
		})
	}
	if flow.HasPushPop || flow.HasGoto || hasFlowFinish(flow) {
		lowered, err := l.lowerValidatedFlow(stmt, flow, flowScope, locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		l.flows = append(l.flows, lowered)
		block.Statements = append(block.Statements, vdmir.FlowStmt{Provenance: l.provenance, Flow: lowered})
		return vdmir.BlockStmt{Provenance: l.provenance, Body: block}, nil
	}
	l.flows = append(l.flows, l.lowerValidatedFlowMetadata(stmt, flow))
	for _, state := range stmt.States {
		lowered, err := l.lowerBlock(state.Body, cloneScope(flowScope), locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, lowered.Statements...)
	}
	return vdmir.BlockStmt{Provenance: l.provenance, Body: block}, nil
}

func hasFlowFinish(flow validate.ValidatedFlow) bool {
	for _, state := range flow.States {
		if state.Terminator.Kind == validate.FlowFinish {
			return true
		}
	}
	return false
}

func (l *lowering) lowerValidatedFlowMetadata(stmt ast.FlowStmt, flow validate.ValidatedFlow) vdmir.Flow {
	out := vdmir.Flow{
		Provenance:    l.provenance,
		Name:          flow.Name,
		FunctionName:  l.currentFunction,
		ShaderName:    l.currentShader,
		Entry:         flow.Entry,
		MaxStackDepth: flow.MaxStackDepth,
		HasPushPop:    flow.HasPushPop,
		HasGoto:       flow.HasGoto,
		SourceSpan:    stmt.Span,
	}
	out.States = make([]vdmir.FlowState, 0, len(flow.States))
	for _, state := range flow.States {
		out.States = append(out.States, vdmir.FlowState{
			ID:                  state.ID,
			Name:                state.Name,
			Terminator:          lowerFlowTerminator(state.Terminator),
			HasWorkgroupBarrier: state.HasWorkgroupBarrier,
			Reachable:           state.Reachable,
			ReachableDepths:     append([]uint32(nil), state.ReachableDepths...),
			SourceSpan:          state.SourceSpan,
			NameSpan:            state.NameSpan,
		})
	}
	return out
}

func (l *lowering) lowerValidatedFlow(stmt ast.FlowStmt, flow validate.ValidatedFlow, scope map[string]binding, locals map[string]vdmir.Type, shaderName string, returnType ast.TypeRef) (vdmir.Flow, error) {
	out := l.lowerValidatedFlowMetadata(stmt, flow)
	for i, state := range flow.States {
		body, err := l.lowerBlock(ast.Block{Statements: state.Statements}, cloneScope(scope), locals, shaderName, returnType)
		if err != nil {
			return vdmir.Flow{}, err
		}
		out.States[i].Body = body
	}
	return out, nil
}

func lowerFlowTerminator(term validate.ValidatedFlowTerminator) vdmir.FlowTerminator {
	return vdmir.FlowTerminator{
		Kind:     vdmir.FlowTerminatorKind(term.Kind),
		Target:   term.Target,
		ReturnTo: term.ReturnTo,
		Span:     term.Span,
	}
}

func (l *lowering) lowerGuardWhenStmt(stmt ast.GuardWhenStmt, scope map[string]binding, locals map[string]vdmir.Type, shaderName string, returnType ast.TypeRef) (vdmir.Stmt, error) {
	var currentElse *vdmir.Block
	if stmt.ElseBody != nil {
		body, err := l.lowerBlock(*stmt.ElseBody, cloneBindings(scope), locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		currentElse = &body
	}
	for i := len(stmt.Cases) - 1; i >= 0; i-- {
		c := stmt.Cases[i]
		condition, err := l.lowerExpr(c.Condition, scope, shaderName)
		if err != nil {
			return nil, err
		}
		body, err := l.lowerBlock(c.Body, cloneBindings(scope), locals, shaderName, returnType)
		if err != nil {
			return nil, err
		}
		next := vdmir.IfStmt{Provenance: l.provenance, Condition: condition, ThenBody: body, ElseBody: currentElse}
		currentElse = &vdmir.Block{Statements: []vdmir.Stmt{next}}
	}
	if currentElse == nil || len(currentElse.Statements) == 0 {
		return nil, fmt.Errorf("guard when requires at least one case")
	}
	return currentElse.Statements[0], nil
}

func (l *lowering) lowerExpr(expr ast.Expr, scope map[string]binding, shaderName string) (vdmir.Expr, error) {
	return l.lowerExprWithExpected(expr, scope, shaderName, nil)
}

func (l *lowering) lowerExprWithExpected(expr ast.Expr, scope map[string]binding, shaderName string, expected *ast.TypeRef) (vdmir.Expr, error) {
	switch e := expr.(type) {
	case ast.ArrayLiteral:
		if expected == nil || expected.Name != "ndarray" {
			return nil, fmt.Errorf("ndarray literal requires ndarray target type")
		}
		elements := make([]vdmir.Expr, 0, len(e.Elements))
		for _, element := range e.Elements {
			value, err := l.lowerExprWithExpected(element, scope, shaderName, &expected.Args[0])
			if err != nil {
				return nil, err
			}
			elements = append(elements, value)
		}
		return vdmir.NDArrayLiteral{Provenance: l.provenance, ExprType: l.lowerTypeRef(*expected), Elements: elements}, nil
	case ast.ForeignShaderExpr:
		return vdmir.ForeignShaderExpr{Provenance: l.provenance, ExprType: l.lowerTypeRef(l.resolveAlias(e.ResultType)), TargetLanguage: e.TargetLanguage, RawSource: e.RawSource, Captures: e.Captures, SourceLine: e.Line}, nil
	case ast.IntegerLiteral:
		typ := vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			typ = vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
		}
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: typ, Kind: vdmir.LiteralInteger, Value: e.Value}, nil
	case ast.FloatLiteral:
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}, Kind: vdmir.LiteralFloat, Value: e.Value}, nil
	case ast.BoolLiteral:
		value := "false"
		if e.Value {
			value = "true"
		}
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}, Kind: vdmir.LiteralBool, Value: value}, nil
	case ast.StringLiteral:
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeBuiltin, Name: "string"}, Kind: vdmir.LiteralString, Value: fmt.Sprintf("%q", e.Value)}, nil
	case ast.IdentifierExpr:
		if e.Name == "TestInput" {
			return nil, fmt.Errorf("TestInput is not a first-class value")
		}
		if b, ok := scope[e.Name]; ok {
			return vdmir.VarRefExpr{Provenance: l.provenance, ExprType: l.lowerTypeRef(l.resolveAlias(b.typ)), Name: b.name, Kind: b.kind}, nil
		}
		if fn, ok := l.lookupFunction(shaderName, e.Name); ok {
			return vdmir.VarRefExpr{Provenance: l.provenance, ExprType: l.lowerTypeRef(fn.returnType), Name: fn.emittedName, Kind: vdmir.VarFunction}, nil
		}
		return nil, fmt.Errorf("unknown identifier %s", e.Name)
	case ast.FieldAccessExpr:
		if root, ok := e.Target.(ast.IdentifierExpr); ok && root.Name == "TestInput" {
			return l.lowerTestInputFieldAccess(e)
		}
		// Assert is a validator-recognized test intrinsic namespace.  Preserve it
		// as a first-class call shape in VD-MIR; the test backend gives it its
		// compiler-owned failure-state semantics rather than treating it as a user
		// function or requiring a second parse of the test source.
		if id, ok := e.Target.(ast.IdentifierExpr); ok && id.Name == "Assert" {
			root := vdmir.VarRefExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}, Name: "Assert", Kind: vdmir.VarFunction}
			return vdmir.FieldAccessExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}, Target: root, Field: e.Field}, nil
		}
		if id, ok := e.Target.(ast.IdentifierExpr); ok {
			if enumInfo, exists := l.types[id.Name]; exists && enumInfo.kind == "enum" {
				return vdmir.EnumConstructExpr{
					Provenance:  l.provenance,
					ExprType:    l.lowerTypeRef(ast.TypeRef{Name: id.Name}),
					EnumName:    id.Name,
					VariantName: e.Field,
				}, nil
			}
		}
		target, err := l.lowerExprWithExpected(e.Target, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		return vdmir.FieldAccessExpr{
			Provenance: l.provenance,
			ExprType:   l.lowerTypeRef(l.fieldType(target.Type(), e.Field)),
			Target:     target,
			Field:      e.Field,
		}, nil
	case ast.IndexExpr:
		if field, ok := e.Target.(ast.FieldAccessExpr); ok {
			if root, ok := field.Target.(ast.IdentifierExpr); ok && root.Name == "TestInput" {
				return l.lowerTestInputIndex(field, e, scope, shaderName)
			}
		}
		target, err := l.lowerExprWithExpected(e.Target, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		all := ast.IndexExpressions(e)
		if target.Type().Kind == vdmir.TypeArray || target.Type().Kind == vdmir.TypeNDArray {
			indices := make([]vdmir.Expr, 0, len(all))
			for _, axis := range all {
				lowered, err := l.lowerExprWithExpected(axis, scope, shaderName, nil)
				if err != nil {
					return nil, err
				}
				indices = append(indices, lowered)
			}
			typ := target.Type()
			extents := make([]uint32, 0, len(all))
			if typ.Kind == vdmir.TypeNDArray {
				// ndarray already owns a flat physical extent; tensor metadata
				// supplies rank validation while lowering only needs total storage.
				if len(indices) != len(typ.Shape) {
					return nil, fmt.Errorf("ndarray rank metadata does not match index count")
				}
				extents = append(extents, typ.Shape...)
				typ = elementType(typ)
			} else {
				for range indices {
					if typ.Kind != vdmir.TypeArray || !typ.HasArraySize || typ.ArraySize <= 0 {
						return nil, fmt.Errorf("SDSL-V M32b.1 compiler-boundary error: invalid fixed-array linearization metadata")
					}
					extents = append(extents, uint32(typ.ArraySize))
					typ = elementType(typ)
				}
			}
			if typ.Name == "" {
				return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: invalid rank-general indexing metadata")
			}
			return vdmir.IndexNExpr{Provenance: l.provenance, ExprType: typ, Target: target, Indices: indices, Extents: extents, Layout: vdmir.IndexLayoutRowMajorLinear}, nil
		}
		if len(all) == 0 {
			return nil, fmt.Errorf("SDSL-V M32b compiler-boundary error: empty index tuple")
		}
		index, err := l.lowerExprWithExpected(all[0], scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		if len(all) == 2 {
			index2, err := l.lowerExprWithExpected(all[1], scope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			return vdmir.Index2DExpr{
				Provenance: l.provenance,
				ExprType:   elementType(target.Type()),
				Target:     target,
				Row:        index,
				Col:        index2,
				Stride:     tileOrViewStride(target.Type()),
			}, nil
		}
		return vdmir.IndexExpr{
			Provenance: l.provenance,
			ExprType:   elementType(target.Type()),
			Target:     target,
			Index:      index,
		}, nil
	case ast.GuardedReadExpr:
		target, err := l.lowerExprWithExpected(e.Target, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		condition, err := l.lowerExprWithExpected(e.Condition, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		fallback, err := l.lowerExprWithExpected(e.Fallback, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		return vdmir.GuardedReadExpr{
			Provenance: l.provenance,
			ExprType:   target.Type(),
			Target:     target,
			Condition:  condition,
			Fallback:   fallback,
		}, nil
	case ast.CallExpr:
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && id.Name == "reg_tile_zero" {
			return nil, fmt.Errorf("reg_tile_zero() is only supported as a direct reg_tile local initializer")
		}
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && id.Name == "row_major" {
			args := make([]vdmir.Expr, 0, len(e.Arguments))
			for _, arg := range e.Arguments {
				lowered, err := l.lowerExprWithExpected(arg, scope, shaderName, nil)
				if err != nil {
					return nil, err
				}
				args = append(args, lowered)
			}
			access := vdmir.ResourceReadOnly
			if ref, ok := args[0].(vdmir.VarRefExpr); ok {
				if b, exists := scope[ref.Name]; exists && b.access == "readwrite" {
					access = vdmir.ResourceReadWrite
				}
			}
			elem := elementType(args[0].Type())
			viewType := vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: access}
			return vdmir.RowMajorViewExpr{
				Provenance: l.provenance,
				ExprType:   viewType,
				Buffer:     args[0],
				Rows:       args[1],
				Cols:       args[2],
				Access:     access,
			}, nil
		}
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && isBarrierBuiltin(id.Name) {
			args := make([]vdmir.Expr, 0, len(e.Arguments))
			for _, arg := range e.Arguments {
				lowered, err := l.lowerExprWithExpected(arg, scope, shaderName, nil)
				if err != nil {
					return nil, err
				}
				args = append(args, lowered)
			}
			return vdmir.IntrinsicCallExpr{
				Provenance: l.provenance,
				ExprType:   vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"},
				Intrinsic:  lowerIntrinsic(id.Name),
				Arguments:  args,
			}, nil
		}
		callee, err := l.lowerExprWithExpected(e.Callee, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		args := make([]vdmir.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			lowered, err := l.lowerExprWithExpected(arg, scope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			args = append(args, lowered)
		}
		return vdmir.CallExpr{
			Provenance: l.provenance,
			ExprType:   l.callResultType(e, scope, shaderName),
			Callee:     callee,
			Arguments:  args,
		}, nil
	case ast.BinaryExpr:
		left, err := l.lowerExprWithExpected(e.Left, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExprWithExpected(e.Right, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		return vdmir.BinaryExpr{
			Provenance: l.provenance,
			ExprType:   binaryResultType(left.Type(), lowerBinaryOperator(e.Operator), right.Type()),
			Left:       left,
			Operator:   lowerBinaryOperator(e.Operator),
			Right:      right,
		}, nil
	case ast.UnaryExpr:
		operand, err := l.lowerExprWithExpected(e.Operand, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		return vdmir.UnaryExpr{
			Provenance: l.provenance,
			ExprType:   operand.Type(),
			Operator:   lowerUnaryOperator(e.Operator),
			Operand:    operand,
		}, nil
	case ast.ParenExpr:
		return l.lowerExprWithExpected(e.Inner, scope, shaderName, nil)
	case ast.EnumConstructExpr:
		fields := make([]vdmir.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			value, err := l.lowerExprWithExpected(field.Value, scope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			fields = append(fields, vdmir.FieldInit{Name: field.Name, Value: value})
		}
		return vdmir.EnumConstructExpr{
			Provenance:  l.provenance,
			ExprType:    l.lowerTypeRef(ast.TypeRef{Name: e.EnumName}),
			EnumName:    e.EnumName,
			VariantName: e.VariantName,
			Fields:      fields,
		}, nil
	case ast.BoardLiteralExpr:
		fields := make([]vdmir.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			value, err := l.lowerExprWithExpected(field.Value, scope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			fields = append(fields, vdmir.FieldInit{Name: field.Name, Value: value})
		}
		return vdmir.BoardConstructExpr{
			Provenance: l.provenance,
			ExprType:   l.lowerTypeRef(ast.TypeRef{Name: e.TypeName}),
			TypeName:   e.TypeName,
			Fields:     fields,
		}, nil
	case ast.DeriveExpr:
		if expected == nil {
			return nil, fmt.Errorf("derive requires an explicit record or board target type")
		}
		target := l.resolveAlias(*expected)
		info, ok := l.types[target.Name]
		if !ok || (info.kind != "record" && info.kind != "board") {
			return nil, fmt.Errorf("derive requires an explicit record or board target type")
		}
		deriveScope := cloneBindings(scope)
		fields := make([]vdmir.DeriveField, 0, len(e.Fields))
		for _, field := range e.Fields {
			tempName := l.nextDeriveTemp(field.Name)
			value, err := l.lowerExprWithExpected(field.Value, deriveScope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			fields = append(fields, vdmir.DeriveField{Name: field.Name, TempName: tempName, Value: value})
			if decl, ok := info.fields[field.Name]; ok {
				deriveScope[field.Name] = binding{name: tempName, kind: vdmir.VarLocal, typ: decl.typ}
			}
		}
		return vdmir.DeriveExpr{
			Provenance: l.provenance,
			ExprType:   l.lowerTypeRef(target),
			TypeName:   target.Name,
			Fields:     fields,
		}, nil
	case ast.MatchExpr:
		subject, err := l.lowerExprWithExpected(e.Subject, scope, shaderName, nil)
		if err != nil {
			return nil, err
		}
		enumInfo := l.types[subject.Type().Name]
		arms := make([]vdmir.MatchArm, 0, len(e.Arms))
		var resultType vdmir.Type
		for i, arm := range e.Arms {
			variant := enumInfo.enumVariants[arm.VariantName]
			armScope := cloneScope(scope)
			bindingType := vdmir.Type{}
			if arm.BindingName != "" {
				bindingType = l.lowerTypeRef(ast.TypeRef{Name: variant.payloadType})
				armScope[arm.BindingName] = binding{name: arm.BindingName, kind: vdmir.VarLocal, typ: ast.TypeRef{Name: variant.payloadType}}
			}
			value, err := l.lowerExprWithExpected(arm.Value, armScope, shaderName, nil)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				resultType = value.Type()
			}
			arms = append(arms, vdmir.MatchArm{
				EnumName:     arm.EnumName,
				VariantName:  arm.VariantName,
				BindingName:  arm.BindingName,
				BindingType:  bindingType,
				VariantIndex: variant.variantIndex,
				Value:        value,
			})
		}
		return vdmir.MatchExpr{
			Provenance: l.provenance,
			ExprType:   resultType,
			Subject:    subject,
			Arms:       arms,
		}, nil
	case ast.WhenUtilityExpr:
		cases := make([]vdmir.WhenUtilityCase, 0, len(e.Cases))
		var resultType vdmir.Type
		for i, c := range e.Cases {
			value, err := l.lowerExpr(c.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
			guard, err := l.lowerExpr(c.Condition, scope, shaderName)
			if err != nil {
				return nil, err
			}
			score, err := l.lowerExpr(c.Score, scope, shaderName)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				resultType = value.Type()
			}
			cases = append(cases, vdmir.WhenUtilityCase{
				Provenance: l.provenance,
				Value:      value,
				Guard:      guard,
				Score:      score,
			})
		}
		elseExpr, err := l.lowerExpr(e.Else, scope, shaderName)
		if err != nil {
			return nil, err
		}
		if resultType.Kind == "" {
			resultType = elseExpr.Type()
		}
		return vdmir.WhenUtilityExpr{
			Provenance: l.provenance,
			ExprType:   resultType,
			Cases:      cases,
			Else:       elseExpr,
		}, nil
	case ast.WithExpr:
		base, err := l.lowerExpr(e.Base, scope, shaderName)
		if err != nil {
			return nil, err
		}
		updates := make([]vdmir.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			value, err := l.lowerExpr(update.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
			updates = append(updates, vdmir.FieldUpdate{Name: update.Name, Value: value})
		}
		return vdmir.WithExpr{
			Provenance: l.provenance,
			ExprType:   base.Type(),
			Base:       base,
			Updates:    updates,
		}, nil
	case ast.ReductionExpr:
		start, err := l.lowerExpr(e.Start, scope, shaderName)
		if err != nil {
			return nil, err
		}
		end, err := l.lowerExpr(e.End, scope, shaderName)
		if err != nil {
			return nil, err
		}
		step, err := l.lowerExpr(e.Step, scope, shaderName)
		if err != nil {
			return nil, err
		}
		bodyScope := cloneScope(scope)
		bodyScope[e.Name] = binding{name: e.Name, kind: vdmir.VarLocal, typ: astTypeFromVDMIR(start.Type())}
		body, err := l.lowerExpr(e.Body, bodyScope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ReductionExpr{
			Provenance: l.provenance,
			ExprType:   body.Type(),
			LoopHint:   lowerLoopHint(e.Attributes),
			Op:         lowerReductionOp(e.Op),
			Name:       e.Name,
			IndexType:  start.Type(),
			Start:      start,
			End:        end,
			Step:       step,
			Body:       body,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported expression in GoOct SDSL-V M3")
	}
}

func (l *lowering) nextDeriveTemp(fieldName string) string {
	name := fmt.Sprintf("__sdslv_derive_%s_%d", sanitizeName(fieldName), l.deriveTemp)
	l.deriveTemp++
	return name
}

func (l *lowering) lowerComputeEntryPoint(shader ast.ShaderDecl, fn ast.FunctionDecl, resources []ast.ResourceDecl) vdmir.ComputeEntryPoint {
	entry := vdmir.ComputeEntryPoint{
		Provenance:   l.provenance,
		ShaderName:   shader.Name,
		FunctionName: fn.Name,
		EmittedName:  shader.Name + "_" + fn.Name,
		NumThreadsX:  mustConcreteInt(fn.NumThreads.X),
		NumThreadsY:  mustConcreteInt(fn.NumThreads.Y),
		NumThreadsZ:  mustConcreteInt(fn.NumThreads.Z),
		Metadata:     lowerEntryMetadata(shader.SpecializedConfig),
		ConfigValues: lowerConfigValues(shader.SpecializedConfig),
	}
	directBuiltins := referencedBuiltins(fn.Body)
	threadParams := l.computeThreadBindings(fn.Parameters)
	for _, param := range fn.Parameters {
		if binding, ok := computeThreadBindingForParam(param); ok {
			entry.ThreadParams = append(entry.ThreadParams, binding)
			continue
		}
		entry.Params = append(entry.Params, vdmir.Parameter{
			Provenance: l.provenance,
			Name:       param.Name,
			Type:       l.lowerTypeRef(param.Type),
		})
	}
	entry.Builtins = []vdmir.BuiltinParam{
		{Name: "DispatchThreadID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_DispatchThreadID", Builtin: vdmir.BuiltinDispatchThreadID, Available: true, Referenced: directBuiltins["DispatchThreadID"] || threadParams["DispatchThreadID"]},
		{Name: "GroupThreadID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_GroupThreadID", Builtin: vdmir.BuiltinGroupThreadID, Available: true, Referenced: directBuiltins["GroupThreadID"] || threadParams["GroupThreadID"]},
		{Name: "GroupID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_GroupID", Builtin: vdmir.BuiltinGroupID, Available: true, Referenced: directBuiltins["GroupID"] || threadParams["GroupID"]},
		{Name: "GroupIndex", Type: vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}, Semantic: "SV_GroupIndex", Builtin: vdmir.BuiltinGroupIndex, Available: true, Referenced: directBuiltins["GroupIndex"] || threadParams["GroupIndex"]},
	}
	_ = resources
	return entry
}

func lowerEntryMetadata(config map[string]uint32) []vdmir.MetadataField {
	if len(config) == 0 {
		return nil
	}
	names := []string{
		"OUTPUTS_PER_INVOCATION_M",
		"OUTPUTS_PER_INVOCATION_N",
		"TILE_M",
		"TILE_N",
		"TILE_K",
		"UNROLL_K",
	}
	fields := make([]vdmir.MetadataField, 0, len(names))
	for _, name := range names {
		value, ok := config[name]
		if !ok {
			continue
		}
		fields = append(fields, vdmir.MetadataField{Name: name, Value: value})
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func lowerConfigValues(config map[string]uint32) []vdmir.MetadataField {
	if len(config) == 0 {
		return nil
	}
	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	slices.Sort(names)
	fields := make([]vdmir.MetadataField, 0, len(names))
	for _, name := range names {
		fields = append(fields, vdmir.MetadataField{Name: name, Value: config[name]})
	}
	return fields
}

func (l *lowering) computeThreadBindings(params []ast.Parameter) map[string]bool {
	out := map[string]bool{}
	for _, param := range params {
		binding, ok := computeThreadBindingForParam(param)
		if !ok {
			continue
		}
		for _, field := range binding.Fields {
			out[field.BuiltinName] = true
		}
	}
	return out
}

func computeThreadBindingForParam(param ast.Parameter) (vdmir.ComputeThreadBinding, bool) {
	if param.Type.Name != "ComputeThread" {
		return vdmir.ComputeThreadBinding{}, false
	}
	return vdmir.ComputeThreadBinding{
		ParamName: param.Name,
		TypeName:  "ComputeThread",
		Fields: []vdmir.ComputeThreadFieldBinding{
			{FieldName: "DispatchId", BuiltinName: "DispatchThreadID"},
			{FieldName: "GroupId", BuiltinName: "GroupID"},
			{FieldName: "GroupThreadId", BuiltinName: "GroupThreadID"},
			{FieldName: "GroupIndex", BuiltinName: "GroupIndex"},
		},
	}, true
}

func (l *lowering) lowerTypeRef(ref ast.TypeRef) vdmir.Type {
	resolved := l.resolveAlias(ref)
	switch resolved.Name {
	case "void":
		return vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}
	case "bool":
		return vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
	case "i32":
		return vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
	case "u32":
		return vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
	case "f32", "float":
		return vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
	case "uint2":
		return vdmir.Type{Kind: vdmir.TypeUint2, Name: "uint2"}
	case "uint3":
		return vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}
	case "uint4":
		return vdmir.Type{Kind: vdmir.TypeUint4, Name: "uint4"}
	case "float2":
		return vdmir.Type{Kind: vdmir.TypeFloat2, Name: "float2"}
	case "float3":
		return vdmir.Type{Kind: vdmir.TypeFloat3, Name: "float3"}
	case "float4":
		return vdmir.Type{Kind: vdmir.TypeFloat4, Name: "float4"}
	case "array":
		elem := l.lowerTypeRef(resolved.Args[0])
		if resolved.HasArraySize {
			return vdmir.Type{Kind: vdmir.TypeArray, Name: "array", Element: &elem, ArraySize: mustConcreteInt(resolved.ArraySize), HasArraySize: true}
		}
		return vdmir.Type{Kind: vdmir.TypeRuntimeArray, Name: "array", Element: &elem}
	case "ndarray":
		elem := l.lowerTypeRef(resolved.Args[0])
		count := 1
		shape := make([]uint32, 0, len(resolved.NDArrayShape))
		for _, extent := range resolved.NDArrayShape {
			n := mustConcreteInt(extent)
			count *= n
			shape = append(shape, uint32(n))
		}
		return vdmir.Type{Kind: vdmir.TypeNDArray, Name: "ndarray", Element: &elem, ArraySize: count, HasArraySize: true, Shape: shape}
	case "tile":
		elem := l.lowerTypeRef(resolved.Args[0])
		rows := mustConcreteInt(resolved.TileRows)
		cols := mustConcreteInt(resolved.TileCols)
		return vdmir.Type{Kind: vdmir.TypeTile, Name: "tile", Element: &elem, Rows: rows, Cols: cols, ArraySize: rows * cols, HasArraySize: true}
	case "reg_tile":
		elem := l.lowerTypeRef(resolved.Args[0])
		rows := mustConcreteInt(resolved.TileRows)
		cols := mustConcreteInt(resolved.TileCols)
		return vdmir.Type{Kind: vdmir.TypeRegTile, Name: "reg_tile", Element: &elem, Rows: rows, Cols: cols, ArraySize: rows * cols, HasArraySize: true}
	case "matrix_view":
		elem := l.lowerTypeRef(resolved.Args[0])
		return vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: lowerResourceAccess(resolved.Access)}
	default:
		if info, ok := l.types[resolved.Name]; ok {
			switch info.kind {
			case "record":
				return vdmir.Type{Kind: vdmir.TypeRecord, Name: resolved.Name}
			case "board":
				return vdmir.Type{Kind: vdmir.TypeBoard, Name: resolved.Name}
			case "stream":
				return vdmir.Type{Kind: vdmir.TypeStream, Name: resolved.Name}
			case "enum":
				return vdmir.Type{Kind: vdmir.TypeEnum, Name: resolved.Name}
			}
		}
		return vdmir.Type{Kind: vdmir.TypeBuiltin, Name: resolved.Name}
	}
}

func (l *lowering) lowerResourceElementType(ref ast.TypeRef) vdmir.Type {
	if len(ref.Args) == 0 {
		return vdmir.Type{}
	}
	return l.lowerTypeRef(ref.Args[0])
}

func (l *lowering) resolveAlias(ref ast.TypeRef) ast.TypeRef {
	info, ok := l.types[ref.Name]
	if !ok || info.kind != "alias" {
		return ref
	}
	return l.resolveAlias(info.target)
}

func (l *lowering) fieldType(target vdmir.Type, field string) ast.TypeRef {
	if target.Name == "uint2" || target.Name == "uint3" || target.Name == "uint4" {
		return ast.TypeRef{Name: "u32"}
	}
	info, ok := l.types[target.Name]
	if !ok || info.fields == nil {
		return ast.TypeRef{Name: "<error>"}
	}
	return l.resolveAlias(info.fields[field].typ)
}

func (l *lowering) lookupFunction(shaderName, name string) (functionInfo, bool) {
	if shaderName != "" {
		if info, ok := l.functions[shaderName+"_"+name]; ok {
			return info, true
		}
	}
	info, ok := l.functions[name]
	return info, ok
}

func (l *lowering) callResultType(call ast.CallExpr, scope map[string]binding, shaderName string) vdmir.Type {
	if id, ok := call.Callee.(ast.IdentifierExpr); ok {
		switch id.Name {
		case "float2", "float3", "float4", "uint2", "uint3", "uint4":
			return l.lowerTypeRef(ast.TypeRef{Name: id.Name})
		case "WorkgroupBarrier", "WorkgroupMemoryBarrier", "WorkgroupMemoryBarrierWithSync":
			return vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}
		case "row_major":
			if len(call.Arguments) == 3 {
				if first, ok := call.Arguments[0].(ast.IdentifierExpr); ok {
					if b, exists := scope[first.Name]; exists {
						elem := elementType(l.lowerTypeRef(b.typ))
						return vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: lowerResourceAccess(b.access)}
					}
				}
			}
		}
		if info, ok := l.lookupFunction(shaderName, id.Name); ok {
			return l.lowerTypeRef(info.returnType)
		}
	}
	return vdmir.Type{}
}

func (l *lowering) lowerTestInputFieldAccess(expr ast.FieldAccessExpr) (vdmir.Expr, error) {
	if expr.Field == "Length" {
		if l.currentTestInput == nil {
			return nil, fmt.Errorf("TestInput.Length requires declared test input")
		}
		return vdmir.LiteralExpr{
			Provenance: l.provenance,
			ExprType:   vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"},
			Kind:       vdmir.LiteralInteger,
			Value:      fmt.Sprintf("%du", l.currentTestInput.ElementCount),
		}, nil
	}
	return nil, fmt.Errorf("TestInput.%s must be indexed", expr.Field)
}

func (l *lowering) lowerTestInputIndex(field ast.FieldAccessExpr, expr ast.IndexExpr, scope map[string]binding, shaderName string) (vdmir.Expr, error) {
	if l.currentTestInput == nil {
		return nil, fmt.Errorf("TestInput access requires declared test input")
	}
	index, err := l.lowerExprWithExpected(expr.Index, scope, shaderName, nil)
	if err != nil {
		return nil, err
	}
	elementType := vdmir.Type{}
	switch l.currentTestInput.Kind {
	case validate.TestInputKindBool:
		elementType = vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
	case validate.TestInputKindInt:
		elementType = vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
	case validate.TestInputKindUInt:
		elementType = vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
	case validate.TestInputKindFloat:
		elementType = vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
	default:
		return nil, fmt.Errorf("TestInput access requires declared test input")
	}
	return vdmir.IndexExpr{
		Provenance: l.provenance,
		ExprType:   elementType,
		Target: vdmir.VarRefExpr{
			Provenance: l.provenance,
			ExprType:   vdmir.Type{Kind: vdmir.TypeRuntimeArray, Name: "array", Element: &vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}},
			Name:       vdmir.TestInputResourceName,
			Kind:       vdmir.VarResource,
		},
		Index: index,
	}, nil
}

func addBuiltinBindings(scope map[string]binding) {
	scope["DispatchThreadID"] = binding{name: "DispatchThreadID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupThreadID"] = binding{name: "GroupThreadID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupID"] = binding{name: "GroupID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupIndex"] = binding{name: "GroupIndex", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "u32"}}
}

func collectLocals(locals map[string]vdmir.Type, provenance vdmir.Provenance) []vdmir.Local {
	names := make([]string, 0, len(locals))
	for name := range locals {
		names = append(names, name)
	}
	sortStrings(names)
	out := make([]vdmir.Local, 0, len(names))
	for _, name := range names {
		out = append(out, vdmir.Local{Provenance: provenance, Name: name, Type: locals[name]})
	}
	return out
}

func referencedBuiltins(block ast.Block) map[string]bool {
	out := map[string]bool{}
	for _, stmt := range block.Statements {
		walkStmt(stmt, out)
	}
	return out
}

func walkStmt(stmt ast.Stmt, builtins map[string]bool) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if s.Value != nil {
			walkExpr(s.Value, builtins)
		}
	case ast.AssignStmt:
		walkExpr(s.Target, builtins)
		walkExpr(s.Value, builtins)
	case ast.GuardedWriteStmt:
		walkExpr(s.Target, builtins)
		walkExpr(s.Value, builtins)
		walkExpr(s.Condition, builtins)
	case ast.ReturnStmt:
		if s.Value != nil {
			walkExpr(s.Value, builtins)
		}
	case ast.ExprStmt:
		walkExpr(s.Value, builtins)
	case ast.IfStmt:
		walkExpr(s.Condition, builtins)
		for _, nested := range s.ThenBody.Statements {
			walkStmt(nested, builtins)
		}
		if s.ElseBody != nil {
			for _, nested := range s.ElseBody.Statements {
				walkStmt(nested, builtins)
			}
		}
	case ast.GuardWhenStmt:
		for _, c := range s.Cases {
			walkExpr(c.Condition, builtins)
			for _, nested := range c.Body.Statements {
				walkStmt(nested, builtins)
			}
		}
		if s.ElseBody != nil {
			for _, nested := range s.ElseBody.Statements {
				walkStmt(nested, builtins)
			}
		}
	case ast.FlowStmt:
		for _, board := range s.Boards {
			walkExpr(board.Initializer, builtins)
		}
		for _, state := range s.States {
			for _, nested := range state.Body.Statements {
				walkStmt(nested, builtins)
			}
		}
	case ast.ForStmt:
		walkExpr(s.Start, builtins)
		walkExpr(s.End, builtins)
		walkExpr(s.Step, builtins)
		for _, nested := range s.Body.Statements {
			walkStmt(nested, builtins)
		}
	}
}

func walkExpr(expr ast.Expr, builtins map[string]bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		switch e.Name {
		case "DispatchThreadID", "GroupThreadID", "GroupID", "GroupIndex":
			builtins[e.Name] = true
		}
	case ast.FieldAccessExpr:
		walkExpr(e.Target, builtins)
	case ast.IndexExpr:
		walkExpr(e.Target, builtins)
		walkExpr(e.Index, builtins)
		if e.HasSecond {
			walkExpr(e.Index2, builtins)
		}
	case ast.GuardedReadExpr:
		walkExpr(e.Target, builtins)
		walkExpr(e.Condition, builtins)
		walkExpr(e.Fallback, builtins)
	case ast.CallExpr:
		walkExpr(e.Callee, builtins)
		for _, arg := range e.Arguments {
			walkExpr(arg, builtins)
		}
	case ast.BinaryExpr:
		walkExpr(e.Left, builtins)
		walkExpr(e.Right, builtins)
	case ast.UnaryExpr:
		walkExpr(e.Operand, builtins)
	case ast.ParenExpr:
		walkExpr(e.Inner, builtins)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			walkExpr(field.Value, builtins)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			walkExpr(field.Value, builtins)
		}
	case ast.DeriveExpr:
		for _, field := range e.Fields {
			walkExpr(field.Value, builtins)
		}
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			walkExpr(c.Value, builtins)
			walkExpr(c.Condition, builtins)
			walkExpr(c.Score, builtins)
		}
		if e.Else != nil {
			walkExpr(e.Else, builtins)
		}
	case ast.WithExpr:
		walkExpr(e.Base, builtins)
		for _, update := range e.Updates {
			walkExpr(update.Value, builtins)
		}
	case ast.MatchExpr:
		walkExpr(e.Subject, builtins)
		for _, arm := range e.Arms {
			walkExpr(arm.Value, builtins)
		}
	case ast.ReductionExpr:
		walkExpr(e.Start, builtins)
		walkExpr(e.End, builtins)
		walkExpr(e.Step, builtins)
		walkExpr(e.Body, builtins)
	}
}

func lowerResourceAccess(access string) vdmir.ResourceAccess {
	if access == "readwrite" {
		return vdmir.ResourceReadWrite
	}
	return vdmir.ResourceReadOnly
}

func elementType(t vdmir.Type) vdmir.Type {
	if t.Element == nil {
		return vdmir.Type{}
	}
	return *t.Element
}

func tileOrViewStride(t vdmir.Type) vdmir.Expr {
	if t.Kind == vdmir.TypeTile || t.Kind == vdmir.TypeRegTile {
		return vdmir.LiteralExpr{ExprType: vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}, Kind: vdmir.LiteralInteger, Value: strconv.Itoa(t.Cols)}
	}
	return nil
}

func binaryResultType(left vdmir.Type, op string, right vdmir.Type) vdmir.Type {
	switch op {
	case "&&", "||", "==", "!=", "<", "<=", ">", ">=":
		return vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
	default:
		if left.Kind == vdmir.TypeF32 || right.Kind == vdmir.TypeF32 {
			return vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
		}
		if left.Kind == vdmir.TypeU32 || right.Kind == vdmir.TypeU32 {
			return vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
		}
		return left
	}
}

func lowerBinaryOperator(op string) string {
	switch op {
	case "and":
		return "&&"
	case "or":
		return "||"
	default:
		return op
	}
}

func lowerUnaryOperator(op string) string {
	if op == "not" {
		return "!"
	}
	return op
}

func astTypeFromVDMIR(t vdmir.Type) ast.TypeRef {
	switch t.Kind {
	case vdmir.TypeBool:
		return ast.TypeRef{Name: "bool"}
	case vdmir.TypeI32:
		return ast.TypeRef{Name: "i32"}
	case vdmir.TypeU32:
		return ast.TypeRef{Name: "u32"}
	case vdmir.TypeF32:
		return ast.TypeRef{Name: "f32"}
	case vdmir.TypeRegTile:
		if t.Element == nil {
			return ast.TypeRef{Name: "reg_tile"}
		}
		return ast.TypeRef{
			Name:         "reg_tile",
			Args:         []ast.TypeRef{astTypeFromVDMIR(*t.Element)},
			TileRows:     consteval.LiteralExpr(consteval.Value{Type: ast.TypeRef{Name: "u32"}, Int32: int64(t.Rows), IsKnown: true}),
			TileCols:     consteval.LiteralExpr(consteval.Value{Type: ast.TypeRef{Name: "u32"}, Int32: int64(t.Cols), IsKnown: true}),
			HasTileShape: true,
		}
	default:
		return ast.TypeRef{Name: t.Name}
	}
}

func isRegTileZeroCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Callee.(ast.IdentifierExpr)
	return ok && id.Name == "reg_tile_zero" && len(call.Arguments) == 0
}

func cloneScope(scope map[string]binding) map[string]binding {
	out := make(map[string]binding, len(scope))
	for k, v := range scope {
		out[k] = v
	}
	return out
}

func replaceComptimeTypeRef(ref ast.TypeRef, comptime map[string]comptimeBinding) ast.TypeRef {
	out := ref
	if len(ref.Args) > 0 {
		out.Args = make([]ast.TypeRef, 0, len(ref.Args))
		for _, arg := range ref.Args {
			out.Args = append(out.Args, replaceComptimeTypeRef(arg, comptime))
		}
	}
	if ref.HasArraySize {
		out.ArraySize = replaceComptimeExpr(ref.ArraySize, comptime)
	}
	if ref.HasTileShape {
		out.TileRows = replaceComptimeExpr(ref.TileRows, comptime)
		out.TileCols = replaceComptimeExpr(ref.TileCols, comptime)
	}
	return out
}

func payloadTypeName(enumName, variantName string) string {
	return enumName + "_" + variantName + "Payload"
}

func collectConceptFields(concept ast.ConceptDecl) map[string]fieldInfo {
	out := map[string]fieldInfo{}
	for _, spec := range conceptFieldSpecs(concept) {
		out[spec.Path] = fieldInfo{typ: spec.Type}
	}
	return out
}

func conceptFieldSpecs(concept ast.ConceptDecl) []conceptFieldSpec {
	var out []conceptFieldSpec
	var walk func([]ast.ConceptMember, []string)
	walk = func(members []ast.ConceptMember, prefix []string) {
		for _, member := range members {
			switch m := member.(type) {
			case ast.ConceptField:
				out = append(out, conceptFieldSpec{
					Path:         joinConceptPath(prefix, m.Name),
					Type:         m.Type,
					DefaultValue: m.DefaultValue,
					ZeroAllowed:  m.Type.ZeroAllowed,
				})
			case ast.ConceptGroup:
				walk(m.Members, append(append([]string(nil), prefix...), m.Name))
			}
		}
	}
	walk(concept.Members, nil)
	return out
}

func joinConceptPath(prefix []string, name string) string {
	if len(prefix) == 0 {
		return name
	}
	return strings.Join(append(append([]string(nil), prefix...), name), ".")
}

func expandConfig(concept ast.ConceptDecl, config ast.ConfigDecl) (map[string]specializeValue, error) {
	assignments := map[string]ast.Expr{}
	for _, field := range config.Fields {
		assignments[field.Path] = field.Value
	}
	values := map[string]specializeValue{}
	for _, spec := range conceptFieldSpecs(concept) {
		if expr, ok := assignments[spec.Path]; ok {
			value, err := evalSpecializeConstExpr(expr, values)
			if err != nil {
				return nil, fmt.Errorf("config %s field %s: %w", config.Name, spec.Path, err)
			}
			if spec.Type.Name == "u32" && !spec.ZeroAllowed && value.int32 == 0 {
				return nil, fmt.Errorf("config field %s is nonzero by default; use u32! if zero is intentional", spec.Path)
			}
			values[spec.Path] = value
			continue
		}
		if spec.DefaultValue == nil {
			return nil, fmt.Errorf("config field %s missing", spec.Path)
		}
		value, err := evalSpecializeConstExpr(spec.DefaultValue, values)
		if err != nil {
			return nil, fmt.Errorf("config %s field %s default: %w", config.Name, spec.Path, err)
		}
		if spec.Type.Name == "u32" && !spec.ZeroAllowed && value.int32 == 0 {
			return nil, fmt.Errorf("config field %s is nonzero by default; use u32! if zero is intentional", spec.Path)
		}
		values[spec.Path] = value
	}
	return values, nil
}

func specializeFieldPath(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.FieldAccessExpr:
		if id, ok := e.Target.(ast.IdentifierExpr); ok {
			return id.Name + "." + e.Field, true
		}
		prefix, ok := specializeFieldPath(e.Target)
		if !ok {
			return "", false
		}
		return prefix + "." + e.Field, true
	default:
		return "", false
	}
}

func flattenConfigName(path string) string {
	parts := strings.Split(path, ".")
	words := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		words = append(words, splitConfigWords(part)...)
	}
	return strings.Join(words, "_")
}

func splitConfigWords(part string) []string {
	part = strings.ReplaceAll(part, "-", "_")
	if part == "" {
		return nil
	}
	var words []string
	start := 0
	for i := 1; i < len(part); i++ {
		prev := part[i-1]
		curr := part[i]
		next := byte(0)
		if i+1 < len(part) {
			next = part[i+1]
		}
		if curr == '_' {
			if start < i {
				words = append(words, strings.ToUpper(part[start:i]))
			}
			start = i + 1
			continue
		}
		if isLowerASCII(prev) && isUpperASCII(curr) || isUpperASCII(prev) && isUpperASCII(curr) && next != 0 && isLowerASCII(next) {
			words = append(words, strings.ToUpper(part[start:i]))
			start = i
		}
	}
	if start < len(part) {
		words = append(words, strings.ToUpper(part[start:]))
	}
	return words
}

func isLowerASCII(b byte) bool { return b >= 'a' && b <= 'z' }
func isUpperASCII(b byte) bool { return b >= 'A' && b <= 'Z' }

func collectFields(fields []ast.Field) map[string]fieldInfo {
	out := make(map[string]fieldInfo, len(fields))
	for _, field := range fields {
		out[field.Name] = fieldInfo{access: field.Access, typ: field.Type, attributes: field.Attributes}
	}
	return out
}

func lowerLoopHint(attributes []ast.Attribute) vdmir.LoopHint {
	for _, attr := range attributes {
		switch attr.Name {
		case "unroll":
			return vdmir.LoopHintUnroll
		case "loop":
			return vdmir.LoopHintLoop
		}
	}
	return vdmir.LoopHintNone
}

func lowerReductionOp(op ast.ReductionOp) vdmir.ReductionOp {
	switch op {
	case ast.ReductionSum:
		return vdmir.ReductionSum
	case ast.ReductionProduct:
		return vdmir.ReductionProduct
	case ast.ReductionMax:
		return vdmir.ReductionMax
	case ast.ReductionMin:
		return vdmir.ReductionMin
	default:
		return ""
	}
}

func resolveResourceBinding(resources []ast.ResourceDecl, index int) vdmir.Binding {
	used := map[int]struct{}{}
	for _, resource := range resources {
		if binding, ok := explicitBinding(resource.Attributes); ok {
			used[binding] = struct{}{}
		}
	}
	resource := resources[index]
	if binding, ok := explicitBinding(resource.Attributes); ok {
		return vdmir.Binding{Set: 0, Binding: binding, Explicit: true}
	}
	next := 0
	for i := 0; i < index; i++ {
		if binding, ok := explicitBinding(resources[i].Attributes); ok {
			used[binding] = struct{}{}
			continue
		}
		for {
			if _, exists := used[next]; !exists {
				used[next] = struct{}{}
				break
			}
			next++
		}
	}
	for {
		if _, exists := used[next]; !exists {
			return vdmir.Binding{Set: 0, Binding: next}
		}
		next++
	}
}

func explicitBinding(attributes []ast.Attribute) (int, bool) {
	for _, attr := range attributes {
		if attr.Name != "binding" || len(attr.Arguments) != 1 {
			continue
		}
		lit, ok := attr.Arguments[0].(ast.IntegerLiteral)
		if !ok {
			return 0, false
		}
		value, err := strconv.Atoi(strings.TrimRight(lit.Value, "uU"))
		if err != nil {
			return 0, false
		}
		return value, true
	}
	return 0, false
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func builtinUintVectorType(dim int) typeInfo {
	fields := map[string]fieldInfo{
		"x": {typ: ast.TypeRef{Name: "u32"}},
		"y": {typ: ast.TypeRef{Name: "u32"}},
		"z": {typ: ast.TypeRef{Name: "u32"}},
		"w": {typ: ast.TypeRef{Name: "u32"}},
	}
	trimmed := map[string]fieldInfo{}
	for _, name := range []string{"x", "y", "z", "w"}[:dim] {
		trimmed[name] = fields[name]
	}
	return typeInfo{kind: "builtin", fields: trimmed}
}

func isBarrierBuiltin(name string) bool {
	return name == "WorkgroupBarrier" || name == "WorkgroupMemoryBarrier" || name == "WorkgroupMemoryBarrierWithSync"
}

func lowerIntrinsic(name string) vdmir.Intrinsic {
	switch name {
	case "WorkgroupBarrier":
		return vdmir.IntrinsicWorkgroupBarrier
	case "WorkgroupMemoryBarrier":
		return vdmir.IntrinsicWorkgroupMemoryBarrier
	default:
		return vdmir.IntrinsicWorkgroupMemoryBarrierWithSync
	}
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
