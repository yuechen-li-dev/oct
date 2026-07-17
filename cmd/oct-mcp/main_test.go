package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests own MCP transport and virtual-workspace behavior only. Oct
// semantics remain specified by existing contracts under Language/.
func TestWorkspaceInfoAndInputBoundary(t *testing.T) {
	setRuntimeRoot(t)
	service, err := newService(defaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	_, envelope, err := service.workspaceInfo(context.Background(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil || !envelope.OK || envelope.Tool != "oct_workspace_info" {
		t.Fatalf("workspace info failed: %#v, %v", envelope, err)
	}
	_, _, _, err = service.materialize(SourceInput{Files: []File{{Path: "../main.octest", Content: "x"}}}, "main.octest", false)
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("path traversal was accepted: %v", err)
	}
	limited := defaultLimits()
	limited.SourceBytes = 8
	service, err = newService(limited)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = service.materialize(SourceInput{Source: strings.Repeat("x", 9)}, "main.octest", false)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize source was accepted: %v", err)
	}
}

func TestArtifactsAreExecutionScopedAndExpire(t *testing.T) {
	service, err := newService(defaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	artifact := service.artifacts.put("execution-a", "result.json", "application/json", []byte("{}"), time.Nanosecond)
	if _, ok := service.artifacts.get("execution-b", artifact.ID); ok {
		t.Fatal("artifact crossed execution boundary")
	}
	time.Sleep(time.Millisecond)
	if _, ok := service.artifacts.get("execution-a", artifact.ID); ok {
		t.Fatal("expired artifact remained readable")
	}
}

func TestLocalPluginConfigurationUsesRepositoryServer(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "plugins", "oct", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "\"command\": \"oct-mcp\"") || !strings.Contains(string(contents), "--stdio") {
		t.Fatalf("plugin no longer points at the local server: %s", contents)
	}
}

func TestStdioGoldenFlowAndArtifactRetrieval(t *testing.T) {
	setRuntimeRoot(t)
	executable := buildServer(t)
	invalid := readFixture(t, "..", "..", "testdata", "m11", "invalid", "non_bool_if.oct")
	valid := readFixture(t, "..", "..", "Language", "Types", "UnitsM1", "valid", "signed_exponents_and_hz_alias_m1.octest")
	artifactSource := readFixture(t, "..", "..", "Experiments", "OctErgonomicsLab", "M0", "oct_ergonomics_lab_m0.oct")
	artifactSuite := readFixture(t, "..", "..", "Experiments", "OctErgonomicsLab", "M0", "oct_ergonomics_lab_m0.octest")

	client := mcp.NewClient(&mcp.Implementation{Name: "oct-mcp-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: exec.Command(executable, "--stdio")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"oct_workspace_info": true, "oct_test": true, "oct_artifact": true, "oct_run": true, "oct_get_artifact": true}
	if len(tools.Tools) != len(want) {
		t.Fatalf("tool discovery = %d, want %d", len(tools.Tools), len(want))
	}
	for _, tool := range tools.Tools {
		if !want[tool.Name] || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("unexpected public tool: %#v", tool)
		}
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools: %#v", want)
	}

	invalidCall, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "oct_test", Arguments: map[string]any{"source": string(invalid)}})
	if err != nil || !invalidCall.IsError {
		t.Fatalf("invalid contract did not return a structured failure: %v, %#v", err, invalidCall)
	}
	invalidEnvelope := decodeToolEnvelope(t, invalidCall)
	if len(invalidEnvelope.Diagnostics) == 0 || !strings.Contains(invalidEnvelope.Diagnostics[0].Message, "must be Bool") {
		t.Fatalf("invalid contract lost actionable diagnostic: %#v", invalidEnvelope)
	}

	validCall, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "oct_test", Arguments: map[string]any{"source": string(valid), "execution": "auto"}})
	if err != nil || validCall.IsError {
		t.Fatalf("repaired contract did not pass: %v, %#v", err, validCall)
	}
	validEnvelope := decodeToolEnvelope(t, validCall)
	if !validEnvelope.OK || !strings.Contains(validEnvelope.Stdout, "PASS UnitsM1Valid.SignedExponentsAndHzAliasM1") {
		t.Fatalf("valid contract result is incomplete: %#v", validEnvelope)
	}
	validResult, ok := validEnvelope.Result.(map[string]any)
	if !ok || validResult["schemaVersion"] != "oct.cli.result.v1" {
		t.Fatalf("MCP lost the CLI result identity: %#v", validEnvelope.Result)
	}
	execution, ok := validResult["execution"].(map[string]any)
	if !ok || execution["compiledCases"] == nil || execution["interpretedFallbacks"] == nil {
		t.Fatalf("MCP lost explicit execution/fallback fields: %#v", validEnvelope.Result)
	}

	artifactCall, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "oct_artifact", Arguments: map[string]any{"entry": "suite.octest", "files": []map[string]string{{"path": "main.oct", "content": string(artifactSource)}, {"path": "suite.octest", "content": string(artifactSuite)}}}})
	if err != nil || artifactCall.IsError {
		t.Fatalf("artifact workflow failed: %v, %#v", err, artifactCall)
	}
	artifactEnvelope := decodeToolEnvelope(t, artifactCall)
	if len(artifactEnvelope.Artifacts) != 4 {
		t.Fatalf("artifact metadata = %#v, want four generated files", artifactEnvelope.Artifacts)
	}
	first := artifactEnvelope.Artifacts[0]
	retrieval, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "oct_get_artifact", Arguments: map[string]any{"executionId": artifactEnvelope.ExecutionID, "artifactId": first.ID}})
	if err != nil || retrieval.IsError {
		t.Fatalf("artifact retrieval failed: %v, %#v", err, retrieval)
	}
	if !strings.Contains(toolText(t, retrieval), "base64") || !strings.Contains(toolText(t, retrieval), first.SHA256) {
		t.Fatalf("retrieval omitted verified content metadata: %s", toolText(t, retrieval))
	}
}

func TestStreamableHTTPToolDiscovery(t *testing.T) {
	setRuntimeRoot(t)
	service, err := newService(defaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newServer(service) }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "oct-mcp-http-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 5 {
		t.Fatalf("HTTP discovery failed: %v, %#v", err, tools)
	}
}

func buildServer(t *testing.T) string {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "oct-mcp")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	build := exec.Command("go", "build", "-o", executable, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, output)
	}
	return executable
}

func setRuntimeRoot(t *testing.T) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MCP_RUNTIME_ROOT", repoRoot)
}

func readFixture(t *testing.T, elements ...string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(elements...))
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func decodeToolEnvelope(t *testing.T, call *mcp.CallToolResult) Envelope {
	t.Helper()
	var envelope Envelope
	if err := json.Unmarshal([]byte(toolText(t, call)), &envelope); err != nil {
		t.Fatalf("decode tool envelope: %v", err)
	}
	return envelope
}

func toolText(t *testing.T, call *mcp.CallToolResult) string {
	t.Helper()
	if len(call.Content) != 1 {
		t.Fatalf("tool content = %#v", call.Content)
	}
	text, ok := call.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool content type = %T", call.Content[0])
	}
	return text.Text
}
