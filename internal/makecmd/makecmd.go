package makecmd

import (
	"bytes"
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

type Config struct {
	Profile, StateDir string
	Trace             bool
	Staleness         string
}

type Plan struct {
	Default         string
	Config          Config
	CommandTargets  []CommandTarget
	FunctionTargets []FunctionTarget
	FlowTargets     []FlowTarget
	PhonyTargets    []PhonyTarget
}
type CommandTarget struct {
	Name                  string
	Inputs, Outputs, Deps []string
	Program               string
	Args                  []string
	Cwd                   string
	Env                   []string
}
type FunctionTarget struct {
	Name                  string
	Inputs, Outputs, Deps []string
	Function              string
}
type FlowTarget struct {
	Name                  string
	Inputs, Outputs, Deps []string
	Flow                  string
	MaxSteps              int64
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
	flow       *FlowTarget
	phony      *PhonyTarget
}
type decision struct {
	Name, Kind, Status, Reason        string
	Deps, Inputs, Outputs             []string
	CommandProgram                    string
	CommandArgs                       []string
	CommandCwd                        string
	CommandEnv                        []string
	Function                          string
	Flow                              string
	MaxSteps                          int64
	Steps                             int
	FinalState                        string
	StateHistory                      []string
	ResultCode                        int64
	Suspended                         bool
	ExitCode                          int
	Stdout, Stderr, Error             string
	StartedUnixNano, FinishedUnixNano int64
}

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
		return maybeTrace(opts, plan, root, makeFile, selected, nil, nil, time.Now(), time.Now())
	}
	started := time.Now()
	decisions, runErr := run(order, targets, root, program, plan, opts, stdout, stderr)
	finished := time.Now()
	traceErr := maybeTrace(opts, plan, root, makeFile, selected, order, decisions, started, finished)
	stateErr := maybeState(opts, plan, root, selected, targets, decisions)
	if runErr != nil {
		if traceErr != nil {
			return traceErr
		}
		if stateErr != nil {
			return stateErr
		}
		return runErr
	}
	if traceErr != nil {
		return traceErr
	}
	return stateErr
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
	return Plan{Default: str(f, "Default"), Config: config(f["Config"]), CommandTargets: commands(f["CommandTargets"]), FunctionTargets: functions(f["FunctionTargets"]), FlowTargets: flows(f["FlowTargets"]), PhonyTargets: phonies(f["PhonyTargets"])}, nil
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
func integer(m map[string]interpret.Value, k string) int64 {
	if v, ok := m[k]; ok && v.Kind == interpret.ValueInt {
		return v.Int
	}
	return 0
}

