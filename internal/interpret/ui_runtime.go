package interpret

import (
	"encoding/json"
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
	machinalayout "github.com/yuechen-li-dev/oct/internal/machina/layout"
	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

type uiirNodeKind = machinauiir.NodeKind

const (
	uiirNodeText         uiirNodeKind = machinauiir.NodeText
	uiirNodeButton       uiirNodeKind = machinauiir.NodeButton
	uiirNodeColumn       uiirNodeKind = machinauiir.NodeColumn
	uiirNodeRow          uiirNodeKind = machinauiir.NodeRow
	uiirNodeGrid         uiirNodeKind = machinauiir.NodeGrid
	uiirNodeSpacer       uiirNodeKind = machinauiir.NodeSpacer
	uiirNodeAbsoluteBox  uiirNodeKind = machinauiir.NodeAbsoluteBox
	uiirNodeAnchorBox    uiirNodeKind = machinauiir.NodeAnchorBox
	uiirRootLayoutExtent float64      = machinalayout.RootLayoutExtent
	uiirAbsCoordLimit    float64      = machinalayout.AbsCoordLimit
	uiirMinZOrder        int64        = machinalayout.MinZOrder
	uiirMaxZOrder        int64        = machinalayout.MaxZOrder
)

type uiirBoxKind = machinauiir.BoxKind

const (
	uiirBoxAbsolute uiirBoxKind = machinauiir.BoxAbsolute
	uiirBoxAnchored uiirBoxKind = machinauiir.BoxAnchored
)

type uiirBoxSpec = machinauiir.BoxSpec

type uiirResolvedBox = machinauiir.ResolvedRect

type uiirNode = machinauiir.Node

type uiMount struct {
	Root   *uiirNode
	Events []string
}

const uiirJSONABI = machinauiir.JSONABI

var uiirJSONNull = json.RawMessage("null")

type uiirSerializedEvent = machinauiir.SerializedEvent

func serializeUIIRCanonicalJSON(root *uiirNode) (string, error) {
	return machinauiir.SerializeCanonicalJSON(root)
}

func deserializeUIIRCanonicalJSON(serialized string) (*uiirNode, error) {
	return machinauiir.DeserializeCanonicalJSON(serialized)
}

func serializeUIIREventCanonicalJSON(token string, payload json.RawMessage) (string, error) {
	return machinauiir.SerializeEventCanonicalJSON(token, payload)
}

func deserializeUIIREventCanonicalJSON(serialized string) (uiirSerializedEvent, error) {
	return machinauiir.DeserializeEventCanonicalJSON(serialized)
}

func uiirNormalizedPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return uiirJSONNull
	}
	return payload
}


func cloneUIIR(node *uiirNode) *uiirNode {
	return machinauiir.Clone(node)
}


func withUIIRNodeIDs(node *uiirNode) *uiirNode { return machinauiir.WithNodeIDs(node) }


func uiirSignature(node *uiirNode) string {
	return machinauiir.Signature(node)
}


