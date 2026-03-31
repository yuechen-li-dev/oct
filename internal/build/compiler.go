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
	Flows        []MIRFlow
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

type MIRFlow struct {
	Package    string
	Name       string
	Parameters []MIRField
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

type MIRFlowExpr interface{ mirFlowExpr() }

type MIRFlowLiteralExpr struct {
	Value string
}

func (MIRFlowLiteralExpr) mirFlowExpr() {}

type MIRFlowIdentifierExpr struct {
	Name string
}

func (MIRFlowIdentifierExpr) mirFlowExpr() {}

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

type MIRFlowUtilityWhenExpr struct {
	SiteID     int
	ResultType string
	Hysteresis MIRFlowExpr
	MinCommit  MIRFlowExpr
	Cases      []MIRFlowUtilityCase
	Else       MIRFlowExpr
}

func (MIRFlowUtilityWhenExpr) mirFlowExpr() {}

type MIRFlowUtilityCase struct {
	Value     MIRFlowExpr
	Condition MIRFlowExpr
	Score     MIRFlowExpr
}

type MIRField struct {
	Name string
	Type string
}

type MIRFunction struct {
	Package    string
	Name       string
	Params     []MIRField
	Return     string
	IsFallible bool
	ErrorType  string
	Locals     []MIRField
	Blocks     []MIRBlock
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

type MIRBatchMap struct {
	Target     string
	Input      string
	Worker     string
	InputType  string
	ResultType string
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
	batchID int
	retType string
	fn      ast.FunctionDecl
	extra   []MIRFunction
	lastRet string
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
		for _, flow := range pkg.Flows {
			mirFlow, err := lowerFlow(pkgName, flow, pkg)
			if err != nil {
				return MIRModule{}, fmt.Errorf("flow %s.%s: %w", pkgName, flow.Name, err)
			}
			module.Flows = append(module.Flows, mirFlow)
		}
		for _, fn := range pkg.Functions {
			if fn.IsTestFile || fn.IsTheory || fn.IsFact || fn.IsArtifact || fn.IsBenchmark {
				continue
			}
			lowered, err := lowerFunction(program, pkg, fn)
			if err != nil {
				return MIRModule{}, err
			}
			module.Functions = append(module.Functions, lowered...)
		}
	}
	if module.EntryFunc == "" {
		return MIRModule{}, fmt.Errorf("entry package '%s' is missing main/Main function", program.Entry)
	}
	return module, nil
}

func lowerFunction(program project.Program, pkg project.Package, fn ast.FunctionDecl) ([]MIRFunction, error) {
	ctx := &lowerCtx{pkg: pkg, program: program, locals: map[string]string{}, retType: typeRefStringForPackage(pkg.Name, fn.ReturnType), fn: fn}
	mirFn := MIRFunction{Package: pkg.Name, Name: fn.Name, Return: ctx.retType, IsFallible: fn.IsFallible, ErrorType: typeRefStringForPackage(pkg.Name, fn.ErrorType)}
	for _, p := range fn.Parameters {
		t := typeRefStringForPackage(pkg.Name, p.Type)
		mirFn.Params = append(mirFn.Params, MIRField{Name: p.Name, Type: t})
		ctx.locals[p.Name] = t
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
			v, t, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.locals[s.Name] = t
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.VarStmt:
			v, t, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.locals[s.Name] = t
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.AssignStmt:
			v, _, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			if _, ok := c.locals[s.Name]; !ok {
				return fmt.Errorf("assignment to unknown local '%s'", s.Name)
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: s.Name, Value: v})
		case ast.IndexAssignStmt:
			idx, _, _, err := c.lowerExpr(s.Index)
			if err != nil {
				return err
			}
			val, _, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: fmt.Sprintf("%s[%s]", s.Target, idx), Value: val})
		case ast.ExprStmt:
			v, _, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: "_", Value: v})
		case ast.ReturnStmt:
			if s.Value == nil {
				c.blocks[c.cur].Terminator = MIRReturn{}
				continue
			}
			v, t, _, err := c.lowerExpr(s.Value)
			if err != nil {
				return err
			}
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
			return unsupported("for")
		case ast.WhileStmt:
			return unsupported("while")
		case ast.MatchStmt:
			if err := c.lowerMatchStmt(s); err != nil {
				return err
			}
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
	return name
}

