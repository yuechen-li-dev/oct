package toolchain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type CompileOptions struct {
	InputPath          string
	OutputPath         string
	EntryPoint         string
	DXCPath            string
	HLSLPath           string
	ExtraDXCArgs       []string
	Validate           bool
	RequireSPIRVVal    bool
	SPIRVValidatorPath string
}

type CompileResult struct {
	InputPath                 string
	HLSLPath                  string
	SPIRVPath                 string
	DXCPath                   string
	SPIRVValidatorPath        string
	EntryPoint                string
	Stage                     vdmir.ShaderStage
	Profile                   string
	DXCArgs                   []string
	SPIRVValidatorArgs        []string
	ValidationAttempted       bool
	ValidationSucceeded       bool
	Requirements              []vdmir.CapabilityRequirement
	TargetEnvironment         string
	RequiredVulkanExtensions  []string
	RequiredSPIRVExtensions   []string
	RequiredSPIRVCapabilities []string
}

type GenerateHeaderOptions struct {
	InputPath          string
	OutputPath         string
	Symbol             string
	EntryPoint         string
	DXCPath            string
	HLSLPath           string
	SPIRVPath          string
	ExtraDXCArgs       []string
	Validate           bool
	RequireSPIRVVal    bool
	SPIRVValidatorPath string
	CommandLine        string
}

type GenerateHeaderResult struct {
	CompileResult
	HeaderPath string
	Symbol     string
}

type host struct {
	getenv   func(string) string
	lookPath func(string) (string, error)
	runner   CommandRunner
}

func defaultHost() host {
	return host{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		runner:   defaultCommandRunner,
	}
}

func CompileToSPIRV(opts CompileOptions) (CompileResult, error) {
	return compileToSPIRV(defaultHost(), opts)
}

func GenerateHeader(opts GenerateHeaderOptions) (GenerateHeaderResult, error) {
	_, mir, _, err := loadModuleAndHLSL(opts.InputPath)
	if err != nil {
		return GenerateHeaderResult{}, err
	}
	entry, err := resolveEntryPoint(mir, opts.EntryPoint)
	if err != nil {
		return GenerateHeaderResult{}, err
	}
	result, err := compileToSPIRV(defaultHost(), CompileOptions{
		InputPath:          opts.InputPath,
		OutputPath:         choosePath(opts.SPIRVPath, replaceExt(opts.OutputPath, ".spv")),
		EntryPoint:         opts.EntryPoint,
		DXCPath:            opts.DXCPath,
		HLSLPath:           choosePath(opts.HLSLPath, replaceExt(opts.OutputPath, ".hlsl")),
		ExtraDXCArgs:       opts.ExtraDXCArgs,
		Validate:           opts.Validate,
		RequireSPIRVVal:    opts.RequireSPIRVVal,
		SPIRVValidatorPath: opts.SPIRVValidatorPath,
	})
	if err != nil {
		return GenerateHeaderResult{}, err
	}
	bytes, err := os.ReadFile(result.SPIRVPath)
	if err != nil {
		return GenerateHeaderResult{}, fmt.Errorf("read SPIR-V %s: %w", result.SPIRVPath, err)
	}
	headerText, err := HeaderFromSPIRVBytes(bytes, HeaderOptions{
		Symbol:      opts.Symbol,
		SourcePath:  opts.InputPath,
		Compute:     computeHeaderMetadata(entry),
		CommandLine: opts.CommandLine,
		HeaderPath:  opts.OutputPath,
	})
	if err != nil {
		return GenerateHeaderResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return GenerateHeaderResult{}, fmt.Errorf("create header directory: %w", err)
	}
	if err := os.WriteFile(opts.OutputPath, []byte(headerText), 0o644); err != nil {
		return GenerateHeaderResult{}, fmt.Errorf("write header %s: %w", opts.OutputPath, err)
	}
	return GenerateHeaderResult{
		CompileResult: result,
		HeaderPath:    opts.OutputPath,
		Symbol:        opts.Symbol,
	}, nil
}

