// Package test owns SDSL-V test-suite discovery.  It deliberately does not
// share Prometheus' SGEMM dispatch contract: .sdslvtest cases are transient
// compiler artifacts with a fixed assertion-result interface.
package test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const ResultABIVersion uint32 = 1

// InvocationResult is the versioned, fixed-width GPU-to-host ABI.  Every
// field is a 32-bit word so its HLSL/StructuredBuffer layout is unambiguous.
// It is intentionally not a public resource-fixture API.
type InvocationResult struct {
	ABIVersion, Failed, AssertionID, SourceLine, SourceColumn uint32
	InvocationX, InvocationY, InvocationZ                     uint32
	ValueKind, ComponentCount                                 uint32
	ExpectedBits, ActualBits, ToleranceBits                   [4]uint32
}

type Launch struct {
	WorkgroupSize  [3]uint32 `json:"workgroup_size"`
	DispatchGroups [3]uint32 `json:"dispatch_groups"`
}
type Case struct {
	StableID    string   `json:"stable_id"`
	DisplayName string   `json:"display_name"`
	Source      string   `json:"source"`
	Function    string   `json:"function"`
	Kind        string   `json:"kind"`
	TheoryRow   *int     `json:"theory_row,omitempty"`
	InlineData  []string `json:"inline_data,omitempty"`
	Launch      Launch   `json:"launch"`
}
type Manifest struct {
	SchemaVersion    int    `json:"schema_version"`
	ResultABIVersion uint32 `json:"result_abi_version"`
	Source           string `json:"source"`
	Cases            []Case `json:"cases"`
	Interface        struct {
		DescriptorSet uint32 `json:"descriptor_set"`
		Binding       uint32 `json:"binding"`
		Resource      string `json:"resource"`
	} `json:"fixed_interface"`
}

var attrRE = regexp.MustCompile(`^\s*\[([A-Za-z][A-Za-z0-9]*)(?:\((.*)\))?\]\s*$`)
var fnRE = regexp.MustCompile(`^\s*fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)`)

// Discover validates the deliberately small M29 attribute surface and returns
// deterministic per-Fact/per-row cases.  Source parsing remains owned by the
// SDSL-V parser; this scanner only reads file-level test annotations, which
// are not valid production SDSL-V declarations.
func Discover(path string) (Manifest, error) {
	return discoverAST(path)
}

// discoverRegexDeprecated is retained only temporarily for migration audit;
// it is not called. M29a makes parsed FunctionDecl attributes authoritative.
func discoverRegexDeprecated(path string) (Manifest, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}
	identity := sourceIdentity(abs)
	m := Manifest{SchemaVersion: 1, ResultABIVersion: ResultABIVersion, Source: identity}
	m.Interface.DescriptorSet, m.Interface.Binding = 0, 0
	m.Interface.Resource = "compiler-owned assertion result buffer"
	lines := strings.Split(string(text), "\n")
	var attrs []attribute
	seen := map[string]bool{}
	for i, line := range lines {
		if match := attrRE.FindStringSubmatch(line); match != nil {
			attrs = append(attrs, attribute{name: match[1], args: splitArgs(match[2]), line: i + 1})
			continue
		}
		match := fnRE.FindStringSubmatch(line)
		if match == nil {
			if strings.TrimSpace(line) != "" {
				attrs = nil
			}
			continue
		}
		fn, params := match[1], parseParams(match[2])
		if seen[fn] {
			return Manifest{}, fmt.Errorf("%s:%d: duplicate test name %s", path, i+1, fn)
		}
		seen[fn] = true
		fact, theory, rows := false, false, [][]string{}
		launch := Launch{WorkgroupSize: [3]uint32{1, 1, 1}, DispatchGroups: [3]uint32{1, 1, 1}}
		for _, a := range attrs {
			switch a.name {
			case "Fact":
				fact = true
			case "Theory":
				theory = true
			case "InlineData":
				rows = append(rows, a.args)
			case "WorkgroupSize":
				if err := setLaunch(&launch.WorkgroupSize, a.args); err != nil {
					return Manifest{}, fmt.Errorf("%s:%d: WorkgroupSize: %w", path, a.line, err)
				}
			case "DispatchGroups":
				if err := setLaunch(&launch.DispatchGroups, a.args); err != nil {
					return Manifest{}, fmt.Errorf("%s:%d: DispatchGroups: %w", path, a.line, err)
				}
			default:
				return Manifest{}, fmt.Errorf("%s:%d: unsupported .sdslvtest attribute [%s]", path, a.line, a.name)
			}
		}
		if !fact && !theory {
			attrs = nil
			continue
		}
		if fact && theory {
			return Manifest{}, fmt.Errorf("%s:%d: [Fact] and [Theory] cannot both apply to %s", path, i+1, fn)
		}
		if fact {
			if len(params) != 0 {
				return Manifest{}, fmt.Errorf("%s:%d: [Fact] must not declare parameters", path, i+1)
			}
			if len(rows) != 0 {
				return Manifest{}, fmt.Errorf("%s:%d: [InlineData] cannot be used with [Fact]", path, i+1)
			}
			m.Cases = append(m.Cases, newCase(identity, fn, "Fact", nil, nil, launch))
		}
		if theory {
			if len(params) == 0 {
				return Manifest{}, fmt.Errorf("%s:%d: [Theory] function must declare at least one parameter", path, i+1)
			}
			if len(rows) == 0 {
				return Manifest{}, fmt.Errorf("%s:%d: [Theory] function must declare at least one [InlineData] row", path, i+1)
			}
			for row, values := range rows {
				if len(values) != len(params) {
					return Manifest{}, fmt.Errorf("%s:%d: [InlineData] row has %d value(s); %s requires %d", path, i+1, len(values), fn, len(params))
				}
				for n, value := range values {
					if !literalMatches(params[n], value) {
						return Manifest{}, fmt.Errorf("%s:%d: [InlineData] value %q does not match parameter %s", path, i+1, value, params[n])
					}
				}
				r := row
				m.Cases = append(m.Cases, newCase(identity, fn, "Theory", &r, values, launch))
			}
		}
		attrs = nil
	}
	if len(m.Cases) == 0 {
		return Manifest{}, fmt.Errorf("no [Fact] or [Theory] tests found in %s", path)
	}
	sort.Slice(m.Cases, func(i, j int) bool { return m.Cases[i].StableID < m.Cases[j].StableID })
	return m, nil
}

