package document

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LatexBundle is the stable textual publishing artifact and the files it
// references. Paths are always slash-separated and relative to the .tex file.
type LatexBundle struct {
	Source []byte
	Files  []LatexFile
}

type LatexFile struct {
	Path  string
	Bytes []byte
}

type latexRenderer struct {
	doc            Doc
	out            strings.Builder
	filesByHash    map[string]LatexFile
	figurePaths    map[string]string
	bibliography   map[string]string
	equationNumber int
	sectionNumber  int
	err            error
}

var unsafeEquationCommands = regexp.MustCompile(`(?i)\\(begin|end|documentclass|usepackage|input|include|write|openout|read|catcode|def|edef|gdef|xdef|newcommand|renewcommand|providecommand|csname|special|immediate|loop|repeat)\b`)

func Latex(doc Doc) (LatexBundle, error) {
	if diagnostics := ValidateLatex(doc); len(diagnostics) != 0 {
		return LatexBundle{}, fmt.Errorf("Document LaTeX validation: %s", strings.Join(diagnostics, "; "))
	}
	r := &latexRenderer{doc: doc, filesByHash: map[string]LatexFile{}, figurePaths: map[string]string{}, bibliography: map[string]string{}}
	r.collectFiles(doc.Content)
	if r.err != nil {
		return LatexBundle{}, r.err
	}
	r.writePreamble()
	r.writeBlocks(doc.Content)
	r.out.WriteString("\\end{document}\n")
	if r.err != nil {
		return LatexBundle{}, r.err
	}
	files := make([]LatexFile, 0, len(r.filesByHash))
	for _, file := range r.filesByHash {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return LatexBundle{Source: []byte(r.out.String()), Files: files}, nil
}

func ValidateLatex(doc Doc) []string {
	diagnostics := Validate(doc)
	var visit func([]Block)
	visit = func(blocks []Block) {
		for _, block := range blocks {
			switch block.Kind {
			case EquationBlockKind:
				if unsafeEquationCommands.MatchString(block.Equation.Latex) {
					diagnostics = append(diagnostics, "Document equation contains a document-level LaTeX command")
				}
			case FigureBlockKind:
				if block.Figure.Placement.Kind == AnchoredPlacement {
					a := block.Figure.Placement.Anchored
					if a.Anchor != PageAnchor && a.Anchor != MarginAnchor {
						diagnostics = append(diagnostics, "Document LaTeX anchored figures support only Page and Margin anchors")
					}
					if a.ZOrder > 1 {
						diagnostics = append(diagnostics, "Document LaTeX anchored figures support only the baseline z-order (0 or 1)")
					}
				}
			case SectionBlockKind:
				visit(block.Section.Children)
			case AbstractBlockKind:
				visit(block.Abstract.Children)
			case GroupBlockKind:
				visit(block.Children)
			}
		}
	}
	visit(doc.Content)
	return diagnostics
}

func (r *latexRenderer) collectFiles(blocks []Block) {
	for _, block := range blocks {
		switch block.Kind {
		case FigureBlockKind:
			r.collectFigure(block.Figure)
		case BibliographyBlockKind:
			r.collectBibliography(block.Bibliography)
		case SectionBlockKind:
			r.collectFiles(block.Section.Children)
		case AbstractBlockKind:
			r.collectFiles(block.Abstract.Children)
		case GroupBlockKind:
			r.collectFiles(block.Children)
		}
	}
}

func (r *latexRenderer) readSource(path string) ([]byte, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(r.doc.SourceRoot, filepath.FromSlash(path))
	}
	return os.ReadFile(resolved)
}

