package makecmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type Options struct {
	File, Backend, Target string
	List, DryRun, Trace   bool
}

type Plan struct {
	Default         string
	CommandTargets  []CommandTarget
	FunctionTargets []FunctionTarget
	PhonyTargets    []PhonyTarget
}
type CommandTarget struct {
	Name                  string
	Inputs, Outputs, Deps []string
	Program               string
	Args                  []string
	Cwd                   string
}
type FunctionTarget struct {
	Name                  string
	Inputs, Outputs, Deps []string
	Function              string
}
type PhonyTarget struct {
	Name string
	Deps []string
}
type target struct {
	name, kind string
	order      int
	command    *CommandTarget
	function   *FunctionTarget
	phony      *PhonyTarget
}
type decision struct{ Name, Kind, Status, Reason string }

func Execute(opts Options, stdout, stderr io.Writer) error {
	if opts.Backend == "" {
		opts.Backend = "direct"
	}
	if opts.Backend != "direct" {
		return fmt.Errorf("unsupported make backend %q; only %q is available in M0", opts.Backend, "direct")
	}
	makeFile, root, err := discover(opts.File)
	if err != nil {
		return err
	}
	program, err := project.Load(makeFile)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}
	val, err := withMakeAuthorityValue(func() (interpret.Value, error) {
		return interpret.CallFunctionWithArgsAndOptions(program, program.Entry, "Plan", nil, stdout, interpret.ExecuteOptions{})
	})
	if err != nil {
		return fmt.Errorf("call Plan(): %w", err)
	}
	plan, err := convertPlan(val)
	if err != nil {
		return err
	}
	targets, err := validate(plan)
	if err != nil {
		return err
	}
	selected, err := selectTarget(opts.Target, plan, targets)
	if err != nil {
		return err
	}
	order, err := closure(selected, targets)
	if err != nil {
		return err
	}
	if opts.List {
		list(plan, targets, stdout)
		return maybeTrace(opts, root, makeFile, selected, nil, nil)
	}
	decisions, runErr := run(order, targets, root, program, opts, stdout, stderr)
	traceErr := maybeTrace(opts, root, makeFile, selected, order, decisions)
	if runErr != nil {
		return runErr
	}
	return traceErr
}

func discover(explicit string) (string, string, error) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", "", err
		}
		if _, err := os.Stat(abs); err != nil {
			return "", "", err
		}
		return abs, filepath.Dir(abs), nil
	}
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "manifest.oct")); err == nil {
			p := filepath.Join(dir, "Make.oct")
			if _, err := os.Stat(p); err == nil {
				return p, dir, nil
			}
			return "", "", fmt.Errorf("could not find Make.oct; pass --file <path> or create Make.oct at the project root")
		}
		p := filepath.Join(dir, "Make.oct")
		if _, err := os.Stat(p); err == nil {
			return p, dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", fmt.Errorf("could not find Make.oct; pass --file <path> or create Make.oct at the project root")
}

func convertPlan(v interpret.Value) (Plan, error) {
	if v.Kind != interpret.ValueRecord {
		return Plan{}, fmt.Errorf("Plan() must return Make.Plan")
	}
	f := v.Record.Fields
	return Plan{Default: str(f, "Default"), CommandTargets: commands(f["CommandTargets"]), FunctionTargets: functions(f["FunctionTargets"]), PhonyTargets: phonies(f["PhonyTargets"])}, nil
}
func str(m map[string]interpret.Value, k string) string {
	if v, ok := m[k]; ok && v.Kind == interpret.ValueString {
		return v.Text
	}
	return ""
}
func arr(v interpret.Value) []string {
	out := []string{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueString {
			out = append(out, e.Text)
		}
	}
	return out
}
func commands(v interpret.Value) []CommandTarget {
	out := []CommandTarget{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueRecord {
			f := e.Record.Fields
			out = append(out, CommandTarget{Name: str(f, "Name"), Inputs: arr(f["Inputs"]), Outputs: arr(f["Outputs"]), Deps: arr(f["Deps"]), Program: str(f, "Program"), Args: arr(f["Args"]), Cwd: str(f, "Cwd")})
		}
	}
	return out
}
func functions(v interpret.Value) []FunctionTarget {
	out := []FunctionTarget{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueRecord {
			f := e.Record.Fields
			out = append(out, FunctionTarget{Name: str(f, "Name"), Inputs: arr(f["Inputs"]), Outputs: arr(f["Outputs"]), Deps: arr(f["Deps"]), Function: str(f, "Function")})
		}
	}
	return out
}
func phonies(v interpret.Value) []PhonyTarget {
	out := []PhonyTarget{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueRecord {
			f := e.Record.Fields
			out = append(out, PhonyTarget{Name: str(f, "Name"), Deps: arr(f["Deps"])})
		}
	}
	return out
}