func computeHeaderMetadata(entry vdmir.ComputeEntryPoint) *ComputeHeaderMetadata {
	return &ComputeHeaderMetadata{
		EntryPoint:   entry.EmittedName,
		NumThreadsX:  uint32(entry.NumThreadsX),
		NumThreadsY:  uint32(entry.NumThreadsY),
		NumThreadsZ:  uint32(entry.NumThreadsZ),
		Metadata:     copyMetadataFields(entry.Metadata),
		ConfigValues: copyMetadataFields(entry.ConfigValues),
	}
}

func copyMetadataFields(fields []vdmir.MetadataField) []MetadataField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]MetadataField, 0, len(fields))
	for _, field := range fields {
		out = append(out, MetadataField{Name: field.Name, Value: field.Value})
	}
	return out
}

func compileToSPIRV(host host, opts CompileOptions) (CompileResult, error) {
	if strings.TrimSpace(opts.InputPath) == "" {
		return CompileResult{}, fmt.Errorf("input path must be non-empty")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return CompileResult{}, fmt.Errorf("SPIR-V output path must be non-empty")
	}
	module, mir, hlslText, err := loadModuleAndHLSL(opts.InputPath)
	if err != nil {
		return CompileResult{}, err
	}
	entry, err := resolveShaderEntry(mir, opts.EntryPoint)
	if err != nil {
		return CompileResult{}, err
	}
	if entry.Compute != nil {
		if err := validateEntryParams(module, *entry.Compute); err != nil {
			return CompileResult{}, err
		}
	}
	hlslPath := choosePath(opts.HLSLPath, replaceExt(opts.OutputPath, ".hlsl"))
	hlslPath = mustAbs(hlslPath)
	outputPath := mustAbs(opts.OutputPath)
	if err := os.MkdirAll(filepath.Dir(hlslPath), 0o755); err != nil {
		return CompileResult{}, fmt.Errorf("create HLSL directory: %w", err)
	}
	if err := os.WriteFile(hlslPath, []byte(hlslText), 0o644); err != nil {
		return CompileResult{}, fmt.Errorf("write HLSL %s: %w", hlslPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return CompileResult{}, fmt.Errorf("create SPIR-V directory: %w", err)
	}
	dxcPath, err := resolveDXCPath(host, opts.DXCPath)
	if err != nil {
		return CompileResult{}, err
	}
	target := targetContractForStage(mir, entry.Stage)
	dxcArgs := buildDXCArgsForTarget(entry.Name, outputPath, hlslPath, opts.ExtraDXCArgs, target)
	result, runErr := host.runner(Command{
		Program: dxcPath,
		Args:    dxcArgs,
		Dir:     resolveCommandDir("", filepath.Dir(hlslPath)),
	})
	if runErr != nil {
		return CompileResult{}, fmt.Errorf("dxc failed (exit code %d)\ncommand: %s %s\nstdout:\n%s\nstderr:\n%s", result.ExitCode, filepath.ToSlash(dxcPath), strings.Join(dxcArgs, " "), strings.TrimSpace(result.Stdout), strings.TrimSpace(result.Stderr))
	}
	compileResult := CompileResult{
		InputPath:                 opts.InputPath,
		HLSLPath:                  hlslPath,
		SPIRVPath:                 outputPath,
		DXCPath:                   dxcPath,
		EntryPoint:                entry.Name,
		Stage:                     entry.Stage,
		Profile:                   target.profile,
		DXCArgs:                   append([]string(nil), dxcArgs...),
		Requirements:              append([]vdmir.CapabilityRequirement(nil), mir.Requirements...),
		TargetEnvironment:         target.environment,
		RequiredVulkanExtensions:  append([]string(nil), target.vulkanExtensions...),
		RequiredSPIRVExtensions:   append([]string(nil), target.spirvExtensions...),
		RequiredSPIRVCapabilities: append([]string(nil), target.spirvCapabilities...),
	}
	if opts.Validate || opts.RequireSPIRVVal {
		compileResult.ValidationAttempted = true
		validatorPath, ok, err := resolveSPIRVValidatorPath(host, opts.SPIRVValidatorPath)
		if err != nil {
			return CompileResult{}, err
		}
		if !ok {
			if opts.RequireSPIRVVal {
				return CompileResult{}, fmt.Errorf("spirv-val was requested but not found; pass --require-spirv-val only when spirv-val is installed or set VULKAN_SDK/PATH appropriately")
			}
			return compileResult, nil
		}
		args := []string{outputPath}
		if target.environment != "vulkan1.0" {
			args = []string{"--target-env", target.environment, outputPath}
		}
		validationResult, validationErr := host.runner(Command{
			Program: validatorPath,
			Args:    args,
			Dir:     resolveCommandDir("", filepath.Dir(outputPath)),
		})
		compileResult.SPIRVValidatorPath = validatorPath
		compileResult.SPIRVValidatorArgs = append([]string(nil), args...)
		if validationErr != nil {
			return CompileResult{}, fmt.Errorf("spirv-val failed (exit code %d)\ncommand: %s %s\nstdout:\n%s\nstderr:\n%s", validationResult.ExitCode, filepath.ToSlash(validatorPath), strings.Join(args, " "), strings.TrimSpace(validationResult.Stdout), strings.TrimSpace(validationResult.Stderr))
		}
		compileResult.ValidationSucceeded = true
	}
	return compileResult, nil
}

func loadModuleAndHLSL(path string) (ast.Module, vdmir.Module, string, error) {
	file, err := source.Load(path)
	if err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	if err := validate.Module(module); err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	mir, err := lower.Module(module)
	if err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	text, err := hlsl.Emit(mir)
	if err != nil {
		return ast.Module{}, vdmir.Module{}, "", err
	}
	return module, mir, text, nil
}

func resolveEntryPoint(module vdmir.Module, requested string) (vdmir.ComputeEntryPoint, error) {
	if len(module.EntryPoints) == 0 {
		return vdmir.ComputeEntryPoint{}, fmt.Errorf("module does not contain a compute entry point")
	}
	if requested != "" {
		for _, entry := range module.EntryPoints {
			if entry.EmittedName == requested {
				return entry, nil
			}
		}
		names := make([]string, 0, len(module.EntryPoints))
		for _, entry := range module.EntryPoints {
			names = append(names, entry.EmittedName)
		}
		return vdmir.ComputeEntryPoint{}, fmt.Errorf("compute entry point %q was not found; available entries: %s", requested, strings.Join(names, ", "))
	}
	if len(module.EntryPoints) > 1 {
		names := make([]string, 0, len(module.EntryPoints))
		for _, entry := range module.EntryPoints {
			names = append(names, entry.EmittedName)
		}
		return vdmir.ComputeEntryPoint{}, fmt.Errorf("module contains multiple compute entry points; pass --entry to choose one: %s", strings.Join(names, ", "))
	}
	return module.EntryPoints[0], nil
}

type resolvedShaderEntry struct {
	Name    string
	Stage   vdmir.ShaderStage
	Compute *vdmir.ComputeEntryPoint
	Graphic *vdmir.GraphicsEntryPoint
}

func resolveShaderEntry(module vdmir.Module, requested string) (resolvedShaderEntry, error) {
	entries := make([]resolvedShaderEntry, 0, len(module.EntryPoints)+len(module.GraphicsEntryPoints))
	for i := range module.EntryPoints {
		entry := &module.EntryPoints[i]
		entries = append(entries, resolvedShaderEntry{Name: entry.EmittedName, Stage: vdmir.StageCompute, Compute: entry})
	}
	for i := range module.GraphicsEntryPoints {
		entry := &module.GraphicsEntryPoints[i]
		entries = append(entries, resolvedShaderEntry{Name: entry.EmittedName, Stage: entry.Stage, Graphic: entry})
	}
	if len(entries) == 0 {
		return resolvedShaderEntry{}, fmt.Errorf("module does not contain a shader entry point")
	}
	if requested != "" {
		for _, entry := range entries {
			if entry.Name == requested {
				return entry, nil
			}
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name)
		}
		return resolvedShaderEntry{}, fmt.Errorf("shader entry point %q was not found; available entries: %s", requested, strings.Join(names, ", "))
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name)
		}
		return resolvedShaderEntry{}, fmt.Errorf("module contains multiple shader entry points; pass --entry to choose one: %s", strings.Join(names, ", "))
	}
	return entries[0], nil
}

