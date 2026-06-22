package makecmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type Options struct {
	File, Backend, Target string
	Mode                  string
	PlanOut               string
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
	CommandHash                       string
	PreviousCommandHash               string
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
	FailureArtifactPath               string
	StartedUnixNano, FinishedUnixNano int64
}

func Execute(opts Options, stdout, stderr io.Writer) error {
	if opts.Mode == "" {
		opts.Mode = "run"
	}
	if opts.Backend == "" {
		opts.Backend = "direct"
	}
	if opts.Backend != "direct" {
		return fmt.Errorf("unsupported make backend %q; only %q is available in M0", opts.Backend, "direct")
	}
	ctx, err := loadContext(opts, stdout)
	if err != nil {
		return err
	}
	makeFile, root, program, plan, targets := ctx.makeFile, ctx.root, ctx.program, ctx.plan, ctx.targets
	if opts.PlanOut != "" {
		if err := writePlanSnapshot(opts.PlanOut, plan, makeFile); err != nil {
			return err
		}
		if opts.Mode == "run" && !opts.List && !opts.DryRun && !opts.Trace {
			return nil
		}
	}
	if opts.Mode == "doctor" {
		return doctorWithProgram(makeFile, root, plan, targets, program, stdout)
	}
	selected, err := selectTarget(opts.Target, plan, targets)
	if err != nil {
		return err
	}
	order, err := closure(selected, targets)
	if err != nil {
		return err
	}
	switch opts.Mode {
	case "explain":
		return explain(selected, order, targets, root, plan, stdout)
	case "doctor":
		return doctorWithProgram(makeFile, root, plan, targets, program, stdout)
	case "run":
	default:
		return fmt.Errorf("unknown make mode %q", opts.Mode)
	}
	if opts.List {
		list(plan, targets, stdout)
		return maybeTrace(opts, plan, root, makeFile, selected, nil, nil, time.Now(), time.Now())
	}
	started := time.Now()
	decisions, runErr := run(order, targets, root, program, plan, opts, stdout, stderr)
	finished := time.Now()
	failurePath, failureErr := maybeFailureArtifact(opts, plan, root, makeFile, decisions, finished)
	if failurePath != "" {
		for i := range decisions {
			if decisions[i].Status == "Failed" {
				decisions[i].FailureArtifactPath = failurePath
				break
			}
		}
	}
	traceErr := maybeTrace(opts, plan, root, makeFile, selected, order, decisions, started, finished)
	stateErr := maybeState(opts, plan, root, selected, targets, decisions)
	if runErr != nil {
		if failureErr != nil {
			return failureErr
		}
		if traceErr != nil {
			return traceErr
		}
		if stateErr != nil {
			return stateErr
		}
		if failurePath != "" {
			return fmt.Errorf("%w\nfailure artifact: %s", runErr, filepath.ToSlash(failurePath))
		}
		return runErr
	}
	if failureErr != nil {
		return failureErr
	}
	if traceErr != nil {
		return traceErr
	}
	return stateErr
}

type context struct {
	makeFile string
	root     string
	program  project.Program
	plan     Plan
	targets  map[string]*target
}

