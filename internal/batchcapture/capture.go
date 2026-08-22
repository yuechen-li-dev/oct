package batchcapture

import (
	"sort"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// Names performs lexical free-variable analysis over one batch body. Callers
// intersect the result with their own binding authority, so top-level function
// and package names remain ordinary symbols.
func Names(body ast.Block, itemName string) []string {
	free := map[string]struct{}{}
	defined := map[string]struct{}{itemName: {}}
	collectBatchBlockFree(body, defined, free)
	names := make([]string, 0, len(free))
	for name := range free {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func collectBatchBlockFree(block ast.Block, defined map[string]struct{}, free map[string]struct{}) {
	for _, statement := range block.Statements {
		switch node := statement.(type) {
		case ast.LetStmt:
			collectBatchExprFree(node.Value, defined, free)
			defined[node.Name] = struct{}{}
		case ast.VarStmt:
			collectBatchExprFree(node.Value, defined, free)
			defined[node.Name] = struct{}{}
		case ast.AssignStmt:
			collectBatchTargetFree(node.Name, defined, free)
			collectBatchExprFree(node.Value, defined, free)
		case ast.DestructureAssignStmt:
			collectBatchExprFree(node.Value, defined, free)
			for _, name := range node.Names {
				if _, ok := defined[name]; !ok {
					defined[name] = struct{}{}
				}
			}
		case ast.IndexAssignStmt:
			collectBatchTargetFree(node.Target, defined, free)
			for _, index := range node.Indices {
				collectBatchExprFree(index, defined, free)
			}
			collectBatchExprFree(node.Value, defined, free)
		case ast.FieldAssignStmt:
			collectBatchTargetFree(node.Target, defined, free)
			collectBatchExprFree(node.Value, defined, free)
		case ast.FieldIndexAssignStmt:
			collectBatchTargetFree(node.Target, defined, free)
			for _, index := range node.Indices {
				collectBatchExprFree(index, defined, free)
			}
			collectBatchExprFree(node.Value, defined, free)
		case ast.ReturnStmt:
			collectBatchExprFree(node.Value, defined, free)
		case ast.ExprStmt:
			collectBatchExprFree(node.Value, defined, free)
		case ast.ForStmt:
			collectBatchExprFree(node.Range, defined, free)
			collectBatchExprFree(node.DescendStep, defined, free)
			child := cloneBatchScope(defined)
			child[node.Name] = struct{}{}
			collectBatchBlockFree(node.Body, child, free)
		case ast.MatchStmt:
			collectBatchExprFree(node.Subject, defined, free)
			okScope := cloneBatchScope(defined)
			okScope[node.OkName] = struct{}{}
			collectBatchBlockFree(node.OkBody, okScope, free)
			errScope := cloneBatchScope(defined)
			errScope[node.ErrName] = struct{}{}
			collectBatchBlockFree(node.ErrBody, errScope, free)
		case ast.IfStmt:
			collectBatchExprFree(node.Condition, defined, free)
			collectBatchBlockFree(node.ThenBody, cloneBatchScope(defined), free)
			if node.ElseBody != nil {
				collectBatchBlockFree(*node.ElseBody, cloneBatchScope(defined), free)
			}
		case ast.WhileStmt:
			collectBatchExprFree(node.Condition, defined, free)
			collectBatchBlockFree(node.Body, cloneBatchScope(defined), free)
		case ast.PrometheusStmt:
			collectBatchBlockFree(node.Body, cloneBatchScope(defined), free)
		case ast.YieldStmt:
			collectBatchExprFree(node.Value, defined, free)
		case ast.WhenStmt:
			for _, whenCase := range node.Cases {
				collectBatchExprFree(whenCase.Condition, defined, free)
				collectBatchWhenActionFree(whenCase.Action, cloneBatchScope(defined), free)
			}
			collectBatchWhenActionFree(node.Else, cloneBatchScope(defined), free)
		}
	}
}

func collectBatchWhenActionFree(action ast.WhenAction, defined map[string]struct{}, free map[string]struct{}) {
	switch node := action.(type) {
	case ast.WhenReturnAction:
		collectBatchExprFree(node.Value, defined, free)
	case ast.WhenBlockAction:
		collectBatchBlockFree(ast.Block{Statements: node.Statements}, defined, free)
	}
}

func collectBatchExprFree(expr ast.Expr, defined map[string]struct{}, free map[string]struct{}) {
	if expr == nil {
		return
	}
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		collectBatchTargetFree(node.Name, defined, free)
	case ast.ArrayLiteralExpr:
		for _, element := range node.Elements {
			collectBatchExprFree(element, defined, free)
		}
	case ast.VectorLiteralExpr:
		for _, element := range node.Elements {
			collectBatchExprFree(element, defined, free)
		}
	case ast.MatrixLiteralExpr:
		for _, row := range node.Rows {
			for _, element := range row {
				collectBatchExprFree(element, defined, free)
			}
		}
	case ast.CallExpr:
		collectBatchExprFree(node.Callee, defined, free)
		for _, argument := range node.Arguments {
			collectBatchExprFree(argument, defined, free)
		}
	case ast.IndexExpr:
		collectBatchExprFree(node.Target, defined, free)
		for _, index := range node.Indices {
			collectBatchExprFree(index, defined, free)
		}
	case ast.FieldAccessExpr:
		collectBatchExprFree(node.Target, defined, free)
	case ast.BinaryExpr:
		collectBatchExprFree(node.Left, defined, free)
		collectBatchExprFree(node.Right, defined, free)
	case ast.UnaryExpr:
		collectBatchExprFree(node.Operand, defined, free)
	case ast.RangeExpr:
		collectBatchExprFree(node.Start, defined, free)
		collectBatchExprFree(node.End, defined, free)
		collectBatchExprFree(node.Step, defined, free)
	case ast.ParenExpr:
		collectBatchExprFree(node.Inner, defined, free)
	case ast.PropagateExpr:
		collectBatchExprFree(node.Inner, defined, free)
	case ast.UnwrapExpr:
		collectBatchExprFree(node.Inner, defined, free)
	case ast.SwitchExpr:
		collectBatchExprFree(node.Subject, defined, free)
		for _, switchCase := range node.Cases {
			collectBatchExprFree(switchCase.Match, defined, free)
			collectBatchExprFree(switchCase.Value, defined, free)
		}
		collectBatchExprFree(node.Else, defined, free)
	case ast.MatchExpr:
		collectBatchExprFree(node.Subject, defined, free)
		for _, matchCase := range node.Cases {
			child := cloneBatchScope(defined)
			if matchCase.Binding != "" {
				child[matchCase.Binding] = struct{}{}
			}
			collectBatchExprFree(matchCase.Value, child, free)
		}
	case ast.IfExpr:
		collectBatchExprFree(node.Condition, defined, free)
		collectBatchExprFree(node.ThenExpr, defined, free)
		collectBatchExprFree(node.ElseExpr, defined, free)
	case ast.UtilityWhenExpr:
		collectBatchExprFree(node.Policy.Hysteresis, defined, free)
		collectBatchExprFree(node.Policy.MinCommit, defined, free)
		for _, utilityCase := range node.Cases {
			collectBatchExprFree(utilityCase.Value, defined, free)
			collectBatchExprFree(utilityCase.Condition, defined, free)
			collectBatchExprFree(utilityCase.Score, defined, free)
		}
		collectBatchExprFree(node.Else, defined, free)
	case ast.BatchExpr:
		collectBatchExprFree(node.Input, defined, free)
		child := cloneBatchScope(defined)
		child[node.ItemName] = struct{}{}
		collectBatchBlockFree(node.Body, child, free)
	case ast.RecordLiteralExpr:
		for _, field := range node.Fields {
			collectBatchExprFree(field.Value, defined, free)
		}
	case ast.RecordUpdateExpr:
		collectBatchExprFree(node.Source, defined, free)
		for _, field := range node.Fields {
			collectBatchExprFree(field.Value, defined, free)
		}
	}
}

func collectBatchTargetFree(name string, defined map[string]struct{}, free map[string]struct{}) {
	if _, ok := defined[name]; !ok {
		free[name] = struct{}{}
	}
}

func cloneBatchScope(input map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(input))
	for name := range input {
		clone[name] = struct{}{}
	}
	return clone
}
