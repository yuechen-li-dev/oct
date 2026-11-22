package builtin

var namespaceAliases = map[string]map[string]string{
	"Array": {
		"CrossSection": "ArrayCrossSection",
		"Where":        "ArrayWhere",
	},
	// Libraries/UI placement builtins that are polymorphic over the app's
	// region enum, so they cannot be ordinary UI package functions.
	"UI": {
		"Region":  "UIRegion",
		"Arrange": "UIArrange",
		"Button":  "UIButtonMsg",
		// Inputs are generic over the message their callback builds.
		"Checkbox":  "UICheckbox",
		"TextField": "UITextField",
		"Slider":    "UISlider",
	},
	"String": {
		"Concat":     "StringConcat",
		"From":       "StringFrom",
		"Join":       "StringJoin",
		"ReplaceAll": "StringReplaceAll",
		"Contains":   "StringContains",
		"StartsWith": "StringStartsWith",
		"EndsWith":   "StringEndsWith",
		"Trim":       "StringTrim",
		"SplitLines": "StringSplitLines",
		"EscapeJson": "StringEscapeJSON",
		"EscapeJSON": "StringEscapeJSON",
		"QuoteJson":  "StringQuoteJSON",
		"QuoteJSON":  "StringQuoteJSON",
		"ByteLength": "StringByteLength",
		"RuneCount":  "StringRuneCount",
	},
	"IO": {
		"ReadText":   "FileReadText",
		"WriteText":  "FileWriteText",
		"ReadLines":  "FileReadLines",
		"WriteLines": "FileWriteLines",
	},
	"Csv": {
		"Read":        "CsvRead",
		"ReadRows":    "CsvReadRows",
		"ReadTable":   "CsvReadTable",
		"ReadMatrix":  "CsvReadMatrix",
		"Write":       "CsvWrite",
		"WriteRows":   "CsvWriteRows",
		"WriteTable":  "CsvWriteTable",
		"WriteMatrix": "CsvWriteMatrix",
	},
	"Artifact": {
		"WriteText":         "ArtifactWriteText",
		"WriteLines":        "ArtifactWriteLines",
		"WriteMarkdown":     "ArtifactWriteMarkdown",
		"WriteCsv":          "ArtifactWriteCsv",
		"WriteOctagon":      "ArtifactWriteOctagon",
		"Markdown":          "ArtifactDocumentMarkdown",
		"Docx":              "ArtifactDocumentDocx",
		"Latex":             "ArtifactDocumentLatex",
		"Pdf":               "ArtifactDocumentPdf",
		"WriteCompiledData": "ArtifactCompileData",
		"Progress":          "ArtifactProgress",
		"Checkpoint":        "ArtifactCheckpoint",
	},
	"Markdown": {
		"H1":               "MarkdownH1",
		"H2":               "MarkdownH2",
		"H3":               "MarkdownH3",
		"Title":            "MarkdownH1",
		"Subtitle":         "MarkdownH2",
		"Paragraph":        "MarkdownParagraph",
		"Blank":            "MarkdownBlank",
		"HorizontalRule":   "MarkdownHorizontalRule",
		"Bullets":          "MarkdownBullets",
		"Numbered":         "MarkdownNumbered",
		"CodeBlock":        "MarkdownCodeBlock",
		"Callout":          "MarkdownCallout",
		"Image":            "MarkdownImage",
		"Figure":           "MarkdownFigure",
		"Table":            "MarkdownTable",
		"TableWithColumns": "MarkdownTableWithColumns",
		"KeyValueTable":    "MarkdownKeyValueTable",
		"Section":          "MarkdownSection",
		"Subsection":       "MarkdownSubsection",
		"Report":           "MarkdownReport",
		"EscapeText":       "MarkdownEscapeText",
		"EscapeTableCell":  "MarkdownEscapeTableCell",
	},
}

func ResolveNamespacedAlias(namespace string, symbol string) (string, bool) {
	ns, ok := namespaceAliases[namespace]
	if !ok {
		return "", false
	}
	name, ok := ns[symbol]
	return name, ok
}

func IsCompilerOwnedNamespace(namespace string) bool {
	switch namespace {
	case "Array", "Artifact", EntropyNamespace, JsonNamespace:
		return true
	default:
		return false
	}
}

// ArtifactPhaseSpelling returns the written name of a builtin that exists only
// during `oct artifact` evaluation, such as "Artifact.WriteText" for
// "ArtifactWriteText". The second result is false for any other builtin.
func ArtifactPhaseSpelling(name string) (string, bool) {
	for symbol, canonical := range namespaceAliases["Artifact"] {
		if canonical == name {
			return "Artifact." + symbol, true
		}
	}
	return "", false
}

// markupParameters names the parameters of compiler-owned builtins that can
// be written as Oct-XML tags, in call order. They have no Oct declaration to
// take the names from (they are generic over the app's message enum), and
// they take no body: every parameter is an attribute.
var markupParameters = map[string][]string{
	"UIButtonMsg": {"label", "on", "enabled"},
	"UICheckbox":  {"label", "checked", "on"},
	"UITextField": {"value", "on"},
	"UISlider":    {"value", "min", "max", "on"},
}

// MarkupParameters reports the parameter names of a compiler-owned builtin
// written as a markup tag.
func MarkupParameters(builtinName string) ([]string, bool) {
	names, ok := markupParameters[builtinName]
	return names, ok
}