func uiirTreeContainsEvent(node *uiirNode, token string) bool {
	return machinauiir.TreeContainsEvent(node, token)
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
		return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: uiirNodeText, Text: content.value.Text}}}, nil
	case "UIButton":
		if len(argumentExprs) != 3 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIButton expects 3 arguments")
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
		enabled, err := i.evalExpr(env, pkgName, argumentExprs[2])
		if err != nil {
			return evalResult{}, err
		}
		if enabled.hasError {
			return evalResult{hasError: true, errorVal: enabled.errorVal}, nil
		}
		if label.value.Kind != ValueString || event.value.Kind != ValueString || enabled.value.Kind != ValueBool {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIButton expects (String, String, Bool)")
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: uiirNodeButton, Label: label.value.Text, Event: event.value.Text, Enabled: enabled.value.Bool}}}, nil
	case "UIColumn", "UIRow", "UICanvas", "UIGrid":
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
		nodes := make([]*uiirNode, 0, len(children.value.Array))
		for idx, child := range children.value.Array {
			if child.Kind != ValueUI || child.UI == nil {
				return evalResult{}, fmt.Errorf("runtime invariant violation: %s children[%d] expects UI", callee, idx)
			}
			nodes = append(nodes, cloneUIIR(child.UI))
		}
		kind := uiirNodeColumn
		if callee == "UIRow" {
			kind = uiirNodeRow
		}
		if callee == "UICanvas" {
			kind = uiirNodeAbsoluteBox
		}
		if callee == "UIGrid" {
			kind = uiirNodeGrid
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: kind, Children: nodes}}}, nil
	case "UISpacer":
		if len(argumentExprs) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UISpacer expects 0 arguments")
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: uiirNodeSpacer}}}, nil
	case "UIPlaceAbsolute":
		node, errResult, err := i.evalUIPlaceAbsolute(env, pkgName, argumentExprs)
		if err != nil {
			return evalResult{}, err
		}
		if errResult != nil {
			return *errResult, nil
		}
		return evalResult{value: Value{Kind: ValueUI, UI: node}}, nil
	case "UIPlaceAnchored":
		node, errResult, err := i.evalUIPlaceAnchored(env, pkgName, argumentExprs)
		if err != nil {
			return evalResult{}, err
		}
		if errResult != nil {
			return *errResult, nil
		}
		return evalResult{value: Value{Kind: ValueUI, UI: node}}, nil
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
		canonicalRoot := withUIIRNodeIDs(root.value.UI)
		resolvedRoot, resolveErr := resolveUIIRBoxes(canonicalRoot, &uiirResolvedBox{X: 0, Y: 0, Width: uiirRootLayoutExtent, Height: uiirRootLayoutExtent})
		if resolveErr != nil {
			return wrapperErrorResult(callee, resolveErr), nil
		}
		handle := i.uiMounts.allocate(&uiMount{Root: resolvedRoot})
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
		canonicalRoot := withUIIRNodeIDs(nextRoot)
		resolvedRoot, resolveErr := resolveUIIRBoxes(canonicalRoot, &uiirResolvedBox{X: 0, Y: 0, Width: uiirRootLayoutExtent, Height: uiirRootLayoutExtent})
		if resolveErr != nil {
			return wrapperErrorResult(callee, resolveErr), nil
		}
		handle.Root = resolvedRoot
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
		if !uiirTreeContainsEvent(mount.Root, eventValue.value.Text) {
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
		return evalResult{value: Value{Kind: ValueString, Text: uiirSignature(withUIIRNodeIDs(root.value.UI))}}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported UI builtin %s", callee)
	}
}