func validateEntryParams(module ast.Module, entry vdmir.ComputeEntryPoint) error {
	if len(entry.Params) == 0 {
		return nil
	}
	if len(entry.Params) > 1 {
		return fmt.Errorf("compute entry point %s uses %d parameters; GoOct SDSL-V M2 SPIR-V generation currently supports at most one entry parameter lowered as a Vulkan push constant record", entry.EmittedName, len(entry.Params))
	}
	param := entry.Params[0]
	if param.Type.Kind != vdmir.TypeRecord {
		return fmt.Errorf("compute entry point %s parameter %s must be a record for GoOct SDSL-V M2 push-constant lowering", entry.EmittedName, param.Name)
	}
	recordNames := map[string]bool{}
	for _, decl := range module.Decls {
		if record, ok := decl.(ast.RecordDecl); ok {
			recordNames[record.Name] = true
		}
	}
	if !recordNames[param.Type.Name] {
		return fmt.Errorf("compute entry point %s parameter %s record %s was not found", entry.EmittedName, param.Name, param.Type.Name)
	}
	return nil
}

func buildDXCArgs(entry, outputPath, inputPath string, extra []string) []string {
	return buildDXCArgsForTarget(entry, outputPath, inputPath, extra, shaderTargetContract{
		profile: "cs_6_0", environment: "vulkan1.0",
	})
}

