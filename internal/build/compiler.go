package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"oct/internal/ast"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type Result struct {
	ArtifactPath string
	MIRDumpPath  string
}

type MIRModule struct {
	EntryPackage string
	EntryFunc    string
	Records      []MIRRecord
	Enums        []MIREnum
	Functions    []MIRFunction
}

type MIRRecord struct {
	Package string
	Name    string
	Fields  []MIRField
}

type MIREnum struct {
	Package  string
	Name     string
	Variants []string
}

type MIRField struct {
	Name string
	Type string
}

type MIRFunction struct {
	Package string
	Name    string
	Params  []MIRField
	Return  string
	Locals  []MIRField
	Blocks  []MIRBlock
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
	Target  string
	Callee  string
	Args    []string
	Builtin bool
	RetType string
}

func (MIRCall) mirStmt() {}

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

type MIRTerminator interface{ mirTerminator() }

type MIRReturn struct{ Value string }

func (MIRReturn) mirTerminator() {}

type MIRJump struct{ Target string }

func (MIRJump) mirTerminator() {}

type MIRBranch struct{ Cond, TrueTarget, FalseTarget string }

func (MIRBranch) mirTerminator() {}

func Compile(path string) (Result, error) {
	program, err := project.Load(path)
	if err != nil {
		return Result{}, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return Result{}, err
	}

	module, err := lowerProgram(program)
	if err != nil {
		return Result{}, err
	}

	goSrc, err := emitGo(module)
	if err != nil {
		return Result{}, err
	}

	artifactPath := artifactPathFor(program.EntrySource)
	genPath := artifactPath + ".gen.go"
	if err := os.WriteFile(genPath, []byte(goSrc), 0o644); err != nil {
		return Result{}, fmt.Errorf("write generated go %s: %w", genPath, err)
	}
	defer os.Remove(genPath)

	cmd := exec.Command("go", "build", "-o", artifactPath, genPath)
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

type lowerCtx struct {
	pkg     project.Package
	program project.Program
	locals  map[string]string
	blocks  []MIRBlock
	cur     int
	tempID  int
	retType string
	fn      ast.FunctionDecl
}

func lowerProgram(program project.Program) (MIRModule, error) {
	module := MIRModule{EntryPackage: program.Entry}
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
			module.Enums = append(module.Enums, MIREnum{Package: pkgName, Name: e.Name, Variants: append([]string{}, e.Variants...)})
		}
		for _, fn := range pkg.Functions {
			if fn.IsTestFile || fn.IsTheory || fn.IsFact || fn.IsArtifact || fn.IsBenchmark {
				continue
			}
			mirFn, err := lowerFunction(program, pkg, fn)
			if err != nil {
				return MIRModule{}, err
			}
			module.Functions = append(module.Functions, mirFn)
		}
	}
	if module.EntryFunc == "" {
		return MIRModule{}, fmt.Errorf("entry package '%s' is missing main/Main function", program.Entry)
	}
	return module, nil
}

func lowerFunction(program project.Program, pkg project.Package, fn ast.FunctionDecl) (MIRFunction, error) {
	ctx := &lowerCtx{pkg: pkg, program: program, locals: map[string]string{}, retType: typeRefStringForPackage(pkg.Name, fn.ReturnType), fn: fn}
	mirFn := MIRFunction{Package: pkg.Name, Name: fn.Name, Return: ctx.retType}
	for _, p := range fn.Parameters {
		t := typeRefStringForPackage(pkg.Name, p.Type)
		mirFn.Params = append(mirFn.Params, MIRField{Name: p.Name, Type: t})
		ctx.locals[p.Name] = t
	}
	ctx.blocks = append(ctx.blocks, MIRBlock{Label: "entry"})
	ctx.cur = 0
	if err := ctx.lowerBlock(fn.Body); err != nil {
		return MIRFunction{}, fmt.Errorf("function %s.%s: %w", pkg.Name, fn.Name, err)
	}
	if ctx.blocks[ctx.cur].Terminator == nil {
		if mirFn.Return == "Void" {
			ctx.blocks[ctx.cur].Terminator = MIRReturn{Value: ""}
		} else {
			return MIRFunction{}, fmt.Errorf("missing return")
		}
	}
	mirFn.Blocks = ctx.blocks
	for n, t := range ctx.locals {
		isParam := false
		for _, p := range mirFn.Params {
			if p.Name == n {
				isParam = true
				break
			}
		}
		if !isParam {
			mirFn.Locals = append(mirFn.Locals, MIRField{Name: n, Type: t})
		}
	}
	sort.Slice(mirFn.Locals, func(i, j int) bool { return mirFn.Locals[i].Name < mirFn.Locals[j].Name })
	return mirFn, nil
}

