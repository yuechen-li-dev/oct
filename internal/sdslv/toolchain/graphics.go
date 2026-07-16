package toolchain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

const GraphicsBundleSchema = "sdslv.graphics-program-bundle.v1"

type GraphicsBundleOptions struct {
	InputPath          string
	OutputDirectory    string
	Program            string
	DXCPath            string
	ExtraDXCArgs       []string
	RequireSPIRVVal    bool
	SPIRVValidatorPath string
}

type GraphicsBundle struct {
	Schema               string              `json:"schema"`
	Program              string              `json:"program"`
	ReplayIdentity       string              `json:"replay_identity"`
	Source               BundleSource        `json:"source"`
	Compiler             BundleCompiler      `json:"compiler"`
	Vertex               BundleStageArtifact `json:"vertex"`
	Pixel                BundleStageArtifact `json:"pixel"`
	Interface            BundleInterface     `json:"interface"`
	Resources            []BundleResource    `json:"resources"`
	Material             *BundleMaterial     `json:"material,omitempty"`
	RequiredCapabilities []string            `json:"required_capabilities"`
	ManifestPath         string              `json:"-"`
}

type BundleSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type BundleCompiler struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type BundleStageArtifact struct {
	Stage             string               `json:"stage"`
	EntryPoint        string               `json:"entry_point"`
	Profile           string               `json:"profile"`
	TargetEnvironment string               `json:"target_environment"`
	HLSLPath          string               `json:"hlsl_path"`
	HLSLSHA256        string               `json:"hlsl_sha256"`
	SPIRVPath         string               `json:"spirv_path"`
	SPIRVSHA256       string               `json:"spirv_sha256"`
	DXCArgs           []string             `json:"dxc_args"`
	SPIRVValidated    bool                 `json:"spirv_validated"`
	StructuralFacts   StructuralSPIRVFacts `json:"structural_spirv_facts"`
}

type StructuralSPIRVFacts struct {
	ExecutionModel string          `json:"execution_model"`
	EntryPoint     string          `json:"entry_point"`
	Locations      []uint32        `json:"locations"`
	Builtins       []string        `json:"builtins"`
	Bindings       []BundleBinding `json:"bindings"`
}

type BundleInterface struct {
	VertexInputs []BundleInterfaceField `json:"vertex_inputs"`
	Varyings     []BundleInterfaceField `json:"varyings"`
	PixelInputs  []BundleInterfaceField `json:"pixel_inputs"`
	Builtins     []BundleBuiltin        `json:"builtins"`
	PixelTargets []BundlePixelTarget    `json:"pixel_targets"`
}

