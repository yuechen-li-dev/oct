package tester

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type artifactCase struct {
	pkg      string
	filePath string
	name     string
	provider string
}

type ArtifactOptions struct {
	Execution       string
	OutputRoot      string
	AllPackages     bool
	JSON            bool
	Report          *ArtifactReport
	NativeApprovals []string
}

type ArtifactReport struct {
	Execution          string
	RequestedExecution string
	OutputRoot         string
	MetadataComplete   bool
	Artifacts          []GeneratedArtifact
	Capabilities       []ArtifactCapabilityProvenance `json:"capabilities,omitempty"`
	Measurements       ArtifactCapabilityMeasurements `json:"measurements"`
}

type ArtifactCapabilityMeasurements struct {
	RequestAtomsDiscovered           int   `json:"requestAtomsDiscovered"`
	NormalizedRequestAtoms           int   `json:"normalizedRequestAtoms"`
	RequestNormalizationMicroseconds int64 `json:"requestNormalizationMicroseconds"`
	GrantsEvaluated                  int   `json:"grantsEvaluated"`
	ApprovalsAccepted                int   `json:"approvalsAccepted"`
	ApprovalsRejected                int   `json:"approvalsRejected"`
	NativeDispatches                 int   `json:"nativeDispatches"`
	BackendGenerations               int   `json:"backendGenerations"`
	ArtifactHostExecutablesCreated   int   `json:"artifactHostExecutablesCreated"`
}

type ArtifactCapabilityProvenance struct {
	Artifact       string `json:"artifact"`
	SourcePath     string `json:"sourcePath"`
	Capability     string `json:"capability"`
	Package        string `json:"package"`
	Wrapper        string `json:"wrapper"`
	Operation      string `json:"operation"`
	WireOperation  string `json:"wireOperation"`
	Family         string `json:"family"`
	Protocol       string `json:"protocol"`
	SidecarCommand string `json:"sidecarCommand"`
	Requested      bool   `json:"requested"`
	Approved       bool   `json:"approved"`
	Granted        bool   `json:"granted"`
	Dispatched     bool   `json:"dispatched"`
	SidecarPath    string `json:"sidecarPath,omitempty"`
	SidecarSHA256  string `json:"sidecarSha256,omitempty"`
}

type GeneratedArtifact struct {
	Function   string `json:"function"`
	SourcePath string `json:"sourcePath"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	MIMEType   string `json:"mimeType"`
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256"`
}

func ExecuteArtifacts(path string, stdout io.Writer) error {
	return ExecuteArtifactsWithOptions(path, stdout, ArtifactOptions{})
}

func ExecuteArtifactsWithOptions(path string, stdout io.Writer, options ArtifactOptions) (retErr error) {
	requested := strings.TrimSpace(options.Execution)
	if requested == "" {
		requested = "interpreted"
	}
	if requested != "compiled" && requested != "interpreted" {
		return fmt.Errorf("invalid artifact execution mode %q (expected compiled|interpreted)", requested)
	}
	outputRoot := strings.TrimSpace(options.OutputRoot)
	if outputRoot == "" {
		var err error
		outputRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("read artifact output root: %w", err)
		}
	}
	rootAbs, err := filepath.Abs(outputRoot)
	if err != nil {
		return fmt.Errorf("resolve artifact output root %s: %w", outputRoot, err)
	}
	if options.Report != nil {
		options.Report.Execution = "build-time-interpreted"
		options.Report.RequestedExecution = requested
		options.Report.OutputRoot = filepath.ToSlash(filepath.Clean(rootAbs))
		options.Report.MetadataComplete = true
		options.Report.Artifacts = nil
		options.Report.Capabilities = nil
		options.Report.Measurements = ArtifactCapabilityMeasurements{}
	}
	publisher, err := newArtifactPublisher(rootAbs)
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := publisher.close(); cleanupErr != nil && retErr == nil {
			retErr = cleanupErr
		}
	}()

	_, _ = fmt.Fprintln(stdout, "Execution: build-time-interpreted")
	_, _ = fmt.Fprintf(stdout, "Output root: %s\n", filepath.ToSlash(rootAbs))
	if requested == "compiled" {
		_, _ = fmt.Fprintln(stdout, "Compatibility: --execution compiled delegates to the build-time interpreter; no backend is generated or compiled")
	}
	passed := 0
	if err := executeForPathOrExperiment(path, stdout, "artifact", func(singlePath string, singleStdout io.Writer) error {
		count, err := evaluateArtifactsSingleRoot(singlePath, singleStdout, options.AllPackages, publisher, options.NativeApprovals, options.Report)
		passed += count
		return err
	}); err != nil {
		return err
	}
	generated, err := publisher.publish()
	if err != nil {
		return err
	}
	produced, unchanged := 0, 0
	for _, artifact := range generated {
		if artifact.Status == "unchanged" {
			unchanged++
		} else {
			produced++
		}
		_, _ = fmt.Fprintf(stdout, "%s %s\n", strings.ToUpper(artifact.Status), artifact.Path)
	}
	if options.Report != nil {
		options.Report.Artifacts = append(options.Report.Artifacts, generated...)
		sort.Slice(options.Report.Artifacts, func(i, j int) bool { return options.Report.Artifacts[i].Path < options.Report.Artifacts[j].Path })
	}
	_, _ = fmt.Fprintf(stdout, "Outputs: %d produced, %d unchanged\n", produced, unchanged)
	_, _ = fmt.Fprintf(stdout, "Result: %d artifact(s) passed, 0 failed\n", passed)
	return nil
}