func (c *lowerCtx) lowerBlock(block ast.Block) error {
	for _, stmt := range block.Statements {
		if c.blocks[c.cur].Terminator != nil {
			return nil
		}
		switch s := stmt.(type) {
		case ast.LetStmt:
			v, t, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.locals[s.Name] = t
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.VarStmt:
			v, t, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.locals[s.Name] = t
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.AssignStmt:
			v, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			if _, ok := c.locals[s.Name]; !ok {
				return fmt.Errorf("assignment to unknown local '%s'", s.Name)
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.IndexAssignStmt:
			idx, _, err := c.lowerExpr(s.Index)
			if err != nil {
				return err
			}
			val, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: fmt.Sprintf("%s[%s]", s.Target, idx), Value: val})
		case ast.ExprStmt:
			v, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: v})
		case ast.ReturnStmt:
			if s.Value == nil {
				c.blocks[c.cur].Terminator = MIRReturn{}
				continue
			}
			v, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.blocks[c.cur].Terminator = MIRReturn{Value: v}
		case ast.IfStmt:
			if err := c.lowerIfStmt(s); err != nil {
				return err
			}
		case ast.ForStmt:
			return unsupported("for")
		case ast.WhileStmt:
			return unsupported("while")
		case ast.MatchStmt:
			return unsupported("match")
		case ast.GotoStmt:
			return unsupported("flow/goto")
		case ast.SuspendStmt:
			return unsupported("suspend")
		case ast.RememberStmt:
			return unsupported("remember")
		case ast.ResumeStmt:
			return unsupported("resume")
		case ast.WhenStmt:
			return unsupported("when")
		default:
			return fmt.Errorf("unsupported statement %T", s)
		}
	}
	return nil
}

func unsupported(feature string) error {
	return fmt.Errorf("compiled mode does not yet support %s", feature)
}

