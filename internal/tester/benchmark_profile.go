package tester

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	pprofprofile "github.com/google/pprof/profile"

	"github.com/yuechen-li-dev/oct/internal/interpret"
)

type benchmarkProfiler struct {
	path       string
	format     string
	octagonOut string
	rawOut     string
	captureOut string
	captureTmp bool
	file       *os.File
	stopped    bool
}

type BenchmarkProfileInputs struct {
	Run        BenchmarkRun
	RootPath   string
	Invocation string
	Filter     string
}

type BenchmarkProfileReport struct {
	RunID        string
	Mode         string
	TimestampUTC string
	DurationNs   int64
	SampleType   string
	SampleUnit   string
	RootPath     string
	Invocation   string
	Filter       string
	Benchmarks   []string
	Environment  string
	TopFunctions []BenchmarkProfileFunction
	TopModules   []BenchmarkProfileModule
	RawPprofPath string
	OctagonPath  string
}

type BenchmarkProfileFunction struct {
	Name             string
	Module           string
	File             string
	Flat             int64
	Cumulative       int64
	PercentTotalFlat float64
	Callers          []string
	Callees          []string
}

type BenchmarkProfileModule struct {
	Name       string
	Flat       int64
	Cumulative int64
	Percent    float64
}

type functionAccumulator struct {
	Name       string
	Module     string
	File       string
	Flat       int64
	Cumulative int64
	Callers    map[string]struct{}
	Callees    map[string]struct{}
}

func newBenchmarkProfiler(path string, options BenchmarkOptions) (*benchmarkProfiler, error) {
	captureOut := options.ProfileOutPath
	captureTmp := false
	if captureOut == "" {
		captureOut = DefaultCPUProfilePath(path)
	}
	if options.ProfileFormat == "octagon" {
		f, err := os.CreateTemp("", "oct-bench-*.cpu.pprof")
		if err != nil {
			return nil, err
		}
		captureOut = f.Name()
		_ = f.Close()
		captureTmp = true
	}
	if err := os.MkdirAll(filepath.Dir(captureOut), 0o755); err != nil {
		return nil, err
	}
	file, err := os.Create(captureOut)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &benchmarkProfiler{
		path:       path,
		format:     options.ProfileFormat,
		octagonOut: options.ProfileOctagonOutPath,
		rawOut:     options.ProfileRawOut,
		captureOut: captureOut,
		captureTmp: captureTmp,
		file:       file,
	}, nil
}

func (p *benchmarkProfiler) Stop() {
	if p.stopped {
		return
	}
	pprof.StopCPUProfile()
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
	p.stopped = true
}

func (p *benchmarkProfiler) Finalize(inputs BenchmarkProfileInputs) (BenchmarkProfileReport, error) {
	report, err := buildProfileReportFromPprof(p.captureOut, inputs)
	if err != nil {
		return BenchmarkProfileReport{}, err
	}
	report.OctagonPath = p.octagonOut
	if p.format == "pprof" {
		report.OctagonPath = ""
	}
	if p.rawOut != "" {
		report.RawPprofPath = p.rawOut
	}
	if p.rawOut != "" && p.captureOut != p.rawOut {
		if err := os.MkdirAll(filepath.Dir(p.rawOut), 0o755); err != nil {
			return BenchmarkProfileReport{}, err
		}
		if err := copyFile(p.captureOut, p.rawOut); err != nil {
			return BenchmarkProfileReport{}, err
		}
	}
	if p.captureTmp {
		_ = os.Remove(p.captureOut)
	}
	return report, nil
}

func buildProfileReportFromPprof(path string, inputs BenchmarkProfileInputs) (BenchmarkProfileReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return BenchmarkProfileReport{}, err
	}
	defer file.Close()
	parsed, err := pprofprofile.Parse(file)
	if err != nil {
		return BenchmarkProfileReport{}, err
	}
	if len(parsed.SampleType) == 0 {
		return BenchmarkProfileReport{}, fmt.Errorf("invalid pprof profile: missing sample types")
	}
	benchmarks := make([]string, 0, len(inputs.Run.Cases))
	for _, c := range inputs.Run.Cases {
		benchmarks = append(benchmarks, c.Name)
	}
	sort.Strings(benchmarks)
	total := int64(0)
	functions := map[string]*functionAccumulator{}
	for _, sample := range parsed.Sample {
		if len(sample.Value) == 0 {
			continue
		}
		weight := sample.Value[0]
		total += weight
		stack := flattenSampleFunctions(sample, functions)
		if len(stack) == 0 {
			continue
		}
		leaf := stack[0]
		leaf.Flat += weight
		for i, fn := range stack {
			fn.Cumulative += weight
			if i > 0 {
				caller := stack[i].Name
				callee := stack[i-1].Name
				stack[i].Callees[callee] = struct{}{}
				stack[i-1].Callers[caller] = struct{}{}
			}
		}
	}
	topFunctions := sortedFunctions(functions, total, 10)
	topModules := topModulesFromFunctions(topFunctions, total)
	timestamp := time.Now().UTC()
	report := BenchmarkProfileReport{
		RunID:        fmt.Sprintf("bench-profile-%d", timestamp.UnixNano()),
		Mode:         "cpu",
		TimestampUTC: timestamp.Format(time.RFC3339Nano),
		DurationNs:   int64(parsed.DurationNanos),
		SampleType:   parsed.SampleType[0].Type,
		SampleUnit:   parsed.SampleType[0].Unit,
		RootPath:     inputs.RootPath,
		Invocation:   inputs.Invocation,
		Filter:       inputs.Filter,
		Benchmarks:   benchmarks,
		Environment:  fmt.Sprintf("goos=%s goarch=%s", runtime.GOOS, runtime.GOARCH),
		TopFunctions: topFunctions,
		TopModules:   topModules,
	}
	return report, nil
}