func (i interpreter) evalUIPlaceAbsolute(env *environment, pkgName string, argumentExprs []ast.Expr) (*uiirNode, *evalResult, error) {
	if len(argumentExprs) != 6 {
		return nil, nil, fmt.Errorf("runtime invariant violation: UIPlaceAbsolute expects 6 arguments")
	}
	x, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[0], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	y, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[1], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	width, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[2], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	height, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[3], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	child, errResult, err := i.evalUIArg(env, pkgName, argumentExprs[4], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	zOrder, errResult, err := i.evalIntArg(env, pkgName, argumentExprs[5], "UIPlaceAbsolute")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	if width < 0 || height < 0 {
		errEval := wrapperErrorResult("UIPlaceAbsolute", fmt.Errorf("absolute box width and height must be >= 0"))
		return nil, &errEval, nil
	}
	if x < -uiirAbsCoordLimit || y < -uiirAbsCoordLimit || x > uiirAbsCoordLimit || y > uiirAbsCoordLimit || width > uiirAbsCoordLimit || height > uiirAbsCoordLimit {
		errEval := wrapperErrorResult("UIPlaceAbsolute", fmt.Errorf("absolute box values exceed runtime bounds"))
		return nil, &errEval, nil
	}
	if zOrder < uiirMinZOrder || zOrder > uiirMaxZOrder {
		errEval := wrapperErrorResult("UIPlaceAbsolute", fmt.Errorf("z-order must be within [%d, %d]", uiirMinZOrder, uiirMaxZOrder))
		return nil, &errEval, nil
	}
	return &uiirNode{
		Kind: uiirNodeAbsoluteBox,
		Box: &uiirBoxSpec{
			Kind:   uiirBoxAbsolute,
			ZOrder: zOrder,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		},
		Children: []*uiirNode{cloneUIIR(child)},
	}, nil, nil
}

func (i interpreter) evalUIPlaceAnchored(env *environment, pkgName string, argumentExprs []ast.Expr) (*uiirNode, *evalResult, error) {
	if len(argumentExprs) != 6 {
		return nil, nil, fmt.Errorf("runtime invariant violation: UIPlaceAnchored expects 6 arguments")
	}
	left, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[0], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	top, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[1], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	right, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[2], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	bottom, errResult, err := i.evalFloatArg(env, pkgName, argumentExprs[3], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	child, errResult, err := i.evalUIArg(env, pkgName, argumentExprs[4], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	zOrder, errResult, err := i.evalIntArg(env, pkgName, argumentExprs[5], "UIPlaceAnchored")
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	if right < left || bottom < top {
		errEval := wrapperErrorResult("UIPlaceAnchored", fmt.Errorf("anchored box requires Right >= Left and Bottom >= Top"))
		return nil, &errEval, nil
	}
	if zOrder < uiirMinZOrder || zOrder > uiirMaxZOrder {
		errEval := wrapperErrorResult("UIPlaceAnchored", fmt.Errorf("z-order must be within [%d, %d]", uiirMinZOrder, uiirMaxZOrder))
		return nil, &errEval, nil
	}
	return &uiirNode{
		Kind: uiirNodeAnchorBox,
		Box: &uiirBoxSpec{
			Kind:   uiirBoxAnchored,
			ZOrder: zOrder,
			Left:   left,
			Top:    top,
			Right:  right,
			Bottom: bottom,
		},
		Children: []*uiirNode{cloneUIIR(child)},
	}, nil, nil
}

func (i interpreter) evalFloatArg(env *environment, pkgName string, expr ast.Expr, callee string) (float64, *evalResult, error) {
	value, err := i.evalExpr(env, pkgName, expr)
	if err != nil {
		return 0, nil, err
	}
	if value.hasError {
		errEval := evalResult{hasError: true, errorVal: value.errorVal}
		return 0, &errEval, nil
	}
	if value.value.Kind != ValueFloat {
		return 0, nil, fmt.Errorf("runtime invariant violation: %s expects Float box values", callee)
	}
	return value.value.Float, nil, nil
}

func (i interpreter) evalIntArg(env *environment, pkgName string, expr ast.Expr, callee string) (int64, *evalResult, error) {
	value, err := i.evalExpr(env, pkgName, expr)
	if err != nil {
		return 0, nil, err
	}
	if value.hasError {
		errEval := evalResult{hasError: true, errorVal: value.errorVal}
		return 0, &errEval, nil
	}
	if value.value.Kind != ValueInt {
		return 0, nil, fmt.Errorf("runtime invariant violation: %s expects Int z-order", callee)
	}
	return value.value.Int, nil, nil
}

func (i interpreter) evalUIArg(env *environment, pkgName string, expr ast.Expr, callee string) (*uiirNode, *evalResult, error) {
	value, err := i.evalExpr(env, pkgName, expr)
	if err != nil {
		return nil, nil, err
	}
	if value.hasError {
		errEval := evalResult{hasError: true, errorVal: value.errorVal}
		return nil, &errEval, nil
	}
	if value.value.Kind != ValueUI || value.value.UI == nil {
		return nil, nil, fmt.Errorf("runtime invariant violation: %s expects UI child", callee)
	}
	return value.value.UI, nil, nil
}

func resolveUIIRBoxes(node *uiirNode, parent *uiirResolvedBox) (*uiirNode, error) {
	return machinalayout.ResolveBoxes(node, parent)
}


func (i interpreter) evalUIMountAndRootArgs(env *environment, pkgName string, callee string, mountExpr ast.Expr, rootExpr ast.Expr) (*uiMount, *uiirNode, *evalResult, error) {
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
