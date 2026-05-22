package octagon

import (
	"fmt"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func Load(path string) (ast.Expr, error) {
	if filepath.Ext(path) != ".octagon" {
		return nil, fmt.Errorf("octagon file must use .octagon extension")
	}
	file, err := source.Load(path)
	if err != nil {
		return nil, err
	}
	lexed, err := lex.Analyze(file)
	if err != nil {
		return nil, err
	}
	return parse.BuildDataValue(lexed)
}
