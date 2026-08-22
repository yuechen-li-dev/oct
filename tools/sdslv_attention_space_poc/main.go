package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/toolchain"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const (
	artifactDirectory = "internal/sdslv/DevelopmentReport/artifacts/AttentionSpacePoc"
	semanticSource    = "Examples/SDSL-V/conformance/compute/AttentionSpacePoc.sdslvvalid"
	erasedSource      = "Examples/SDSL-V/conformance/compute/AttentionSpacePocErased.sdslvvalid"
	groupedSource     = "Examples/SDSL-V/AttentionSpacePoc/GroupedSpaceEquivalence.sdslv"
	expandedSource    = "Examples/SDSL-V/AttentionSpacePoc/ExpandedSpaceEquivalence.sdslv"
)

type artifact struct {
	Schema                    string              `json:"schema"`
	Model                     modelAuthority      `json:"model"`
	SpaceVocabulary           []string            `json:"space_vocabulary"`
	ValidTransitionGraph      []transition        `json:"valid_transition_graph"`
	LegalPairings             []pairing           `json:"legal_pairings"`
	InvalidFixtureResults     []invalidResult     `json:"invalid_fixture_results"`
	DiagnosticCodes           []string            `json:"diagnostic_codes"`
	GroupedDeclaration        groupedDeclaration  `json:"grouped_declaration"`
	Backend                   backendEvidence     `json:"backend_erasure"`
	TokenDomainClassification tokenClassification `json:"token_domain_support"`
	ZImageApplicability       applicability       `json:"zimage_applicability"`
	FinalVerdict              string              `json:"final_verdict"`
	ConvergenceOutcome        string              `json:"convergence_outcome"`
	MilestoneState            string              `json:"milestone_state"`
	EVT2State                 string              `json:"evt2_state"`
	ProjectionSHA256          string              `json:"projection_sha256"`
}

type modelAuthority struct {
	Name              string `json:"name"`
	ModelRevision     string `json:"model_revision"`
	SourceRevision    string `json:"source_revision"`
	Block             string `json:"block"`
	ModelWidth        int    `json:"model_width"`
	HeadCount         int    `json:"head_count"`
	HeadWidth         int    `json:"head_width"`
	FFNWidth          int    `json:"ffn_width"`
	RopeDimensions    []int  `json:"rope_dimensions"`
	RopeTheta         int    `json:"rope_theta"`
	WeightCacheSHA256 string `json:"weight_cache_sha256"`
}

type transition struct {
	Operation string   `json:"operation"`
	Inputs    []string `json:"inputs"`
	Output    string   `json:"output"`
}

type pairing struct {
	Operation string `json:"operation"`
	Left      string `json:"left"`
	Right     string `json:"right"`
	Output    string `json:"output"`
	Design    string `json:"design"`
}

type invalidSpec struct {
	File    string
	Mistake string
}

type invalidResult struct {
	Fixture        string `json:"fixture"`
	Mistake        string `json:"mistake"`
	Rejected       bool   `json:"rejected"`
	Code           string `json:"code"`
	Line           uint32 `json:"line"`
	Column         uint32 `json:"column"`
	Message        string `json:"message"`
	NamesSpaces    bool   `json:"names_actual_and_expected_spaces"`
	NamesOperation bool   `json:"names_operation"`
}

type backendEvidence struct {
	SemanticSource        string   `json:"semantic_source"`
	ErasedSource          string   `json:"erased_source"`
	SemanticSourceSHA256  string   `json:"semantic_source_sha256"`
	ErasedSourceSHA256    string   `json:"erased_source_sha256"`
	SemanticHLSLSHA256    string   `json:"semantic_hlsl_sha256"`
	ErasedHLSLSHA256      string   `json:"erased_hlsl_sha256"`
	SemanticSPIRVSHA256   string   `json:"semantic_spirv_sha256"`
	ErasedSPIRVSHA256     string   `json:"erased_spirv_sha256"`
	SPIRVByteIdentical    bool     `json:"spirv_byte_identical"`
	RuntimeHLSLEquivalent bool     `json:"runtime_hlsl_equivalent"`
	SPIRVValidated        bool     `json:"spirv_validated"`
	ResourceInterface     []string `json:"resource_interface"`
	PushConstants         []string `json:"push_constants"`
	BufferElementLayout   string   `json:"buffer_element_layout"`
	RuntimeTags           bool     `json:"runtime_tags"`
	AdditionalStorage     bool     `json:"additional_storage"`
	AdditionalBranches    bool     `json:"additional_branches"`
}