func (c *lowerCtx) lowerExpr(expr ast.Expr) (string, string, bool, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		return e.Value, "Int", false, nil
	case ast.FloatLiteral:
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
		if !ok {
			return "", "", false, fmt.Errorf("unknown identifier '%s'", e.Name)
		}
		return e.Name, t, false, nil
	case ast.BinaryExpr:
		l, lt, _, err := c.lowerExpr(e.Left)
		if err != nil {
			return "", "", false, err
		}
		r, _, _, err := c.lowerExpr(e.Right)
		if err != nil {
			return "", "", false, err
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
		callee, ret, builtin, fallible, err := c.resolveCall(e.Callee)
		if err != nil {
			return "", "", false, err
		}
		args := make([]string, 0, len(e.Arguments))
		for _, a := range e.Arguments {
			v, _, _, err := c.lowerExpr(a)
			if err != nil {
				return "", "", false, err
			}
			args = append(args, v)
		}
		localType := ret
		if fallible {
			localType = fallibleType(ret)
		}
		tmp := c.temp(localType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: args, Builtin: builtin, RetType: ret})
		return tmp, ret, fallible, nil
	case ast.ArrayLiteralExpr:
		vals := []string{}
		typeName := "Int"
		for i, el := range e.Elements {
			v, t, _, err := c.lowerExpr(el)
			if err != nil {
				return "", "", false, err
			}
			vals = append(vals, v)
			if i == 0 {
				typeName = t
			}
		}
		tmp := c.temp(typeName + "[]")
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: typeName, Values: vals})
		return tmp, typeName + "[]", false, nil
	case ast.FieldAccessExpr:
		t, targetType, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", false, err
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
		if len(e.Indices) != 1 {
			return "", "", false, fmt.Errorf("compiled mode only supports single-dimension indexing")
		}
		idx, _, _, err := c.lowerExpr(e.Indices[0])
		if err != nil {
			return "", "", false, err
		}
		elemType := strings.TrimSuffix(targetType, "[]")
		tmp := c.temp(elemType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: fmt.Sprintf("%s[%s]", target, idx)})
		return tmp, elemType, false, nil
	case ast.RecordLiteralExpr:
		vals := []string{}
		names := []string{}
		for _, f := range e.Fields {
			v, _, _, err := c.lowerExpr(f.Value)
			if err != nil {
				return "", "", false, err
			}
			vals = append(vals, v)
			names = append(names, f.Name)
		}
		typeName := c.pkg.Name + "." + e.TypeName
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
		return fmt.Sprintf("%s_%s", e.EnumName, e.Variant), e.EnumName, false, nil
	case ast.IfExpr:
		return c.lowerIfExpr(e)
	case ast.SwitchExpr:
		return "", "", false, unsupported("switch expression")
	case ast.RangeExpr:
		return "", "", false, unsupported("range")
	case ast.PropagateExpr:
		return c.lowerPropagateExpr(e)
	case ast.UnwrapExpr:
		return c.lowerUnwrapExpr(e)
	case ast.BatchExpr:
		return c.lowerBatchExpr(e)
	case ast.UtilityWhenExpr:
		return "", "", false, unsupported("utility when")
	case ast.ParenExpr:
		return c.lowerExpr(e.Inner)
	default:
		return "", "", false, fmt.Errorf("unsupported expression %T", e)
	}
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
	worker, resultType, err := c.lowerBatchWorker(e, itemType)
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

