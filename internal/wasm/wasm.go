// Package wasm lowers Oct's current backend-neutral MIR directly to a core
// WebAssembly binary. It intentionally has no dependency on generated Go.
package wasm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/build"
)

type Result struct {
	ArtifactPath string
	SHA256       string
}

func Compile(path string) (Result, error) {
	return compile(path, false)
}

// CompileOptimized selects the backend-neutral Chapter 4 MIR optimizer before
// WebAssembly encoding. The default Compile path remains byte-compatible.
func CompileOptimized(path string) (Result, error) {
	return compile(path, true)
}

func compile(path string, optimize bool) (Result, error) {
	module, entrySource, err := build.LoadMIR(path)
	if err != nil {
		return Result{}, err
	}
	if optimize {
		module, _, err = build.OptimizeMIR(module)
		if err != nil {
			return Result{}, err
		}
	}
	bytes, err := Encode(module)
	if err != nil {
		return Result{}, err
	}
	out := outputPath(entrySource)
	if err := os.WriteFile(out, bytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("write WebAssembly module %s: %w", out, err)
	}
	sum := sha256.Sum256(bytes)
	return Result{ArtifactPath: out, SHA256: hex.EncodeToString(sum[:])}, nil
}

func outputPath(source string) string {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		clean := filepath.Clean(source)
		return filepath.Join(clean, filepath.Base(clean)+".wasm")
	}
	return strings.TrimSuffix(source, filepath.Ext(source)) + ".wasm"
}

const (
	i32 byte = 0x7f
	i64 byte = 0x7e
	f64 byte = 0x7c
)

type encoder struct {
	m             build.MIRModule
	types         [][]byte
	typeIndex     map[string]uint32
	functionIndex map[string]uint32
	enumTags      map[string]map[string]int32
}

func Encode(m build.MIRModule) ([]byte, error) {
	e := &encoder{m: m, typeIndex: map[string]uint32{}, functionIndex: map[string]uint32{}, enumTags: map[string]map[string]int32{}}
	for _, enum := range m.Enums {
		name := qualified(enum.Package, enum.Name)
		tags := map[string]int32{}
		for i, variant := range enum.Variants {
			if variant.PayloadType != "" {
				return nil, fmt.Errorf("wasm backend: enum %s.%s payload variant %s needs runtime representation and is unsupported in M0", enum.Package, enum.Name, variant.Name)
			}
			tags[variant.Name] = int32(i)
		}
		e.enumTags[name] = tags
		if enum.Package == m.EntryPackage {
			e.enumTags[enum.Name] = tags
		}
	}
	for i, fn := range m.Functions {
		e.functionIndex[qualified(fn.Package, fn.Name)] = uint32(i)
	}
	fnTypes := make([]uint32, len(m.Functions))
	for i, fn := range m.Functions {
		params := make([]byte, len(fn.Params))
		for j, p := range fn.Params {
			var err error
			params[j], err = e.valueType(p.Type)
			if err != nil {
				return nil, e.fnError(fn, err)
			}
		}
		results := []byte{}
		if fn.Return != "Void" {
			t, err := e.valueType(fn.Return)
			if err != nil {
				return nil, e.fnError(fn, err)
			}
			results = append(results, t)
		}
		if fn.IsFallible {
			return nil, e.fnError(fn, fmt.Errorf("fallible return needs an ABI and is unsupported in M0"))
		}
		key := string(append(append(append([]byte{}, params...), 0xff), results...))
		idx, ok := e.typeIndex[key]
		if !ok {
			idx = uint32(len(e.types))
			e.typeIndex[key] = idx
			e.types = append(e.types, funcType(params, results))
		}
		fnTypes[i] = idx
	}
	b := newModule()
	b.section(1, vector(e.types))
	functions := u32(uint32(len(fnTypes)))
	for _, idx := range fnTypes {
		functions = append(functions, u32(idx)...)
	}
	b.section(3, functions)
	exports := []namedExport{}
	for i, fn := range m.Functions {
		if fn.Package == m.EntryPackage && !strings.HasPrefix(fn.Name, "__") {
			exports = append(exports, namedExport{fn.Name, uint32(i)})
		}
	}
	sort.Slice(exports, func(i, j int) bool { return exports[i].name < exports[j].name })
	exportPayload := u32(uint32(len(exports)))
	for _, ex := range exports {
		exportPayload = append(exportPayload, name(ex.name)...)
		exportPayload = append(exportPayload, 0x00)
		exportPayload = append(exportPayload, u32(ex.index)...)
	}
	b.section(7, exportPayload)
	code := u32(uint32(len(m.Functions)))
	for _, fn := range m.Functions {
		body, err := e.functionBody(fn)
		if err != nil {
			return nil, e.fnError(fn, err)
		}
		code = append(code, u32(uint32(len(body)))...)
		code = append(code, body...)
	}
	b.section(10, code)
	b.custom("oct.backend", []byte("wasm-m0;mir=structured;cfg=dispatch-loop"))
	return b.bytes, nil
}