type BundleInterfaceField struct {
	Stream        string `json:"stream"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Location      int    `json:"location"`
	Interpolation string `json:"interpolation"`
}

type BundleBuiltin struct {
	Stage    string `json:"stage"`
	Name     string `json:"name"`
	Builtin  string `json:"builtin"`
	Type     string `json:"type"`
	Semantic string `json:"semantic"`
}

type BundlePixelTarget struct {
	Name   string `json:"name"`
	Target int    `json:"target"`
	Type   string `json:"type"`
}

type BundleBinding struct {
	Set     uint32 `json:"set"`
	Binding uint32 `json:"binding"`
}

type BundleResource struct {
	Name    string        `json:"name"`
	Kind    string        `json:"kind"`
	Type    string        `json:"type"`
	Access  string        `json:"access"`
	Binding BundleBinding `json:"binding"`
}

type BundleMaterial struct {
	TypeName string                `json:"type_name"`
	Binding  BundleBinding         `json:"binding"`
	Size     uint32                `json:"size"`
	Fields   []BundleMaterialField `json:"fields"`
}

type BundleMaterialField struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Offset    uint32 `json:"offset"`
	Size      uint32 `json:"size"`
	Alignment uint32 `json:"alignment"`
}

func CompileGraphicsBundle(opts GraphicsBundleOptions) (GraphicsBundle, error) {
	if strings.TrimSpace(opts.OutputDirectory) == "" {
		return GraphicsBundle{}, fmt.Errorf("graphics bundle output directory must be non-empty")
	}
	// Validation evidence is part of the bundle contract, not an optional CLI
	// convenience. Keep the option for API compatibility but always require the
	// validator for a successfully published graphics bundle.
	opts.RequireSPIRVVal = true
	_, mir, _, err := loadModuleAndHLSL(opts.InputPath)
	if err != nil {
		return GraphicsBundle{}, err
	}
	program, vertex, pixel, err := selectGraphicsProgram(mir, opts.Program)
	if err != nil {
		return GraphicsBundle{}, err
	}
	if err := os.MkdirAll(opts.OutputDirectory, 0o755); err != nil {
		return GraphicsBundle{}, fmt.Errorf("create graphics bundle directory: %w", err)
	}
	compile := func(stage string, entry vdmir.GraphicsEntryPoint) (CompileResult, error) {
		base := sanitizeBundleName(program.Name) + "." + stage
		return CompileToSPIRV(CompileOptions{
			InputPath:          opts.InputPath,
			OutputPath:         filepath.Join(opts.OutputDirectory, base+".spv"),
			EntryPoint:         entry.EmittedName,
			DXCPath:            opts.DXCPath,
			HLSLPath:           filepath.Join(opts.OutputDirectory, base+".hlsl"),
			ExtraDXCArgs:       append([]string(nil), opts.ExtraDXCArgs...),
			Validate:           true,
			RequireSPIRVVal:    opts.RequireSPIRVVal,
			SPIRVValidatorPath: opts.SPIRVValidatorPath,
		})
	}
	vertexResult, err := compile("vertex", vertex)
	if err != nil {
		return GraphicsBundle{}, err
	}
	pixelResult, err := compile("pixel", pixel)
	if err != nil {
		return GraphicsBundle{}, err
	}
	vertexArtifact, err := bundleStageArtifact(opts.OutputDirectory, vertexResult)
	if err != nil {
		return GraphicsBundle{}, err
	}
	pixelArtifact, err := bundleStageArtifact(opts.OutputDirectory, pixelResult)
	if err != nil {
		return GraphicsBundle{}, err
	}
	sourceBytes, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return GraphicsBundle{}, err
	}
	bundle := GraphicsBundle{
		Schema:               GraphicsBundleSchema,
		Program:              program.Name,
		Source:               BundleSource{Path: filepath.ToSlash(opts.InputPath), SHA256: hashBytes(sourceBytes)},
		Compiler:             BundleCompiler{Name: filepath.Base(vertexResult.DXCPath), SHA256: hashFile(vertexResult.DXCPath)},
		Vertex:               vertexArtifact,
		Pixel:                pixelArtifact,
		Interface:            bundleInterface(vertex, pixel),
		Resources:            bundleResources(mir.Resources),
		RequiredCapabilities: bundleCapabilities(mir),
	}
	for _, material := range mir.Materials {
		if material.ShaderName == program.Name {
			bundle.Material = bundleMaterial(material)
			break
		}
	}
	identityProjection := bundle
	identityProjection.ReplayIdentity = ""
	identityProjection.ManifestPath = ""
	identityBytes, _ := json.Marshal(identityProjection)
	bundle.ReplayIdentity = hashBytes(identityBytes)
	manifestBytes, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return GraphicsBundle{}, err
	}
	manifestBytes = append(manifestBytes, '\n')
	manifestPath := filepath.Join(opts.OutputDirectory, sanitizeBundleName(program.Name)+".bundle.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return GraphicsBundle{}, fmt.Errorf("write graphics bundle manifest: %w", err)
	}
	bundle.ManifestPath = manifestPath
	return bundle, nil
}

func selectGraphicsProgram(module vdmir.Module, requested string) (vdmir.GraphicsProgram, vdmir.GraphicsEntryPoint, vdmir.GraphicsEntryPoint, error) {
	if len(module.GraphicsPrograms) == 0 {
		return vdmir.GraphicsProgram{}, vdmir.GraphicsEntryPoint{}, vdmir.GraphicsEntryPoint{}, fmt.Errorf("module does not contain a paired vertex/pixel graphics program")
	}
	var program *vdmir.GraphicsProgram
	for i := range module.GraphicsPrograms {
		if requested == "" && len(module.GraphicsPrograms) == 1 || module.GraphicsPrograms[i].Name == requested {
			program = &module.GraphicsPrograms[i]
			break
		}
	}
	if program == nil {
		if requested == "" {
			return vdmir.GraphicsProgram{}, vdmir.GraphicsEntryPoint{}, vdmir.GraphicsEntryPoint{}, fmt.Errorf("module contains multiple graphics programs; select one explicitly")
		}
		return vdmir.GraphicsProgram{}, vdmir.GraphicsEntryPoint{}, vdmir.GraphicsEntryPoint{}, fmt.Errorf("graphics program %q was not found", requested)
	}
	var vertex, pixel vdmir.GraphicsEntryPoint
	for _, entry := range module.GraphicsEntryPoints {
		if entry.EmittedName == program.Vertex {
			vertex = entry
		}
		if entry.EmittedName == program.Pixel {
			pixel = entry
		}
	}
	if vertex.EmittedName == "" || pixel.EmittedName == "" {
		return vdmir.GraphicsProgram{}, vdmir.GraphicsEntryPoint{}, vdmir.GraphicsEntryPoint{}, fmt.Errorf("graphics program %s has incomplete entry-point ownership", program.Name)
	}
	return *program, vertex, pixel, nil
}

func bundleStageArtifact(root string, result CompileResult) (BundleStageArtifact, error) {
	facts, err := inspectSPIRV(result.SPIRVPath, result.EntryPoint, result.Stage)
	if err != nil {
		return BundleStageArtifact{}, err
	}
	return BundleStageArtifact{
		Stage: string(result.Stage), EntryPoint: result.EntryPoint, Profile: result.Profile,
		TargetEnvironment: result.TargetEnvironment,
		HLSLPath:          filepath.ToSlash(relativeOrBase(root, result.HLSLPath)), HLSLSHA256: hashFile(result.HLSLPath),
		SPIRVPath: filepath.ToSlash(relativeOrBase(root, result.SPIRVPath)), SPIRVSHA256: hashFile(result.SPIRVPath),
		DXCArgs: normalizeDXCArgs(result.DXCArgs), SPIRVValidated: result.ValidationSucceeded,
		StructuralFacts: facts,
	}, nil
}

func inspectSPIRV(path, entry string, stage vdmir.ShaderStage) (StructuralSPIRVFacts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StructuralSPIRVFacts{}, err
	}
	if len(data) < 20 || len(data)%4 != 0 || binary.LittleEndian.Uint32(data[:4]) != 0x07230203 {
		return StructuralSPIRVFacts{}, fmt.Errorf("%s is not a valid SPIR-V module", path)
	}
	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	facts := StructuralSPIRVFacts{EntryPoint: entry}
	locations := map[uint32]bool{}
	builtins := map[string]bool{}
	sets, bindings := map[uint32]uint32{}, map[uint32]uint32{}
	for i := 5; i < len(words); {
		count, opcode := int(words[i]>>16), uint16(words[i]&0xffff)
		if count <= 0 || i+count > len(words) {
			return StructuralSPIRVFacts{}, fmt.Errorf("malformed SPIR-V instruction at word %d", i)
		}
		operands := words[i+1 : i+count]
		switch opcode {
		case 15: // OpEntryPoint
			if len(operands) >= 3 {
				name, _ := spirvString(operands[2:])
				if name == entry {
					facts.ExecutionModel = executionModelName(operands[0])
				}
			}
		case 71: // OpDecorate
			if len(operands) >= 3 {
				id, decoration, value := operands[0], operands[1], operands[2]
				switch decoration {
				case 11:
					builtins[builtinName(value)] = true
				case 30:
					locations[value] = true
				case 33:
					bindings[id] = value
				case 34:
					sets[id] = value
				}
			}
		}
		i += count
	}
	wantModel := map[vdmir.ShaderStage]string{vdmir.StageVertex: "Vertex", vdmir.StagePixel: "Fragment"}[stage]
	if facts.EntryPoint != entry || facts.ExecutionModel != wantModel {
		return StructuralSPIRVFacts{}, fmt.Errorf("SPIR-V entry %s execution model = %s, want %s", entry, facts.ExecutionModel, wantModel)
	}
	for location := range locations {
		facts.Locations = append(facts.Locations, location)
	}
	for builtin := range builtins {
		facts.Builtins = append(facts.Builtins, builtin)
	}
	for id, binding := range bindings {
		facts.Bindings = append(facts.Bindings, BundleBinding{Set: sets[id], Binding: binding})
	}
	sort.Slice(facts.Locations, func(i, j int) bool { return facts.Locations[i] < facts.Locations[j] })
	sort.Strings(facts.Builtins)
	sort.Slice(facts.Bindings, func(i, j int) bool {
		if facts.Bindings[i].Set != facts.Bindings[j].Set {
			return facts.Bindings[i].Set < facts.Bindings[j].Set
		}
		return facts.Bindings[i].Binding < facts.Bindings[j].Binding
	})
	return facts, nil
}

// InspectGraphicsSPIRV re-derives the normalized, runtime-independent facts
// recorded in a graphics bundle. It is exported for repository and independent
// artifact verification; it does not expose the compiler AST or VD-MIR.
func InspectGraphicsSPIRV(path, entry, stage string) (StructuralSPIRVFacts, error) {
	shaderStage := vdmir.ShaderStage(stage)
	if shaderStage != vdmir.StageVertex && shaderStage != vdmir.StagePixel {
		return StructuralSPIRVFacts{}, fmt.Errorf("unsupported graphics SPIR-V stage %q", stage)
	}
	return inspectSPIRV(path, entry, shaderStage)
}

func bundleInterface(vertex, pixel vdmir.GraphicsEntryPoint) BundleInterface {
	result := BundleInterface{}
	result.VertexInputs = bundleInterfaceFields(vertex.Inputs)
	result.Varyings = bundleInterfaceFields(vertex.Outputs)
	result.PixelInputs = bundleInterfaceFields(pixel.Inputs)
	for _, entry := range []vdmir.GraphicsEntryPoint{vertex, pixel} {
		for _, builtin := range entry.Builtins {
			result.Builtins = append(result.Builtins, BundleBuiltin{Stage: string(entry.Stage), Name: builtin.Name, Builtin: builtin.Builtin, Type: bundleType(builtin.Type), Semantic: builtin.Semantic})
		}
	}
	for _, target := range pixel.Targets {
		result.PixelTargets = append(result.PixelTargets, BundlePixelTarget{Name: target.Name, Target: target.Target, Type: bundleType(target.Type)})
	}
	return result
}

func bundleInterfaceFields(fields []vdmir.InterfaceField) []BundleInterfaceField {
	out := make([]BundleInterfaceField, 0, len(fields))
	for _, field := range fields {
		out = append(out, BundleInterfaceField{Stream: field.Stream, Name: field.Name, Type: bundleType(field.Type), Location: field.Location, Interpolation: field.Interpolation})
	}
	return out
}

func bundleResources(resources []vdmir.Resource) []BundleResource {
	out := make([]BundleResource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, BundleResource{Name: resource.Name, Kind: string(resource.Kind), Type: bundleType(resource.Type), Access: string(resource.Access), Binding: BundleBinding{Set: uint32(resource.Binding.Set), Binding: uint32(resource.Binding.Binding)}})
	}
	return out
}

func bundleMaterial(material vdmir.Material) *BundleMaterial {
	out := &BundleMaterial{TypeName: material.TypeName, Binding: BundleBinding{Set: uint32(material.Binding.Set), Binding: uint32(material.Binding.Binding)}, Size: material.Size}
	for _, field := range material.Fields {
		out.Fields = append(out.Fields, BundleMaterialField{Name: field.Name, Type: bundleType(field.Type), Offset: field.Offset, Size: field.Size, Alignment: field.Alignment})
	}
	return out
}

func bundleCapabilities(module vdmir.Module) []string {
	out := make([]string, 0, len(module.Requirements))
	for _, requirement := range module.Requirements {
		out = append(out, requirement.Kind)
	}
	sort.Strings(out)
	return out
}

func bundleType(typ vdmir.Type) string {
	name := typ.Name
	if typ.Element != nil {
		name += "<" + bundleType(*typ.Element) + ">"
	}
	if typ.Space != "" {
		name += "@space(" + typ.Space + ")"
	}
	return name
}

func normalizeDXCArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := range out {
		if filepath.IsAbs(out[i]) {
			out[i] = filepath.Base(out[i])
		}
		out[i] = filepath.ToSlash(out[i])
	}
	return out
}

func relativeOrBase(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(path)
	}
	return rel
}

func spirvString(words []uint32) (string, int) {
	var bytes []byte
	for i, word := range words {
		for shift := 0; shift < 32; shift += 8 {
			b := byte(word >> shift)
			if b == 0 {
				return string(bytes), i + 1
			}
			bytes = append(bytes, b)
		}
	}
	return string(bytes), len(words)
}

func executionModelName(model uint32) string {
	switch model {
	case 0:
		return "Vertex"
	case 4:
		return "Fragment"
	case 5:
		return "GLCompute"
	default:
		return fmt.Sprintf("ExecutionModel(%d)", model)
	}
}

func builtinName(value uint32) string {
	switch value {
	case 0:
		return "Position"
	case 5:
		return "VertexId"
	case 6:
		return "InstanceId"
	case 15:
		return "FragCoord"
	case 17:
		return "FrontFacing"
	case 42:
		return "VertexIndex"
	case 43:
		return "InstanceIndex"
	default:
		return fmt.Sprintf("BuiltIn(%d)", value)
	}
}

func sanitizeBundleName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	return hashBytes(data)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
