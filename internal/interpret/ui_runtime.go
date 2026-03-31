package interpret

import (
	"fmt"
	"strings"

	"oct/internal/ast"
)

type uiNodeKind string

const (
	uiNodeText         uiNodeKind = "Text"
	uiNodeButton       uiNodeKind = "Button"
	uiNodeColumn       uiNodeKind = "Column"
	uiNodeRow          uiNodeKind = "Row"
	uiNodeCanvas       uiNodeKind = "Canvas"
	uiNodePlaced       uiNodeKind = "Placed"
	uiRootLayoutExtent float64    = 1000.0
	uiAbsCoordLimit    float64    = 1000000.0
)

type uiBoxKind string

const (
	uiBoxAbsolute uiBoxKind = "absolute"
	uiBoxAnchored uiBoxKind = "anchored"
)

type uiBoxSpec struct {
	Kind         uiBoxKind
	X            float64
	Y            float64
	Width        float64
	Height       float64
	Left         float64
	Top          float64
	Right        float64
	Bottom       float64
	ResolvedRect *uiResolvedBox
}

type uiResolvedBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type uiNode struct {
	Kind     uiNodeKind
	Text     string
	Label    string
	Event    string
	Enabled  bool
	Children []*uiNode
	Box      *uiBoxSpec
	Layout   *uiResolvedBox
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
	return &uiNode{
		Kind:     node.Kind,
		Text:     node.Text,
		Label:    node.Label,
		Event:    node.Event,
		Enabled:  node.Enabled,
		Children: children,
		Box:      cloneUIBoxSpec(node.Box),
		Layout:   cloneUIResolvedBox(node.Layout),
	}
}

func uiSignature(node *uiNode) string {
	if node == nil {
		return "<nil>"
	}
	switch node.Kind {
	case uiNodeText:
		return "Text" + uiLayoutSuffix(node.Layout) + "(" + node.Text + ")"
	case uiNodeButton:
		return "Button" + uiLayoutSuffix(node.Layout) + "(" + node.Label + "->" + node.Event + ",enabled=" + fmt.Sprintf("%t", node.Enabled) + ")"
	case uiNodeColumn, uiNodeRow, uiNodeCanvas:
		parts := make([]string, 0, len(node.Children))
		for _, child := range node.Children {
			parts = append(parts, uiSignature(child))
		}
		return string(node.Kind) + uiLayoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
	case uiNodePlaced:
		if len(node.Children) != 1 {
			return "Placed<invalid>"
		}
		return "Placed(" + uiBoxSignature(node.Box) + ")->" + uiSignature(node.Children[0])
	default:
		return "<unknown>"
	}
}

func uiLayoutSuffix(layout *uiResolvedBox) string {
	if layout == nil {
		return ""
	}
	return fmt.Sprintf("@(%.2f,%.2f,%.2f,%.2f)", layout.X, layout.Y, layout.Width, layout.Height)
}

func uiBoxSignature(spec *uiBoxSpec) string {
	if spec == nil {
		return "<nil>"
	}
	if spec.Kind == uiBoxAbsolute {
		return fmt.Sprintf("absolute(%.2f,%.2f,%.2f,%.2f)", spec.X, spec.Y, spec.Width, spec.Height)
	}
	return fmt.Sprintf("anchored(%.2f,%.2f,%.2f,%.2f)", spec.Left, spec.Top, spec.Right, spec.Bottom)
}

func uiTreeContainsEvent(node *uiNode, token string) bool {
	if node == nil {
		return false
	}
	if node.Kind == uiNodeButton && node.Enabled && node.Event == token {
		return true
	}
	for _, child := range node.Children {
		if uiTreeContainsEvent(child, token) {
			return true
		}
	}
	return false
}

func cloneUIBoxSpec(spec *uiBoxSpec) *uiBoxSpec {
	if spec == nil {
		return nil
	}
	return &uiBoxSpec{
		Kind:         spec.Kind,
		X:            spec.X,
		Y:            spec.Y,
		Width:        spec.Width,
		Height:       spec.Height,
		Left:         spec.Left,
		Top:          spec.Top,
		Right:        spec.Right,
		Bottom:       spec.Bottom,
		ResolvedRect: cloneUIResolvedBox(spec.ResolvedRect),
	}
}