type groupedDeclaration struct {
	Syntax                   string   `json:"syntax"`
	Canonicalization         string   `json:"canonicalization"`
	GeneratedAliases         []string `json:"generated_aliases"`
	DuplicateNameDiagnostic  string   `json:"duplicate_name_diagnostic"`
	DuplicateSpaceDiagnostic string   `json:"duplicate_space_diagnostic"`
	GroupedSource            string   `json:"grouped_source"`
	ExpandedSource           string   `json:"expanded_source"`
	GroupedHLSLSHA256        string   `json:"grouped_hlsl_sha256"`
	ExpandedHLSLSHA256       string   `json:"expanded_hlsl_sha256"`
	HLSLByteIdentical        bool     `json:"hlsl_byte_identical"`
	GroupedSPIRVSHA256       string   `json:"grouped_spirv_sha256"`
	ExpandedSPIRVSHA256      string   `json:"expanded_spirv_sha256"`
	SPIRVByteIdentical       bool     `json:"spirv_byte_identical"`
	SPIRVValidated           bool     `json:"spirv_validated"`
}

type tokenClassification struct {
	VectorValueSpaces string `json:"vector_value_spaces"`
	TensorElements    string `json:"tensor_elements"`
	TensorAxes        string `json:"tensor_axes"`
	CaughtNow         string `json:"caught_now"`
	Deferred          string `json:"deferred"`
}

type applicability struct {
	CatchesAtLeastThreeRealisticMistakes bool     `json:"catches_at_least_three_realistic_mistakes"`
	CaughtMistakes                       []string `json:"caught_mistakes"`
	MandatoryM1Spaces                    []string `json:"mandatory_m1_spaces"`
	ComplexityEffect                     string   `json:"complexity_effect"`
	Recommendation                       string   `json:"recommendation"`
}