func validate(p Plan) (map[string]*target, error) {
	m := map[string]*target{}
	order := 0
	add := func(n, k string, t *target) error {
		if n == "" {
			return fmt.Errorf("target %q: name must be non-empty", n)
		}
		if _, ok := m[n]; ok {
			return fmt.Errorf("target %q: duplicate target name", n)
		}
		t.name = n
		t.kind = k
		t.order = order
		order++
		m[n] = t
		return nil
	}
	for i := range p.CommandTargets {
		if p.CommandTargets[i].Program == "" {
			return nil, fmt.Errorf("target %q: Program must be non-empty", p.CommandTargets[i].Name)
		}
		if err := add(p.CommandTargets[i].Name, "command", &target{command: &p.CommandTargets[i]}); err != nil {
			return nil, err
		}
	}
	for i := range p.FunctionTargets {
		if p.FunctionTargets[i].Function == "" {
			return nil, fmt.Errorf("target %q: Function must be non-empty", p.FunctionTargets[i].Name)
		}
		if err := add(p.FunctionTargets[i].Name, "function", &target{function: &p.FunctionTargets[i]}); err != nil {
			return nil, err
		}
	}
	for i := range p.PhonyTargets {
		if err := add(p.PhonyTargets[i].Name, "phony", &target{phony: &p.PhonyTargets[i]}); err != nil {
			return nil, err
		}
	}
	for _, t := range m {
		for _, d := range deps(t) {
			if _, ok := m[d]; !ok {
				return nil, fmt.Errorf("target %q: dependency %q does not exist", t.name, d)
			}
		}
	}
	_, err := closureAll(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}
func deps(t *target) []string {
	if t.command != nil {
		return t.command.Deps
	}
	if t.function != nil {
		return t.function.Deps
	}
	return t.phony.Deps
}
func selectTarget(arg string, p Plan, m map[string]*target) (string, error) {
	if arg != "" {
		if _, ok := m[arg]; !ok {
			return "", fmt.Errorf("target %q: not found", arg)
		}
		return arg, nil
	}
	if p.Default != "" {
		if _, ok := m[p.Default]; !ok {
			return "", fmt.Errorf("target %q: default target does not exist", p.Default)
		}
		return p.Default, nil
	}
	if _, ok := m["Build"]; ok {
		return "Build", nil
	}
	if len(m) == 1 {
		for n := range m {
			return n, nil
		}
	}
	return "", fmt.Errorf("no default target; run oct make --list")
}
func closureAll(m map[string]*target) ([]string, error) {
	out := []string{}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool { return m[names[i]].order < m[names[j]].order })
	for _, n := range names {
		c, err := closure(n, m)
		if err != nil {
			return nil, err
		}
		out = append(out, c...)
	}
	return out, nil
}
func closure(sel string, m map[string]*target) ([]string, error) {
	temp := map[string]bool{}
	perm := map[string]bool{}
	out := []string{}
	var visit func(string) error
	visit = func(n string) error {
		if temp[n] {
			return fmt.Errorf("target %q: dependency cycle detected", n)
		}
		if perm[n] {
			return nil
		}
		temp[n] = true
		ds := append([]string(nil), deps(m[n])...)
		sort.SliceStable(ds, func(i, j int) bool { return m[ds[i]].order < m[ds[j]].order })
		for _, d := range ds {
			if err := visit(d); err != nil {
				return err
			}
		}
		temp[n] = false
		perm[n] = true
		out = append(out, n)
		return nil
	}
	return out, visit(sel)
}

