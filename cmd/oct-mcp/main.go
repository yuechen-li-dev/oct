// Command oct-mcp exposes bounded hosted Oct workflows over MCP.
//
// Local Codex should use the repository's oct CLI directly. This server owns
// the different hosted boundary: submitted virtual files, a fresh temporary
// workspace, a fixed command allowlist, and scoped artifact retrieval.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yuechen-li-dev/oct/internal/cli"
)

const (
	serverVersion     = "0.1.0"
	toolSchemaVersion = "2.0"
)

type Limits struct {
	SourceBytes          int `json:"sourceBytes"`
	FileCount            int `json:"fileCount"`
	ExecutionSeconds     int `json:"executionSeconds"`
	StdoutBytes          int `json:"stdoutBytes"`
	StderrBytes          int `json:"stderrBytes"`
	ArtifactCount        int `json:"artifactCount"`
	ArtifactBytes        int `json:"artifactBytes"`
	ConcurrentExecutions int `json:"concurrentExecutions"`
	ArtifactTTLSeconds   int `json:"artifactTTLSeconds"`
}

func defaultLimits() Limits {
	return Limits{SourceBytes: 256 << 10, FileCount: 32, ExecutionSeconds: 15, StdoutBytes: 64 << 10, StderrBytes: 64 << 10, ArtifactCount: 8, ArtifactBytes: 2 << 20, ConcurrentExecutions: 2, ArtifactTTLSeconds: 600}
}

type File struct {
	Path    string `json:"path" jsonschema:"slash-separated relative .oct, .octest, or .octfail path"`
	Content string `json:"content" jsonschema:"UTF-8 file content"`
}

// SourceInput describes one bounded virtual project. Source is convenient for
// a one-file submission; Files supports a small project and requires Entry
// when more than one file is supplied.
type SourceInput struct {
	Source         string `json:"source,omitempty" jsonschema:"Oct source; mutually exclusive with files"`
	Entry          string `json:"entry,omitempty" jsonschema:"relative virtual entry file"`
	Files          []File `json:"files,omitempty" jsonschema:"bounded virtual project files"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"requested timeout capped by oct_workspace_info limits"`
}

type TestInput struct {
	SourceInput
	Execution   string `json:"execution,omitempty" jsonschema:"auto (default), compiled, or interpreted"`
	Suite       string `json:"suite,omitempty" jsonschema:"optional exact suite name"`
	AllPackages bool   `json:"allPackages,omitempty" jsonschema:"include imported package test contracts"`
}

type ArtifactWorkflowInput struct {
	SourceInput
	Execution   string `json:"execution,omitempty" jsonschema:"interpreted (default); compiled is a compatibility alias for the same build-time interpreter"`
	AllPackages bool   `json:"allPackages,omitempty" jsonschema:"include imported package artifact lanes"`
}

type ArtifactInput struct {
	ExecutionID string `json:"executionId" jsonschema:"execution identity returned by oct_artifact"`
	ArtifactID  string `json:"artifactId" jsonschema:"artifact identity returned by oct_artifact"`
}

type Position struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type Diagnostic struct {
	Code     string   `json:"code,omitempty"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Start    Position `json:"start,omitempty"`
	End      Position `json:"end,omitempty"`
	Phase    string   `json:"phase"`
}

type Artifact struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MIMEType  string `json:"mimeType"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256"`
	ExpiresAt string `json:"expiresAt"`
}

type Timing struct {
	Milliseconds int64 `json:"milliseconds"`
}

type Envelope struct {
	OK              bool              `json:"ok"`
	ProtocolVersion string            `json:"protocolVersion"`
	OctVersion      string            `json:"octVersion"`
	CommandIdentity string            `json:"commandIdentity"`
	Tool            string            `json:"tool"`
	ExecutionID     string            `json:"executionId,omitempty"`
	Diagnostics     []Diagnostic      `json:"diagnostics"`
	Stdout          string            `json:"stdout,omitempty"`
	Stderr          string            `json:"stderr,omitempty"`
	Result          any               `json:"result,omitempty"`
	Artifacts       []Artifact        `json:"artifacts,omitempty"`
	Timing          Timing            `json:"timing,omitempty"`
	Limits          Limits            `json:"limits"`
	Provenance      map[string]string `json:"provenance"`
	ErrorCode       string            `json:"errorCode,omitempty"`
}