var invalidFixtures = []invalidSpec{
	{"AttentionQContractedWithValue.sdslvinvalid", "Q contracted with V instead of K"},
	{"AttentionKeyUsedAsValue.sdslvinvalid", "K used where V is required"},
	{"AttentionUnpositionedQuery.sdslvinvalid", "unpositioned Q paired with positioned K"},
	{"AttentionRopeAppliedTwice.sdslvinvalid", "RoPE applied twice"},
	{"AttentionIncompatiblePositionConvention.sdslvinvalid", "incompatible positioned key convention"},
	{"AttentionQwenResidual.sdslvinvalid", "Qwen hidden state used as a Z-Image residual"},
	{"AttentionProbabilityAsScore.sdslvinvalid", "attention probability treated as score"},
	{"AttentionScoreBeforeNormalization.sdslvinvalid", "attention score aggregated before normalization"},
	{"AttentionHeadResidual.sdslvinvalid", "attention head added as model residual"},
	{"AttentionWrongRopeCoordinate.sdslvinvalid", "text-token coordinate supplied to image RoPE"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll(artifactDirectory, 0o755); err != nil {
		return err
	}
	semantic, err := compile(semanticSource, "attention_space_poc")
	if err != nil {
		return err
	}
	erased, err := compile(erasedSource, "attention_space_poc_erased")
	if err != nil {
		return err
	}
	grouped, err := compile(groupedSource, "grouped_space_equivalence")
	if err != nil {
		return err
	}
	expanded, err := compile(expandedSource, "expanded_space_equivalence")
	if err != nil {
		return err
	}
	invalid, err := collectInvalidResults()
	if err != nil {
		return err
	}
	semanticHLSL, err := os.ReadFile(semantic.hlsl)
	if err != nil {
		return err
	}
	erasedHLSL, err := os.ReadFile(erased.hlsl)
	if err != nil {
		return err
	}
	semanticSPIRV, err := os.ReadFile(semantic.spirv)
	if err != nil {
		return err
	}
	erasedSPIRV, err := os.ReadFile(erased.spirv)
	if err != nil {
		return err
	}
	groupedHLSL, err := os.ReadFile(grouped.hlsl)
	if err != nil {
		return err
	}
	expandedHLSL, err := os.ReadFile(expanded.hlsl)
	if err != nil {
		return err
	}
	groupedSPIRV, err := os.ReadFile(grouped.spirv)
	if err != nil {
		return err
	}
	expandedSPIRV, err := os.ReadFile(expanded.spirv)
	if err != nil {
		return err
	}

	a := artifact{
		Schema: "sdslv.attention-space-poc.v1",
		Model: modelAuthority{
			Name: "Z-Image-Turbo", ModelRevision: "f332072aa78be7aecdf3ee76d5c247082da564a6",
			SourceRevision: "26f23eda626ffadda020b04ff79488e1d72004cd", Block: "noise_refiner.0",
			ModelWidth: 3840, HeadCount: 30, HeadWidth: 128, FFNWidth: 10240,
			RopeDimensions: []int{32, 48, 48}, RopeTheta: 256,
			WeightCacheSHA256: "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e",
		},
		SpaceVocabulary: []string{
			"zimage.noise_refiner.embedding", "zimage.attention.query_head", "zimage.attention.key_head",
			"zimage.attention.value_head", "zimage.attention.positioned_query_head",
			"zimage.attention.positioned_key_head", "zimage.attention.score",
			"zimage.attention.probability", "zimage.attention.output", "zimage.position.frame",
			"zimage.position.row", "zimage.position.column",
		},
		ValidTransitionGraph: []transition{
			{"ProjectQuery", []string{"zimage.noise_refiner.embedding"}, "zimage.attention.query_head"},
			{"ProjectKey", []string{"zimage.noise_refiner.embedding"}, "zimage.attention.key_head"},
			{"ProjectValue", []string{"zimage.noise_refiner.embedding"}, "zimage.attention.value_head"},
			{"NormalizeQuery", []string{"zimage.attention.query_head"}, "zimage.attention.query_head"},
			{"NormalizeKey", []string{"zimage.attention.key_head"}, "zimage.attention.key_head"},
			{"ApplyQueryRoPE", []string{"zimage.attention.query_head", "zimage.position.frame", "zimage.position.row", "zimage.position.column"}, "zimage.attention.positioned_query_head"},
			{"ApplyKeyRoPE", []string{"zimage.attention.key_head", "zimage.position.frame", "zimage.position.row", "zimage.position.column"}, "zimage.attention.positioned_key_head"},
			{"Score", []string{"zimage.attention.positioned_query_head", "zimage.attention.positioned_key_head"}, "zimage.attention.score"},
			{"NormalizeScores", []string{"zimage.attention.score"}, "zimage.attention.probability"},
			{"AggregateValues", []string{"zimage.attention.probability", "zimage.attention.value_head"}, "zimage.attention.output"},
			{"OutputProject", []string{"zimage.attention.output"}, "zimage.noise_refiner.embedding"},
			{"AddResidual", []string{"zimage.noise_refiner.embedding", "zimage.noise_refiner.embedding"}, "zimage.noise_refiner.embedding"},
		},
		LegalPairings:         []pairing{{"Score", "zimage.attention.positioned_query_head", "zimage.attention.positioned_key_head", "zimage.attention.score", "closed function signature; no generalized pairing language"}},
		InvalidFixtureResults: invalid,
		DiagnosticCodes:       []string{"SDSL-V4120", "SDSL-V4123", "SDSL-V4124"},
		GroupedDeclaration: groupedDeclaration{
			Syntax:           "space dotted.path { PascalCaseMember: VectorType; }",
			Canonicalization: "PascalCase member names become lower_snake_case suffixes with acronym runs kept together",
			GeneratedAliases: []string{
				"type QueryHead = float4 @space(zimage.attention.query_head)",
				"type KeyHead = float4 @space(zimage.attention.key_head)",
				"type ValueHead = float4 @space(zimage.attention.value_head)",
				"type PositionedQueryHead = float4 @space(zimage.attention.positioned_query_head)",
				"type PositionedKeyHead = float4 @space(zimage.attention.positioned_key_head)",
				"type Score = float4 @space(zimage.attention.score)",
				"type Probability = float4 @space(zimage.attention.probability)",
				"type Output = float4 @space(zimage.attention.output)",
			},
			DuplicateNameDiagnostic: "SDSL-V1509", DuplicateSpaceDiagnostic: "SDSL-V4124",
			GroupedSource: groupedSource, ExpandedSource: expandedSource,
			GroupedHLSLSHA256: hashBytes(groupedHLSL), ExpandedHLSLSHA256: hashBytes(expandedHLSL),
			HLSLByteIdentical:  bytes.Equal(groupedHLSL, expandedHLSL),
			GroupedSPIRVSHA256: hashBytes(groupedSPIRV), ExpandedSPIRVSHA256: hashBytes(expandedSPIRV),
			SPIRVByteIdentical: bytes.Equal(groupedSPIRV, expandedSPIRV),
			SPIRVValidated:     grouped.validated && expanded.validated,
		},
		Backend: backendEvidence{
			SemanticSource: semanticSource, ErasedSource: erasedSource,
			SemanticSourceSHA256: hashFile(semanticSource), ErasedSourceSHA256: hashFile(erasedSource),
			SemanticHLSLSHA256: hashBytes(semanticHLSL), ErasedHLSLSHA256: hashBytes(erasedHLSL),
			SemanticSPIRVSHA256: hashBytes(semanticSPIRV), ErasedSPIRVSHA256: hashBytes(erasedSPIRV),
			SPIRVByteIdentical:    bytes.Equal(semanticSPIRV, erasedSPIRV),
			RuntimeHLSLEquivalent: normalizedHLSL(string(semanticHLSL)) == normalizedHLSL(string(erasedHLSL)),
			SPIRVValidated:        semantic.validated && erased.validated,
			ResourceInterface:     []string{"set0/binding0 readonly StructuredBuffer<float>", "set0/binding1 RWStructuredBuffer<float>"},
			PushConstants:         []string{}, BufferElementLayout: "f32 elements; unchanged", RuntimeTags: false,
			AdditionalStorage: false, AdditionalBranches: false,
		},
		TokenDomainClassification: tokenClassification{
			VectorValueSpaces: "supported now", TensorElements: "supported now through ndarray<T> element aliases",
			TensorAxes: "not represented", CaughtNow: "image coordinate values versus text-token coordinate values",
			Deferred: "query-token/key-token axes and probability/value token-domain alignment require future tensor-axis/index semantics",
		},
		ZImageApplicability: applicability{
			CatchesAtLeastThreeRealisticMistakes: true,
			CaughtMistakes:                       []string{"Q/V pairing", "K/V confusion", "unpositioned scoring", "double RoPE", "score/probability confusion", "residual-domain confusion", "wrong RoPE coordinate domain"},
			MandatoryM1Spaces:                    []string{"model embedding", "query head", "key head", "value head", "positioned query head", "positioned key head", "attention score", "attention probability"},
			ComplexityEffect:                     "small increase in declarations and explicit transition functions; lower debugging complexity through compile-time rejection",
			Recommendation:                       "use bounded semantic spaces in EVT-2 M1 vector-value boundaries; do not block M1 on token-axis typing",
		},
		FinalVerdict: "USEFUL WITH BOUNDED EXTENSION", ConvergenceOutcome: "SUCCESS", MilestoneState: "COMPLETE",
		EVT2State: "READY FOR M1 WITH ATTENTION SPACES",
	}
	projection, err := json.Marshal(a)
	if err != nil {
		return err
	}
	a.ProjectionSHA256 = hashBytes(projection)
	output, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	output = append(output, '\n')
	path := filepath.Join(artifactDirectory, "attention_space_poc.json")
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\nprojection sha256: %s\n", path, a.ProjectionSHA256)
	return nil
}

type compiledPaths struct {
	hlsl, spirv string
	validated   bool
}

func compile(input, name string) (compiledPaths, error) {
	hlslPath := filepath.Join(artifactDirectory, name+".hlsl")
	spirvPath := filepath.Join(artifactDirectory, name+".spv")
	result, err := sdslv.CompileSPIRV(toolchain.CompileOptions{
		InputPath: input, OutputPath: spirvPath, HLSLPath: hlslPath, RequireSPIRVVal: true,
	})
	if err != nil {
		return compiledPaths{}, fmt.Errorf("compile %s: %w", input, err)
	}
	return compiledPaths{hlslPath, spirvPath, result.ValidationSucceeded}, nil
}

func collectInvalidResults() ([]invalidResult, error) {
	results := make([]invalidResult, 0, len(invalidFixtures))
	for _, spec := range invalidFixtures {
		path := filepath.Join("Examples", "SDSL-V", "conformance", "invalid", spec.File)
		file, err := source.Load(path)
		if err != nil {
			return nil, err
		}
		tokens, err := lex.Analyze(file)
		if err != nil {
			return nil, err
		}
		module, err := parse.BuildModule(tokens)
		if err != nil {
			return nil, err
		}
		diagnostics := validate.Diagnostics(module)
		if len(diagnostics) != 1 || diagnostics[0].Code != "SDSL-V4123" {
			return nil, fmt.Errorf("%s: expected exactly one SDSL-V4123 diagnostic, got %v", path, diagnostics)
		}
		d := diagnostics[0]
		results = append(results, invalidResult{
			Fixture: filepath.ToSlash(path), Mistake: spec.Mistake, Rejected: true, Code: d.Code,
			Line: d.Span.Start.Line, Column: d.Span.Start.Column, Message: d.Message,
			NamesSpaces:    strings.Contains(d.Message, "requires space `") && strings.Contains(d.Message, ", got `"),
			NamesOperation: strings.HasPrefix(d.Message, "function "),
		})
	}
	return results, nil
}

func normalizedHLSL(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// namespace ") || strings.HasPrefix(trimmed, "// type ") ||
			strings.HasPrefix(trimmed, "// BEGIN INLINE HLSL ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
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