func loadContext(opts Options, stdout io.Writer) (context, error) {
	makeFile, root, err := discover(opts.File)
	if err != nil {
		return context{}, err
	}
	program, err := project.Load(makeFile)
	if err != nil {
		return context{}, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return context{}, err
	}
	val, err := withMakeAuthorityValue(func() (interpret.Value, error) {
		return interpret.CallFunctionWithArgsAndOptions(program, program.Entry, "Plan", nil, stdout, interpret.ExecuteOptions{})
	})
	if err != nil {
		return context{}, fmt.Errorf("call Plan(): %w", err)
	}
	plan, err := convertPlan(val)
	if err != nil {
		return context{}, err
	}
	targets, err := validate(plan)
	if err != nil {
		return context{}, err
	}
	return context{makeFile: makeFile, root: root, program: program, plan: plan, targets: targets}, nil
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
	state := loadState(root, plan)
	ran := map[string]bool{}
	decs := []decision{}
	for _, n := range order {
		t := m[n]
		stale, reason, err := stale(t, root, ran, plan.Config.Staleness, state)
		if err != nil {
			return decs, err
		}
		if t.kind == "phony" {
			stale = true
			reason = "Phony"
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
		if t.command != nil {
			d.PreviousCommandHash = state.commandHashes[t.name]
		}
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

func stale(t *target, root string, ran map[string]bool, policy string, state makeState) (bool, string, error) {
	if policy == "Always" && t.kind != "phony" {
		return true, "Always", nil
	}
	for _, d := range deps(t) {
		if ran[d] {
			return true, "DependencyRan", nil
		}
	}
	if t.kind == "phony" {
		return true, "Phony", nil
	}
	inputs, outputs := paths(t, root)
	if len(outputs) == 0 {
		return true, "NoOutputs", nil
	}
	var oldestOut time.Time
	for i, p := range outputs {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return true, "MissingOutput", nil
			}
			return false, "", err
		}
		if i == 0 || info.ModTime().Before(oldestOut) {
			oldestOut = info.ModTime()
		}
	}
	inputNewer := false
	for _, p := range inputs {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return false, "", fmt.Errorf("target %q: input %q is missing", t.name, p)
			}
			return false, "", err
		}
		if info.ModTime().After(oldestOut) {
			inputNewer = true
		}
	}
	if t.command != nil {
		cur := commandHash(*t.command)
		prev, ok := state.commandHashes[t.name]
		if !ok || prev == "" {
			return true, "CommandHashMissing", nil
		}
		if prev != cur {
			return true, "CommandChanged", nil
		}
	}
	if inputNewer {
		return true, "InputNewerThanOutput", nil
	}
	return false, "UpToDate", nil
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
		d.CommandHash = commandHash(*t.command)
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
		fmt.Fprintf(&b, "\n            CommandHash: %q\n            PreviousCommandHash: %q\n            Function: %q\n            ExitCode: %d\n            Stdout: %q\n            Stderr: %q\n            Error: %q\n            FailureArtifactPath: %q\n            StartedUnixNano: %d\n            FinishedUnixNano: %d\n", d.CommandHash, d.PreviousCommandHash, d.Function, d.ExitCode, d.Stdout, d.Stderr, d.Error, filepath.ToSlash(d.FailureArtifactPath), d.StartedUnixNano, d.FinishedUnixNano)
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

func maybeFailureArtifact(opts Options, plan Plan, root, makeFile string, decs []decision, at time.Time) (string, error) {
	if opts.DryRun {
		return "", nil
	}
	for _, d := range decs {
		if d.Status != "Failed" {
			continue
		}
		runID := at.UTC().Format("20060102T150405.000000000Z")
		dir := filepath.Join(stateDir(root, plan), "failures", sanitizeTargetName(d.Name), runID)
		path := filepath.Join(dir, "failure.octagon")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(failureArtifact(plan, makeFile, path, d, runID, at)), 0644); err != nil {
			return "", err
		}
		return path, nil
	}
	return "", nil
}