type shaderTargetContract struct {
	profile           string
	environment       string
	extraArgs         []string
	vulkanExtensions  []string
	spirvExtensions   []string
	spirvCapabilities []string
}

func targetContract(module vdmir.Module) shaderTargetContract {
	result := shaderTargetContract{profile: "cs_6_0", environment: "vulkan1.0"}
	for _, requirement := range module.Requirements {
		if requirement.Kind != vdmir.CapabilityCooperativeMatrixF16F32M16N16K16Subgroup {
			continue
		}
		result.profile = "cs_6_9"
		result.environment = "vulkan1.3"
		result.extraArgs = []string{"-fspv-use-vulkan-memory-model", "-enable-16bit-types"}
		result.vulkanExtensions = []string{"VK_KHR_cooperative_matrix"}
		result.spirvExtensions = []string{"SPV_KHR_cooperative_matrix"}
		result.spirvCapabilities = []string{"CooperativeMatrixKHR", "Float16", "VulkanMemoryModel"}
	}
	return result
}

func targetContractForStage(module vdmir.Module, stage vdmir.ShaderStage) shaderTargetContract {
	switch stage {
	case vdmir.StageVertex:
		return shaderTargetContract{profile: "vs_6_0", environment: "vulkan1.0"}
	case vdmir.StagePixel:
		return shaderTargetContract{profile: "ps_6_0", environment: "vulkan1.0"}
	default:
		return targetContract(module)
	}
}

func buildDXCArgsForTarget(entry, outputPath, inputPath string, extra []string, target shaderTargetContract) []string {
	args := []string{
		"-spirv",
		"-T", target.profile,
		"-E", entry,
		"-Fo", outputPath,
		"-fspv-target-env=" + target.environment,
		"-O3",
	}
	args = append(args, target.extraArgs...)
	args = append(args, extra...)
	args = append(args, inputPath)
	return args
}

func resolveDXCPath(host host, explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return mustAbs(path), nil
	}
	if path := strings.TrimSpace(host.getenv("SDSLV_DXC")); path != "" {
		return mustAbs(path), nil
	}
	for _, candidate := range toolCandidates("dxc") {
		if path, err := host.lookPath(candidate); err == nil {
			return path, nil
		}
	}
	if sdk := strings.TrimSpace(host.getenv("VULKAN_SDK")); sdk != "" {
		candidate := filepath.Join(sdk, "Bin", executableName("dxc"))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("dxc was not found; pass --dxc <path>, set SDSLV_DXC, add dxc to PATH, or install a Vulkan SDK that provides dxc")
}

func resolveSPIRVValidatorPath(host host, explicit string) (string, bool, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return mustAbs(path), true, nil
	}
	for _, candidate := range toolCandidates("spirv-val") {
		if path, err := host.lookPath(candidate); err == nil {
			return path, true, nil
		}
	}
	if sdk := strings.TrimSpace(host.getenv("VULKAN_SDK")); sdk != "" {
		candidate := filepath.Join(sdk, "Bin", executableName("spirv-val"))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

func toolCandidates(base string) []string {
	if runtime.GOOS == "windows" {
		return []string{base, base + ".exe"}
	}
	return []string{base}
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func choosePath(explicit, fallback string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return fallback
}

func replaceExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
