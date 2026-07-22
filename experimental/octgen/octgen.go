// Package octgen exposes the experimental external Oct generation boundary.
//
// It is not part of Oct 1.0. Hosts own their typed model decoding and Go
// rendering; this package owns normal Oct execution and declared Go-output
// handling without granting interpreted Oct filesystem authority.
package octgen

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// Kind identifies the bounded runtime value forms that can cross the
// interpreter/host boundary. It intentionally is not an Oct reflection API.
type Kind string

const (
	ValueInt    Kind = "Int"
	ValueBool   Kind = "Bool"
	ValueString Kind = "String"
	ValueArray  Kind = "Array"
	ValueRecord Kind = "Record"
	ValueEnum   Kind = "Enum"
)

// Value is a host-safe structural view of an interpreted Oct value. Unsupported
// runtime kinds are retained by their Kind but expose no implementation details.
type Value struct {
	Kind   Kind
	Int    int64
	Bool   bool
	Text   string
	Array  []Value
	Record Record
	Enum   Enum
}

// Record is the bounded record view made available to a consumer decoder.
type Record struct {
	TypeName string
	Fields   map[string]Value
}

// Enum is the bounded enum view made available to a consumer decoder.
type Enum struct {
	TypeName string
	Variant  string
}

// Artifact is a fully rendered Go output. The caller supplies all destinations;
// Oct models never choose paths.
type Artifact struct {
	Path    string
	Content []byte
}

