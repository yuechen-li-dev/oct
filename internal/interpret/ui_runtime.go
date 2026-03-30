package interpret

import (
	"fmt"
	"strings"

	"oct/internal/ast"
)

type uiNodeKind string

const (
	uiNodeText   uiNodeKind = "Text"
	uiNodeButton uiNodeKind = "Button"
	uiNodeColumn uiNodeKind = "Column"
	uiNodeRow    uiNodeKind = "Row"
)

type uiNode struct {
	Kind     uiNodeKind
	Text     string
	Label    string
	Event    string
	Children []*uiNode
}

type uiMount struct {
	Root   *uiNode
	Events []string
}

func cloneUI(node *uiNode) *uiNode {
	if node == nil {
		return nil
	}
	children := make([]*uiNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, cloneUI(child))
	}
	return &uiNode{Kind: node.Kind, Text: node.Text, Label: node.Label, Event: node.Event, Children: children}
}

func uiSignature(node *uiNode) string {
	if node == nil {
		return "<nil>"
	}
	switch node.Kind {
	case uiNodeText:
		return "Text(" + node.Text + ")"
	case uiNodeButton:
		return "Button(" + node.Label + "->" + node.Event + ")"
	case uiNodeColumn, uiNodeRow:
		parts := make([]string, 0, len(node.Children))
		for _, child := range node.Children {
			parts = append(parts, uiSignature(child))
		}
		return string(node.Kind) + "[" + strings.Join(parts, ",") + "]"
	default:
		return "<unknown>"
	}
}

func uiTreeContainsEvent(node *uiNode, token string) bool {
	if node == nil {
		return false
	}
	if node.Kind == uiNodeButton && node.Event == token {
		return true
	}
	for _, child := range node.Children {
		if uiTreeContainsEvent(child, token) {
			return true
		}
	}
	return false
}