func sanitizeTargetName(name string) string {
	if name == "" {
		return "_"
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func failureArtifact(plan Plan, makeFile, path string, d decision, runID string, at time.Time) string {
	durationMs := int64(0)
	if d.StartedUnixNano > 0 && d.FinishedUnixNano >= d.StartedUnixNano {
		durationMs = (d.FinishedUnixNano - d.StartedUnixNano) / int64(time.Millisecond)
	}
	reason := "TargetFailed"
	if d.Kind == "command" {
		reason = "CommandFailed"
	} else if d.Kind == "function" {
		reason = "FunctionFailed"
	} else if d.Kind == "flow" {
		reason = "FlowFailed"
	}
	var b strings.Builder
	b.WriteString("MakeFailureArtifact {\n")
	fmt.Fprintf(&b, "    Version: 0\n    RunId: %q\n    TimeUtc: %q\n    MakeFile: %q\n    StateDir: %q\n    TracePath: %q\n    FailureArtifactPath: %q\n    Target: %q\n    TargetKind: %q\n    Reason: %q\n    Message: %q\n", runID, at.UTC().Format(time.RFC3339Nano), filepath.ToSlash(makeFile), filepath.ToSlash(plan.Config.StateDir), filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path)))), "trace.octagon")), filepath.ToSlash(path), d.Name, d.Kind, reason, d.Error)
	fmt.Fprintf(&b, "    Decision: TargetDecision { Status: %q Reason: %q DurationMs: %d }\n", d.Status, d.Reason, durationMs)
	switch d.Kind {
	case "command":
		fmt.Fprintf(&b, "    Command: CommandFailure {\n        Target: %q\n        TargetKind: %q\n        Program: %q\n        Args: ", d.Name, d.Kind, d.CommandProgram)
		writeStringArray(&b, d.CommandArgs)
		fmt.Fprintf(&b, "\n        Env: ")
		writeStringArray(&b, d.CommandEnv)
		fmt.Fprintf(&b, "\n        Cwd: %q\n        Inputs: ", d.CommandCwd)
		writeStringArray(&b, d.Inputs)
		fmt.Fprintf(&b, "\n        Outputs: ")
		writeStringArray(&b, d.Outputs)
		fmt.Fprintf(&b, "\n        Deps: ")
		writeStringArray(&b, d.Deps)
		fmt.Fprintf(&b, "\n        ExitCode: %d\n        Stdout: %q\n        Stderr: %q\n        CommandHash: %q\n        PreviousCommandHash: %q\n        DecisionReason: %q\n        DurationMs: %d\n    }\n", d.ExitCode, d.Stdout, d.Stderr, d.CommandHash, d.PreviousCommandHash, d.Reason, durationMs)
	case "function":
		fmt.Fprintf(&b, "    Function: FunctionFailure {\n        Target: %q\n        TargetKind: %q\n        Function: %q\n        Inputs: ", d.Name, d.Kind, d.Function)
		writeStringArray(&b, d.Inputs)
		fmt.Fprintf(&b, "\n        Outputs: ")
		writeStringArray(&b, d.Outputs)
		fmt.Fprintf(&b, "\n        Deps: ")
		writeStringArray(&b, d.Deps)
		fmt.Fprintf(&b, "\n        Error: %q\n        DecisionReason: %q\n        DurationMs: %d\n    }\n", d.Error, d.Reason, durationMs)
	case "flow":
		fmt.Fprintf(&b, "    Flow: FlowFailure {\n        Target: %q\n        TargetKind: %q\n        Flow: %q\n        MaxSteps: %d\n        Inputs: ", d.Name, d.Kind, d.Flow, d.MaxSteps)
		writeStringArray(&b, d.Inputs)
		fmt.Fprintf(&b, "\n        Outputs: ")
		writeStringArray(&b, d.Outputs)
		fmt.Fprintf(&b, "\n        Deps: ")
		writeStringArray(&b, d.Deps)
		fmt.Fprintf(&b, "\n        FinalState: %q\n        StepCount: %d\n        StateHistory: ", d.FinalState, d.Steps)
		writeStringArray(&b, d.StateHistory)
		fmt.Fprintf(&b, "\n        Suspended: %t\n        ResultCode: %d\n        Error: %q\n        DecisionReason: %q\n        DurationMs: %d\n    }\n", d.Suspended, d.ResultCode, d.Error, d.Reason, durationMs)
	}
	b.WriteString("}\n")
	return b.String()
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
		if t.command != nil {
			fmt.Fprintf(&b, "            CommandHash: %q\n", commandHash(*t.command))
		}
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

type makeState struct {
	commandHashes map[string]string
}

func loadState(root string, plan Plan) makeState {
	state := makeState{commandHashes: map[string]string{}}
	body, err := os.ReadFile(filepath.Join(stateDir(root, plan), "state.octagon"))
	if err != nil {
		return state
	}
	text := string(body)
	for _, block := range strings.Split(text, "MakeTargetState {")[1:] {
		end := strings.Index(block, "\n        }")
		if end >= 0 {
			block = block[:end]
		}
		name := fieldString(block, "Name")
		if name == "" {
			continue
		}
		state.commandHashes[name] = fieldString(block, "CommandHash")
	}
	return state
}