func evaluateArtifactsSingleRoot(path string, stdout io.Writer, allPackages bool, publisher *artifactPublisher, approvals []string, report *ArtifactReport) (int, error) {
	normalizationStarted := time.Now()
	program, err := project.LoadForTest(path)
	if err != nil {
		return 0, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return 0, err
	}
	artifacts, err := discoverArtifactCases(program, path, allPackages)
	if err != nil {
		return 0, err
	}
	requestedByArtifact := make(map[string][]interpret.NativeOperation, len(artifacts))
	allRequested := map[string]struct{}{}
	for _, artifact := range artifacts {
		if artifact.provider == "" {
			continue
		}
		value, discoveryErr := interpret.CallFunctionWithArgsAndOptions(program, artifact.pkg, artifact.provider, nil, io.Discard, interpret.ExecuteOptions{RequestDiscovery: true})
		if discoveryErr != nil {
			return 0, fmt.Errorf("capability discovery for %s.%s (%s): %w", artifact.pkg, artifact.name, filepath.ToSlash(artifact.filePath), discoveryErr)
		}
		operations, normalizeErr := normalizeNativeRequests(program, value)
		if normalizeErr != nil {
			return 0, fmt.Errorf("malformed capability request for %s.%s (%s): %w", artifact.pkg, artifact.name, filepath.ToSlash(artifact.filePath), normalizeErr)
		}
		qualified := artifact.pkg + "." + artifact.name
		requestedByArtifact[qualified] = operations
		if report != nil {
			report.Measurements.NormalizedRequestAtoms += len(operations)
			if native, ok := value.Record.Fields["Native"]; ok && native.Kind == interpret.ValueArray {
				report.Measurements.RequestAtomsDiscovered += len(native.Array)
			}
		}
		for _, operation := range operations {
			allRequested[operation.Identity()] = struct{}{}
			_, _ = fmt.Fprintf(stdout, "REQUEST %s for %s.%s (%s)\n", operation.Identity(), artifact.pkg, artifact.name, filepath.ToSlash(artifact.filePath))
		}
	}
	approved, err := normalizeNativeApprovals(approvals)
	if err != nil {
		if report != nil {
			report.Measurements.ApprovalsRejected++
		}
		return 0, err
	}
	for identity := range approved {
		if _, ok := allRequested[identity]; !ok {
			if report != nil {
				report.Measurements.ApprovalsRejected++
			}
			return 0, fmt.Errorf("native approval %q is broader than the discovered requests", identity)
		}
	}
	if report != nil {
		report.Measurements.ApprovalsAccepted += len(approved)
		report.Measurements.RequestNormalizationMicroseconds += time.Since(normalizationStarted).Microseconds()
	}

	for index, artifact := range artifacts {
		qualified := artifact.pkg + "." + artifact.name
		grant := &artifactNativeGrant{requested: map[string]interpret.NativeOperation{}, approved: approved}
		for _, operation := range requestedByArtifact[qualified] {
			grant.requested[operation.Identity()] = operation
			if report != nil {
				report.Measurements.GrantsEvaluated++
			}
		}
		grant.onDispatch = func(operation interpret.NativeOperation, sidecarPath, digest string) {
			_, _ = fmt.Fprintf(stdout, "DISPATCH %s sidecar=%s sha256=%s\n", operation.Identity(), sidecarPath, digest)
			if report == nil {
				return
			}
			report.Measurements.NativeDispatches++
			for idx := range report.Capabilities {
				capability := &report.Capabilities[idx]
				if capability.Artifact == qualified && capability.Capability == operation.Identity() {
					capability.Dispatched, capability.SidecarPath, capability.SidecarSHA256 = true, sidecarPath, digest
				}
			}
		}
		if report != nil {
			for _, operation := range requestedByArtifact[qualified] {
				_, isApproved := approved[operation.Identity()]
				report.Capabilities = append(report.Capabilities, ArtifactCapabilityProvenance{Artifact: qualified, SourcePath: filepath.ToSlash(artifact.filePath), Capability: operation.Identity(), Package: operation.Package, Wrapper: operation.Wrapper, Operation: operation.Operation, WireOperation: operation.WireOperation, Family: operation.Family, Protocol: operation.Protocol, SidecarCommand: operation.SidecarCommand, Requested: true, Approved: isApproved, Granted: isApproved})
			}
		}
		for _, operation := range requestedByArtifact[qualified] {
			if _, ok := approved[operation.Identity()]; ok {
				_, _ = fmt.Fprintf(stdout, "GRANT %s\n", operation.Identity())
			} else {
				_, _ = fmt.Fprintf(stdout, "DENY %s requested but not approved\n", operation.Identity())
			}
		}
		_, _ = fmt.Fprintf(stdout, "RUN  %s (%s)\n", qualified, shortPath(path, artifact.filePath))
		err := interpret.ExecuteFunctionWithArgsAndOptions(program, artifact.pkg, artifact.name, nil, stdout, interpret.ExecuteOptions{
			ArtifactCapability:  publisher,
			ArtifactNativeGrant: grant,
			ArtifactSourcePath:  artifact.filePath,
			ArtifactProgressRecorder: func(event interpret.ArtifactProgressEvent) {
				if event.Kind == "checkpoint" {
					_, _ = fmt.Fprintf(stdout, "CHECKPOINT %s: %s\n", qualified, event.Label)
				} else if event.Kind == "progress" {
					_, _ = fmt.Fprintf(stdout, "PROGRESS %s: %s %d/%d\n", qualified, event.Label, event.Current, event.Total)
				}
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, artifact.filePath), err)
			return index, fmt.Errorf("1 artifact(s) failed")
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, artifact.filePath))
	}
	return len(artifacts), nil
}