func config(v interpret.Value) Config {
	c := Config{Profile: "Default", StateDir: ".octmake", Trace: false, Staleness: "Timestamp"}
	if v.Kind != interpret.ValueRecord {
		return c
	}
	f := v.Record.Fields
	if p := str(f, "Profile"); p != "" {
		c.Profile = p
	}
	c.StateDir = str(f, "StateDir")
	if c.StateDir == "" {
		c.StateDir = ".octmake"
	}
	if tv, ok := f["Trace"]; ok && tv.Kind == interpret.ValueBool {
		c.Trace = tv.Bool
	}
	if sv, ok := f["Staleness"]; ok && sv.Kind == interpret.ValueEnum {
		c.Staleness = sv.Enum.Variant
	}
	return c
}
func commands(v interpret.Value) []CommandTarget {
	out := []CommandTarget{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueRecord {
			f := e.Record.Fields
			out = append(out, CommandTarget{Name: str(f, "Name"), Inputs: arr(f["Inputs"]), Outputs: arr(f["Outputs"]), Deps: arr(f["Deps"]), Program: str(f, "Program"), Args: arr(f["Args"]), Cwd: str(f, "Cwd"), Env: arr(f["Env"])})
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
func flows(v interpret.Value) []FlowTarget {
	out := []FlowTarget{}
	if v.Kind != interpret.ValueArray {
		return out
	}
	for _, e := range v.Array {
		if e.Kind == interpret.ValueRecord {
			f := e.Record.Fields
			out = append(out, FlowTarget{Name: str(f, "Name"), Inputs: arr(f["Inputs"]), Outputs: arr(f["Outputs"]), Deps: arr(f["Deps"]), Flow: str(f, "Flow"), MaxSteps: integer(f, "MaxSteps")})
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
	if p.Config.Profile == "" {
		p.Config.Profile = "Default"
	}
	if p.Config.Staleness != "Timestamp" && p.Config.Staleness != "Always" {
		return nil, fmt.Errorf("Config.Staleness must be Timestamp or Always")
	}
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
	for i := range p.FlowTargets {
		if p.FlowTargets[i].Flow == "" {
			return nil, fmt.Errorf("target %q: Flow must be non-empty", p.FlowTargets[i].Name)
		}
		if p.FlowTargets[i].MaxSteps <= 0 {
			return nil, fmt.Errorf("target %q: MaxSteps must be positive", p.FlowTargets[i].Name)
		}
		if err := add(p.FlowTargets[i].Name, "flow", &target{flow: &p.FlowTargets[i]}); err != nil {
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
	if t.flow != nil {
		return t.flow.Deps
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

func run(order []string, m map[string]*target, root string, program project.Program, plan Plan, opts Options, stdout, stderr io.Writer) ([]decision, error) {
	ran := map[string]bool{}
	decs := []decision{}
	for _, n := range order {
		t := m[n]
		stale, reason, err := stale(t, root, ran, plan.Config.Staleness)
		if err != nil {
			return decs, err
		}
		if t.kind == "phony" {
			stale = true
			reason = "phony"
		}
		status := "Skipped"
		if stale {
			status = "Ran"
		}
		if opts.DryRun && stale {
			status = "Skipped"
			reason = "DryRunWouldRun"
		}
		d := newDecision(t, root, status, reason)
		d.StartedUnixNano = time.Now().UnixNano()
		verb := "skip"
		if stale {
			verb = "run"
		}
		fmt.Fprintf(stdout, "%s %s (%s)\n", verb, n, reason)
		if opts.DryRun || !stale {
			d.FinishedUnixNano = time.Now().UnixNano()
			decs = append(decs, d)
			continue
		}
		if t.command != nil {
			if out, errOut, code, err := runCommand(*t.command, root, stdout, stderr); err != nil {
				d.Status = "Failed"
				d.ExitCode = code
				d.Stdout = out
				d.Stderr = errOut
				d.Error = err.Error()
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: %w", n, err)
			} else {
				d.ExitCode = code
				d.Stdout = out
				d.Stderr = errOut
			}
		} else if t.function != nil {
			fn := t.function.Function
			_, err := withMakeAuthorityValue(func() (interpret.Value, error) {
				return interpret.CallFunctionWithArgsAndOptions(program, program.Entry, fn, nil, stdout, interpret.ExecuteOptions{})
			})
			if err != nil {
				d.Status = "Failed"
				d.Error = err.Error()
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: function %q failed: %w", n, fn, err)
			}
		} else if t.flow != nil {
			ft := t.flow
			result, err := withMakeAuthorityFlow(func() (interpret.FlowRunResult, error) {
				return interpret.RunFlowToCompletionWithOptions(program, program.Entry, ft.Flow, int(ft.MaxSteps), stdout, interpret.ExecuteOptions{})
			})
			d.Steps = result.Steps
			d.FinalState = result.FinalState
			d.StateHistory = result.StateHistory
			d.Suspended = result.Suspended
			if result.Result.Kind == interpret.ValueInt {
				d.ResultCode = result.Result.Int
				d.ExitCode = int(result.Result.Int)
			}
			if err != nil {
				d.Status = "Failed"
				d.Error = err.Error()
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: flow %q failed: %w", n, ft.Flow, err)
			}
			if result.Suspended {
				msg := fmt.Sprintf("target %q: flow %q suspended before completion; persistent make flow resume is not supported in MAKE4", n, ft.Flow)
				d.Status = "Failed"
				d.Error = msg
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("%s", msg)
			}
			if !result.Completed {
				msg := fmt.Sprintf("flow %q did not complete", ft.Flow)
				d.Status = "Failed"
				d.Error = msg
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: %s", n, msg)
			}
			if result.Result.Kind != interpret.ValueInt {
				msg := fmt.Sprintf("flow %q returned %s; FlowTarget requires Int result", ft.Flow, result.Result.Kind)
				d.Status = "Failed"
				d.Error = msg
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: %s", n, msg)
			}
			if result.Result.Int != 0 {
				msg := fmt.Sprintf("flow %q returned nonzero result %d", ft.Flow, result.Result.Int)
				d.Status = "Failed"
				d.Error = msg
				d.FinishedUnixNano = time.Now().UnixNano()
				decs = append(decs, d)
				return decs, fmt.Errorf("target %q: %s", n, msg)
			}
		}
		d.FinishedUnixNano = time.Now().UnixNano()
		decs = append(decs, d)
		ran[n] = true
	}
	return decs, nil
}
func runCommand(c CommandTarget, root string, stdout, stderr io.Writer) (string, string, int, error) {
	cwd := c.Cwd
	if cwd == "" {
		cwd = root
	}
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(root, cwd)
	}
	cmd := exec.Command(c.Program, c.Args...)
	cmd.Dir = cwd
	if len(c.Env) > 0 {
		cmd.Env = mergeEnv(os.Environ(), c.Env)
	}
	var outb, errb bytes.Buffer
	cmd.Stdout = io.MultiWriter(stdout, &outb)
	cmd.Stderr = io.MultiWriter(stderr, &errb)
	err := cmd.Run()
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return outb.String(), errb.String(), code, err
}
func mergeEnv(base, overlay []string) []string {
	index := map[string]int{}
	out := append([]string(nil), base...)
	for i, kv := range out {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			index[kv[:eq]] = i
		}
	}
	for _, kv := range overlay {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			key := kv[:eq]
			if i, ok := index[key]; ok {
				out[i] = kv
			} else {
				index[key] = len(out)
				out = append(out, kv)
			}
		}
	}
	return out
}

func stale(t *target, root string, ran map[string]bool, policy string) (bool, string, error) {
	if policy == "Always" && t.kind != "phony" {
		return true, "Always", nil
	}
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
	} else if t.flow != nil {
		ins = t.flow.Inputs
		outs = t.flow.Outputs
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
func stateDir(root string, p Plan) string {
	d := p.Config.StateDir
	if d == "" {
		d = ".octmake"
	}
	if filepath.IsAbs(d) {
		return filepath.Clean(d)
	}
	return filepath.Join(root, filepath.Clean(d))
}
func quote(s string) string { return fmt.Sprintf("%q", s) }
func writeStringArray(b *strings.Builder, xs []string) {
	b.WriteString("[")
	for i, x := range xs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quote(x))
	}
	b.WriteString("]")
}
func newDecision(t *target, root, status, reason string) decision {
	ins, outs := rawPaths(t)
	d := decision{Name: t.name, Kind: t.kind, Status: status, Reason: reason, Deps: deps(t), Inputs: ins, Outputs: outs, ExitCode: 0}
	if t.command != nil {
		d.CommandProgram = t.command.Program
		d.CommandArgs = t.command.Args
		d.CommandCwd = t.command.Cwd
		d.CommandEnv = t.command.Env
	}
	if t.function != nil {
		d.Function = t.function.Function
	}
	if t.flow != nil {
		d.Flow = t.flow.Flow
		d.MaxSteps = t.flow.MaxSteps
	}
	return d
}
func rawPaths(t *target) ([]string, []string) {
	if t.command != nil {
		return t.command.Inputs, t.command.Outputs
	}
	if t.function != nil {
		return t.function.Inputs, t.function.Outputs
	}
	if t.flow != nil {
		return t.flow.Inputs, t.flow.Outputs
	}
	return nil, nil
}
func maybeTrace(opts Options, plan Plan, root, makeFile, selected string, order []string, decs []decision, started, finished time.Time) error {
	if !opts.Trace && !plan.Config.Trace {
		return nil
	}
	dir := stateDir(root, plan)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("MakeTrace {\n")
	fmt.Fprintf(&b, "    Version: 0\n    Backend: %q\n    MakeFile: %q\n    SelectedTarget: %q\n    DefaultTarget: %q\n    Profile: %q\n    StateDir: %q\n    DryRun: %t\n    TraceRequested: %t\n    StartedUnixNano: %d\n    FinishedUnixNano: %d\n", opts.Backend, filepath.ToSlash(makeFile), selected, plan.Default, plan.Config.Profile, filepath.ToSlash(plan.Config.StateDir), opts.DryRun, opts.Trace || plan.Config.Trace, started.UnixNano(), finished.UnixNano())
	b.WriteString("    Decisions: [\n")
	for _, d := range decs {
		fmt.Fprintf(&b, "        MakeDecision {\n            Name: %q\n            Kind: %q\n            Status: %q\n            Reason: %q\n            Deps: ", d.Name, d.Kind, d.Status, d.Reason)
		writeStringArray(&b, d.Deps)
		b.WriteString("\n            Inputs: ")
		writeStringArray(&b, d.Inputs)
		b.WriteString("\n            Outputs: ")
		writeStringArray(&b, d.Outputs)
		fmt.Fprintf(&b, "\n            CommandProgram: %q\n            CommandArgs: ", d.CommandProgram)
		writeStringArray(&b, d.CommandArgs)
		fmt.Fprintf(&b, "\n            CommandCwd: %q\n            CommandEnv: ", d.CommandCwd)
		writeStringArray(&b, d.CommandEnv)
		fmt.Fprintf(&b, "\n            Function: %q\n            ExitCode: %d\n            Stdout: %q\n            Stderr: %q\n            Error: %q\n            StartedUnixNano: %d\n            FinishedUnixNano: %d\n", d.Function, d.ExitCode, d.Stdout, d.Stderr, d.Error, d.StartedUnixNano, d.FinishedUnixNano)
		if d.Kind == "flow" {
			fmt.Fprintf(&b, "            Flow: %q\n            MaxSteps: %d\n            Steps: %d\n            FinalState: %q\n            StateHistory: ", d.Flow, d.MaxSteps, d.Steps, d.FinalState)
			writeStringArray(&b, d.StateHistory)
			fmt.Fprintf(&b, "\n            ResultCode: %d\n            Suspended: %t\n", d.ResultCode, d.Suspended)
		}
		b.WriteString("        }\n")
	}
	b.WriteString("    ]\n}\n")
	return os.WriteFile(filepath.Join(dir, "trace.octagon"), []byte(b.String()), 0644)
}
func maybeState(opts Options, plan Plan, root, selected string, targets map[string]*target, decs []decision) error {
	if opts.DryRun {
		return nil
	}
	dir := stateDir(root, plan)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	status := map[string]string{}
	for _, d := range decs {
		if d.Status == "Failed" {
			status[d.Name] = "Failed"
		} else if d.Status == "Ran" {
			status[d.Name] = "Succeeded"
		} else {
			status[d.Name] = "Skipped"
		}
	}
	var b strings.Builder
	b.WriteString("MakeState {\n")
	fmt.Fprintf(&b, "    Version: 0\n    Backend: %q\n    LastRunTarget: %q\n    Targets: [\n", "direct", selected)
	names := make([]string, 0, len(targets))
	for n := range targets {
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool { return targets[names[i]].order < targets[names[j]].order })
	for _, n := range names {
		t := targets[n]
		ins, outs := rawPaths(t)
		fmt.Fprintf(&b, "        MakeTargetState {\n            Name: %q\n            Kind: %q\n            LastStatus: %q\n            LastRunUnixNano: %d\n", n, t.kind, status[n], time.Now().UnixNano())
		if d, ok := decisionByName(decs, n); ok && t.kind == "flow" {
			fmt.Fprintf(&b, "            FinalState: %q\n            ResultCode: %d\n", d.FinalState, d.ResultCode)
		}
		b.WriteString("            Inputs: [\n")
		for _, p := range ins {
			writePathState(&b, root, p)
		}
		b.WriteString("            ]\n            Outputs: [\n")
		for _, p := range outs {
			writePathState(&b, root, p)
		}
		b.WriteString("            ]\n        }\n")
	}
	b.WriteString("    ]\n}\n")
	return os.WriteFile(filepath.Join(dir, "state.octagon"), []byte(b.String()), 0644)
}
func writePathState(b *strings.Builder, root, p string) {
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, p)
	}
	exists := true
	mod := int64(0)
	if info, err := os.Stat(full); err == nil {
		mod = info.ModTime().UnixNano()
	} else {
		exists = false
	}
	fmt.Fprintf(b, "                MakePathState { Path: %q Exists: %t ModifiedUnixNano: %d Hash: %q }\n", filepath.ToSlash(p), exists, mod, "")
}
func decisionByName(decs []decision, name string) (decision, bool) {
	for _, d := range decs {
		if d.Name == name {
			return d, true
		}
	}
	return decision{}, false
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
func withMakeAuthorityFlow(fn func() (interpret.FlowRunResult, error)) (interpret.FlowRunResult, error) {
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