func fieldString(block, field string) string {
	needle := field + ": \""
	start := strings.Index(block, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	var b strings.Builder
	escaped := false
	for _, r := range block[start:] {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			return b.String()
		}
		b.WriteRune(r)
	}
	return ""
}

func commandHash(c CommandTarget) string {
	h := sha256.New()
	writeHashField(h, "Kind", "command")
	writeHashField(h, "Name", c.Name)
	writeHashField(h, "Program", c.Program)
	writeHashList(h, "Args", c.Args)
	writeHashList(h, "Env", c.Env)
	writeHashField(h, "Cwd", c.Cwd)
	writeHashList(h, "Outputs", c.Outputs)
	writeHashList(h, "Inputs", c.Inputs)
	writeHashList(h, "Deps", c.Deps)
	return hex.EncodeToString(h.Sum(nil))
}

func writeHashField(w io.Writer, name, value string) {
	fmt.Fprintf(w, "%d:%s=%d:%s\n", len(name), name, len(value), value)
}

func writeHashList(w io.Writer, name string, values []string) {
	fmt.Fprintf(w, "%d:%s[%d]\n", len(name), name, len(values))
	for _, value := range values {
		fmt.Fprintf(w, "%d:%s\n", len(value), value)
	}
}

func writePlanSnapshot(path string, plan Plan, makeFile string) error {
	if !strings.HasSuffix(path, ".octagon") {
		return fmt.Errorf("make --plan-out path must end with .octagon")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString("MakePlanSnapshot {\n")
	fmt.Fprintf(&b, "    Version: 0\n    MakeFile: %q\n    Default: %q\n    Config: MakeConfigSnapshot { Profile: %q StateDir: %q Trace: %t Staleness: %q }\n", filepath.ToSlash(makeFile), plan.Default, plan.Config.Profile, plan.Config.StateDir, plan.Config.Trace, plan.Config.Staleness)
	b.WriteString("    CommandTargets: [\n")
	for i, t := range plan.CommandTargets {
		fmt.Fprintf(&b, "        MakeCommandTargetSnapshot { Name: %q Inputs: ", t.Name)
		writeStringArray(&b, t.Inputs)
		b.WriteString(" Outputs: ")
		writeStringArray(&b, t.Outputs)
		b.WriteString(" Deps: ")
		writeStringArray(&b, t.Deps)
		fmt.Fprintf(&b, " Program: %q Args: ", t.Program)
		writeStringArray(&b, t.Args)
		fmt.Fprintf(&b, " Env: ")
		writeStringArray(&b, t.Env)
		fmt.Fprintf(&b, " Cwd: %q CommandHash: %q }", t.Cwd, commandHash(t))
		if i+1 < len(plan.CommandTargets) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("    ]\n    FunctionTargets: [\n")
	for i, t := range plan.FunctionTargets {
		fmt.Fprintf(&b, "        MakeFunctionTargetSnapshot { Name: %q Inputs: ", t.Name)
		writeStringArray(&b, t.Inputs)
		b.WriteString(" Outputs: ")
		writeStringArray(&b, t.Outputs)
		b.WriteString(" Deps: ")
		writeStringArray(&b, t.Deps)
		fmt.Fprintf(&b, " Function: %q }", t.Function)
		if i+1 < len(plan.FunctionTargets) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("    ]\n    FlowTargets: [\n")
	for i, t := range plan.FlowTargets {
		fmt.Fprintf(&b, "        MakeFlowTargetSnapshot { Name: %q Inputs: ", t.Name)
		writeStringArray(&b, t.Inputs)
		b.WriteString(" Outputs: ")
		writeStringArray(&b, t.Outputs)
		b.WriteString(" Deps: ")
		writeStringArray(&b, t.Deps)
		fmt.Fprintf(&b, " Flow: %q MaxSteps: %d }", t.Flow, t.MaxSteps)
		if i+1 < len(plan.FlowTargets) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("    ]\n    PhonyTargets: [\n")
	for i, t := range plan.PhonyTargets {
		fmt.Fprintf(&b, "        MakePhonyTargetSnapshot { Name: %q Deps: ", t.Name)
		writeStringArray(&b, t.Deps)
		b.WriteString(" }")
		if i+1 < len(plan.PhonyTargets) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("    ]\n}\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func explain(selected string, order []string, m map[string]*target, root string, plan Plan, out io.Writer) error {
	fmt.Fprintf(out, "Explain target %s:\n", selected)
	ran := map[string]bool{}
	for _, n := range order {
		t := m[n]
		wouldRun, reason, err := stale(t, root, ran, plan.Config.Staleness, loadState(root, plan))
		if err != nil {
			return err
		}
		if t.kind == "phony" {
			wouldRun = true
			reason = "Phony"
		}
		status := "would skip"
		if wouldRun {
			status = "would run"
			ran[n] = true
		}
		fmt.Fprintf(out, "  %s [%s]: %s\n    reason: %s\n", n, t.kind, status, reason)
	}
	return nil
}

func doctor(makeFile, root string, plan Plan, m map[string]*target, out io.Writer) error {
	return doctorWithProgram(makeFile, root, plan, m, project.Program{}, out)
}

func doctorWithProgram(makeFile, root string, plan Plan, m map[string]*target, program project.Program, out io.Writer) error {
	state := filepath.Join(stateDir(root, plan), "state.octagon")
	trace := filepath.Join(stateDir(root, plan), "trace.octagon")
	fmt.Fprintln(out, "oct make doctor")
	fmt.Fprintf(out, "Make file: %s\n", filepath.ToSlash(makeFile))
	fmt.Fprintf(out, "Profile: %s\n", plan.Config.Profile)
	fmt.Fprintf(out, "State dir: %s\n", filepath.ToSlash(plan.Config.StateDir))
	fmt.Fprintln(out, "Backend: direct")
	fmt.Fprintf(out, "Default target: %s\n", plan.Default)
	counts := map[string]int{}
	for _, t := range m {
		counts[t.kind]++
	}
	fmt.Fprintln(out, "Targets:")
	fmt.Fprintf(out, "  command: %d\n  function: %d\n  flow: %d\n  phony: %d\n", counts["command"], counts["function"], counts["flow"], counts["phony"])
	writeMakeAttributeDiagnostics(out, program)
	fmt.Fprintln(out, "Validation: ok")
	fmt.Fprintln(out, "Dependencies: ok")
	fmt.Fprintf(out, "State: %s (%s)\n", filepath.ToSlash(state), existence(state))
	fmt.Fprintf(out, "Trace: %s (%s)\n", filepath.ToSlash(trace), existence(trace))
	programs := []string{}
	for _, t := range plan.CommandTargets {
		programs = append(programs, t.Program)
	}
	sort.Strings(programs)
	fmt.Fprintln(out, "Command identity hashing: enabled")
	fmt.Fprintln(out, "Programs referenced:")
	if len(programs) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	last := ""
	for _, p := range programs {
		if p != last {
			fmt.Fprintf(out, "  %s\n", p)
			last = p
		}
	}
	return nil
}

func writeMakeAttributeDiagnostics(out io.Writer, program project.Program) {
	functions := makeFunctions(program)
	fmt.Fprintln(out, "Make attributes:")
	if len(functions) == 0 {
		fmt.Fprintln(out, "  Plan: metadata unavailable")
		return
	}
	planFound := false
	anyMarkers := false
	for _, fn := range functions {
		markers := makeMarkers(fn)
		if len(markers) > 0 {
			anyMarkers = true
		}
		if fn.Name == "Plan" {
			planFound = true
			if len(markers) == 0 {
				fmt.Fprintln(out, "  Plan: conventional unmarked Plan()")
			} else {
				fmt.Fprintf(out, "  Plan: %s\n", strings.Join(markers, " "))
			}
		}
	}
	if !planFound {
		fmt.Fprintln(out, "  Plan: not found")
	} else if !anyMarkers {
		fmt.Fprintln(out, "  Markers: none")
	}
	writeMarkerGroup(out, "  Authority functions:", functions, func(fn ast.FunctionDecl) bool { return fn.RequiresMakeAuthority })
	writeMarkerGroup(out, "  Pure helpers:", functions, func(fn ast.FunctionDecl) bool { return fn.IsMakePure && fn.Name != "Plan" })
	writePlanEntrypoint(out, functions)
	writeAuthorityDiagnostics(out, functions)
	writePureDiagnostics(out, functions)
	writeTypedMakeIdiomSuggestions(out, functions)
}

func makeFunctions(program project.Program) []ast.FunctionDecl {
	if program.Packages == nil {
		return nil
	}
	pkg, ok := program.Packages[program.Entry]
	if !ok {
		return nil
	}
	out := []ast.FunctionDecl{}
	for _, fn := range pkg.Functions {
		if fn.IsMakeFile {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func makeMarkers(fn ast.FunctionDecl) []string {
	markers := []string{}
	if fn.IsMakePlan {
		markers = append(markers, "[MakePlan]")
	}
	if fn.IsMakePure {
		markers = append(markers, "[Pure]")
	}
	if fn.IsMakeNoWhile {
		markers = append(markers, "[NoWhile]")
	}
	if fn.RequiresMakeAuthority {
		markers = append(markers, "[RequiresAuthority]")
	}
	return markers
}

func writeMarkerGroup(out io.Writer, header string, functions []ast.FunctionDecl, include func(ast.FunctionDecl) bool) {
	wroteHeader := false
	for _, fn := range functions {
		if !include(fn) {
			continue
		}
		if !wroteHeader {
			fmt.Fprintln(out, header)
			wroteHeader = true
		}
		fmt.Fprintf(out, "    %s: %s\n", fn.Name, strings.Join(makeMarkers(fn), " "))
	}
}

func writePlanEntrypoint(out io.Writer, functions []ast.FunctionDecl) {
	for _, fn := range functions {
		if fn.Name != "Plan" {
			continue
		}
		markers := makeMarkers(fn)
		if fn.IsMakePlan {
			fmt.Fprintf(out, "Plan entrypoint: Plan() %s\n", strings.Join(markers, " "))
			return
		}
		fmt.Fprintln(out, "Plan entrypoint: Plan() conventional")
		fmt.Fprintln(out, "Suggestion: consider [MakePlan] [Pure] [NoWhile] for stricter validation")
		return
	}
}

var makeHostPrimitives = map[string]bool{
	"Exec": true, "ExecIn": true, "Tool": true, "Exists": true, "IsFile": true, "IsDir": true,
	"MkdirAll": true, "Remove": true, "Copy": true, "ReadText": true, "WriteText": true,
	"Glob": true, "ModifiedTime": true, "HashFile": true, "Env": true,
}

func writeAuthorityDiagnostics(out io.Writer, functions []ast.FunctionDecl) {
	fmt.Fprintln(out, "Authority diagnostics:")
	wrote := false
	for _, fn := range functions {
		calls := directMakeHostCalls(fn)
		if len(calls) == 0 {
			continue
		}
		wrote = true
		if fn.RequiresMakeAuthority {
			fmt.Fprintf(out, "  %s: ok ([RequiresAuthority])\n", fn.Name)
		} else {
			fmt.Fprintf(out, "  %s calls %s but is not marked [RequiresAuthority]\n", fn.Name, joinCallList(calls))
			fmt.Fprintf(out, "    suggestion: add [RequiresAuthority] to %s\n", fn.Name)
		}
		if fn.IsMakePure {
			fmt.Fprintf(out, "  %s is marked [Pure] but calls %s\n", fn.Name, joinCallList(calls))
		}
	}
	if !wrote {
		fmt.Fprintln(out, "  no direct Make host primitive calls found")
	}
}

func joinCallList(calls []string) string {
	prefixed := make([]string, len(calls))
	for i, c := range calls {
		prefixed[i] = "Make." + c
	}
	if len(prefixed) <= 1 {
		return strings.Join(prefixed, "")
	}
	return strings.Join(prefixed[:len(prefixed)-1], ", ") + " and " + prefixed[len(prefixed)-1]
}

func directMakeHostCalls(fn ast.FunctionDecl) []string {
	seen := map[string]bool{}
	walkBlock(fn.Body, func(call ast.CallExpr) {
		if name, ok := makeCallName(call.Callee); ok && makeHostPrimitives[name] {
			seen[name] = true
		}
	})
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func writeTypedMakeIdiomSuggestions(out io.Writer, functions []ast.FunctionDecl) {
	suggestions := []string{}
	for _, fn := range functions {
		walkBlock(fn.Body, func(call ast.CallExpr) {
			if name, ok := makeCallName(call.Callee); !ok || name != "Exec" {
				return
			}
			if len(call.Arguments) < 2 || stringLiteral(call.Arguments[0]) != "bash" {
				return
			}
			args := stringArrayLiteral(call.Arguments[1])
			if len(args) < 2 || args[0] != "-c" {
				return
			}
			command := args[1]
			if strings.Contains(command, "command -v ") {
				tool := shellWordAfter(command, "command -v ")
				if tool != "" {
					suggestions = append(suggestions, fmt.Sprintf("  %s uses shell-shaped tool probe; prefer Make.Tool(\"%s\")", fn.Name, tool))
				}
			}
			if strings.Contains(command, "test \"$") {
				env := shellWordAfter(command, "test \"$")
				if env != "" {
					suggestions = append(suggestions, fmt.Sprintf("  %s uses shell-shaped env gate; prefer Make.Env(\"%s\")", fn.Name, env))
				}
			}
		})
	}
	if len(suggestions) == 0 {
		return
	}
	fmt.Fprintln(out, "Typed Make idiom suggestions:")
	for _, suggestion := range suggestions {
		fmt.Fprintln(out, suggestion)
	}
}

func shellWordAfter(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '>' || r == '=' })
	if end >= 0 {
		rest = rest[:end]
	}
	return strings.Trim(rest, "$")
}

func makeCallName(expr ast.Expr) (string, bool) {
	field, ok := expr.(ast.FieldAccessExpr)
	if !ok || field.Field == "" {
		return "", false
	}
	target, ok := field.Target.(ast.IdentifierExpr)
	return field.Field, ok && target.Name == "Make"
}

func stringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(ast.StringLiteralExpr); ok {
		return lit.Value
	}
	return ""
}

func stringArrayLiteral(expr ast.Expr) []string {
	arr, ok := expr.(ast.ArrayLiteralExpr)
	if !ok {
		return nil
	}
	out := []string{}
	for _, elem := range arr.Elements {
		lit, ok := elem.(ast.StringLiteralExpr)
		if !ok {
			return nil
		}
		out = append(out, lit.Value)
	}
	return out
}

func walkBlock(block ast.Block, visitCall func(ast.CallExpr)) {
	for _, stmt := range block.Statements {
		walkStmt(stmt, visitCall)
	}
}

func walkStmt(stmt ast.Stmt, visitCall func(ast.CallExpr)) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		walkExpr(s.Value, visitCall)
	case ast.VarStmt:
		walkExpr(s.Value, visitCall)
	case ast.AssignStmt:
		walkExpr(s.Value, visitCall)
	case ast.IndexAssignStmt:
		for _, idx := range s.Indices {
			walkExpr(idx, visitCall)
		}
		walkExpr(s.Value, visitCall)
	case ast.FieldAssignStmt:
		walkExpr(s.Value, visitCall)
	case ast.FieldIndexAssignStmt:
		for _, idx := range s.Indices {
			walkExpr(idx, visitCall)
		}
		walkExpr(s.Value, visitCall)
	case ast.ReturnStmt:
		walkExpr(s.Value, visitCall)
	case ast.ExprStmt:
		walkExpr(s.Value, visitCall)
	case ast.ForStmt:
		walkExpr(s.Range, visitCall)
		walkBlock(s.Body, visitCall)
	case ast.MatchStmt:
		walkExpr(s.Subject, visitCall)
		walkBlock(s.OkBody, visitCall)
		walkBlock(s.ErrBody, visitCall)
	case ast.IfStmt:
		walkExpr(s.Condition, visitCall)
		walkBlock(s.ThenBody, visitCall)
		if s.ElseBody != nil {
			walkBlock(*s.ElseBody, visitCall)
		}
	case ast.WhileStmt:
		walkExpr(s.Condition, visitCall)
		walkBlock(s.Body, visitCall)
	case ast.PrometheusStmt:
		walkBlock(s.Body, visitCall)
	case ast.WhenStmt:
		for _, c := range s.Cases {
			walkExpr(c.Condition, visitCall)
			walkWhenAction(c.Action, visitCall)
		}
		walkWhenAction(s.Else, visitCall)
	}
}

func walkWhenAction(action ast.WhenAction, visitCall func(ast.CallExpr)) {
	switch a := action.(type) {
	case ast.WhenReturnAction:
		walkExpr(a.Value, visitCall)
	case ast.WhenBlockAction:
		for _, stmt := range a.Statements {
			walkStmt(stmt, visitCall)
		}
	}
}

func walkExpr(expr ast.Expr, visitCall func(ast.CallExpr)) {
	switch e := expr.(type) {
	case ast.CallExpr:
		visitCall(e)
		walkExpr(e.Callee, visitCall)
		for _, arg := range e.Arguments {
			walkExpr(arg, visitCall)
		}
	case ast.ArrayLiteralExpr:
		for _, elem := range e.Elements {
			walkExpr(elem, visitCall)
		}
	case ast.VectorLiteralExpr:
		for _, elem := range e.Elements {
			walkExpr(elem, visitCall)
		}
	case ast.MatrixLiteralExpr:
		for _, row := range e.Rows {
			for _, elem := range row {
				walkExpr(elem, visitCall)
			}
		}
	case ast.IndexExpr:
		walkExpr(e.Target, visitCall)
		for _, idx := range e.Indices {
			walkExpr(idx, visitCall)
		}
	case ast.FieldAccessExpr:
		walkExpr(e.Target, visitCall)
	case ast.BinaryExpr:
		walkExpr(e.Left, visitCall)
		walkExpr(e.Right, visitCall)
	case ast.UnaryExpr:
		walkExpr(e.Operand, visitCall)
	case ast.RangeExpr:
		walkExpr(e.Start, visitCall)
		walkExpr(e.End, visitCall)
		walkExpr(e.Step, visitCall)
	case ast.ParenExpr:
		walkExpr(e.Inner, visitCall)
	case ast.PropagateExpr:
		walkExpr(e.Inner, visitCall)
	case ast.UnwrapExpr:
		walkExpr(e.Inner, visitCall)
	case ast.SwitchExpr:
		walkExpr(e.Subject, visitCall)
		for _, c := range e.Cases {
			walkExpr(c.Match, visitCall)
			walkExpr(c.Value, visitCall)
		}
		walkExpr(e.Else, visitCall)
	case ast.MatchExpr:
		walkExpr(e.Subject, visitCall)
		for _, c := range e.Cases {
			walkExpr(c.Value, visitCall)
		}
	case ast.IfExpr:
		walkExpr(e.Condition, visitCall)
		walkExpr(e.ThenExpr, visitCall)
		walkExpr(e.ElseExpr, visitCall)
	case ast.UtilityWhenExpr:
		walkExpr(e.Policy.Hysteresis, visitCall)
		walkExpr(e.Policy.MinCommit, visitCall)
		for _, c := range e.Cases {
			walkExpr(c.Value, visitCall)
			walkExpr(c.Condition, visitCall)
			walkExpr(c.Score, visitCall)
		}
		walkExpr(e.Else, visitCall)
	case ast.BatchExpr:
		walkExpr(e.Input, visitCall)
		walkBlock(e.Body, visitCall)
	case ast.RecordLiteralExpr:
		for _, f := range e.Fields {
			walkExpr(f.Value, visitCall)
		}
	case ast.RecordUpdateExpr:
		walkExpr(e.Source, visitCall)
		for _, f := range e.Fields {
			walkExpr(f.Value, visitCall)
		}
	}
}

func existence(path string) string {
	if _, err := os.Stat(path); err == nil {
		return "present"
	}
	return "missing"
}
