package sdslv

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func CheckFile(path string) error {
	module, err := loadModule(path)
	if err != nil {
		return err
	}
	return validate.Module(module)
}

func EmitHLSLFile(path string) (string, error) {
	mir, err := LowerFile(path)
	if err != nil {
		return "", err
	}
	return hlsl.Emit(mir)
}

func EmitVDMIRFile(path string) (string, error) {
	mir, err := LowerFile(path)
	if err != nil {
		return "", err
	}
	return vdmir.Dump(mir), nil
}

func WriteHLSLFile(inputPath, outputPath string) error {
	text, err := EmitHLSLFile(inputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write HLSL %s: %w", outputPath, err)
	}
	return nil
}

func LowerFile(path string) (vdmir.Module, error) {
	module, err := loadModule(path)
	if err != nil {
		return vdmir.Module{}, err
	}
	if err := validate.Module(module); err != nil {
		return vdmir.Module{}, err
	}
	return lower.Module(module)
}

func loadModule(path string) (ast.Module, error) {
	file, err := source.Load(path)
	if err != nil {
		return ast.Module{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return ast.Module{}, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return ast.Module{}, err
	}
	return module, nil
}