func run(order []string, m map[string]*target, root string, program project.Program, opts Options, stdout, stderr io.Writer) ([]decision, error) {
	ran := map[string]bool{}
	decs := []decision{}
	for _, n := range order {
		t := m[n]
		stale, reason, err := stale(t, root, ran)
		if err != nil {
			return decs, err
		}
		if t.kind == "phony" {
			stale = true
			reason = "phony"
		}
		status := "skip"
		if stale {
			status = "run"
		}
		decs = append(decs, decision{n, t.kind, status, reason})
		fmt.Fprintf(stdout, "%s %s (%s)\n", status, n, reason)
		if opts.DryRun || !stale {
			continue
		}
		if t.command != nil {
			if err := runCommand(*t.command, root, stdout, stderr); err != nil {
				return decs, fmt.Errorf("target %q: %w", n, err)
			}
		} else if t.function != nil {
			fn := t.function.Function
			_, err := withMakeAuthorityValue(func() (interpret.Value, error) {
				return interpret.CallFunctionWithArgsAndOptions(program, program.Entry, fn, nil, stdout, interpret.ExecuteOptions{})
			})
			if err != nil {
				return decs, fmt.Errorf("target %q: function %q failed: %w", n, fn, err)
			}
		}
		ran[n] = true
	}
	return decs, nil
}
func runCommand(c CommandTarget, root string, stdout, stderr io.Writer) error {
	cwd := c.Cwd
	if cwd == "" {
		cwd = root
	}
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(root, cwd)
	}
	cmd := exec.Command(c.Program, c.Args...)
	cmd.Dir = cwd
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
func stale(t *target, root string, ran map[string]bool) (bool, string, error) {
	for _, d := range deps(t) {
		if ran[d] {
			return true, "dependency ran", nil
		}
	}
	if t.kind == "phony" {
		return true, "phony", nil
	}
	inputs, outputs := paths(t, root)
	if len(outputs) == 0 {
		return true, "no outputs", nil
	}
	var oldestOut time.Time
	for i, p := range outputs {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return true, "output missing", nil
			}
			return false, "", err
		}
		if i == 0 || info.ModTime().Before(oldestOut) {
			oldestOut = info.ModTime()
		}
	}
	for _, p := range inputs {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return false, "", fmt.Errorf("target %q: input %q is missing", t.name, p)
			}
			return false, "", err
		}
		if info.ModTime().After(oldestOut) {
			return true, "input newer than output", nil
		}
	}
	return false, "outputs up to date", nil
}
func paths(t *target, root string) ([]string, []string) {
	var ins, outs []string
	if t.command != nil {
		ins = t.command.Inputs
		outs = t.command.Outputs
	} else if t.function != nil {
		ins = t.function.Inputs
		outs = t.function.Outputs
	}
	conv := func(xs []string) []string {
		r := []string{}
		for _, p := range xs {
			if filepath.IsAbs(p) {
				r = append(r, p)
			} else {
				r = append(r, filepath.Join(root, p))
			}
		}
		return r
	}
	return conv(ins), conv(outs)
}
func list(p Plan, m map[string]*target, out io.Writer) {
	fmt.Fprintln(out, "Targets:")
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool { return m[names[i]].order < m[names[j]].order })
	for i, n := range names {
		mark := " "
		if i == 0 {
			mark = " "
		}
		def := ""
		if n == p.Default {
			def = "  default"
		}
		prefix := " "
		if n == p.Default {
			prefix = "*"
		}
		fmt.Fprintf(out, "%s %-8s %s%s\n", prefix, n, m[n].kind, def)
		_ = mark
	}
}
func maybeTrace(opts Options, root, makeFile, selected string, order []string, decs []decision) error {
	if !opts.Trace {
		return nil
	}
	dir := filepath.Join(root, ".octmake")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("MakeTrace {\n")
	fmt.Fprintf(&b, "    Selected: %q\n    Backend: %q\n    MakeFile: %q\n", selected, opts.Backend, filepath.ToSlash(makeFile))
	b.WriteString("    Order: [")
	for i, n := range order {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", n)
	}
	b.WriteString("]\n    Decisions: [\n")
	for _, d := range decs {
		fmt.Fprintf(&b, "        Decision { Name: %q Kind: %q Status: %q Reason: %q }\n", d.Name, d.Kind, d.Status, d.Reason)
	}
	b.WriteString("    ]\n}\n")
	return os.WriteFile(filepath.Join(dir, "trace.octagon"), []byte(b.String()), 0644)
}
func withMakeAuthorityValue(fn func() (interpret.Value, error)) (interpret.Value, error) {
	old, had := os.LookupEnv("OCT_MAKE_AUTHORITY")
	os.Setenv("OCT_MAKE_AUTHORITY", "1")
	defer func() {
		if had {
			os.Setenv("OCT_MAKE_AUTHORITY", old)
		} else {
			os.Unsetenv("OCT_MAKE_AUTHORITY")
		}
	}()
	return fn()
}