type commandResult struct {
	SchemaVersion      string `json:"schemaVersion"`
	Command            string `json:"command"`
	OctVersion         string `json:"octVersion"`
	ExecutionIdentity  string `json:"executionIdentity"`
	Target             string `json:"target"`
	OK                 bool   `json:"ok"`
	ExitStatus         int    `json:"exitStatus"`
	TimingMilliseconds int64  `json:"timingMilliseconds"`
	Diagnostics        []struct {
		Severity string `json:"severity"`
		Phase    string `json:"phase"`
		Message  string `json:"message"`
	} `json:"diagnostics"`
	Stdout              string   `json:"stdout"`
	DiscoveredTestFiles []string `json:"discoveredTestFiles"`
	Execution           struct {
		Requested            string `json:"requested"`
		CompiledCases        int    `json:"compiledCases"`
		InterpretedFallbacks int    `json:"interpretedFallbacks"`
	} `json:"execution"`
	Summary struct {
		Discovered int `json:"discovered"`
		Passed     int `json:"passed"`
		Failed     int `json:"failed"`
		Skipped    int `json:"skipped"`
	} `json:"summary"`
	Artifacts []struct {
		Function string `json:"function"`
		Path     string `json:"path"`
		MIMEType string `json:"mimeType"`
		Bytes    int64  `json:"bytes"`
		SHA256   string `json:"sha256"`
	} `json:"artifacts"`
	ArtifactMetadataComplete bool `json:"artifactMetadataComplete"`
}

type storedArtifact struct {
	meta    Artifact
	data    []byte
	expires time.Time
}

type artifactStore struct {
	mu     sync.Mutex
	values map[string]storedArtifact
}

func newArtifactStore() *artifactStore {
	return &artifactStore{values: make(map[string]storedArtifact)}
}

func (s *artifactStore) put(executionID string, name, mime string, data []byte, ttl time.Duration) Artifact {
	sum := sha256.Sum256(data)
	id := randomID()
	expires := time.Now().Add(ttl)
	meta := Artifact{ID: id, Name: name, MIMEType: mime, Bytes: len(data), SHA256: hex.EncodeToString(sum[:]), ExpiresAt: expires.UTC().Format(time.RFC3339)}
	s.mu.Lock()
	s.values[executionID+":"+id] = storedArtifact{meta: meta, data: append([]byte(nil), data...), expires: expires}
	s.mu.Unlock()
	return meta
}

func (s *artifactStore) get(executionID, id string) (storedArtifact, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[executionID+":"+id]
	if !ok || time.Now().After(value.expires) {
		delete(s.values, executionID+":"+id)
		return storedArtifact{}, false
	}
	return value, true
}

type service struct {
	limits      Limits
	artifacts   *artifactStore
	sem         chan struct{}
	executable  string
	tempRoot    string
	runtimeRoot string
}

func newService(limits Limits) (*service, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	runtimeRoot := strings.TrimSpace(os.Getenv("OCT_MCP_RUNTIME_ROOT"))
	if runtimeRoot == "" {
		runtimeRoot, _ = os.Getwd()
	}
	if runtimeRoot != "" {
		runtimeRoot, _ = filepath.Abs(runtimeRoot)
	}
	return &service{
		limits:      limits,
		artifacts:   newArtifactStore(),
		sem:         make(chan struct{}, limits.ConcurrentExecutions),
		executable:  executable,
		tempRoot:    filepath.Join(os.TempDir(), "oct-mcp"),
		runtimeRoot: runtimeRoot,
	}, nil
}

func randomID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func baseEnvelope(tool string, limits Limits) Envelope {
	return Envelope{
		ProtocolVersion: toolSchemaVersion,
		OctVersion:      "0.1.0-preview",
		CommandIdentity: "gooct-cli",
		Tool:            tool,
		Diagnostics:     []Diagnostic{},
		Artifacts:       []Artifact{},
		Limits:          limits,
		Provenance:      map[string]string{"serverVersion": serverVersion, "execution": "isolated-child-process", "compiler": "internal/cli"},
	}
}

func errorEnvelope(tool, code, message string, limits Limits) Envelope {
	envelope := baseEnvelope(tool, limits)
	envelope.ErrorCode = code
	envelope.Diagnostics = []Diagnostic{{Code: code, Severity: "error", Message: message, Phase: "mcp"}}
	return envelope
}