func discoverArtifactCases(program project.Program, path string, allPackages bool) ([]artifactCase, error) {
	selectedSources, err := selectedTestSources(path)
	if err != nil {
		return nil, err
	}
	var artifacts []artifactCase
	for pkgName, pkg := range program.Packages {
		if !allPackages && pkgName != program.Entry {
			continue
		}
		for _, fn := range pkg.Functions {
			if fn.IsArtifact && isSelectedSource(selectedSources, fn.SourcePath) {
				artifacts = append(artifacts, artifactCase{pkg: pkgName, filePath: fn.SourcePath, name: fn.Name, provider: fn.ArtifactCapabilityProvider})
			}
		}
	}
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].pkg != artifacts[j].pkg {
			return artifacts[i].pkg < artifacts[j].pkg
		}
		if artifacts[i].filePath != artifacts[j].filePath {
			return artifacts[i].filePath < artifacts[j].filePath
		}
		return artifacts[i].name < artifacts[j].name
	})
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no [Artifact] functions found")
	}
	return artifacts, nil
}

type artifactNativeGrant struct {
	requested  map[string]interpret.NativeOperation
	approved   map[string]struct{}
	onDispatch func(interpret.NativeOperation, string, string)
}

func (g *artifactNativeGrant) AuthorizeArtifactNative(operation interpret.NativeOperation) error {
	requested, ok := g.requested[operation.Identity()]
	if !ok {
		return fmt.Errorf("artifact native operation %s was not requested", operation.Identity())
	}
	if requested != operation {
		return fmt.Errorf("artifact native operation %s does not match the granted manifest identity", operation.Identity())
	}
	if _, ok := g.approved[operation.Identity()]; !ok {
		return fmt.Errorf("artifact native operation %s was requested but not approved", operation.Identity())
	}
	return nil
}

func (g *artifactNativeGrant) RecordArtifactNativeDispatch(operation interpret.NativeOperation, path, digest string) {
	if g.onDispatch != nil {
		g.onDispatch(operation, path, digest)
	}
}