func (c *lowerCtx) lowerBatchWorker(e ast.BatchExpr, itemType string) (MIRFunction, string, error) {
	name := fmt.Sprintf("__batch_%s_%d", c.fn.Name, c.batchID)
	c.batchID++
	const retPlaceholder = "__oct_batch_ret__"
	workerDecl := ast.FunctionDecl{Name: name, IsFallible: true, ErrorType: ast.TypeRef{Name: "Error"}}
	wctx := &lowerCtx{
		pkg:     c.pkg,
		program: c.program,
		locals:  map[string]string{e.ItemName: itemType},
		blocks:  []MIRBlock{{Label: "entry"}},
		cur:     0,
		retType: retPlaceholder,
		fn:      workerDecl,
	}
	if err := wctx.lowerBlock(e.Body); err != nil {
		return MIRFunction{}, "", err
	}
	if wctx.blocks[wctx.cur].Terminator == nil {
		return MIRFunction{}, "", fmt.Errorf("batch body missing return")
	}
	if wctx.lastRet == "" {
		return MIRFunction{}, "", fmt.Errorf("batch body return type could not be inferred")
	}
	worker := MIRFunction{
		Package:    c.pkg.Name,
		Name:       name,
		Params:     []MIRField{{Name: e.ItemName, Type: itemType}},
		Return:     wctx.lastRet,
		IsFallible: true,
		ErrorType:  "Error",
		Blocks:     patchBatchReturnType(wctx.blocks, retPlaceholder, wctx.lastRet),
	}
	for n, t := range wctx.locals {
		if n == e.ItemName {
			continue
		}
		worker.Locals = append(worker.Locals, MIRField{Name: n, Type: t})
	}
	sort.Slice(worker.Locals, func(i, j int) bool { return worker.Locals[i].Name < worker.Locals[j].Name })
	return worker, wctx.lastRet, nil
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

func (c *lowerCtx) resolveCall(callee ast.Expr) (string, string, bool, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		switch x.Name {
		case "Step", "Active", "Result", "Complete", "StateHistory", "ResumeTarget":
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
			}
		}
		if x.Name == "Len" {
			return "Len", "Int", true, false, nil
		}
		if x.Name == "Append" {
			return "Append", "Int[]", true, false, nil
		}
		if x.Name == "Print" {
			return "Print", "Int", true, false, nil
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
		return "", "", false, false, fmt.Errorf("unknown function '%s'", x.Name)
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false, false, fmt.Errorf("unsupported call target")
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

func flowInstanceTypeString(resultType string) string {
	return "FlowInstance<" + resultType + ">"
}

func parseFlowInstanceType(t string) (string, bool) {
	if !strings.HasPrefix(t, "FlowInstance<") || !strings.HasSuffix(t, ">") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(t, "FlowInstance<"), ">"), true
}

func lowerFlow(pkgName string, flow ast.FlowDecl, pkg project.Package) (MIRFlow, error) {
	env := map[string]string{}
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
	for _, st := range flow.States {
		lowered, err := lowerFlowBlock(st.Body, env, pkg.Name)
		if err != nil {
			return MIRFlow{}, fmt.Errorf("state %s: %w", st.Name, err)
		}
		out.States = append(out.States, MIRFlowState{Name: st.Name, Statements: lowered})
	}
	return out, nil
}

func lowerFlowBlock(block ast.Block, env map[string]string, pkg string) ([]MIRFlowStmt, error) {
	out := make([]MIRFlowStmt, 0, len(block.Statements))
	for _, stmt := range block.Statements {
		s, err := lowerFlowStmt(stmt, env, pkg)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func lowerFlowStmt(stmt ast.Stmt, env map[string]string, pkg string) (MIRFlowStmt, error) {
	switch s := stmt.(type) {
	case ast.GotoStmt:
		return MIRFlowGoto{Target: s.Target}, nil
	case ast.SuspendStmt:
		return MIRFlowSuspend{}, nil
	case ast.RememberStmt:
		return MIRFlowRemember{}, nil
	case ast.ResumeStmt:
		return MIRFlowResume{}, nil
	case ast.FieldAssignStmt:
		v, err := lowerFlowExpr(s.Value, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowFieldAssign{Target: s.Target, Field: s.Field, Value: v}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return MIRFlowReturn{}, nil
		}
		v, err := lowerFlowExpr(s.Value, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowReturn{Value: v}, nil
	case ast.IfStmt:
		cond, err := lowerFlowExpr(s.Condition, env, pkg)
		if err != nil {
			return nil, err
		}
		thenBody, err := lowerFlowBlock(s.ThenBody, env, pkg)
		if err != nil {
			return nil, err
		}
		var elseBody []MIRFlowStmt
		if s.ElseBody != nil {
			elseBody, err = lowerFlowBlock(*s.ElseBody, env, pkg)
			if err != nil {
				return nil, err
			}
		}
		return MIRFlowIf{Condition: cond, Then: thenBody, Else: elseBody}, nil
	case ast.WhenStmt:
		cases := make([]MIRFlowWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			cond, err := lowerFlowExpr(c.Condition, env, pkg)
			if err != nil {
				return nil, err
			}
			action, err := lowerFlowWhenAction(c.Action, env, pkg)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowWhenCase{Condition: cond, Action: action})
		}
		elseAction, err := lowerFlowWhenAction(s.Else, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhen{Cases: cases, Else: elseAction}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow statement %T", stmt))
	}
}

func lowerFlowWhenAction(action ast.WhenAction, env map[string]string, pkg string) (MIRFlowWhenAction, error) {
	switch a := action.(type) {
	case ast.WhenGotoAction:
		return MIRFlowWhenGoto{Target: a.Target}, nil
	case ast.WhenSuspendAction:
		return MIRFlowWhenSuspend{}, nil
	case ast.WhenReturnAction:
		v, err := lowerFlowExpr(a.Value, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhenReturn{Value: v}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow when action %T", action))
	}
}

func lowerFlowExpr(expr ast.Expr, env map[string]string, pkg string) (MIRFlowExpr, error) {
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
		return MIRFlowIdentifierExpr{Name: e.Name}, nil
	case ast.BinaryExpr:
		l, err := lowerFlowExpr(e.Left, env, pkg)
		if err != nil {
			return nil, err
		}
		r, err := lowerFlowExpr(e.Right, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowBinaryExpr{Left: l, Operator: e.Operator, Right: r}, nil
	case ast.UnaryExpr:
		v, err := lowerFlowExpr(e.Operand, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowUnaryExpr{Operator: e.Operator, Operand: v}, nil
	case ast.UtilityWhenExpr:
		resultType, err := inferFlowExprType(e.Else, env, pkg)
		if err != nil {
			return nil, err
		}
		h, err := lowerFlowExpr(e.Policy.Hysteresis, env, pkg)
		if err != nil {
			return nil, err
		}
		m, err := lowerFlowExpr(e.Policy.MinCommit, env, pkg)
		if err != nil {
			return nil, err
		}
		cases := make([]MIRFlowUtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			val, err := lowerFlowExpr(c.Value, env, pkg)
			if err != nil {
				return nil, err
			}
			cond, err := lowerFlowExpr(c.Condition, env, pkg)
			if err != nil {
				return nil, err
			}
			score, err := lowerFlowExpr(c.Score, env, pkg)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowUtilityCase{Value: val, Condition: cond, Score: score})
		}
		elseExpr, err := lowerFlowExpr(e.Else, env, pkg)
		if err != nil {
			return nil, err
		}
		return MIRFlowUtilityWhenExpr{
			SiteID:     e.SiteID,
			ResultType: resultType,
			Hysteresis: h,
			MinCommit:  m,
			Cases:      cases,
			Else:       elseExpr,
		}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow expression %T", expr))
	}
}

func inferFlowExprType(expr ast.Expr, env map[string]string, pkg string) (string, error) {
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
		return inferFlowExprType(e.Operand, env, pkg)
	case ast.BinaryExpr:
		switch e.Operator {
		case "==", "!=", "<", "<=", ">", ">=", "and", "or":
			return "Bool", nil
		default:
			return inferFlowExprType(e.Left, env, pkg)
		}
	case ast.UtilityWhenExpr:
		return inferFlowExprType(e.Else, env, pkg)
	default:
		return "", unsupported(fmt.Sprintf("flow expression %T", expr))
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
	case MIRFlowBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", dumpFlowExpr(e.Left), e.Operator, dumpFlowExpr(e.Right))
	case MIRFlowUnaryExpr:
		return fmt.Sprintf("(%s%s)", e.Operator, dumpFlowExpr(e.Operand))
	case MIRFlowUtilityWhenExpr:
		return fmt.Sprintf("utility_when[site=%d,hysteresis=%s,min_commit=%s,cases=%d]", e.SiteID, dumpFlowExpr(e.Hysteresis), dumpFlowExpr(e.MinCommit), len(e.Cases))
	default:
		return fmt.Sprintf("unsupported-flow-expr(%T)", expr)
	}
}

func emitGo(m MIRModule) (string, error) {
	var b strings.Builder
	usedBuiltins := map[string]bool{}
	loadTypes := map[string]struct{}{}
	resultTypes := map[string]struct{}{}
	flowResultTypes := map[string]struct{}{}
	needsUtilityHelpers := false
	for _, flow := range m.Flows {
		flowResultTypes[flow.Return] = struct{}{}
		if flowHasUtilityWhen(flow) {
			needsUtilityHelpers = true
		}
	}
	for _, fn := range m.Functions {
		if fn.IsFallible {
			resultTypes[fn.Return] = struct{}{}
		}
		for _, bb := range fn.Blocks {
			for _, st := range bb.Statements {
				call, ok := st.(MIRCall)
				if !ok || !call.Builtin {
					if batch, ok := st.(MIRBatchMap); ok {
						usedBuiltins["BatchMap"] = true
						resultTypes[batch.ResultType+"[]"] = struct{}{}
					}
					continue
				}
				usedBuiltins[call.Callee] = true
				if call.Callee == "LoadOctagon" {
					loadTypes[call.RetType] = struct{}{}
					resultTypes[call.RetType] = struct{}{}
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
	importSet := map[string]struct{}{"fmt": {}}
	if needsUtilityHelpers {
		importSet["reflect"] = struct{}{}
	}
	if usedBuiltins["BatchMap"] {
		for _, pkg := range []string{"runtime", "sync"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["WriteOctagon"] {
		for _, pkg := range []string{"os", "reflect", "strconv", "strings"} {
			importSet[pkg] = struct{}{}
		}
	}
	if usedBuiltins["LoadOctagon"] {
		for _, pkg := range []string{"errors", "os", "reflect", "sort", "strconv", "strings", "unicode", "unicode/utf8"} {
			importSet[pkg] = struct{}{}
		}
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
	resultNames := make([]string, 0, len(resultTypes))
	for t := range resultTypes {
		resultNames = append(resultNames, t)
	}
	sort.Strings(resultNames)
	for _, t := range resultNames {
		fmt.Fprintf(&b, "type %s struct {\n\tValue %s\n\tErr string\n\tIsErr bool\n}\n\n", goResultTypeName(t), goType(t))
	}
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
	flowTypeNames := make([]string, 0, len(flowResultTypes))
	for t := range flowResultTypes {
		flowTypeNames = append(flowTypeNames, t)
	}
	sort.Strings(flowTypeNames)
	for _, t := range flowTypeNames {
		if t == "Void" {
			b.WriteString("type __octVoid struct{}\n\n")
			break
		}
	}
	for _, t := range flowTypeNames {
		fmt.Fprintf(&b, "type __octFlowInstance_%s interface {\n", goSafeName(t))
		b.WriteString("\t__octStep()\n\t__octActive() string\n\t__octComplete() bool\n")
		fmt.Fprintf(&b, "\t__octResult() (%s, bool)\n", goFlowResultType(t))
		b.WriteString("\t__octStateHistory() []string\n\t__octResumeTarget() string\n}\n\n")
		fmt.Fprintf(&b, "type __octResultFlow_%s struct {\n\tValue %s\n\tErr string\n\tIsErr bool\n}\n\n", goSafeName(t), goFlowResultType(t))
	}
	for _, flow := range m.Flows {
		if err := emitGoFlow(&b, flow); err != nil {
			return "", err
		}
	}
	if needsUtilityHelpers {
		b.WriteString(__octUtilityHelpers)
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
				fmt.Fprintf(&b, "%q", v)
			}
			b.WriteString("}},\n")
		}
		b.WriteString("}\n\n")
		b.WriteString(__octSharedOctagonHelpers)
		if usedBuiltins["WriteOctagon"] {
			b.WriteString(__octWriteHelpers)
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
	entryReturn := ""
	entryFallible := false
	for _, fn := range m.Functions {
		if fn.Package == m.EntryPackage && fn.Name == m.EntryFunc {
			entryReturn = fn.Return
			entryFallible = fn.IsFallible
			break
		}
	}
	b.WriteString("\tresult := fn_" + m.EntryPackage + "_" + m.EntryFunc + "()\n")
	if entryFallible {
		b.WriteString("\tif result.IsErr { panic(\"oct error: \" + result.Err) }\n")
		if entryReturn != "Void" {
			b.WriteString("\tfmt.Println(result.Value)\n")
		}
	} else if entryReturn != "Void" {
		b.WriteString("\tfmt.Println(result)\n")
	}
	b.WriteString("}\n")
	return b.String(), nil
}

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

const __octWriteHelpers = `
func __octWriteOctagon(path string, value any) {
	if !strings.HasSuffix(path, ".octagon") {
		panic("WriteOctagon path must end with .octagon")
	}
	rendered, err := __octSerialize(reflect.ValueOf(value))
	if err != nil {
		panic(fmt.Sprintf("WriteOctagon cannot serialize value: %v", err))
	}
	if err := os.WriteFile(path, []byte(rendered+"\n"), 0o644); err != nil {
		panic(fmt.Sprintf("WriteOctagon write %s: %v", path, err))
	}
}

func __octSerialize(v reflect.Value) (string, error) {
	if !v.IsValid() {
		return "", fmt.Errorf("invalid value")
	}
	if v.Kind() == reflect.Interface {
		return __octSerialize(v.Elem())
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
			part, err := __octSerialize(v.Index(i))
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
			value, err := __octSerialize(v.FieldByName(field))
			if err != nil {
				return "", err
			}
			fields = append(fields, fmt.Sprintf("%s: %s", field, value))
		}
		return fmt.Sprintf("%s { %s }", meta.ShortName, strings.Join(fields, " ")), nil
	default:
		return "", fmt.Errorf("value kind %s is not representable in .octagon output", v.Kind().String())
	}
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
	case MIRFlowUtilityWhenExpr:
		return true
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
	fmt.Fprintf(b, "func (f *%s) __octStep() {\n", structName)
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
		return fmt.Sprintf("f.%s.%s = %s\nf.instruction++\ncontinue", s.Target, s.Field, v), nil
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
		thenSrc, err := emitGoFlowBlock(s.Then, pkg, stateIDs, resultType)
		if err != nil {
			return "", err
		}
		out := "if " + cond + " {\n" + thenSrc + "\n}"
		if len(s.Else) > 0 {
			elseSrc, err := emitGoFlowBlock(s.Else, pkg, stateIDs, resultType)
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
	default:
		return "", unsupported(fmt.Sprintf("flow when action %T", action))
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
		return "f." + e.Name, nil
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
		return fmt.Sprintf("__octUtilSelect[%s](f.utilitySites, %d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
			valueType, e.SiteID, h, m, valueType, strings.Join(cases, ", "), elseExpr), nil
	default:
		return "", unsupported(fmt.Sprintf("flow expression %T", expr))
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
	case MIRCall:
		if st.Builtin {
			switch st.Callee {
			case "Len":
				return fmt.Sprintf("%s = len(%s)", st.Target, st.Args[0]), nil
			case "Append":
				return fmt.Sprintf("%s = append(%s, %s)", st.Target, st.Args[0], st.Args[1]), nil
			case "Print":
				return fmt.Sprintf("fmt.Println(%s); %s = 0", st.Args[0], st.Target), nil
			case "WriteOctagon":
				return fmt.Sprintf("__octWriteOctagon(%s, %s); %s = 0", st.Args[0], st.Args[1], st.Target), nil
			case "LoadOctagon":
				return fmt.Sprintf("%s = __octLoadOctagon_%s(%s)", st.Target, goSafeName(st.RetType), st.Args[0]), nil
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
			default:
				return "", fmt.Errorf("compiled mode does not yet support builtin %s", st.Callee)
			}
		}
		return fmt.Sprintf("%s = fn_%s(%s)", st.Target, strings.ReplaceAll(st.Callee, ".", "_"), strings.Join(st.Args, ", ")), nil
	case MIRBatchMap:
		return fmt.Sprintf("%s = func() %s { __vals, __err, __isErr := __octBatchRun(%s, fn_%s, func(r %s) bool { return r.IsErr }, func(r %s) string { return r.Err }, func(r %s) %s { return r.Value }); if __isErr { return %s{Err: __err, IsErr: true} }; return %s{Value: __vals} }()",
			st.Target,
			goType(fallibleType(st.ResultType+"[]")),
			st.Input,
			strings.ReplaceAll(st.Worker, ".", "_"),
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
	if flowRet, ok := parseFlowInstanceType(t); ok {
		return "__octFlowInstance_" + goSafeName(flowRet)
	}
	switch t {
	case "Int":
		return "int"
	case "Float":
		return "float64"
	case "Bool":
		return "bool"
	case "String":
		return "string"
	case "Error":
		return "string"
	case "Void":
		return ""
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
	s := strings.NewReplacer("[]", "Slice", ".", "_", "[", "_", "]", "", ",", "_", " ", "", "*", "_ptr_", "<", "_", ">", "").Replace(valueType)
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