func (c *lowerCtx) lowerIfStmt(s ast.IfStmt) error {
	cond, _, err := c.lowerExpr(s.Condition)
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
	thenFallsThrough := c.blocks[c.cur].Terminator == nil

	c.cur = elseID
	if s.ElseBody != nil {
		if err := c.lowerBlock(*s.ElseBody); err != nil {
			return err
		}
	}
	elseFallsThrough := c.blocks[c.cur].Terminator == nil

	if !thenFallsThrough && !elseFallsThrough {
		c.cur = thenID
		return nil
	}

	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	if thenFallsThrough {
		c.blocks[thenID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	if elseFallsThrough {
		c.blocks[elseID].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	}
	c.cur = mergeID
	return nil
}

func (c *lowerCtx) temp(t string) string {
	name := fmt.Sprintf("_t%d", c.tempID)
	c.tempID++
	c.locals[name] = t
	return name
}

func (c *lowerCtx) lowerExpr(expr ast.Expr) (string, string, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		return e.Value, "Int", nil
	case ast.FloatLiteral:
		return e.Value, "Float", nil
	case ast.BoolLiteral:
		if e.Value {
			return "true", "Bool", nil
		}
		return "false", "Bool", nil
	case ast.StringLiteralExpr:
		return fmt.Sprintf("%q", e.Value), "String", nil
	case ast.IdentifierExpr:
		t, ok := c.locals[e.Name]
		if !ok {
			return "", "", fmt.Errorf("unknown identifier '%s'", e.Name)
		}
		return e.Name, t, nil
	case ast.BinaryExpr:
		l, lt, err := c.lowerExpr(e.Left)
		if err != nil {
			return "", "", err
		}
		r, _, err := c.lowerExpr(e.Right)
		if err != nil {
			return "", "", err
		}
		ret := lt
		switch e.Operator {
		case "==", "!=", "<", "<=", ">", ">=", "and", "or":
			ret = "Bool"
		}
		tmp := c.temp(ret)
		op := e.Operator
		if op == "and" {
			op = "&&"
		}
		if op == "or" {
			op = "||"
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("(%s %s %s)", l, op, r)})
		return tmp, ret, nil
	case ast.UnaryExpr:
		v, t, err := c.lowerExpr(e.Operand)
		if err != nil {
			return "", "", err
		}
		tmp := c.temp(t)
		op := e.Operator
		if op == "not" {
			op = "!"
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("(%s%s)", op, v)})
		return tmp, t, nil
	case ast.CallExpr:
		callee, ret, builtin, err := c.resolveCall(e.Callee)
		if err != nil {
			return "", "", err
		}
		args := make([]string, 0, len(e.Arguments))
		for _, a := range e.Arguments {
			v, _, err := c.lowerExpr(a)
			if err != nil {
				return "", "", err
			}
			args = append(args, v)
		}
		tmp := c.temp(ret)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: args, Builtin: builtin, RetType: ret})
		return tmp, ret, nil
	case ast.ArrayLiteralExpr:
		vals := []string{}
		typeName := "Int"
		for i, el := range e.Elements {
			v, t, err := c.lowerExpr(el)
			if err != nil {
				return "", "", err
			}
			vals = append(vals, v)
			if i == 0 {
				typeName = t
			}
		}
		tmp := c.temp(typeName + "[]")
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: typeName, Values: vals})
		return tmp, typeName + "[]", nil
	case ast.FieldAccessExpr:
		t, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", err
		}
		tmp := c.temp("Int")
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("%s.%s", t, e.Field)})
		return tmp, "Int", nil
	case ast.IndexExpr:
		target, targetType, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", err
		}
		if len(e.Indices) != 1 {
			return "", "", fmt.Errorf("compiled mode only supports single-dimension indexing")
		}
		idx, _, err := c.lowerExpr(e.Indices[0])
		if err != nil {
			return "", "", err
		}
		elemType := strings.TrimSuffix(targetType, "[]")
		tmp := c.temp(elemType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("%s[%s]", target, idx)})
		return tmp, elemType, nil
	case ast.RecordLiteralExpr:
		vals := []string{}
		names := []string{}
		for _, f := range e.Fields {
			v, _, err := c.lowerExpr(f.Value)
			if err != nil {
				return "", "", err
			}
			vals = append(vals, v)
			names = append(names, f.Name)
		}
		typeName := c.pkg.Name + "." + e.TypeName
		tmp := c.temp(typeName)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructRecord{Target: tmp, TypeName: typeName, FieldNames: names, FieldVals: vals})
		return tmp, typeName, nil
	case ast.EnumValueExpr:
		return fmt.Sprintf("%s_%s", e.EnumName, e.Variant), e.EnumName, nil
	case ast.IfExpr:
		return c.lowerIfExpr(e)
	case ast.SwitchExpr:
		return "", "", unsupported("switch expression")
	case ast.RangeExpr:
		return "", "", unsupported("range")
	case ast.PropagateExpr:
		return "", "", unsupported("fallible propagation")
	case ast.UnwrapExpr:
		return "", "", unsupported("fallible unwrap")
	case ast.BatchExpr:
		return "", "", unsupported("batch")
	case ast.UtilityWhenExpr:
		return "", "", unsupported("utility when")
	case ast.ParenExpr:
		return c.lowerExpr(e.Inner)
	default:
		return "", "", fmt.Errorf("unsupported expression %T", e)
	}
}

func (c *lowerCtx) lowerIfExpr(e ast.IfExpr) (string, string, error) {
	cond, _, err := c.lowerExpr(e.Condition)
	if err != nil {
		return "", "", err
	}
	thenVal, thenType, err := c.lowerExpr(e.ThenExpr)
	if err != nil {
		return "", "", err
	}
	elseVal, _, err := c.lowerExpr(e.ElseExpr)
	if err != nil {
		return "", "", err
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
	return out, thenType, nil
}

func (c *lowerCtx) resolveCall(callee ast.Expr) (string, string, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		if x.Name == "Len" {
			return "Len", "Int", true, nil
		}
		if x.Name == "Append" {
			return "Append", "Int[]", true, nil
		}
		if x.Name == "Print" {
			return "Print", "Int", true, nil
		}
		for _, fn := range c.pkg.Functions {
			if fn.Name == x.Name {
				return c.pkg.Name + "." + x.Name, typeRefStringForPackage(c.pkg.Name, fn.ReturnType), false, nil
			}
		}
		return "", "", false, fmt.Errorf("unknown function '%s'", x.Name)
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false, fmt.Errorf("unsupported call target")
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", false, fmt.Errorf("unknown package '%s'", pkgIdent.Name)
		}
		for _, fn := range importPkg.Functions {
			if fn.Name == x.Field {
				return pkgIdent.Name + "." + x.Field, typeRefStringForPackage(pkgIdent.Name, fn.ReturnType), false, nil
			}
		}
		return "", "", false, fmt.Errorf("unknown function '%s.%s'", pkgIdent.Name, x.Field)
	default:
		return "", "", false, fmt.Errorf("unsupported callee %T", callee)
	}
}

