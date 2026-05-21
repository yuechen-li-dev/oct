package builtin

var namespaceAliases = map[string]map[string]string{
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
	"Json": {
		"Load": "JsonLoad",
		"Save": "JsonSave",
	},
	"Artifact": {
		"WriteText":     "ArtifactWriteText",
		"WriteLines":    "ArtifactWriteLines",
		"WriteMarkdown": "ArtifactWriteMarkdown",
		"WriteCsv":      "ArtifactWriteCsv",
		"WriteJson":     "ArtifactWriteJson",
		"WriteOctagon":  "ArtifactWriteOctagon",
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