func (i interpreter) evalUIBuiltinCallExpr(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	switch callee {
	case "UIText":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIText expects 1 argument")
		}
		content, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if content.hasError {
			return evalResult{hasError: true, errorVal: content.errorVal}, nil
		}
		if content.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIText expects String argument")
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiNode{Kind: uiNodeText, Text: content.value.Text}}}, nil
	case "UIButton":
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIButton expects 2 arguments")
		}
		label, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if label.hasError {
			return evalResult{hasError: true, errorVal: label.errorVal}, nil
		}
		event, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if event.hasError {
			return evalResult{hasError: true, errorVal: event.errorVal}, nil
		}
		if label.value.Kind != ValueString || event.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIButton expects String label/event")
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiNode{Kind: uiNodeButton, Label: label.value.Text, Event: event.value.Text}}}, nil
	case "UIColumn", "UIRow":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", callee)
		}
		children, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if children.hasError {
			return evalResult{hasError: true, errorVal: children.errorVal}, nil
		}
		if children.value.Kind != ValueArray {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects UI[]", callee)
		}
		nodes := make([]*uiNode, 0, len(children.value.Array))
		for idx, child := range children.value.Array {
			if child.Kind != ValueUI || child.UI == nil {
				return evalResult{}, fmt.Errorf("runtime invariant violation: %s children[%d] expects UI", callee, idx)
			}
			nodes = append(nodes, cloneUI(child.UI))
		}
		kind := uiNodeColumn
		if callee == "UIRow" {
			kind = uiNodeRow
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiNode{Kind: kind, Children: nodes}}}, nil
	case "UIMount":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIMount expects 1 argument")
		}
		root, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if root.hasError {
			return evalResult{hasError: true, errorVal: root.errorVal}, nil
		}
		if root.value.Kind != ValueUI || root.value.UI == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIMount expects UI")
		}
		handle := i.uiMounts.allocate(&uiMount{Root: cloneUI(root.value.UI)})
		return evalResult{value: Value{Kind: ValueInt, Int: handle}}, nil
	case "UIPatch":
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIPatch expects 2 arguments")
		}
		handle, nextRoot, errResult, err := i.evalUIMountAndRootArgs(env, pkgName, callee, argumentExprs[0], argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if errResult != nil {
			return *errResult, nil
		}
		handle.Root = cloneUI(nextRoot)
		return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
	case "UIUnmount":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIUnmount expects 1 argument")
		}
		handleValue, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if handleValue.hasError {
			return evalResult{hasError: true, errorVal: handleValue.errorVal}, nil
		}
		if handleValue.value.Kind != ValueInt {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIUnmount expects mount Int")
		}
		i.uiMounts.release(handleValue.value.Int)
		return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
	case "UIEmit":
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIEmit expects 2 arguments")
		}
		handleValue, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if handleValue.hasError {
			return evalResult{hasError: true, errorVal: handleValue.errorVal}, nil
		}
		eventValue, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if eventValue.hasError {
			return evalResult{hasError: true, errorVal: eventValue.errorVal}, nil
		}
		if handleValue.value.Kind != ValueInt || eventValue.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIEmit expects (Int, String)")
		}
		mount, err := i.uiMounts.get(handleValue.value.Int)
		if err != nil {
			return wrapperErrorResult(callee, err), nil
		}
		if !uiTreeContainsEvent(mount.Root, eventValue.value.Text) {
			return wrapperErrorResult(callee, fmt.Errorf("event %q not found in mounted UI", eventValue.value.Text)), nil
		}
		mount.Events = append(mount.Events, eventValue.value.Text)
		return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
	case "UIDrainEvents":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIDrainEvents expects 1 argument")
		}
		handleValue, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if handleValue.hasError {
			return evalResult{hasError: true, errorVal: handleValue.errorVal}, nil
		}
		if handleValue.value.Kind != ValueInt {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIDrainEvents expects Int")
		}
		mount, err := i.uiMounts.get(handleValue.value.Int)
		if err != nil {
			return wrapperErrorResult(callee, err), nil
		}
		events := make([]Value, 0, len(mount.Events))
		for _, event := range mount.Events {
			events = append(events, Value{Kind: ValueString, Text: event})
		}
		mount.Events = nil
		return evalResult{value: Value{Kind: ValueArray, Array: events}}, nil
	case "UISignature":
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UISignature expects 1 argument")
		}
		root, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if root.hasError {
			return evalResult{hasError: true, errorVal: root.errorVal}, nil
		}
		if root.value.Kind != ValueUI || root.value.UI == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UISignature expects UI")
		}
		return evalResult{value: Value{Kind: ValueString, Text: uiSignature(root.value.UI)}}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported UI builtin %s", callee)
	}
}

func (i interpreter) evalUIMountAndRootArgs(env *environment, pkgName string, callee string, mountExpr ast.Expr, rootExpr ast.Expr) (*uiMount, *uiNode, *evalResult, error) {
	handleValue, err := i.evalExpr(env, pkgName, mountExpr)
	if err != nil {
		return nil, nil, nil, err
	}
	if handleValue.hasError {
		errResult := evalResult{hasError: true, errorVal: handleValue.errorVal}
		return nil, nil, &errResult, nil
	}
	if handleValue.value.Kind != ValueInt {
		return nil, nil, nil, fmt.Errorf("runtime invariant violation: %s expects Int mount handle", callee)
	}
	mount, err := i.uiMounts.get(handleValue.value.Int)
	if err != nil {
		errResult := wrapperErrorResult(callee, err)
		return nil, nil, &errResult, nil
	}
	rootValue, err := i.evalExpr(env, pkgName, rootExpr)
	if err != nil {
		return nil, nil, nil, err
	}
	if rootValue.hasError {
		errResult := evalResult{hasError: true, errorVal: rootValue.errorVal}
		return nil, nil, &errResult, nil
	}
	if rootValue.value.Kind != ValueUI || rootValue.value.UI == nil {
		return nil, nil, nil, fmt.Errorf("runtime invariant violation: %s expects UI root", callee)
	}
	return mount, rootValue.value.UI, nil, nil
}