func typeRefString(t ast.TypeRef) string {
	return typeRefStringForPackage("", t)
}

func typeRefStringForPackage(currentPkg string, t ast.TypeRef) string {
	base := t.Name
	if t.Package != "" {
		base = t.Package + "." + base
	} else if currentPkg != "" && base != "" && !isBuiltinTypeName(base) {
		base = currentPkg + "." + base
	}
	if base == "" {
		base = "Void"
	}
	if t.IsArray {
		return base + "[]"
	}
	return base
}

func isBuiltinTypeName(name string) bool {
	switch name {
	case "Int", "Float", "Bool", "String", "Error", "Void":
		return true
	default:
		return false
	}
}

func dumpMIR(m MIRModule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module entry=%s\n", m.EntryPackage)
	for _, fn := range m.Functions {
		fmt.Fprintf(&b, "fn %s.%s", fn.Package, fn.Name)
		fmt.Fprintf(&b, "(")
		for i, p := range fn.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:%s", p.Name, p.Type)
		}
		fmt.Fprintf(&b, ") -> %s\n", fn.Return)
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
				}
			}
			switch t := bb.Terminator.(type) {
			case MIRReturn:
				fmt.Fprintf(&b, "    return %s\n", t.Value)
			case MIRJump:
				fmt.Fprintf(&b, "    jump %s\n", t.Target)
			case MIRBranch:
				fmt.Fprintf(&b, "    branch %s ? %s : %s\n", t.Cond, t.TrueTarget, t.FalseTarget)
			}
		}
	}
	return b.String()
}

func emitGo(m MIRModule) (string, error) {
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("import \"fmt\"\n\n")
	for _, r := range m.Records {
		fmt.Fprintf(&b, "type %s_%s struct {\n", r.Package, r.Name)
		for _, f := range r.Fields {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, goType(f.Type))
		}
		b.WriteString("}\n\n")
	}
	for _, e := range m.Enums {
		fmt.Fprintf(&b, "type %s_%s int\nconst (\n", e.Package, e.Name)
		for i, v := range e.Variants {
			fmt.Fprintf(&b, "\t%s_%s %s_%s = %d\n", e.Name, v, e.Package, e.Name, i)
		}
		b.WriteString(")\n\n")
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
		if returnType == "" {
			b.WriteString(") {\n")
		} else {
			fmt.Fprintf(&b, ") %s {\n", returnType)
		}
		for _, l := range fn.Locals {
			fmt.Fprintf(&b, "\tvar %s %s\n", l.Name, goType(l.Type))
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
			term, err := goTerminator(bb.Terminator, labelToIdx)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "\t\t\t%s\n", term)
		}
		b.WriteString("\t\t}\n\t}\n}\n\n")
	}
	b.WriteString("func main() {\n")
	b.WriteString("\tresult := fn_" + m.EntryPackage + "_" + m.EntryFunc + "()\n")
	b.WriteString("\tfmt.Println(result)\n")
	b.WriteString("}\n")
	return b.String(), nil
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
	case MIRCall:
		if st.Builtin {
			switch st.Callee {
			case "Len":
				return fmt.Sprintf("%s = len(%s)", st.Target, st.Args[0]), nil
			case "Append":
				return fmt.Sprintf("%s = append(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Print":
				return fmt.Sprintf("fmt.Println(%s); %s = 0", st.Args[0], st.Target), nil
			default:
				return "", fmt.Errorf("compiled mode does not yet support builtin %s", st.Callee)
			}
		}
		return fmt.Sprintf("%s = fn_%s(%s)", st.Target, strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(st.Args, ", ")), nil
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
		return "return " + term.Value, nil
	case MIRJump:
		return fmt.Sprintf("pc = %d; continue", labels[term.Target]), nil
	case MIRBranch:
		return fmt.Sprintf("if %s { pc = %d } else { pc = %d }; continue", term.Cond, labels[term.TrueTarget], labels[term.FalseTarget]), nil
	default:
		return "", fmt.Errorf("unsupported MIR terminator %T", t)
	}
}

func goType(t string) string {
	switch t {
	case "Int":
		return "int"
	case "Float":
		return "float64"
	case "Bool":
		return "bool"
	case "String":
		return "string"
	case "Void":
		return ""
	}
	if strings.HasSuffix(t, "[]") {
		return "[]" + goType(strings.TrimSuffix(t, "[]"))
	}
	if strings.Contains(t, ".") {
		return strings.ReplaceAll(t, ".", "_")
	}
	return t
}
