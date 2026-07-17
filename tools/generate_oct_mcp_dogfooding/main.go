// Command generate_oct_mcp_dogfooding writes the deterministic machine-readable
// record accompanying the Oct MCP agent dogfooding report.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const defaultOutput = "docs/development/artifacts/oct_mcp_agent_dogfooding.json"

type report struct {
	SchemaVersion        string     `json:"schemaVersion"`
	CLISchemaVersion     string     `json:"cliSchemaVersion"`
	MCPToolSchemaVersion string     `json:"mcpToolSchemaVersion"`
	Generation           string     `json:"generation"`
	Scenarios            []scenario `json:"scenarios"`
	FinalToolSurface     []string   `json:"finalToolSurface"`
	SkillHashes          []fileHash `json:"skillHashes"`
	UnsupportedClaims    []string   `json:"unsupportedClaims"`
}

type scenario struct {
	ID               string   `json:"id"`
	Workflow         string   `json:"workflow"`
	Skill            string   `json:"skill"`
	MCPTools         []string `json:"mcpTools"`
	Commands         []string `json:"commands"`
	RepairIterations int      `json:"repairIterations"`
	Diagnostic       string   `json:"diagnostic"`
	Fallback         string   `json:"fallback"`
	Outcome          string   `json:"outcome"`
}

type fileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func main() {
	output := flag.String("out", defaultOutput, "repository-relative output path")
	flag.Parse()
	root, err := findRepositoryRoot()
	if err != nil {
		fatal(err)
	}
	contents, err := generate(root)
	if err != nil {
		fatal(err)
	}
	target := *output
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		fatal(err)
	}
}

func generate(root string) ([]byte, error) {
	skillHashes, err := collectHashes(root, []string{
		"plugins/oct/skills/oct-workflow/SKILL.md",
		"plugins/oct/skills/oct-experiments/SKILL.md",
	})
	if err != nil {
		return nil, err
	}
	payload := report{
		SchemaVersion:        "oct.mcp.agent.dogfood.v1",
		CLISchemaVersion:     "oct.cli.result.v1",
		MCPToolSchemaVersion: "2.0",
		Generation:           "deterministic; no timestamp, host path, execution ID, or duration",
		Scenarios: []scenario{
			{ID: "experiment-scaffold-default-placement", Workflow: "mixed-local", Skill: "oct-experiments", Commands: []string{"oct new experiment DogfoodDefaultExperiment", "oct test Experiments/DogfoodDefaultExperiment --execution auto --json"}, Diagnostic: "the existing Experiments directory selected the default target; root result reported M0 as the one selected test file", Fallback: "0", Outcome: "compiled M0 scaffold passed"},
			{ID: "experiment-milestone-artifact", Workflow: "mixed-local", Skill: "oct-experiments", Commands: []string{"oct test .dogfood/McpExperiment/M0 --execution auto --json", "oct artifact .dogfood/McpExperiment --execution interpreted --json"}, Diagnostic: "root artifact result retained milestone-prefixed pass counts and a CSV hash", Fallback: "0", Outcome: "focused test and root milestone artifact passed"},
			{ID: "minimal-octest", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/McpDogfood --execution auto --json"}, Diagnostic: "pass/fail summary and execution counts", Fallback: "compiled fallback observed separately", Outcome: "passed"},
			{ID: "syntax-repair", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/McpDogfood --execution auto"}, RepairIterations: 1, Diagnostic: "standalone-expression guidance identified the trailing invalid token", Fallback: "none", Outcome: "repaired then passed"},
			{ID: "type-repair", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/McpDogfood --execution auto"}, RepairIterations: 1, Diagnostic: "Identity argument expected Int, got String", Fallback: "none", Outcome: "repaired then passed"},
			{ID: "unit-repair", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/McpDogfood --execution auto"}, RepairIterations: 1, Diagnostic: "let annotation expected Float<s>, got Float<m/s>", Fallback: "none", Outcome: "repaired then passed"},
			{ID: "record-table", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/McpDogfood --execution auto --json"}, Diagnostic: "record table row indexing compiled limitation was explicit", Fallback: "2 interpreted fallbacks", Outcome: "interpreter pass with disclosed compiled gap"},
			{ID: "artifact", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct artifact .dogfood/McpDogfood --execution interpreted --json"}, RepairIterations: 2, Diagnostic: "artifact import and manifest dependency needed two real repairs", Fallback: "interpreted is canonical artifact default", Outcome: "csv, markdown, and json metadata hashed"},
			{ID: "artifact-package-scope", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct artifact .dogfood/McpDogfood --execution interpreted --json", "oct artifact .dogfood/McpDogfood --execution interpreted --all-packages --json"}, Diagnostic: "imported Artifact package lane was unexpectedly executed before scoped default", Fallback: "none", Outcome: "default narrowed; --all-packages is explicit"},
			{ID: "existing-experiment", Workflow: "mixed-local", Skill: "oct-workflow", Commands: []string{"oct test Experiments/LanguageFriction/ArrayMapGenerics/array_transform_manual.octest --execution auto"}, Diagnostic: "focused file output limited the run to two facts", Fallback: "0", Outcome: "passed after a repository experiment edit"},
			{ID: "compiled-success", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test examples/SmartGreenhouseController --execution compiled"}, Diagnostic: "execution summary", Fallback: "0", Outcome: "7 compiled cases passed"},
			{ID: "empty-workspace", Workflow: "direct-cli", Skill: "oct-workflow", Commands: []string{"oct test .dogfood/Empty --execution auto"}, Diagnostic: "unknown package Main", Fallback: "not applicable", Outcome: "expected failure; no hidden scaffold"},
			{ID: "current-mcp-gap", Workflow: "mcp-only-pre-redesign", Skill: "legacy-five-skills", MCPTools: []string{"oct_check", "oct_run"}, Diagnostic: "source entry was restricted to .oct and could not run .octest contracts", Fallback: "not represented", Outcome: "failed product fit"},
			{ID: "hosted-golden-flow", Workflow: "mcp-only-final", Skill: "oct-workflow", MCPTools: []string{"oct_workspace_info", "oct_test", "oct_artifact", "oct_get_artifact"}, RepairIterations: 1, Diagnostic: "structured oct_test diagnostic repaired an existing invalid fixture", Fallback: "reported by embedded CLI result", Outcome: "valid contract, four artifacts, and scoped retrieval passed"},
			{ID: "http-discovery", Workflow: "mcp-only-final", Skill: "oct-workflow", MCPTools: []string{"oct_workspace_info"}, Diagnostic: "streamable HTTP tool discovery", Fallback: "not applicable", Outcome: "passed"},
		},
		FinalToolSurface: []string{"oct_workspace_info", "oct_test", "oct_artifact", "oct_run", "oct_get_artifact"},
		SkillHashes:      skillHashes,
		UnsupportedClaims: []string{
			"arbitrary shell", "unrestricted filesystem", "unrestricted network", "Python fallback", "Prometheus GPU execution", "image generation", "package installation", "notebooks", "IDE UI", "generic remote build service",
		},
	}
	for index := range payload.Scenarios {
		if payload.Scenarios[index].MCPTools == nil {
			payload.Scenarios[index].MCPTools = []string{}
		}
		if payload.Scenarios[index].Commands == nil {
			payload.Scenarios[index].Commands = []string{}
		}
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func collectHashes(root string, paths []string) ([]fileHash, error) {
	hashes := make([]fileHash, 0, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(contents)
		hashes = append(hashes, fileHash{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}
	return hashes, nil
}

func findRepositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not find repository root")
		}
		current = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