func (s *service) materialize(in SourceInput, defaultEntry string, runtimeImports bool) (string, string, map[string]struct{}, error) {
	if in.Source == "" && len(in.Files) == 0 {
		return "", "", nil, errors.New("source or files is required")
	}
	if in.Source != "" && len(in.Files) > 0 {
		return "", "", nil, errors.New("source and files cannot be combined")
	}
	files := in.Files
	entry := in.Entry
	if in.Source != "" {
		if entry == "" {
			entry = defaultEntry
		}
		files = []File{{Path: entry, Content: in.Source}}
	}
	if len(files) > s.limits.FileCount {
		return "", "", nil, fmt.Errorf("file count exceeds %d", s.limits.FileCount)
	}
	if entry == "" {
		if len(files) != 1 {
			return "", "", nil, errors.New("entry is required when files contains more than one file")
		}
		entry = files[0].Path
	}
	if err := safeVirtualPath(entry); err != nil {
		return "", "", nil, fmt.Errorf("invalid entry: %w", err)
	}
	if !isOctSourcePath(entry) {
		return "", "", nil, errors.New("entry must end in .oct, .octest, or .octfail")
	}
	total := 0
	for _, file := range files {
		if err := safeVirtualPath(file.Path); err != nil {
			return "", "", nil, fmt.Errorf("invalid file path: %w", err)
		}
		if !isOctSourcePath(file.Path) {
			return "", "", nil, fmt.Errorf("unsupported virtual file %q", file.Path)
		}
		total += len(file.Content)
		if total > s.limits.SourceBytes {
			return "", "", nil, fmt.Errorf("source exceeds %d bytes", s.limits.SourceBytes)
		}
	}
	if err := os.MkdirAll(s.tempRoot, 0o700); err != nil {
		return "", "", nil, err
	}
	workspace, err := os.MkdirTemp(s.tempRoot, "execution-")
	if err != nil {
		return "", "", nil, err
	}
	if runtimeImports {
		if err := s.copyRuntimeImports(workspace); err != nil {
			_ = os.RemoveAll(workspace)
			return "", "", nil, err
		}
	}
	inputs := make(map[string]struct{}, len(files))
	for _, file := range files {
		target := filepath.Join(workspace, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			_ = os.RemoveAll(workspace)
			return "", "", nil, err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o600); err != nil {
			_ = os.RemoveAll(workspace)
			return "", "", nil, err
		}
		inputs[target] = struct{}{}
	}
	return workspace, filepath.ToSlash(entry), inputs, nil
}

func safeVirtualPath(path string) error {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return errors.New("path must be a non-empty slash-separated relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("path traversal is not allowed")
	}
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) > 0 && (parts[0] == "Libraries" || parts[0] == "Packages") {
		return errors.New("virtual projects cannot replace server-provided runtime packages")
	}
	return nil
}

func isOctSourcePath(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".oct" || extension == ".octest" || extension == ".octfail"
}

func (s *service) copyRuntimeImports(workspace string) error {
	if s.runtimeRoot == "" {
		return nil
	}
	for _, name := range []string{"Libraries", "Packages"} {
		source := filepath.Join(s.runtimeRoot, name)
		info, err := os.Stat(source)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect runtime import root %s: %w", name, err)
		}
		if !info.IsDir() {
			continue
		}
		if err := copyTree(source, filepath.Join(workspace, name)); err != nil {
			return fmt.Errorf("copy runtime import root %s: %w", name, err)
		}
	}
	return nil
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("runtime package contains symlink %s", path)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, contents, 0o644)
	})
}

type cappedBuffer struct {
	buffer    bytes.Buffer
	max       int
	truncated bool
}

func (b *cappedBuffer) Write(value []byte) (int, error) {
	remaining := b.max - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(value), nil
	}
	if len(value) > remaining {
		_, _ = b.buffer.Write(value[:remaining])
		b.truncated = true
		return len(value), nil
	}
	return b.buffer.Write(value)
}

func (b *cappedBuffer) String() string {
	value := b.buffer.String()
	if b.truncated {
		return value + "\n[truncated by oct-mcp limit]"
	}
	return value
}