// Execute loads, type-checks, and interprets the ordinary Generate function in
// generatorPath through Oct's existing implementation.
func Execute(generatorPath string) (Value, error) {
	program, err := project.Load(generatorPath)
	if err != nil {
		return Value{}, fmt.Errorf("load OctGen generator %s: %w", generatorPath, err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return Value{}, fmt.Errorf("type-check OctGen generator %s: %w", generatorPath, err)
	}
	value, err := interpret.CallFunctionWithArgsAndOptions(program, program.Entry, "Generate", nil, io.Discard, interpret.ExecuteOptions{})
	if err != nil {
		return Value{}, fmt.Errorf("interpret OctGen generator %s: %w", generatorPath, err)
	}
	return convertValue(value), nil
}

// ValidateArtifacts verifies that every declared output is a distinct .go file
// directly beside the generator. This bounded layout makes path authority
// explicit and prevents traversal or undeclared sibling/parent writes.
func ValidateArtifacts(generatorPath string, artifacts []Artifact) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("OctGen requires at least one declared output")
	}
	generatorDir, err := filepath.Abs(filepath.Dir(generatorPath))
	if err != nil {
		return fmt.Errorf("resolve generator directory: %w", err)
	}
	seen := map[string]struct{}{}
	for _, artifact := range artifacts {
		if filepath.Ext(artifact.Path) != ".go" {
			return fmt.Errorf("OctGen output must be a .go file: %s", artifact.Path)
		}
		outputAbs, err := filepath.Abs(artifact.Path)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		rel, err := filepath.Rel(generatorDir, outputAbs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) || filepath.Dir(rel) != "." {
			return fmt.Errorf("OctGen output escapes declared destination %s: %s", generatorDir, artifact.Path)
		}
		key := filepath.Clean(outputAbs)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("OctGen output destination is duplicated: %s", artifact.Path)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// Check compares rendered artifacts with their committed destinations without
// modifying anything. A combined diagnostic identifies every stale output.
func Check(generatorPath string, artifacts []Artifact) error {
	if err := ValidateArtifacts(generatorPath, artifacts); err != nil {
		return err
	}
	stale := make([]string, 0)
	for _, artifact := range artifacts {
		existing, err := os.ReadFile(artifact.Path)
		if err != nil || !bytes.Equal(existing, artifact.Content) {
			stale = append(stale, artifact.Path)
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("generated Go is stale: %s; run OctGen generation", strings.Join(stale, ", "))
	}
	return nil
}

// Write validates every destination, stages every artifact, then replaces the
// whole set. Filesystem rename cannot be globally atomic across multiple paths;
// on replacement failure Write removes newly installed files and restores staged
// backups before returning the error.
func Write(generatorPath string, artifacts []Artifact) error {
	if err := ValidateArtifacts(generatorPath, artifacts); err != nil {
		return err
	}
	staged, err := stageArtifacts(artifacts)
	if err != nil {
		return err
	}
	defer removePaths(staged.temps)

	if err := staged.backupExisting(); err != nil {
		staged.restoreBackups()
		return err
	}
	if err := staged.install(); err != nil {
		staged.rollbackInstalled()
		staged.restoreBackups()
		return err
	}
	removePaths(staged.backups)
	return nil
}

type stagedArtifacts struct {
	artifacts []Artifact
	temps     []string
	backups   []string
	installed []string
}

func stageArtifacts(artifacts []Artifact) (*stagedArtifacts, error) {
	staged := &stagedArtifacts{artifacts: artifacts, temps: make([]string, 0, len(artifacts)), backups: make([]string, 0, len(artifacts))}
	for _, artifact := range artifacts {
		directory := filepath.Dir(artifact.Path)
		temporary, err := os.CreateTemp(directory, ".octgen-*.tmp")
		if err != nil {
			removePaths(staged.temps)
			return nil, fmt.Errorf("create temporary OctGen output: %w", err)
		}
		name := temporary.Name()
		if _, err := temporary.Write(artifact.Content); err != nil {
			temporary.Close()
			os.Remove(name)
			removePaths(staged.temps)
			return nil, fmt.Errorf("write temporary OctGen output: %w", err)
		}
		if err := temporary.Chmod(0o644); err != nil {
			temporary.Close()
			os.Remove(name)
			removePaths(staged.temps)
			return nil, fmt.Errorf("set temporary OctGen output permissions: %w", err)
		}
		if err := temporary.Close(); err != nil {
			os.Remove(name)
			removePaths(staged.temps)
			return nil, fmt.Errorf("close temporary OctGen output: %w", err)
		}
		staged.temps = append(staged.temps, name)
	}
	return staged, nil
}

func (s *stagedArtifacts) backupExisting() error {
	for _, artifact := range s.artifacts {
		backup := artifact.Path + ".octgen-backup"
		if _, err := os.Stat(artifact.Path); err != nil {
			if os.IsNotExist(err) {
				s.backups = append(s.backups, "")
				continue
			}
			return fmt.Errorf("stat generated Go %s: %w", artifact.Path, err)
		}
		if err := os.Rename(artifact.Path, backup); err != nil {
			return fmt.Errorf("stage existing generated Go %s: %w", artifact.Path, err)
		}
		s.backups = append(s.backups, backup)
	}
	return nil
}

func (s *stagedArtifacts) install() error {
	for index, artifact := range s.artifacts {
		if err := os.Rename(s.temps[index], artifact.Path); err != nil {
			return fmt.Errorf("replace generated Go %s: %w", artifact.Path, err)
		}
		s.installed = append(s.installed, artifact.Path)
	}
	return nil
}

func (s *stagedArtifacts) rollbackInstalled() {
	removePaths(s.installed)
}

func (s *stagedArtifacts) restoreBackups() {
	for index, backup := range s.backups {
		if backup == "" {
			continue
		}
		_ = os.Rename(backup, s.artifacts[index].Path)
	}
}

func removePaths(paths []string) {
	for _, path := range paths {
		if path != "" {
			_ = os.Remove(path)
		}
	}
}

func convertValue(value interpret.Value) Value {
	converted := Value{Kind: Kind(value.Kind), Int: value.Int, Bool: value.Bool, Text: value.Text}
	switch value.Kind {
	case interpret.ValueArray:
		converted.Array = make([]Value, 0, len(value.Array))
		for _, item := range value.Array {
			converted.Array = append(converted.Array, convertValue(item))
		}
	case interpret.ValueRecord:
		converted.Record = Record{TypeName: value.Record.TypeName, Fields: make(map[string]Value, len(value.Record.Fields))}
		for name, field := range value.Record.Fields {
			converted.Record.Fields[name] = convertValue(field)
		}
	case interpret.ValueEnum:
		converted.Enum = Enum{TypeName: value.Enum.TypeName, Variant: value.Enum.Variant}
	}
	return converted
}