// discoverAST is the M29a compiler-front-end authority.  It deliberately
// consumes ordinary lexer/parser nodes; M29b will consume the same functions
// for assertion/body lowering.
func discoverAST(path string) (Manifest, error) {
	file, err := source.Load(path)
	if err != nil {
		return Manifest{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return Manifest{}, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return Manifest{}, err
	}
	if err := validate.Module(module); err != nil {
		return Manifest{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}
	identity := sourceIdentity(abs)
	m := Manifest{SchemaVersion: 1, ResultABIVersion: ResultABIVersion, Source: identity}
	m.Interface.DescriptorSet = 0
	m.Interface.Binding = 0
	m.Interface.Resource = "compiler-owned assertion result buffer"
	seen := map[string]bool{}
	for _, decl := range module.Decls {
		fn, ok := decl.(ast.FunctionDecl)
		if !ok {
			continue
		}
		fact, theory, rows, launch, err := testAttributes(fn)
		if err != nil {
			return Manifest{}, fmt.Errorf("%s:%d:%d: %w", path, fn.Line, fn.Column, err)
		}
		if !fact && !theory {
			continue
		}
		if seen[fn.Name] {
			return Manifest{}, fmt.Errorf("%s:%d:%d: duplicate test name %s", path, fn.Line, fn.Column, fn.Name)
		}
		seen[fn.Name] = true
		if fact && theory {
			return Manifest{}, fmt.Errorf("%s:%d:%d: [Fact] and [Theory] cannot both apply", path, fn.Line, fn.Column)
		}
		if fact {
			if len(fn.Parameters) != 0 {
				return Manifest{}, fmt.Errorf("%s:%d:%d: [Fact] must not declare parameters", path, fn.Line, fn.Column)
			}
			if len(rows) != 0 {
				return Manifest{}, fmt.Errorf("%s:%d:%d: [InlineData] cannot be used with [Fact]", path, fn.Line, fn.Column)
			}
			m.Cases = append(m.Cases, newCase(identity, fn.Name, "Fact", nil, nil, launch))
			continue
		}
		if len(fn.Parameters) == 0 {
			return Manifest{}, fmt.Errorf("%s:%d:%d: [Theory] function must declare parameters", path, fn.Line, fn.Column)
		}
		if len(rows) == 0 {
			return Manifest{}, fmt.Errorf("%s:%d:%d: [Theory] requires [InlineData]", path, fn.Line, fn.Column)
		}
		for row, values := range rows {
			if len(values) != len(fn.Parameters) {
				return Manifest{}, fmt.Errorf("%s:%d:%d: [InlineData] arity mismatch", path, fn.Line, fn.Column)
			}
			for i, value := range values {
				if !typedLiteralMatches(fn.Parameters[i].Type, value) {
					return Manifest{}, fmt.Errorf("%s:%d:%d: [InlineData] does not match parameter %s", path, fn.Line, fn.Column, fn.Parameters[i].Name)
				}
			}
			r := row
			m.Cases = append(m.Cases, newCase(identity, fn.Name, "Theory", &r, values, launch))
		}
	}
	if len(m.Cases) == 0 {
		return Manifest{}, fmt.Errorf("no [Fact] or [Theory] tests found in %s", path)
	}
	sort.SliceStable(m.Cases, func(i, j int) bool { return m.Cases[i].StableID < m.Cases[j].StableID })
	return m, nil
}

func testAttributes(fn ast.FunctionDecl) (bool, bool, [][]string, Launch, error) {
	launch := Launch{WorkgroupSize: [3]uint32{1, 1, 1}, DispatchGroups: [3]uint32{1, 1, 1}}
	var fact, theory bool
	var rows [][]string
	var wg, dispatch int
	for _, a := range fn.Attributes {
		switch a.Name {
		case "Fact":
			fact = true
		case "Theory":
			theory = true
		case "InlineData":
			vs := make([]string, len(a.Arguments))
			for i, x := range a.Arguments {
				v, ok := attributeLiteral(x)
				if !ok {
					return false, false, nil, launch, fmt.Errorf("[InlineData] values must be literal constants")
				}
				vs[i] = v
			}
			rows = append(rows, vs)
		case "WorkgroupSize":
			wg++
			if wg > 1 {
				return false, false, nil, launch, fmt.Errorf("duplicate [WorkgroupSize]")
			}
			if err := attributeLaunch(&launch.WorkgroupSize, a.Arguments); err != nil {
				return false, false, nil, launch, err
			}
		case "DispatchGroups":
			dispatch++
			if dispatch > 1 {
				return false, false, nil, launch, fmt.Errorf("duplicate [DispatchGroups]")
			}
			if err := attributeLaunch(&launch.DispatchGroups, a.Arguments); err != nil {
				return false, false, nil, launch, err
			}
		default:
			return false, false, nil, launch, fmt.Errorf("unsupported function attribute [%s]", a.Name)
		}
	}
	return fact, theory, rows, launch, nil
}
func attributeLiteral(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case ast.IntegerLiteral:
		return x.Value, true
	case ast.FloatLiteral:
		return x.Value, true
	case ast.BoolLiteral:
		if x.Value {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}
func typedLiteralMatches(t ast.TypeRef, value string) bool { return literalMatches(t.Name, value) }
func attributeLaunch(dst *[3]uint32, args []ast.Expr) error {
	if len(args) != 3 {
		return fmt.Errorf("launch attribute requires three positive integer constants")
	}
	ss := make([]string, 3)
	for i, a := range args {
		v, ok := attributeLiteral(a)
		if !ok {
			return fmt.Errorf("launch attribute arguments must be constants")
		}
		ss[i] = v
	}
	return setLaunch(dst, ss)
}

// sourceIdentity avoids machine-specific absolute paths for suites under the
// current project root while preserving a canonical absolute fallback for an
// explicitly external file.  It is the durable component of replay identity.
func sourceIdentity(abs string) string {
	wd, err := os.Getwd()
	if err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

type attribute struct {
	name string
	args []string
	line int
}

func splitArgs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
func parseParams(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	xs := splitArgs(s)
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		p := strings.Split(x, ":")
		if len(p) != 2 {
			return []string{"<invalid>"}
		}
		out = append(out, strings.TrimSpace(p[1]))
	}
	return out
}
func literalMatches(typ, value string) bool {
	value = strings.TrimSpace(value)
	switch strings.ToLower(typ) {
	case "bool":
		return value == "true" || value == "false"
	case "uint", "u32":
		return strings.HasSuffix(strings.ToLower(value), "u") && integer(value[:len(value)-1])
	case "int", "i32":
		return integer(value)
	case "float", "f32":
		return strings.Contains(value, ".") && !strings.HasSuffix(strings.ToLower(value), "u")
	default:
		return false
	}
}
func integer(s string) bool { _, err := strconv.ParseInt(s, 10, 64); return err == nil }
func setLaunch(dst *[3]uint32, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("requires exactly three positive integer arguments")
	}
	for i, a := range args {
		n, e := strconv.ParseUint(a, 10, 32)
		if e != nil || n == 0 {
			return fmt.Errorf("arguments must be positive uint32 values")
		}
		dst[i] = uint32(n)
	}
	return nil
}
func newCase(source, fn, kind string, row *int, values []string, launch Launch) Case {
	identity := source + "\x00" + fn + "\x00" + kind
	display := fn
	if row != nil {
		identity += fmt.Sprintf("\x00%d\x00%s", *row, strings.Join(values, "\x00"))
		display += fmt.Sprintf("[%d]", *row)
	}
	sum := sha256.Sum256([]byte(identity))
	return Case{StableID: "sdslv-" + hex.EncodeToString(sum[:12]), DisplayName: display, Source: filepath.ToSlash(source), Function: fn, Kind: kind, TheoryRow: row, InlineData: values, Launch: launch}
}
func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
