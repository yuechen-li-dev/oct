package interpret

import "fmt"

// NativeOperation is the normalized manifest identity checked at the broker.
// It deliberately contains no filesystem path supplied by Oct code.
type NativeOperation struct {
	Package, Wrapper, Operation, WireOperation, Family, Protocol, SidecarCommand string
}

func (o NativeOperation) Identity() string {
	return o.Package + ":" + o.Wrapper + ":" + o.Operation
}

// ArtifactNativeGrant is host-owned authority. Oct values can describe an
// identical operation, but cannot implement or obtain this interface.
type ArtifactNativeGrant interface {
	AuthorizeArtifactNative(NativeOperation) error
}

type ArtifactNativeDispatchRecorder interface {
	RecordArtifactNativeDispatch(NativeOperation, string, string)
}

// ArtifactOutputRequest identifies one output declared while evaluating a
// typed [Artifact] entry point. The compiler-owned caller supplies the actual
// staging implementation; the interpreter only routes writes through it.
type ArtifactOutputRequest struct {
	Path       string
	Package    string
	Function   string
	SourcePath string
	Kind       string
}

// ArtifactWriteCapability is the narrow mutation seam available during the
// explicit artifact phase. Implementations must confine and de-duplicate paths.
type ArtifactWriteCapability interface {
	StageArtifactOutput(ArtifactOutputRequest) (string, error)
	StageArtifactDirectory(path string) (string, error)
	StagedArtifactReadPath(path string) (string, error)
}

func (i *interpreter) beginArtifactWrite() error {
	if i.artifactCapability == nil {
		return fmt.Errorf("Artifact.Write* is available only during `oct artifact` evaluation")
	}
	i.artifactWriteDepth++
	return nil
}

func (i *interpreter) endArtifactWrite() {
	if i.artifactWriteDepth > 0 {
		i.artifactWriteDepth--
	}
}

func (i *interpreter) prepareArtifactOutput(path string) (actual string, logical string, err error) {
	logical = attributedOutputPath(path)
	if i.artifactCapability == nil || i.artifactWriteDepth == 0 {
		return path, logical, nil
	}
	actual, err = i.artifactCapability.StageArtifactOutput(ArtifactOutputRequest{
		Path:       logical,
		Package:    i.artifactPackage,
		Function:   i.currentFunctionName,
		SourcePath: i.artifactSourcePath,
		Kind:       "file",
	})
	return actual, logical, err
}

func (i *interpreter) prepareArtifactDirectory(path string) (string, error) {
	if i.artifactCapability == nil {
		return path, nil
	}
	return i.artifactCapability.StageArtifactDirectory(attributedOutputPath(path))
}

func (i *interpreter) prepareArtifactRead(path string) (string, error) {
	if i.artifactCapability == nil {
		return path, nil
	}
	return i.artifactCapability.StagedArtifactReadPath(attributedOutputPath(path))
}

func artifactEffectDiagnostic(callee string) string {
	switch callee {
	case "FileDelete", "DirectoryList", "DirectoryRemoveAll", "CsvRead", "CsvReadRows", "CsvReadTable", "CsvReadMatrix",
		"JsonLoad", "HashSha256File", "ZipListEntries", "ZipExtractAll", "ZipCreateFromFiles",
		"GzipCompressFile", "GzipDecompressFile", "ImageLoad", "ImageSave", "PdfSave",
		"PlotRenderLine", "PlotRenderScatter", "PlotRenderHistogram", "TimeNowIso8601", "TimeUnixSecondsNow":
		return fmt.Sprintf("artifact evaluation rejected ambient operation %s", callee)
	case "FileWriteText", "FileWriteBytes", "FileWriteLines", "CsvWrite", "CsvWriteRows", "CsvWriteTable", "CsvWriteMatrix", "JsonSave":
		return fmt.Sprintf("artifact evaluation rejected %s outside Artifact.Write*; use the compiler-owned Artifact capability", callee)
	}
	return ""
}