type namedExport struct {
	name  string
	index uint32
}

func qualified(pkg, name string) string { return pkg + "." + name }
func (e *encoder) fnError(fn build.MIRFunction, err error) error {
	return fmt.Errorf("wasm backend: function %s.%s: %w", fn.Package, fn.Name, err)
}

func (e *encoder) valueType(t string) (byte, error) {
	switch t {
	case "Bool":
		return i32, nil
	case "Int":
		return i64, nil
	case "Float":
		return f64, nil
	case "Void":
		return 0x40, nil
	}
	if _, ok := e.enumTags[t]; ok {
		return i32, nil
	}
	return 0, fmt.Errorf("type %s is unsupported in M0", t)
}

func funcType(params, results []byte) []byte {
	out := []byte{0x60}
	out = append(out, u32(uint32(len(params)))...)
	out = append(out, params...)
	out = append(out, u32(uint32(len(results)))...)
	return append(out, results...)
}

func (e *encoder) functionBody(fn build.MIRFunction) ([]byte, error) {
	if len(fn.CaptureEnv) != 0 {
		return nil, fmt.Errorf("captured functions are unsupported in M0")
	}
	locals := map[string]uint32{}
	localTypes := map[string]string{}
	for i, p := range fn.Params {
		locals[p.Name] = uint32(i)
		localTypes[p.Name] = p.Type
	}
	decls := []byte{}
	for _, l := range fn.Locals {
		t, err := e.valueType(l.Type)
		if err != nil {
			return nil, err
		}
		locals[l.Name] = uint32(len(locals))
		localTypes[l.Name] = l.Type
		decls = append(decls, 0x01, t)
	}
	pc := uint32(len(locals))
	decls = append(decls, 0x01, i32)
	body := u32(uint32(len(fn.Locals) + 1))
	body = append(body, decls...)
	labels := map[string]int32{}
	for i, block := range fn.Blocks {
		labels[block.Label] = int32(i)
	}
	body = append(body, 0x41)
	body = append(body, s32(0)...)
	body = append(body, 0x21)
	body = append(body, u32(pc)...)
	body = append(body, 0x02, 0x40, 0x03, 0x40) // block exit; loop dispatch
	for i, block := range fn.Blocks {
		body = append(body, 0x20)
		body = append(body, u32(pc)...)
		body = append(body, 0x41)
		body = append(body, s32(int32(i))...)
		body = append(body, 0x46, 0x04, 0x40)
		for _, stmt := range block.Statements {
			var err error
			body, err = e.emitStmt(body, fn, stmt, locals, localTypes)
			if err != nil {
				return nil, err
			}
		}
		term := block.Terminator
		if term == nil {
			if i+1 >= len(fn.Blocks) {
				return nil, fmt.Errorf("final block %s has no terminator", block.Label)
			}
			term = build.MIRJump{Target: fn.Blocks[i+1].Label}
		}
		var err error
		body, err = e.emitTerminator(body, fn, term, locals, localTypes, labels, pc)
		if err != nil {
			return nil, err
		}
		body = append(body, 0x0b) // case if
	}
	body = append(body, 0x00, 0x0b, 0x0b) // unreachable; loop; exit
	if fn.Return == "Void" {
		body = append(body, 0x0f)
	} else {
		body = append(body, 0x00)
	}
	body = append(body, 0x0b)
	return body, nil
}