func (s *service) execute(ctx context.Context, tool, command, defaultEntry string, in SourceInput, commandArgs []string, runtimeImports bool) Envelope {
	envelope := baseEnvelope(tool, s.limits)
	started := time.Now()
	executionID := randomID()
	envelope.ExecutionID = executionID
	workspace, entry, _, err := s.materialize(in, defaultEntry, runtimeImports)
	if err != nil {
		return errorEnvelope(tool, "OCT-MCP-INPUT", err.Error(), s.limits)
	}
	defer os.RemoveAll(workspace)
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return errorEnvelope(tool, "OCT-MCP-CANCELLED", ctx.Err().Error(), s.limits)
	}
	timeout := s.limits.ExecutionSeconds
	if in.TimeoutSeconds > 0 && in.TimeoutSeconds < timeout {
		timeout = in.TimeoutSeconds
	}
	runContext, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	args := []string{"internal-exec", "--workspace", workspace, "--", command, entry}
	args = append(args, commandArgs...)
	process := exec.CommandContext(runContext, s.executable, args...)
	process.Dir = workspace
	process.Env = safeEnvironment(workspace)
	stdout := &cappedBuffer{max: s.limits.StdoutBytes}
	stderr := &cappedBuffer{max: s.limits.StderrBytes}
	process.Stdout = stdout
	process.Stderr = stderr
	runErr := process.Run()
	envelope.Timing = Timing{Milliseconds: time.Since(started).Milliseconds()}
	if runContext.Err() != nil {
		envelope.ErrorCode = "OCT-MCP-TIMEOUT"
		envelope.Diagnostics = []Diagnostic{{Code: envelope.ErrorCode, Severity: "error", Message: "execution exceeded its time limit", Phase: "runtime"}}
		return envelope
	}
	if command == "test" || command == "artifact" {
		var structured commandResult
		if err := json.Unmarshal([]byte(stdout.String()), &structured); err == nil {
			envelope.Result = structured
			envelope.Stdout = structured.Stdout
			envelope.Stderr = stderr.String()
			for _, diagnostic := range structured.Diagnostics {
				envelope.Diagnostics = append(envelope.Diagnostics, Diagnostic{Severity: diagnostic.Severity, Message: diagnostic.Message, Phase: diagnostic.Phase})
			}
			if command == "artifact" && structured.OK {
				envelope.Artifacts = s.collectReportedArtifacts(executionID, workspace, structured.Artifacts)
			}
			if structured.OK && runErr == nil {
				envelope.OK = true
				return envelope
			}
			envelope.ErrorCode = "OCT-MCP-EXEC"
			if len(envelope.Diagnostics) == 0 {
				envelope.Diagnostics = []Diagnostic{{Code: envelope.ErrorCode, Severity: "error", Message: "Oct command failed without a structured diagnostic", Phase: command}}
			}
			return envelope
		}
	}
	envelope.Stdout = stdout.String()
	envelope.Stderr = stderr.String()
	if runErr != nil {
		envelope.ErrorCode = "OCT-MCP-EXEC"
		message := strings.TrimSpace(envelope.Stderr)
		if message == "" {
			message = strings.TrimSpace(envelope.Stdout)
		}
		if message == "" {
			message = runErr.Error()
		}
		envelope.Diagnostics = []Diagnostic{{Code: envelope.ErrorCode, Severity: "error", Message: message, Phase: command}}
		return envelope
	}
	envelope.OK = true
	envelope.Result = map[string]string{"status": "completed", "command": "oct " + command}
	return envelope
}