func cloneUIResolvedBox(rect *uiResolvedBox) *uiResolvedBox {
	if rect == nil {
		return nil
	}
	return &uiResolvedBox{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
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
		return evalResult{value: Value{Kind: ValueUI, UI: &uiNode{Kind: uiNodeButton, Label: label.value.Text, Event: event.value.Text, Enabled: enabled.value.Bool}}}, nil
	case "UIColumn", "UIRow", "UICanvas":
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
		if callee == "UICanvas" {
			kind = uiNodeCanvas
		}
		return evalResult{value: Value{Kind: ValueUI, UI: &uiNode{Kind: kind, Children: nodes}}}, nil
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
		resolvedRoot, resolveErr := resolveUIBoxes(cloneUI(root.value.UI), &uiResolvedBox{X: 0, Y: 0, Width: uiRootLayoutExtent, Height: uiRootLayoutExtent})
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
		resolvedRoot, resolveErr := resolveUIBoxes(cloneUI(nextRoot), &uiResolvedBox{X: 0, Y: 0, Width: uiRootLayoutExtent, Height: uiRootLayoutExtent})
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

func (i interpreter) evalUIPlaceAbsolute(env *environment, pkgName string, argumentExprs []ast.Expr) (*uiNode, *evalResult, error) {
	if len(argumentExprs) != 5 {
		return nil, nil, fmt.Errorf("runtime invariant violation: UIPlaceAbsolute expects 5 arguments")
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
	if width < 0 || height < 0 {
		errEval := wrapperErrorResult("UIPlaceAbsolute", fmt.Errorf("absolute box width and height must be >= 0"))
		return nil, &errEval, nil
	}
	if x < -uiAbsCoordLimit || y < -uiAbsCoordLimit || x > uiAbsCoordLimit || y > uiAbsCoordLimit || width > uiAbsCoordLimit || height > uiAbsCoordLimit {
		errEval := wrapperErrorResult("UIPlaceAbsolute", fmt.Errorf("absolute box values exceed runtime bounds"))
		return nil, &errEval, nil
	}
	return &uiNode{
		Kind: uiNodePlaced,
		Box: &uiBoxSpec{
			Kind:   uiBoxAbsolute,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		},
		Children: []*uiNode{cloneUI(child)},
	}, nil, nil
}

func (i interpreter) evalUIPlaceAnchored(env *environment, pkgName string, argumentExprs []ast.Expr) (*uiNode, *evalResult, error) {
	if len(argumentExprs) != 5 {
		return nil, nil, fmt.Errorf("runtime invariant violation: UIPlaceAnchored expects 5 arguments")
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
	if left < 0 || top < 0 || right > 1 || bottom > 1 {
		errEval := wrapperErrorResult("UIPlaceAnchored", fmt.Errorf("anchored box values must be within [0,1]"))
		return nil, &errEval, nil
	}
	if right < left || bottom < top {
		errEval := wrapperErrorResult("UIPlaceAnchored", fmt.Errorf("anchored box requires Right >= Left and Bottom >= Top"))
		return nil, &errEval, nil
	}
	return &uiNode{
		Kind: uiNodePlaced,
		Box: &uiBoxSpec{
			Kind:   uiBoxAnchored,
			Left:   left,
			Top:    top,
			Right:  right,
			Bottom: bottom,
		},
		Children: []*uiNode{cloneUI(child)},
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

func (i interpreter) evalUIArg(env *environment, pkgName string, expr ast.Expr, callee string) (*uiNode, *evalResult, error) {
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

func resolveUIBoxes(node *uiNode, parent *uiResolvedBox) (*uiNode, error) {
	if node == nil {
		return nil, nil
	}
	switch node.Kind {
	case uiNodePlaced:
		if len(node.Children) != 1 || node.Box == nil {
			return nil, fmt.Errorf("placed node must have exactly one child and one box")
		}
		resolved, err := resolveBox(node.Box, parent)
		if err != nil {
			return nil, err
		}
		node.Box.ResolvedRect = cloneUIResolvedBox(resolved)
		node.Children[0].Layout = cloneUIResolvedBox(resolved)
		child, err := resolveUIBoxes(node.Children[0], resolved)
		if err != nil {
			return nil, err
		}
		node.Children[0] = child
		return node, nil
	default:
		for idx := range node.Children {
			child, err := resolveUIBoxes(node.Children[idx], parent)
			if err != nil {
				return nil, err
			}
			node.Children[idx] = child
		}
		return node, nil
	}
}

func resolveBox(spec *uiBoxSpec, parent *uiResolvedBox) (*uiResolvedBox, error) {
	if spec == nil || parent == nil {
		return nil, fmt.Errorf("box resolution requires spec and parent")
	}
	switch spec.Kind {
	case uiBoxAbsolute:
		if spec.Width < 0 || spec.Height < 0 {
			return nil, fmt.Errorf("absolute box width and height must be >= 0")
		}
		return &uiResolvedBox{X: parent.X + spec.X, Y: parent.Y + spec.Y, Width: spec.Width, Height: spec.Height}, nil
	case uiBoxAnchored:
		x := parent.X + (spec.Left * parent.Width)
		y := parent.Y + (spec.Top * parent.Height)
		width := (spec.Right - spec.Left) * parent.Width
		height := (spec.Bottom - spec.Top) * parent.Height
		if width < 0 || height < 0 {
			return nil, fmt.Errorf("anchored box resolved width and height must be >= 0")
		}
		return &uiResolvedBox{X: x, Y: y, Width: width, Height: height}, nil
	default:
		return nil, fmt.Errorf("unsupported box kind %s", spec.Kind)
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