func (e *encoder) emitStmt(out []byte, fn build.MIRFunction, stmt build.MIRStmt, locals map[string]uint32, localTypes map[string]string) ([]byte, error) {
	switch st := stmt.(type) {
	case build.MIRAssign:
		idx, ok := locals[st.Target]
		if !ok {
			return nil, fmt.Errorf("assignment target %s is not a local", st.Target)
		}
		var err error
		out, err = e.emitValue(out, st.Value, locals, localTypes)
		if err != nil {
			return nil, err
		}
		return append(out, append([]byte{0x21}, u32(idx)...)...), nil
	case build.MIRCall:
		if st.Builtin {
			return nil, fmt.Errorf("builtin call %s is unsupported in M0", st.Callee)
		}
		if st.FunctionValue {
			return nil, fmt.Errorf("indirect function call %s is unsupported in M0", st.Callee)
		}
		for _, arg := range st.Args {
			var err error
			out, err = e.emitValue(out, arg, locals, localTypes)
			if err != nil {
				return nil, err
			}
		}
		callee := st.Callee
		if !strings.Contains(callee, ".") {
			callee = qualified(fn.Package, callee)
		}
		idx, ok := e.functionIndex[callee]
		if !ok {
			return nil, fmt.Errorf("call target %s is not in the module", callee)
		}
		out = append(out, 0x10)
		out = append(out, u32(idx)...)
		if st.Target == "" || st.Target == "_" {
			if st.RetType != "Void" {
				out = append(out, 0x1a)
			}
			return out, nil
		}
		local, ok := locals[st.Target]
		if !ok {
			return nil, fmt.Errorf("call target local %s does not exist", st.Target)
		}
		out = append(out, 0x21)
		return append(out, u32(local)...), nil
	case build.MIRConstructRecord:
		return nil, fmt.Errorf("MIRConstructRecord %s needs a record ABI and is unsupported in M0", st.TypeName)
	case build.MIRConstructArray:
		return nil, fmt.Errorf("MIRConstructArray needs linear-memory allocation and is unsupported in M0")
	case build.MIRIndexAssign:
		return nil, fmt.Errorf("MIRIndexAssign needs linear-memory array support and is unsupported in M0")
	default:
		return nil, fmt.Errorf("statement %T is unsupported in M0", stmt)
	}
}

func (e *encoder) emitTerminator(out []byte, fn build.MIRFunction, term build.MIRTerminator, locals map[string]uint32, localTypes map[string]string, labels map[string]int32, pc uint32) ([]byte, error) {
	switch t := term.(type) {
	case build.MIRReturn:
		if t.Value != nil {
			var err error
			out, err = e.emitValue(out, t.Value, locals, localTypes)
			if err != nil {
				return nil, err
			}
		}
		return append(out, 0x0f), nil
	case build.MIRJump:
		target, ok := labels[t.Target]
		if !ok {
			return nil, fmt.Errorf("jump target %s does not exist", t.Target)
		}
		out = append(out, 0x41)
		out = append(out, s32(target)...)
		out = append(out, 0x21)
		out = append(out, u32(pc)...)
		return append(out, 0x0c, 0x01), nil
	case build.MIRBranch:
		trueTarget, ok := labels[t.TrueTarget]
		if !ok {
			return nil, fmt.Errorf("branch target %s does not exist", t.TrueTarget)
		}
		falseTarget, ok := labels[t.FalseTarget]
		if !ok {
			return nil, fmt.Errorf("branch target %s does not exist", t.FalseTarget)
		}
		out = append(out, 0x41)
		out = append(out, s32(trueTarget)...)
		out = append(out, 0x41)
		out = append(out, s32(falseTarget)...)
		var err error
		out, err = e.emitValue(out, t.Cond, locals, localTypes)
		if err != nil {
			return nil, err
		}
		out = append(out, 0x1b, 0x21)
		out = append(out, u32(pc)...)
		return append(out, 0x0c, 0x01), nil
	case build.MIRFail:
		return append(out, 0x00), nil
	default:
		return nil, fmt.Errorf("terminator %T is unsupported in M0", term)
	}
}