func safeEnvironment(workspace string) []string {
	keep := []string{"PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "TMP", "TEMP"}
	environment := []string{"HOME=" + workspace, "USERPROFILE=" + workspace, "GOCACHE=" + filepath.Join(workspace, ".gocache"), "GOPATH=" + filepath.Join(workspace, ".gopath")}
	for _, key := range keep {
		if value := os.Getenv(key); value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

func (s *service) collectReportedArtifacts(executionID, workspace string, reports []struct {
	Function string `json:"function"`
	Path     string `json:"path"`
	MIMEType string `json:"mimeType"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}) []Artifact {
	artifacts := make([]Artifact, 0, len(reports))
	for _, report := range reports {
		if len(artifacts) >= s.limits.ArtifactCount || safeVirtualPath(report.Path) != nil {
			continue
		}
		mime := mimeFor(filepath.Ext(report.Path))
		if mime == "" || mime != report.MIMEType {
			continue
		}
		path := filepath.Join(workspace, filepath.FromSlash(report.Path))
		contents, err := os.ReadFile(path)
		if err != nil || len(contents) > s.limits.ArtifactBytes {
			continue
		}
		sum := sha256.Sum256(contents)
		if int64(len(contents)) != report.Bytes || hex.EncodeToString(sum[:]) != report.SHA256 {
			continue
		}
		artifacts = append(artifacts, s.artifacts.put(executionID, report.Path, mime, contents, time.Duration(s.limits.ArtifactTTLSeconds)*time.Second))
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts
}

func mimeFor(extension string) string {
	switch strings.ToLower(extension) {
	case ".png":
		return "image/png"
	case ".json":
		return "application/json"
	case ".octagon":
		return "application/octet-stream"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		return ""
	}
}

func (s *service) workspaceInfo(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, Envelope, error) {
	envelope := baseEnvelope("oct_workspace_info", s.limits)
	envelope.OK = true
	envelope.Result = map[string]any{
		"workspace": map[string]any{
			"kind":            "fresh virtual workspace per request",
			"entryRule":       "source creates one default entry; files requires entry when more than one file is submitted",
			"allowedFiles":    []string{".oct", ".octest", ".octfail"},
			"runtimePackages": []string{"server-provided Libraries and Packages are copied into fresh test/artifact workspaces; submitted files cannot replace them"},
		},
		"canonicalHostedWorkflow": []string{"oct_test", "repair submitted source from diagnostics", "oct_artifact", "oct_get_artifact"},
		"localCodexWorkflow":      []string{"inspect repository", "edit .oct/.octest", "oct test <target> --json", "oct artifact <target> --json", "inspect local paths"},
		"executionModes":          map[string]string{"test": "auto (compiled first, explicit interpreted fallback)", "artifact": "interpreted by default; compiled is explicit", "run": "source interpretation"},
	}
	return result(envelope), envelope, nil
}

func (s *service) test(ctx context.Context, _ *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, Envelope, error) {
	mode := strings.TrimSpace(input.Execution)
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "compiled" && mode != "interpreted" {
		envelope := errorEnvelope("oct_test", "OCT-MCP-INPUT", "execution must be auto, compiled, or interpreted", s.limits)
		return result(envelope), envelope, nil
	}
	args := []string{"--execution", mode, "--json"}
	if strings.TrimSpace(input.Suite) != "" {
		args = append(args, "--suite", input.Suite)
	}
	if input.AllPackages {
		args = append(args, "--all-packages")
	}
	envelope := s.execute(ctx, "oct_test", "test", "main.octest", input.SourceInput, args, true)
	return result(envelope), envelope, nil
}

func (s *service) artifact(ctx context.Context, _ *mcp.CallToolRequest, input ArtifactWorkflowInput) (*mcp.CallToolResult, Envelope, error) {
	mode := strings.TrimSpace(input.Execution)
	if mode == "" {
		mode = "interpreted"
	}
	if mode != "compiled" && mode != "interpreted" {
		envelope := errorEnvelope("oct_artifact", "OCT-MCP-INPUT", "execution must be compiled or interpreted", s.limits)
		return result(envelope), envelope, nil
	}
	args := []string{"--execution", mode, "--json"}
	if input.AllPackages {
		args = append(args, "--all-packages")
	}
	envelope := s.execute(ctx, "oct_artifact", "artifact", "main.octest", input.SourceInput, args, true)
	return result(envelope), envelope, nil
}

func (s *service) run(ctx context.Context, _ *mcp.CallToolRequest, input SourceInput) (*mcp.CallToolResult, Envelope, error) {
	if input.Entry != "" && filepath.Ext(input.Entry) != ".oct" {
		envelope := errorEnvelope("oct_run", "OCT-MCP-INPUT", "oct_run requires a .oct entry; use oct_test for .octest contracts", s.limits)
		return result(envelope), envelope, nil
	}
	envelope := s.execute(ctx, "oct_run", "run", "main.oct", input, nil, false)
	return result(envelope), envelope, nil
}

func (s *service) getArtifact(ctx context.Context, _ *mcp.CallToolRequest, input ArtifactInput) (*mcp.CallToolResult, Envelope, error) {
	envelope := baseEnvelope("oct_get_artifact", s.limits)
	value, ok := s.artifacts.get(input.ExecutionID, input.ArtifactID)
	if !ok {
		envelope = errorEnvelope(envelope.Tool, "OCT-MCP-ARTIFACT-NOT-FOUND", "artifact does not exist or has expired", s.limits)
	} else {
		envelope.OK = true
		envelope.ExecutionID = input.ExecutionID
		envelope.Result = map[string]any{"artifact": value.meta, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(value.data)}
	}
	return result(envelope), envelope, nil
}

func result(envelope Envelope) *mcp.CallToolResult {
	encoded, _ := json.Marshal(envelope)
	return &mcp.CallToolResult{IsError: !envelope.OK, Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: envelope}
}

func newServer(s *service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "oct-mcp", Version: serverVersion}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "oct_workspace_info", Description: "Use first in a hosted session to learn virtual-workspace limits and the canonical hosted Oct workflow. Local Codex should use its repository CLI instead."}, s.workspaceInfo)
	mcp.AddTool(server, &mcp.Tool{Name: "oct_test", Description: "Canonical hosted validation: run submitted .octest/.octfail contracts through oct test. Use its structured diagnostics to repair source. Auto mode reports every compiled-to-interpreted fallback."}, s.test)
	mcp.AddTool(server, &mcp.Tool{Name: "oct_artifact", Description: "Canonical hosted evidence generation: run submitted [Artifact] functions through oct artifact and return metadata for declared output files. Use oct_get_artifact only with returned IDs."}, s.artifact)
	mcp.AddTool(server, &mcp.Tool{Name: "oct_run", Description: "Hosted playground-only source execution for a bounded .oct entry. It is not the routine repository validation path; use oct_test for contracts."}, s.run)
	mcp.AddTool(server, &mcp.Tool{Name: "oct_get_artifact", Description: "Retrieve one non-expired artifact returned by oct_artifact in the same hosted server process."}, s.getArtifact)
	return server
}

func internalExec(args []string) int {
	flags := flag.NewFlagSet("internal-exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workspace := flags.String("workspace", "", "workspace")
	if err := flags.Parse(args); err != nil || *workspace == "" {
		fmt.Fprintln(os.Stderr, "invalid internal execution request")
		return 2
	}
	argv := flags.Args()
	if len(argv) > 0 && argv[0] == "--" {
		argv = argv[1:]
	}
	if len(argv) < 2 {
		return 2
	}
	err := cli.ExecuteWithContext(argv, cli.ExecutionContext{WorkingDir: *workspace, CacheRoot: filepath.Join(*workspace, ".cache"), Stdout: os.Stdout, Stderr: os.Stderr})
	if err != nil {
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "internal-exec" {
		os.Exit(internalExec(os.Args[2:]))
	}
	flags := flag.NewFlagSet("oct-mcp", flag.ExitOnError)
	stdio := flags.Bool("stdio", false, "serve MCP on stdio")
	listen := flags.String("listen", "", "serve streamable HTTP on address")
	flags.Parse(os.Args[1:])
	args := flags.Args()
	if len(args) > 0 && args[0] == "serve" {
		serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
		listen = serveFlags.String("listen", ":8080", "listen address")
		serveFlags.Parse(args[1:])
	}
	if *listen != "" && *stdio {
		log.Fatal("choose stdio or HTTP, not both")
	}
	service, err := newService(defaultLimits())
	if err != nil {
		log.Fatal(err)
	}
	server := newServer(service)
	if *listen != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		mux := http.NewServeMux()
		mux.Handle("/mcp", http.MaxBytesHandler(handler, int64(1<<20)))
		mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ok\n"))
		})
		httpServer := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
		shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		go func() {
			<-shutdown.Done()
			context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(context)
		}()
		log.Printf("oct-mcp streamable HTTP listening on %s (%s/%s)", *listen, runtime.GOOS, serverVersion)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	}
	if !*stdio && len(args) > 0 {
		log.Fatal("usage: oct-mcp --stdio | oct-mcp serve --listen :8080")
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("oct-mcp exited: %v", err)
		os.Exit(1)
	}
}