func flattenSampleFunctions(sample *pprofprofile.Sample, functions map[string]*functionAccumulator) []*functionAccumulator {
	stack := make([]*functionAccumulator, 0, len(sample.Location))
	seen := map[string]struct{}{}
	for _, location := range sample.Location {
		for _, line := range location.Line {
			if line.Function == nil {
				continue
			}
			name := line.Function.Name
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entry, ok := functions[name]
			if !ok {
				entry = &functionAccumulator{
					Name:    name,
					Module:  moduleFromFunction(name),
					File:    line.Function.Filename,
					Callers: map[string]struct{}{},
					Callees: map[string]struct{}{},
				}
				functions[name] = entry
			}
			stack = append(stack, entry)
		}
	}
	return stack
}

func sortedFunctions(functions map[string]*functionAccumulator, total int64, limit int) []BenchmarkProfileFunction {
	items := make([]*functionAccumulator, 0, len(functions))
	for _, fn := range functions {
		items = append(items, fn)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Flat != items[j].Flat {
			return items[i].Flat > items[j].Flat
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]BenchmarkProfileFunction, 0, len(items))
	for _, fn := range items {
		callers := keysSorted(fn.Callers)
		callees := keysSorted(fn.Callees)
		pct := 0.0
		if total > 0 {
			pct = (float64(fn.Flat) / float64(total)) * 100
		}
		out = append(out, BenchmarkProfileFunction{
			Name:             fn.Name,
			Module:           fn.Module,
			File:             fn.File,
			Flat:             fn.Flat,
			Cumulative:       fn.Cumulative,
			PercentTotalFlat: pct,
			Callers:          trimSlice(callers, 3),
			Callees:          trimSlice(callees, 3),
		})
	}
	return out
}

