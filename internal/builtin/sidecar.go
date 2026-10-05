package builtin

// A sidecar builtin is a builtin of the standard libraries that the compiled
// lane does not implement in generated code. A compiled program reaches it
// through one of the first-party Octxiliary sidecars built from
// cmd/octxiliary-*. The interpreter implements the same builtin in-process
// and never consults this table.
//
// The sidecar is an implementation detail of the compiled lane, exactly as
// the interpreter's Go function is one of the interpreted lane. Neither is a
// wrapper in the sense of a package manifest: a manifest wrapper function is
// native code outside the toolchain, has no source body, and is dispatched to
// its sidecar in both lanes.
//
// A wire function has the builtin's name and the builtin's arguments, in
// order. The one difference is a handle: the builtin carries it as an Int,
// and the sidecar receives and returns it as a typed handle, which lets it
// refuse a handle of another kind.

// SidecarType is the type of one argument or result of a sidecar builtin.
type SidecarType struct {
	// Oct is the type the compiled program holds: Int, Float, Bool, String,
	// Bytes, String[], Float[], or Int<unit>.
	Oct string
	// Handle, when set, is the wire handle type that an Int stands for.
	Handle string
}

// SidecarBuiltin describes how the compiled lane calls one builtin.
type SidecarBuiltin struct {
	// Name is the builtin and the wire function.
	Name string
	// Sidecar is the command of the sidecar that serves it.
	Sidecar string
	// Family is the wire family the sidecar answers to.
	Family   string
	Params   []SidecarType
	Result   SidecarType
	Fallible bool
}

// LookupSidecar reports the sidecar builtin of that name.
func LookupSidecar(name string) (SidecarBuiltin, bool) {
	entry, ok := sidecarBuiltins[name]
	return entry, ok
}

// SidecarBuiltins lists every sidecar builtin, in a stable order.
func SidecarBuiltins() []SidecarBuiltin {
	out := make([]SidecarBuiltin, 0, len(sidecarBuiltinOrder))
	for _, name := range sidecarBuiltinOrder {
		out = append(out, sidecarBuiltins[name])
	}
	return out
}

var (
	sidecarBuiltins     = map[string]SidecarBuiltin{}
	sidecarBuiltinOrder []string
)

func scalar(oct string) SidecarType { return SidecarType{Oct: oct} }

func handle(wireType string) SidecarType { return SidecarType{Oct: "Int", Handle: wireType} }

func sidecarFamily(sidecar string, family string) func(name string, fallible bool, result SidecarType, params ...SidecarType) {
	return func(name string, fallible bool, result SidecarType, params ...SidecarType) {
		if _, duplicate := sidecarBuiltins[name]; duplicate {
			panic("builtin: sidecar builtin declared twice: " + name)
		}
		sidecarBuiltins[name] = SidecarBuiltin{Name: name, Sidecar: sidecar, Family: family, Params: params, Result: result, Fallible: fallible}
		sidecarBuiltinOrder = append(sidecarBuiltinOrder, name)
	}
}

func init() {
	var (
		intT     = scalar("Int")
		floatT   = scalar("Float")
		boolT    = scalar("Bool")
		stringT  = scalar("String")
		bytesT   = scalar("Bytes")
		stringsT = scalar("String[]")
		floatsT  = scalar("Float[]")
		pixels   = scalar("Int<px>")
	)
	const fallible, infallible = true, false

	archive := sidecarFamily("octxiliary-archive", "Archive")
	archive("ZipListEntries", fallible, stringsT, stringT)
	archive("ZipExtractAll", fallible, intT, stringT, stringT)
	archive("ZipCreateFromFiles", fallible, intT, stringT, stringsT)

	compression := sidecarFamily("octxiliary-compression", "Compression")
	compression("GzipCompressBytes", fallible, bytesT, bytesT)
	compression("GzipDecompressBytes", fallible, bytesT, bytesT)
	compression("GzipCompressFile", fallible, intT, stringT, stringT)
	compression("GzipDecompressFile", fallible, intT, stringT, stringT)

	hash := sidecarFamily("octxiliary-hash", "Hash")
	hash("HashSha256Bytes", infallible, stringT, bytesT)
	hash("HashSha256Text", infallible, stringT, stringT)
	hash("HashSha256File", fallible, stringT, stringT)

	text := sidecarFamily("octxiliary-text", "Text")
	text("RegexIsMatch", fallible, boolT, stringT, stringT)
	text("RegexFindAll", fallible, stringsT, stringT, stringT)
	text("RegexReplaceAll", fallible, stringT, stringT, stringT, stringT)
	text("RegexSplit", fallible, stringsT, stringT, stringT)

	time := sidecarFamily("octxiliary-time", "Time")
	time("TimeNowIso8601", infallible, stringT)
	time("TimeParseIso8601", fallible, stringT, stringT)
	time("TimeFormatIso8601", fallible, stringT, stringT)
	time("TimeUnixSecondsNow", infallible, intT)
	time("TimeFormatUnixSecond", fallible, stringT, intT)

	workbook := handle("IO.Workbook")
	xlsx := sidecarFamily("octxiliary-xlsx", "Xlsx")
	xlsx("XlsxCreateWorkbook", infallible, workbook)
	xlsx("XlsxAddSheet", fallible, intT, workbook, stringT)
	xlsx("XlsxSetCellString", fallible, intT, workbook, stringT, stringT, stringT)
	xlsx("XlsxSetCellFloat", fallible, intT, workbook, stringT, stringT, floatT)
	xlsx("XlsxSaveWorkbook", fallible, intT, workbook, stringT)

	picture := handle("Image.ImageHandle")
	image := sidecarFamily("octxiliary-image", "Image")
	image("ImageLoad", fallible, picture, stringT)
	image("ImageSave", fallible, intT, picture, stringT)
	image("ImageEncodePng", fallible, bytesT, picture)
	image("ImageWidth", infallible, pixels, picture)
	image("ImageHeight", infallible, pixels, picture)
	image("ImageFormat", infallible, stringT, picture)

	page := handle("Pdf.PdfPage")
	pdf := sidecarFamily("octxiliary-pdf", "Pdf")
	pdf("PdfNewPage", fallible, page, pixels, pixels)
	pdf("PdfDrawText", fallible, intT, page, pixels, pixels, stringT)
	pdf("PdfDrawTextStyled", fallible, intT, page, pixels, pixels, stringT, pixels, intT, intT, intT)
	pdf("PdfDrawImageBytes", fallible, intT, page, bytesT, stringT, pixels, pixels)
	pdf("PdfDrawImageBytesSized", fallible, intT, page, bytesT, stringT, pixels, pixels, pixels, pixels)
	pdf("PdfSave", fallible, intT, page, stringT)

	plot := sidecarFamily("octxiliary-plot", "Plot")
	plot("PlotRenderLine", fallible, intT, floatsT, floatsT, stringT, pixels, pixels, stringT, stringT, stringT, stringT)
	plot("PlotRenderScatter", fallible, intT, floatsT, floatsT, stringT, pixels, pixels, stringT, stringT, stringT, stringT)
	plot("PlotRenderHistogram", fallible, intT, floatsT, intT, stringT, pixels, pixels, stringT, stringT, stringT, stringT)
}