func normalizeNativeApprovals(values []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		parts := strings.Split(value, ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("malformed native approval %q; expected Package:Wrapper:Operation", raw)
		}
		out[strings.Join(parts, ":")] = struct{}{}
	}
	return out, nil
}

func normalizeNativeRequests(program project.Program, value interpret.Value) ([]interpret.NativeOperation, error) {
	if value.Kind != interpret.ValueRecord || unqualifiedRecordName(value.Record.TypeName) != "ArtifactCapabilityRequest" {
		return nil, fmt.Errorf("provider returned %s, expected ArtifactCapabilityRequest Concept", value.Kind)
	}
	native, ok := value.Record.Fields["Native"]
	if !ok || native.Kind != interpret.ValueArray {
		return nil, fmt.Errorf("ArtifactCapabilityRequest.Native must be NativeComputeRequest[]")
	}
	byIdentity := map[string]interpret.NativeOperation{}
	for idx, atom := range native.Array {
		if atom.Kind != interpret.ValueRecord || unqualifiedRecordName(atom.Record.TypeName) != "NativeComputeRequest" {
			return nil, fmt.Errorf("Native[%d] must be NativeComputeRequest", idx)
		}
		packageName, err := requestStringField(atom, "Package")
		if err != nil {
			return nil, fmt.Errorf("Native[%d]: %w", idx, err)
		}
		wrapperName, err := requestStringField(atom, "Wrapper")
		if err != nil {
			return nil, fmt.Errorf("Native[%d]: %w", idx, err)
		}
		operationName, err := requestStringField(atom, "Operation")
		if err != nil {
			return nil, fmt.Errorf("Native[%d]: %w", idx, err)
		}
		pkg, ok := program.Packages[packageName]
		if !ok {
			return nil, fmt.Errorf("unknown package %q", packageName)
		}
		var resolved *interpret.NativeOperation
		for _, wrapper := range pkg.Wrappers {
			if wrapper.Name != wrapperName {
				continue
			}
			for _, function := range wrapper.Functions {
				if function.OctName == operationName {
					op := interpret.NativeOperation{Package: packageName, Wrapper: wrapper.Name, Operation: function.OctName, WireOperation: function.WireName, Family: wrapper.Family, Protocol: wrapper.Protocol, SidecarCommand: wrapper.SidecarCommand}
					resolved = &op
				}
			}
		}
		if resolved == nil {
			return nil, fmt.Errorf("wrapper operation %s:%s:%s is not declared by the package manifest", packageName, wrapperName, operationName)
		}
		byIdentity[resolved.Identity()] = *resolved
	}
	out := make([]interpret.NativeOperation, 0, len(byIdentity))
	for _, operation := range byIdentity {
		out = append(out, operation)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identity() < out[j].Identity() })
	return out, nil
}

func requestStringField(value interpret.Value, name string) (string, error) {
	field, ok := value.Record.Fields[name]
	if !ok || field.Kind != interpret.ValueString || strings.TrimSpace(field.Text) == "" {
		return "", fmt.Errorf("NativeComputeRequest.%s must be a non-empty String", name)
	}
	return strings.TrimSpace(field.Text), nil
}

func unqualifiedRecordName(name string) string {
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

type stagedArtifactOutput struct {
	request    interpret.ArtifactOutputRequest
	relative   string
	stagedPath string
}

type artifactPublisher struct {
	root      string
	stageRoot string
	outputs   map[string]stagedArtifactOutput
}

func newArtifactPublisher(root string) (*artifactPublisher, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact output root %s: %w", root, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact output root %s: %w", root, err)
	}
	root, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute artifact output root %s: %w", resolvedRoot, err)
	}
	stageRoot, err := os.MkdirTemp(root, ".oct-artifact-stage-")
	if err != nil {
		return nil, fmt.Errorf("create artifact staging directory: %w", err)
	}
	return &artifactPublisher{root: root, stageRoot: stageRoot, outputs: map[string]stagedArtifactOutput{}}, nil
}

func (p *artifactPublisher) StageArtifactOutput(request interpret.ArtifactOutputRequest) (string, error) {
	relative, err := p.validateRelativePath(request.Path)
	if err != nil {
		return "", fmt.Errorf("artifact output %q from %s (%s): %w", request.Path, request.Function, filepath.ToSlash(request.SourcePath), err)
	}
	key := strings.ToLower(filepath.ToSlash(relative))
	if previous, exists := p.outputs[key]; exists {
		return "", fmt.Errorf("duplicate artifact output path %q from %s (%s); already declared by %s (%s)", filepath.ToSlash(relative), request.Function, filepath.ToSlash(request.SourcePath), previous.request.Function, filepath.ToSlash(previous.request.SourcePath))
	}
	stagedPath := filepath.Join(p.stageRoot, relative)
	if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
		return "", fmt.Errorf("prepare staged artifact path %s: %w", filepath.ToSlash(relative), err)
	}
	p.outputs[key] = stagedArtifactOutput{request: request, relative: relative, stagedPath: stagedPath}
	return stagedPath, nil
}

