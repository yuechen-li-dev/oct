package interpret

import (
	"fmt"

	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

func toMachinaUIIRNode(node *uiirNode) (*machinauiir.Node, error) {
	if node == nil {
		return nil, nil
	}
	kind, err := toMachinaNodeKind(node.Kind)
	if err != nil {
		return nil, err
	}
	children := make([]*machinauiir.Node, 0, len(node.Children))
	for _, child := range node.Children {
		next, err := toMachinaUIIRNode(child)
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return &machinauiir.Node{
		NodeID:   node.NodeID,
		Key:      node.Key,
		Kind:     kind,
		Text:     node.Text,
		Label:    node.Label,
		Event:    node.Event,
		Enabled:  node.Enabled,
		Children: children,
		Box:      toMachinaBoxSpec(node.Box),
		Layout:   toMachinaResolvedRect(node.Layout),
	}, nil
}

func fromMachinaUIIRNode(node *machinauiir.Node) (*uiirNode, error) {
	if node == nil {
		return nil, nil
	}
	kind, err := fromMachinaNodeKind(node.Kind)
	if err != nil {
		return nil, err
	}
	children := make([]*uiirNode, 0, len(node.Children))
	for _, child := range node.Children {
		next, err := fromMachinaUIIRNode(child)
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return &uiirNode{
		NodeID:   node.NodeID,
		Key:      node.Key,
		Kind:     kind,
		Text:     node.Text,
		Label:    node.Label,
		Event:    node.Event,
		Enabled:  node.Enabled,
		Children: children,
		Box:      fromMachinaBoxSpec(node.Box),
		Layout:   fromMachinaResolvedRect(node.Layout),
	}, nil
}

func toMachinaNodeKind(kind uiirNodeKind) (machinauiir.NodeKind, error) {
	switch kind {
	case uiirNodeText:
		return machinauiir.NodeText, nil
	case uiirNodeButton:
		return machinauiir.NodeButton, nil
	case uiirNodeColumn:
		return machinauiir.NodeColumn, nil
	case uiirNodeRow:
		return machinauiir.NodeRow, nil
	case uiirNodeGrid:
		return machinauiir.NodeGrid, nil
	case uiirNodeSpacer:
		return machinauiir.NodeSpacer, nil
	case uiirNodeAbsoluteBox:
		return machinauiir.NodeAbsoluteBox, nil
	case uiirNodeAnchorBox:
		return machinauiir.NodeAnchorBox, nil
	default:
		return "", fmt.Errorf("unsupported local uiir kind %q", kind)
	}
}

func fromMachinaNodeKind(kind machinauiir.NodeKind) (uiirNodeKind, error) {
	switch kind {
	case machinauiir.NodeText:
		return uiirNodeText, nil
	case machinauiir.NodeButton:
		return uiirNodeButton, nil
	case machinauiir.NodeColumn:
		return uiirNodeColumn, nil
	case machinauiir.NodeRow:
		return uiirNodeRow, nil
	case machinauiir.NodeGrid:
		return uiirNodeGrid, nil
	case machinauiir.NodeSpacer:
		return uiirNodeSpacer, nil
	case machinauiir.NodeAbsoluteBox:
		return uiirNodeAbsoluteBox, nil
	case machinauiir.NodeAnchorBox:
		return uiirNodeAnchorBox, nil
	default:
		return "", fmt.Errorf("unsupported machina uiir kind %q", kind)
	}
}

func toMachinaBoxSpec(spec *uiirBoxSpec) *machinauiir.BoxSpec {
	if spec == nil {
		return nil
	}
	out := &machinauiir.BoxSpec{
		ZOrder: spec.ZOrder, X: spec.X, Y: spec.Y, Width: spec.Width, Height: spec.Height,
		Left: spec.Left, Top: spec.Top, Right: spec.Right, Bottom: spec.Bottom,
		ResolvedRect: toMachinaResolvedRect(spec.ResolvedRect),
	}
	if spec.Kind == uiirBoxAbsolute {
		out.Kind = machinauiir.BoxAbsolute
	} else {
		out.Kind = machinauiir.BoxAnchored
	}
	return out
}
func fromMachinaBoxSpec(spec *machinauiir.BoxSpec) *uiirBoxSpec {
	if spec == nil {
		return nil
	}
	out := &uiirBoxSpec{ZOrder: spec.ZOrder, X: spec.X, Y: spec.Y, Width: spec.Width, Height: spec.Height, Left: spec.Left, Top: spec.Top, Right: spec.Right, Bottom: spec.Bottom, ResolvedRect: fromMachinaResolvedRect(spec.ResolvedRect)}
	if spec.Kind == machinauiir.BoxAbsolute {
		out.Kind = uiirBoxAbsolute
	} else {
		out.Kind = uiirBoxAnchored
	}
	return out
}
func toMachinaResolvedRect(r *uiirResolvedBox) *machinauiir.ResolvedRect {
	if r == nil {
		return nil
	}
	return &machinauiir.ResolvedRect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}
func fromMachinaResolvedRect(r *machinauiir.ResolvedRect) *uiirResolvedBox {
	if r == nil {
		return nil
	}
	return &uiirResolvedBox{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}