func (e *encoder) emitValue(out []byte, value build.MIRValue, locals map[string]uint32, localTypes map[string]string) ([]byte, error) {
	switch v := value.(type) {
	case build.MIRLiteral:
		switch v.Type {
		case "Bool":
			out = append(out, 0x41)
			if v.Value == "true" {
				return append(out, 0x01), nil
			}
			return append(out, 0x00), nil
		case "Int":
			n, err := strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			out = append(out, 0x42)
			return append(out, s64(n)...), nil
		case "Float":
			n, err := strconv.ParseFloat(v.Value, 64)
			if err != nil {
				return nil, err
			}
			out = append(out, 0x44)
			bits := math.Float64bits(n)
			for i := 0; i < 8; i++ {
				out = append(out, byte(bits>>uint(8*i)))
			}
			return out, nil
		}
		return nil, fmt.Errorf("literal type %s is unsupported in M0", v.Type)
	case build.MIRLocal:
		idx, ok := locals[v.Name]
		if !ok {
			return nil, fmt.Errorf("local %s does not exist", v.Name)
		}
		out = append(out, 0x20)
		return append(out, u32(idx)...), nil
	case build.MIRUnary:
		start := len(out)
		var err error
		out, err = e.emitValue(out, v.Value, locals, localTypes)
		if err != nil {
			return nil, err
		}
		typ := mirValueType(v.Value, localTypes)
		switch v.Op {
		case "not", "!":
			return append(out, 0x45), nil
		case "-":
			if typ == "Float" {
				out = append(out, 0x9a)
				return out, nil
			}
			expr := append([]byte(nil), out[start:]...)
			out = out[:start]
			out = append(out, 0x42, 0x00)
			out = append(out, expr...)
			return append(out, 0x7d), nil
		default:
			return nil, fmt.Errorf("unary operator %s is unsupported", v.Op)
		}
	case build.MIRBinary:
		var err error
		out, err = e.emitValue(out, v.Left, locals, localTypes)
		if err != nil {
			return nil, err
		}
		out, err = e.emitValue(out, v.Right, locals, localTypes)
		if err != nil {
			return nil, err
		}
		typ := mirValueType(v.Left, localTypes)
		op, err := binaryOpcode(typ, v.Op)
		if err != nil {
			return nil, err
		}
		return append(out, op), nil
	case build.MIRConvert:
		source := mirValueType(v.Value, localTypes)
		var err error
		out, err = e.emitValue(out, v.Value, locals, localTypes)
		if err != nil {
			return nil, err
		}
		if source == "Int" && v.TargetType == "Float" {
			return append(out, 0xb9), nil
		}
		if source == "Float" && v.TargetType == "Int" {
			return append(out, 0xb0), nil
		}
		if source == v.TargetType {
			return out, nil
		}
		return nil, fmt.Errorf("conversion %s -> %s is unsupported in M0", source, v.TargetType)
	case build.MIREnumValue:
		tags, ok := e.enumTags[v.EnumType]
		if !ok {
			return nil, fmt.Errorf("enum type %s is unknown", v.EnumType)
		}
		tag, ok := tags[v.Variant]
		if !ok {
			return nil, fmt.Errorf("enum variant %s.%s is unknown", v.EnumType, v.Variant)
		}
		out = append(out, 0x41)
		return append(out, s32(tag)...), nil
	case build.MIRIntrinsicValue:
		if v.Kind == "enum-is" && len(v.Args) == 1 && len(v.Metadata) == 2 {
			var err error
			out, err = e.emitValue(out, v.Args[0], locals, localTypes)
			if err != nil {
				return nil, err
			}
			tags := e.enumTags[v.Metadata[0]]
			tag, ok := tags[v.Metadata[1]]
			if !ok {
				return nil, fmt.Errorf("enum-is variant %s.%s is unknown", v.Metadata[0], v.Metadata[1])
			}
			out = append(out, 0x41)
			out = append(out, s32(tag)...)
			return append(out, 0x46), nil
		}
		return nil, fmt.Errorf("intrinsic %s is unsupported in M0", v.Kind)
	case build.MIRClone:
		return e.emitValue(out, v.Value, locals, localTypes)
	case build.MIRBackendValue:
		return nil, fmt.Errorf("backend-shaped value (%s/%s) is not accepted by direct WASM", v.Backend, v.Reason)
	default:
		return nil, fmt.Errorf("value %T is unsupported in M0", value)
	}
}