func topModulesFromFunctions(functions []BenchmarkProfileFunction, total int64) []BenchmarkProfileModule {
	type agg struct {
		flat int64
		cum  int64
	}
	modules := map[string]agg{}
	for _, fn := range functions {
		current := modules[fn.Module]
		current.flat += fn.Flat
		current.cum += fn.Cumulative
		modules[fn.Module] = current
	}
	out := make([]BenchmarkProfileModule, 0, len(modules))
	for name, values := range modules {
		pct := 0.0
		if total > 0 {
			pct = (float64(values.flat) / float64(total)) * 100
		}
		out = append(out, BenchmarkProfileModule{Name: name, Flat: values.flat, Cumulative: values.cum, Percent: pct})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Flat != out[j].Flat {
			return out[i].Flat > out[j].Flat
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func benchmarkProfileToOctagonValue(report BenchmarkProfileReport) interpret.Value {
	benchmarks := make([]interpret.Value, 0, len(report.Benchmarks))
	for _, b := range report.Benchmarks {
		benchmarks = append(benchmarks, interpret.Value{Kind: interpret.ValueString, Text: b})
	}
	functions := make([]interpret.Value, 0, len(report.TopFunctions))
	for _, fn := range report.TopFunctions {
		callers := make([]interpret.Value, 0, len(fn.Callers))
		for _, caller := range nonEmptyStrings(fn.Callers, "none") {
			callers = append(callers, interpret.Value{Kind: interpret.ValueString, Text: caller})
		}
		callees := make([]interpret.Value, 0, len(fn.Callees))
		for _, callee := range nonEmptyStrings(fn.Callees, "none") {
			callees = append(callees, interpret.Value{Kind: interpret.ValueString, Text: callee})
		}
		functions = append(functions, interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
			TypeName:   "BenchmarkProfileFunction",
			FieldOrder: []string{"Name", "Module", "File", "Flat", "Cumulative", "PercentTotalFlat", "Callers", "Callees"},
			Fields: map[string]interpret.Value{
				"Name":             {Kind: interpret.ValueString, Text: fn.Name},
				"Module":           {Kind: interpret.ValueString, Text: fn.Module},
				"File":             {Kind: interpret.ValueString, Text: fn.File},
				"Flat":             {Kind: interpret.ValueInt, Int: fn.Flat},
				"Cumulative":       {Kind: interpret.ValueInt, Int: fn.Cumulative},
				"PercentTotalFlat": {Kind: interpret.ValueFloat, Float: fn.PercentTotalFlat},
				"Callers":          {Kind: interpret.ValueArray, Array: callers},
				"Callees":          {Kind: interpret.ValueArray, Array: callees},
			},
		}})
	}
	if len(functions) == 0 {
		functions = append(functions, interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
			TypeName:   "BenchmarkProfileFunction",
			FieldOrder: []string{"Name", "Module", "File", "Flat", "Cumulative", "PercentTotalFlat", "Callers", "Callees"},
			Fields: map[string]interpret.Value{
				"Name":             {Kind: interpret.ValueString, Text: "no_samples"},
				"Module":           {Kind: interpret.ValueString, Text: "no_samples"},
				"File":             {Kind: interpret.ValueString, Text: ""},
				"Flat":             {Kind: interpret.ValueInt, Int: 0},
				"Cumulative":       {Kind: interpret.ValueInt, Int: 0},
				"PercentTotalFlat": {Kind: interpret.ValueFloat, Float: 0},
				"Callers":          {Kind: interpret.ValueArray, Array: []interpret.Value{{Kind: interpret.ValueString, Text: "none"}}},
				"Callees":          {Kind: interpret.ValueArray, Array: []interpret.Value{{Kind: interpret.ValueString, Text: "none"}}},
			},
		}})
	}
	modules := make([]interpret.Value, 0, len(report.TopModules))
	for _, mod := range report.TopModules {
		modules = append(modules, interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
			TypeName:   "BenchmarkProfileModule",
			FieldOrder: []string{"Name", "Flat", "Cumulative", "Percent"},
			Fields: map[string]interpret.Value{
				"Name":       {Kind: interpret.ValueString, Text: mod.Name},
				"Flat":       {Kind: interpret.ValueInt, Int: mod.Flat},
				"Cumulative": {Kind: interpret.ValueInt, Int: mod.Cumulative},
				"Percent":    {Kind: interpret.ValueFloat, Float: mod.Percent},
			},
		}})
	}
	if len(modules) == 0 {
		modules = append(modules, interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
			TypeName:   "BenchmarkProfileModule",
			FieldOrder: []string{"Name", "Flat", "Cumulative", "Percent"},
			Fields: map[string]interpret.Value{
				"Name":       {Kind: interpret.ValueString, Text: "no_samples"},
				"Flat":       {Kind: interpret.ValueInt, Int: 0},
				"Cumulative": {Kind: interpret.ValueInt, Int: 0},
				"Percent":    {Kind: interpret.ValueFloat, Float: 0},
			},
		}})
	}
	return interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
		TypeName:   "BenchmarkProfileReport",
		FieldOrder: []string{"RunID", "Mode", "TimestampUTC", "DurationNs", "SampleType", "SampleUnit", "RootPath", "Invocation", "Filter", "Environment", "Benchmarks", "TopFunctions", "TopModules", "RawPprofPath"},
		Fields: map[string]interpret.Value{
			"RunID":        {Kind: interpret.ValueString, Text: report.RunID},
			"Mode":         {Kind: interpret.ValueString, Text: report.Mode},
			"TimestampUTC": {Kind: interpret.ValueString, Text: report.TimestampUTC},
			"DurationNs":   {Kind: interpret.ValueInt, Int: report.DurationNs},
			"SampleType":   {Kind: interpret.ValueString, Text: report.SampleType},
			"SampleUnit":   {Kind: interpret.ValueString, Text: report.SampleUnit},
			"RootPath":     {Kind: interpret.ValueString, Text: report.RootPath},
			"Invocation":   {Kind: interpret.ValueString, Text: report.Invocation},
			"Filter":       {Kind: interpret.ValueString, Text: report.Filter},
			"Environment":  {Kind: interpret.ValueString, Text: report.Environment},
			"Benchmarks":   {Kind: interpret.ValueArray, Array: benchmarks},
			"TopFunctions": {Kind: interpret.ValueArray, Array: functions},
			"TopModules":   {Kind: interpret.ValueArray, Array: modules},
			"RawPprofPath": {Kind: interpret.ValueString, Text: report.RawPprofPath},
		},
	}}
}

func writeProfileSummary(stdout io.Writer, report BenchmarkProfileReport) {
	_, _ = fmt.Fprintf(stdout, "PROFILE mode=%s sample=%s/%s duration=%dns artifact=%s", report.Mode, report.SampleType, report.SampleUnit, report.DurationNs, report.OctagonPath)
	if report.RawPprofPath != "" {
		_, _ = fmt.Fprintf(stdout, " raw=%s", report.RawPprofPath)
	}
	_, _ = fmt.Fprintln(stdout)
	max := len(report.TopFunctions)
	if max > 5 {
		max = 5
	}
	for i := 0; i < max; i++ {
		fn := report.TopFunctions[i]
		_, _ = fmt.Fprintf(stdout, "PROFILE HOT %d %s flat=%d cum=%d pct=%.2f%%\n", i+1, fn.Name, fn.Flat, fn.Cumulative, fn.PercentTotalFlat)
	}
}

func moduleFromFunction(name string) string {
	last := strings.LastIndex(name, "/")
	name = name[last+1:]
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name
	}
	return name[:dot]
}

func keysSorted(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func trimSlice(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func nonEmptyStrings(values []string, fallback string) []string {
	if len(values) != 0 {
		return values
	}
	return []string{fallback}
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