func (r *latexRenderer) collectFigure(figure Figure) {
	if r.err != nil {
		return
	}
	payload, err := r.readSource(figure.Source)
	if err != nil {
		r.err = fmt.Errorf("Document figure %q: missing image %s: %w", figure.ID, figure.Source, err)
		return
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || (format != "png" && format != "jpeg") {
		r.err = fmt.Errorf("Document figure %q: unsupported or invalid image %s (PNG and JPEG are supported)", figure.ID, figure.Source)
		return
	}
	ext := ".png"
	if format == "jpeg" {
		ext = ".jpg"
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	path := "assets/image-" + hash[:16] + ext
	r.filesByHash["image:"+hash] = LatexFile{Path: path, Bytes: payload}
	r.figurePaths[figure.Source] = path
}

func (r *latexRenderer) collectBibliography(bibliography Bibliography) {
	if r.err != nil {
		return
	}
	payload, err := r.readSource(bibliography.Source)
	if err != nil {
		r.err = fmt.Errorf("Document bibliography: missing source %s: %w", bibliography.Source, err)
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	path := "assets/references-" + hash[:16] + ".bib"
	r.filesByHash["bib:"+hash] = LatexFile{Path: path, Bytes: payload}
	r.bibliography[bibliography.Source] = strings.TrimSuffix(path, ".bib")
}

func (r *latexRenderer) writePreamble() {
	page := "letterpaper"
	if r.doc.Layout.Size == A4 {
		page = "a4paper"
	}
	if r.doc.Layout.Orientation == Landscape {
		page += ",landscape"
	}
	r.out.WriteString("% Generated by OctCument. Document.Doc is the semantic source of truth.\n")
	r.out.WriteString("\\documentclass[11pt]{article}\n")
	fmt.Fprintf(&r.out, "\\usepackage[%s,top=%.3fpt,bottom=%.3fpt,left=%.3fpt,right=%.3fpt]{geometry}\n", page, r.doc.Layout.Margins.TopPt, r.doc.Layout.Margins.BottomPt, r.doc.Layout.Margins.LeftPt, r.doc.Layout.Margins.RightPt)
	r.out.WriteString("\\usepackage[T1]{fontenc}\n\\usepackage[utf8]{inputenc}\n\\usepackage{lmodern}\n")
	r.out.WriteString("\\usepackage{graphicx}\n\\usepackage{hyperref}\n\\usepackage{booktabs}\n\\usepackage{array}\n\\usepackage{tabularx}\n\\usepackage{amsmath}\n\\usepackage{caption}\n\\usepackage{fancyhdr}\n\\usepackage{float}\n\\usepackage{ragged2e}\n\\usepackage{xcolor}\n")
	if r.hasAnchored(r.doc.Content) {
		r.out.WriteString("\\usepackage[absolute,overlay]{textpos}\n")
	}
	r.out.WriteString("\\hypersetup{unicode=true,colorlinks=true,linkcolor=blue,urlcolor=blue,pdfcreator={OctCument},pdftitle={" + escapeLatexText(r.doc.Metadata.Title) + "},pdfauthor={" + escapeLatexText(r.doc.Metadata.Author) + "}}\n")
	body := r.doc.Style.Body
	fmt.Fprintf(&r.out, "\\AtBeginDocument{\\fontsize{%spt}{%spt}\\selectfont}\n", latexNumber(body.Text.SizePt), latexNumber(body.Text.SizePt*body.LineSpacing))
	r.writeChrome()
	r.out.WriteString("\\begin{document}\n")
}

func (r *latexRenderer) hasAnchored(blocks []Block) bool {
	for _, block := range blocks {
		if block.Kind == FigureBlockKind && block.Figure.Placement.Kind == AnchoredPlacement {
			return true
		}
		if block.Kind == GroupBlockKind && r.hasAnchored(block.Children) {
			return true
		}
		if block.Kind == SectionBlockKind && r.hasAnchored(block.Section.Children) {
			return true
		}
		if block.Kind == AbstractBlockKind && r.hasAnchored(block.Abstract.Children) {
			return true
		}
	}
	return false
}

func (r *latexRenderer) writeChrome() {
	var chrome *PageChrome
	var find func([]Block)
	find = func(blocks []Block) {
		for _, block := range blocks {
			if block.Kind == PageChromeBlockKind {
				value := block.PageChrome
				chrome = &value
			}
			if block.Kind == GroupBlockKind {
				find(block.Children)
			}
			if block.Kind == SectionBlockKind {
				find(block.Section.Children)
			}
		}
	}
	find(r.doc.Content)
	if chrome == nil {
		return
	}
	r.out.WriteString("\\pagestyle{fancy}\n\\fancyhf{}\n")
	r.out.WriteString("\\fancyhead[C]{" + r.inlines(chrome.Header) + "}\n")
	r.out.WriteString("\\fancyfoot[C]{" + r.inlines(chrome.Footer) + "}\n")
	r.out.WriteString("\\renewcommand{\\headrulewidth}{0.4pt}\n")
}

func (r *latexRenderer) writeBlocks(blocks []Block) {
	for _, block := range blocks {
		r.writeBlock(block)
	}
}

func (r *latexRenderer) writeBlock(block Block) {
	if r.err != nil {
		return
	}
	switch block.Kind {
	case HeadingBlockKind:
		r.writeHeading(block.Heading)
	case ParagraphBlockKind:
		r.writeParagraph(block.Paragraph)
	case ListBlockKind:
		env := "itemize"
		if block.List.Kind == NumberedList {
			env = "enumerate"
		}
		r.out.WriteString("\\begin{" + env + "}\n")
		for _, item := range block.List.Items {
			r.out.WriteString("  \\item " + r.inlines(item) + "\n")
		}
		r.out.WriteString("\\end{" + env + "}\n\n")
	case TableBlockKind:
		r.writeTable(block.Table, nil)
	case LabeledTableBlockKind:
		r.writeTable(block.LabeledTable.Table, &block.LabeledTable)
	case FigureBlockKind:
		r.writeFigure(block.Figure)
	case PageChromeBlockKind:
		// Page chrome is emitted once from the preamble.
	case CodeBlockKind:
		r.out.WriteString("{" + latexStyleCommands(r.doc.Style.Code) + "\\begin{verbatim}\n" + strings.Join(block.Code.Lines, "\n") + "\n\\end{verbatim}}\n\n")
	case CalloutBlockKind:
		r.out.WriteString("\\begin{quote}\n\\textbf{" + escapeLatexText(calloutLabel(block.Callout.Kind)) + ":} " + r.inlines(block.Callout.Content) + "\n\\end{quote}\n\n")
	case HorizontalRuleBlockKind:
		r.out.WriteString("\\noindent\\rule{\\linewidth}{0.4pt}\n\n")
	case PageBreakBlockKind:
		r.out.WriteString("\\clearpage\n\n")
	case GroupBlockKind:
		r.writeBlocks(block.Children)
	case EquationBlockKind:
		r.writeEquation(block.Equation)
	case BibliographyBlockKind:
		base := r.bibliography[block.Bibliography.Source]
		r.out.WriteString("\\bibliographystyle{plain}\n\\bibliography{" + escapeLatexPath(base) + "}\n\n")
	case SectionBlockKind:
		r.sectionNumber++
		command := "section"
		if block.Section.Level == 2 {
			command = "subsection"
		}
		style := r.doc.Style.Heading1
		if block.Section.Level == 2 {
			style = r.doc.Style.Heading2
		}
		r.out.WriteString("\\" + command + "*{{" + latexStyleCommands(style) + strconv.Itoa(r.sectionNumber) + "\\quad " + r.inlines(block.Section.Title) + "}}")
		if block.Section.ID != "" {
			r.out.WriteString("\\label{" + latexLabel("sec", block.Section.ID) + "}")
		}
		r.out.WriteString("\n\n")
		r.writeBlocks(block.Section.Children)
	case AbstractBlockKind:
		body := r.doc.Style.Body
		r.out.WriteString("\\begin{abstract}\n")
		r.out.WriteString("\\fontsize{" + latexNumber(body.Text.SizePt) + "pt}{" + latexNumber(body.Text.SizePt*body.LineSpacing) + "pt}\\selectfont\\RaggedRight\n")
		r.writeBlocks(block.Abstract.Children)
		r.out.WriteString("\\end{abstract}\n\n")
	}
}

func (r *latexRenderer) writeHeading(heading Heading) {
	content := r.inlines(heading.Content)
	switch heading.Role {
	case TitleRole:
		r.out.WriteString("\\title{{" + latexStyleCommands(r.doc.Style.Title) + content + "}}\n")
		if r.doc.Metadata.Author != "" {
			r.out.WriteString("\\author{" + escapeLatexText(r.doc.Metadata.Author) + "}\n")
		}
		r.out.WriteString("\\date{}\n\\maketitle\n\n")
	default:
		command := "section"
		style := r.doc.Style.Heading1
		if heading.Level == 2 {
			command = "subsection"
			style = r.doc.Style.Heading2
		}
		r.out.WriteString("\\" + command + "*{{" + latexStyleCommands(style) + content + "}}\n\n")
	}
}

func (r *latexRenderer) writeParagraph(paragraph Paragraph) {
	style := r.doc.Style.Body
	switch paragraph.Role {
	case SubtitleRole:
		style = r.doc.Style.Subtitle
	case CaptionRole:
		style = r.doc.Style.Caption
	case CodeRole:
		style = r.doc.Style.Code
	case SmallRole:
		style = r.doc.Style.Small
	}
	environment := ""
	if style.Alignment == Center {
		environment = "center"
	} else if style.Alignment == Right {
		environment = "flushright"
	}
	if style.SpaceBeforePt > 0 {
		r.out.WriteString("\\vspace*{" + latexNumber(style.SpaceBeforePt) + "pt}\n")
	}
	if environment != "" {
		r.out.WriteString("\\begin{" + environment + "}\n")
	}
	r.out.WriteString("{" + latexStyleCommands(style) + r.inlines(paragraph.Content) + "}\n")
	if environment != "" {
		r.out.WriteString("\\end{" + environment + "}\n")
	}
	if style.SpaceAfterPt > 0 {
		r.out.WriteString("\\vspace*{" + latexNumber(style.SpaceAfterPt) + "pt}\n")
	}
	r.out.WriteString("\n")
}

func (r *latexRenderer) writeTable(table Table, labeled *LabeledTable) {
	columns := make([]string, len(table.Header.Cells))
	for i, cell := range table.Header.Cells {
		columns[i] = latexAlignment(cell.Alignment)
	}
	r.out.WriteString("\\begin{table}[H]\n\\centering\n\\begin{tabularx}{\\linewidth}{" + strings.Join(columns, "") + "}\n\\toprule\n")
	r.writeRow(table.Header, true)
	r.out.WriteString("\\midrule\n")
	for _, row := range table.Body {
		r.writeRow(row, false)
	}
	r.out.WriteString("\\bottomrule\n\\end{tabularx}\n")
	if labeled != nil && len(labeled.Caption) != 0 {
		r.out.WriteString("\\caption*{{" + latexStyleCommands(r.doc.Style.Caption) + r.inlines(labeled.Caption) + "}}\n")
	}
	if labeled != nil && labeled.ID != "" {
		r.out.WriteString("\\label{" + latexLabel("tab", labeled.ID) + "}\n")
	}
	r.out.WriteString("\\end{table}\n\n")
}

func (r *latexRenderer) writeRow(row Row, header bool) {
	parts := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		parts[i] = r.inlines(cell.Content)
		if header {
			parts[i] = "\\textbf{" + parts[i] + "}"
		}
	}
	r.out.WriteString(strings.Join(parts, " & ") + " \\\\\n")
}

func (r *latexRenderer) writeFigure(figure Figure) {
	path := escapeLatexPath(r.figurePaths[figure.Source])
	size := figure.Placement.Auto.Size
	if figure.Placement.Kind == AnchoredPlacement {
		size = figure.Placement.Anchored.Size
	}
	options := "width=" + latexLength(size.Width)
	if size.Height != nil {
		options += ",height=" + latexLength(*size.Height)
	}
	image := "\\includegraphics[" + options + "]{" + path + "}"
	caption := ""
	if len(figure.Caption) != 0 {
		caption = "\\caption*{{" + latexStyleCommands(r.doc.Style.Caption) + r.inlines(figure.Caption) + "}}\n"
	}
	label := ""
	if figure.ID != "" {
		label = "\\label{" + latexLabel("fig", figure.ID) + "}\n"
	}
	if figure.Placement.Kind == AnchoredPlacement {
		a := figure.Placement.Anchored
		x, y := a.X, a.Y
		if a.Anchor == MarginAnchor {
			x = Length{Unit: Point, Value: lengthPoints(x) + r.doc.Layout.Margins.LeftPt}
			y = Length{Unit: Point, Value: lengthPoints(y) + r.doc.Layout.Margins.TopPt}
		}
		r.out.WriteString("\\begin{textblock*}{" + latexLength(size.Width) + "}(" + latexLength(x) + "," + latexLength(y) + ")\n\\centering\n" + image + "\n" + caption + label + "\\end{textblock*}\n\n")
		return
	}
	r.out.WriteString("\\begin{figure}[H]\n\\centering\n" + image + "\n" + caption + label + "\\end{figure}\n\n")
}

func (r *latexRenderer) writeEquation(equation Equation) {
	if equation.Numbered {
		r.equationNumber++
		r.out.WriteString("\\begin{equation}\n" + equation.Latex + "\n\\tag{" + strconv.Itoa(r.equationNumber) + "}\n")
		if equation.ID != "" {
			r.out.WriteString("\\label{" + latexLabel("eq", equation.ID) + "}\n")
		}
		r.out.WriteString("\\end{equation}\n\n")
		return
	}
	r.out.WriteString("\\[\n" + equation.Latex + "\n\\]\n\n")
}

func (r *latexRenderer) inlines(values []Inline) string {
	var out strings.Builder
	for _, value := range values {
		switch value.Kind {
		case TextInline:
			out.WriteString(escapeLatexText(value.Text))
		case StrongInline:
			out.WriteString("\\textbf{" + escapeLatexText(value.Text) + "}")
		case EmphasisInline:
			out.WriteString("\\emph{" + escapeLatexText(value.Text) + "}")
		case CodeInline:
			out.WriteString("\\texttt{" + escapeLatexText(value.Text) + "}")
		case LinkInline:
			out.WriteString("\\href{\\detokenize{" + escapeLatexURL(value.URL) + "}}{" + escapeLatexText(value.Text) + "}")
		case ReferenceInline:
			out.WriteString("??")
		case CitationInline:
			out.WriteString("\\cite{" + strings.Join(value.Citation, ",") + "}")
		case PageNumberInline:
			out.WriteString("\\thepage")
		case DocumentTitleInline:
			out.WriteString(escapeLatexText(r.doc.Metadata.Title))
		case DocumentAuthorInline:
			out.WriteString(escapeLatexText(r.doc.Metadata.Author))
		case LineBreakInline:
			out.WriteString("\\\\")
		}
	}
	return out.String()
}

func escapeLatexText(value string) string {
	var out strings.Builder
	for _, char := range value {
		switch char {
		case '\\':
			out.WriteString(`\textbackslash{}`)
		case '{':
			out.WriteString(`\{`)
		case '}':
			out.WriteString(`\}`)
		case '$':
			out.WriteString(`\$`)
		case '&':
			out.WriteString(`\&`)
		case '#':
			out.WriteString(`\#`)
		case '%':
			out.WriteString(`\%`)
		case '_':
			out.WriteString(`\_`)
		case '^':
			out.WriteString(`\textasciicircum{}`)
		case '~':
			out.WriteString(`\textasciitilde{}`)
		case '\r', '\n':
			out.WriteByte(' ')
		default:
			out.WriteRune(char)
		}
	}
	return out.String()
}

func escapeLatexURL(value string) string {
	replacer := strings.NewReplacer("%", "%25", "{", "%7B", "}", "%7D", "\\", "%5C", "\r", "", "\n", "%0A")
	return replacer.Replace(value)
}

func escapeLatexPath(value string) string {
	return strings.ReplaceAll(filepath.ToSlash(value), " ", `\space `)
}
func latexStyleCommands(style ParagraphStyle) string {
	commands := "\\fontsize{" + latexNumber(style.Text.SizePt) + "pt}{" + latexNumber(style.Text.SizePt*style.LineSpacing) + "pt}\\selectfont"
	if style.Text.Weight == Bold {
		commands += "\\bfseries"
	} else if style.Text.Weight == Medium {
		commands += "\\bfseries"
	}
	if style.Text.Italic {
		commands += "\\itshape"
	}
	hex := strings.TrimPrefix(style.Text.Color.Hex, "#")
	if matched, _ := regexp.MatchString(`^[0-9A-Fa-f]{6}$`, hex); matched {
		commands += "\\color[HTML]{" + strings.ToUpper(hex) + "}"
	}
	return commands + " "
}
func latexAlignment(value Alignment) string {
	if value == Center {
		return `>{\centering\arraybackslash}X`
	}
	if value == Right {
		return `>{\raggedleft\arraybackslash}X`
	}
	return `>{\raggedright\arraybackslash}X`
}
func latexLength(value Length) string {
	if value.Unit == Millimeter {
		return latexNumber(value.Value) + "mm"
	}
	return latexNumber(value.Value) + "pt"
}
func latexNumber(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }
func latexLabel(kind, id string) string {
	var out strings.Builder
	for _, char := range id {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == ':' {
			out.WriteRune(char)
		} else {
			out.WriteByte('-')
		}
	}
	label := strings.Trim(out.String(), "-")
	if label == "" {
		sum := sha256.Sum256([]byte(id))
		label = fmt.Sprintf("%x", sum[:6])
	}
	return "oct:" + kind + ":" + label
}