func mirValueType(v build.MIRValue, locals map[string]string) string {
	switch x := v.(type) {
	case build.MIRLiteral:
		return x.Type
	case build.MIRLocal:
		if x.Type != "" {
			return x.Type
		}
		return locals[x.Name]
	case build.MIRUnary:
		return x.Type
	case build.MIRBinary:
		return mirValueType(x.Left, locals)
	case build.MIRConvert:
		return x.TargetType
	case build.MIREnumValue:
		return x.EnumType
	case build.MIRClone:
		return x.Type
	}
	return ""
}
func binaryOpcode(t, op string) (byte, error) {
	if t == "Float" {
		m := map[string]byte{"+": 0xa0, "-": 0xa1, "*": 0xa2, "/": 0xa3, "==": 0x61, "!=": 0x62, "<": 0x63, ">": 0x64, "<=": 0x65, ">=": 0x66}
		if x, ok := m[op]; ok {
			return x, nil
		}
	}
	if t == "Int" {
		m := map[string]byte{"+": 0x7c, "-": 0x7d, "*": 0x7e, "/": 0x7f, "%": 0x81, "==": 0x51, "!=": 0x52, "<": 0x53, ">": 0x55, "<=": 0x57, ">=": 0x59}
		if x, ok := m[op]; ok {
			return x, nil
		}
	}
	if t == "Bool" {
		m := map[string]byte{"and": 0x71, "or": 0x72, "==": 0x46, "!=": 0x47}
		if x, ok := m[op]; ok {
			return x, nil
		}
	}
	return 0, fmt.Errorf("binary operator %s for %s is unsupported in M0", op, t)
}

type moduleBuilder struct{ bytes []byte }

func newModule() *moduleBuilder {
	return &moduleBuilder{bytes: []byte{0, 0x61, 0x73, 0x6d, 1, 0, 0, 0}}
}
func (b *moduleBuilder) section(id byte, p []byte) {
	b.bytes = append(b.bytes, id)
	b.bytes = append(b.bytes, u32(uint32(len(p)))...)
	b.bytes = append(b.bytes, p...)
}
func (b *moduleBuilder) custom(n string, p []byte) {
	q := name(n)
	q = append(q, p...)
	b.section(0, q)
}
func vector(items [][]byte) []byte {
	out := u32(uint32(len(items)))
	for _, x := range items {
		out = append(out, x...)
	}
	return out
}
func name(s string) []byte { return append(u32(uint32(len(s))), []byte(s)...) }
func u32(v uint32) []byte {
	out := []byte{}
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}
func s32(v int32) []byte { return signedLEB(int64(v)) }
func s64(v int64) []byte { return signedLEB(v) }
func signedLEB(v int64) []byte {
	out := []byte{}
	for {
		b := byte(v & 0x7f)
		v >>= 7
		sign := b&0x40 != 0
		done := (v == 0 && !sign) || (v == -1 && sign)
		if !done {
			b |= 0x80
		}
		out = append(out, b)
		if done {
			return out
		}
	}
}