func (p *artifactPublisher) StageArtifactDirectory(path string) (string, error) {
	relative, err := p.validateRelativePath(path)
	if err != nil {
		return "", fmt.Errorf("artifact directory %q: %w", path, err)
	}
	return filepath.Join(p.stageRoot, relative), nil
}

func (p *artifactPublisher) StagedArtifactReadPath(path string) (string, error) {
	relative, err := p.validateRelativePath(path)
	if err != nil {
		return "", fmt.Errorf("artifact read %q: %w", path, err)
	}
	key := strings.ToLower(filepath.ToSlash(relative))
	output, exists := p.outputs[key]
	if !exists {
		return "", fmt.Errorf("artifact evaluation rejected ambient filesystem read %q; only outputs already declared in this phase may be read", filepath.ToSlash(relative))
	}
	return output.stagedPath, nil
}

func (p *artifactPublisher) validateRelativePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	normalized := strings.ReplaceAll(path, "\\", "/")
	hasDrivePrefix := len(normalized) >= 2 && ((normalized[0] >= 'A' && normalized[0] <= 'Z') || (normalized[0] >= 'a' && normalized[0] <= 'z')) && normalized[1] == ':'
	if normalized == "" || strings.HasPrefix(normalized, "/") || hasDrivePrefix || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("path must be non-empty and relative to the artifact output root")
	}
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the artifact output root")
	}
	destination := filepath.Join(p.root, clean)
	rel, err := filepath.Rel(p.root, destination)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes the artifact output root")
	}
	return clean, nil
}

func (p *artifactPublisher) publish() ([]GeneratedArtifact, error) {
	outputs := make([]stagedArtifactOutput, 0, len(p.outputs))
	for _, output := range p.outputs {
		outputs = append(outputs, output)
	}
	sort.Slice(outputs, func(i, j int) bool {
		return filepath.ToSlash(outputs[i].relative) < filepath.ToSlash(outputs[j].relative)
	})
	reports := make([]GeneratedArtifact, 0, len(outputs))
	for _, output := range outputs {
		contents, err := os.ReadFile(output.stagedPath)
		if err != nil {
			return nil, fmt.Errorf("artifact %s did not produce its declared staged output: %w", filepath.ToSlash(output.relative), err)
		}
		destination := filepath.Join(p.root, output.relative)
		if err := rejectSymlinkDestination(p.root, output.relative); err != nil {
			return nil, fmt.Errorf("publish artifact %s: %w", filepath.ToSlash(output.relative), err)
		}
		status := "produced"
		if existing, readErr := os.ReadFile(destination); readErr == nil && bytes.Equal(existing, contents) {
			status = "unchanged"
		} else {
			if err := publishArtifactFile(destination, contents); err != nil {
				return nil, fmt.Errorf("publish artifact %s: %w", filepath.ToSlash(output.relative), err)
			}
		}
		sum := sha256.Sum256(contents)
		reports = append(reports, GeneratedArtifact{
			Function:   output.request.Function,
			SourcePath: filepath.ToSlash(output.request.SourcePath),
			Path:       filepath.ToSlash(output.relative),
			Status:     status,
			MIMEType:   artifactMIMEType(filepath.Ext(output.relative)),
			Bytes:      int64(len(contents)),
			SHA256:     hex.EncodeToString(sum[:]),
		})
	}
	return reports, nil
}

func rejectSymlinkDestination(root string, relative string) error {
	current := root
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("output path crosses symbolic link %s", filepath.ToSlash(current))
		}
	}
	return nil
}

func publishArtifactFile(destination string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".oct-artifact-publish-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
}

func (p *artifactPublisher) close() error {
	if err := os.RemoveAll(p.stageRoot); err != nil {
		return fmt.Errorf("remove artifact staging directory %s: %w", p.stageRoot, err)
	}
	return nil
}

func artifactMIMEType(ext string) string {
	switch strings.ToLower(ext) {
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".octagon":
		return "application/octet-stream"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
