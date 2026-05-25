package uiir

import (
	"encoding/json"
	"fmt"
	"strings"
)

type NodeKind string

const (
	NodeText        NodeKind = "Text"
	NodeButton      NodeKind = "Button"
	NodeColumn      NodeKind = "Column"
	NodeRow         NodeKind = "Row"
	NodeGrid        NodeKind = "Grid"
	NodeSpacer      NodeKind = "Spacer"
	NodeAbsoluteBox NodeKind = "AbsoluteBox"
	NodeAnchorBox   NodeKind = "AnchorBox"
	JSONABI                  = "machina.uiir.v1"
)

type BoxKind string

const (
	BoxAbsolute BoxKind = "absolute"
	BoxAnchored BoxKind = "anchored"
)

type BoxSpec struct {
	Kind         BoxKind
	ZOrder       int64
	X            float64
	Y            float64
	Width        float64
	Height       float64
	Left         float64
	Top          float64
	Right        float64
	Bottom       float64
	ResolvedRect *ResolvedRect
}

type ResolvedRect struct{ X, Y, Width, Height float64 }

type Node struct {
	NodeID   string
	Key      string
	Kind     NodeKind
	Text     string
	Label    string
	Event    string
	Enabled  bool
	Children []*Node
	Box      *BoxSpec
	Layout   *ResolvedRect
}

type SerializedEvent struct {
	Token   string          `json:"token"`
	Payload json.RawMessage `json:"payload"`
}
type serializedDocument struct {
	ABI  string         `json:"abi"`
	Root serializedNode `json:"root"`
}
type serializedNode struct {
	ID       string           `json:"id"`
	Kind     string           `json:"kind"`
	Key      *string          `json:"key"`
	Text     *string          `json:"text"`
	Label    *string          `json:"label"`
	Enabled  *bool            `json:"enabled"`
	Event    *SerializedEvent `json:"event"`
	Box      *serializedBox   `json:"box"`
	Layout   *serializedRect  `json:"layout"`
	Children []serializedNode `json:"children"`
}
type serializedBox struct {
	Kind     string                 `json:"kind"`
	Z        int64                  `json:"z"`
	Absolute *serializedRect        `json:"absolute"`
	Anchored *serializedBoxAnchored `json:"anchored"`
}
type serializedBoxAnchored struct{ Left, Top, Right, Bottom float64 }
type serializedRect struct{ X, Y, Width, Height float64 }

var jsonNull = json.RawMessage("null")

func SerializeCanonicalJSON(root *Node) (string, error) {
	if root == nil {
		return "", fmt.Errorf("uiir serialization requires non-nil root")
	}
	doc := serializedDocument{ABI: JSONABI, Root: serializeNode(WithNodeIDs(root))}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("uiir serialization failed: %w", err)
	}
	return string(b), nil
}
func DeserializeCanonicalJSON(serialized string) (*Node, error) {
	var doc serializedDocument
	if err := json.Unmarshal([]byte(serialized), &doc); err != nil {
		return nil, fmt.Errorf("uiir deserialization failed: %w", err)
	}
	if doc.ABI != JSONABI {
		return nil, fmt.Errorf("uiir deserialization failed: unsupported abi %q", doc.ABI)
	}
	return deserializeNode(doc.Root)
}
func SerializeEventCanonicalJSON(token string, payload json.RawMessage) (string, error) {
	if token == "" {
		return "", fmt.Errorf("ui event serialization requires non-empty token")
	}
	ev := SerializedEvent{Token: token, Payload: normalizedPayload(payload)}
	b, err := json.Marshal(ev)
	if err != nil {
		return "", fmt.Errorf("ui event serialization failed: %w", err)
	}
	return string(b), nil
}
func DeserializeEventCanonicalJSON(serialized string) (SerializedEvent, error) {
	var ev SerializedEvent
	if err := json.Unmarshal([]byte(serialized), &ev); err != nil {
		return SerializedEvent{}, fmt.Errorf("ui event deserialization failed: %w", err)
	}
	if ev.Token == "" {
		return SerializedEvent{}, fmt.Errorf("ui event deserialization failed: token is required")
	}
	ev.Payload = normalizedPayload(ev.Payload)
	return ev, nil
}
func normalizedPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return jsonNull
	}
	return payload
}

func Clone(node *Node) *Node {
	if node == nil {
		return nil
	}
	ch := make([]*Node, 0, len(node.Children))
	for _, c := range node.Children {
		ch = append(ch, Clone(c))
	}
	return &Node{NodeID: node.NodeID, Key: node.Key, Kind: node.Kind, Text: node.Text, Label: node.Label, Event: node.Event, Enabled: node.Enabled, Children: ch, Box: cloneBox(node.Box), Layout: cloneRect(node.Layout)}
}
func WithNodeIDs(node *Node) *Node { root := Clone(node); assignNodeIDs(root, "0"); return root }
func assignNodeIDs(node *Node, id string) {
	if node == nil {
		return
	}
	node.NodeID = id
	for i := range node.Children {
		assignNodeIDs(node.Children[i], fmt.Sprintf("%s.%d", id, i))
	}
}
func Signature(node *Node) string {
	if node == nil {
		return "<nil>"
	}
	p := string(node.Kind) + "#" + node.NodeID
	if node.Key != "" {
		p += "{key=" + node.Key + "}"
	}
	switch node.Kind {
	case NodeText:
		return p + layoutSuffix(node.Layout) + "(" + node.Text + ")"
	case NodeSpacer:
		return p + layoutSuffix(node.Layout)
	case NodeButton:
		return p + layoutSuffix(node.Layout) + "(" + node.Label + "->" + node.Event + ",enabled=" + fmt.Sprintf("%t", node.Enabled) + ")"
	case NodeColumn, NodeRow, NodeGrid, NodeAbsoluteBox, NodeAnchorBox:
		parts := make([]string, 0, len(node.Children))
		for _, c := range node.Children {
			parts = append(parts, Signature(c))
		}
		if node.Box != nil {
			return p + "(" + boxSignature(node.Box) + ")" + layoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
		}
		return p + layoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
	default:
		return "<unknown>"
	}
}
func TreeContainsEvent(node *Node, token string) bool {
	if node == nil {
		return false
	}
	if node.Kind == NodeButton && node.Enabled && node.Event == token {
		return true
	}
	for _, c := range node.Children {
		if TreeContainsEvent(c, token) {
			return true
		}
	}
	return false
}

func serializeNode(node *Node) serializedNode {
	s := serializedNode{ID: node.NodeID, Kind: string(node.Kind), Children: make([]serializedNode, 0, len(node.Children))}
	if node.Key != "" {
		k := node.Key
		s.Key = &k
	}
	if node.Layout != nil {
		s.Layout = &serializedRect{X: node.Layout.X, Y: node.Layout.Y, Width: node.Layout.Width, Height: node.Layout.Height}
	}
	switch node.Kind {
	case NodeText:
		t := node.Text
		s.Text = &t
	case NodeButton:
		l := node.Label
		e := node.Enabled
		s.Label = &l
		s.Enabled = &e
		s.Event = &SerializedEvent{Token: node.Event, Payload: jsonNull}
	case NodeAbsoluteBox, NodeAnchorBox:
		s.Box = serializeBox(node.Box)
	}
	for _, c := range node.Children {
		s.Children = append(s.Children, serializeNode(c))
	}
	return s
}
func serializeBox(box *BoxSpec) *serializedBox {
	if box == nil {
		return nil
	}
	s := &serializedBox{Kind: string(box.Kind), Z: box.ZOrder}
	if box.Kind == BoxAbsolute {
		s.Absolute = &serializedRect{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height}
		return s
	}
	s.Anchored = &serializedBoxAnchored{Left: box.Left, Top: box.Top, Right: box.Right, Bottom: box.Bottom}
	return s
}
func deserializeNode(node serializedNode) (*Node, error) {
	if node.ID == "" {
		return nil, fmt.Errorf("uiir node deserialization failed: id is required")
	}
	k := NodeKind(node.Kind)
	r := &Node{NodeID: node.ID, Kind: k, Children: make([]*Node, 0, len(node.Children))}
	if node.Key != nil {
		r.Key = *node.Key
	}
	if node.Layout != nil {
		r.Layout = &ResolvedRect{X: node.Layout.X, Y: node.Layout.Y, Width: node.Layout.Width, Height: node.Layout.Height}
	}
	switch k {
	case NodeText:
		if node.Text == nil {
			return nil, fmt.Errorf("uiir node deserialization failed: text payload missing")
		}
		r.Text = *node.Text
	case NodeButton:
		if node.Label == nil || node.Enabled == nil || node.Event == nil {
			return nil, fmt.Errorf("uiir node deserialization failed: button payload missing")
		}
		r.Label = *node.Label
		r.Enabled = *node.Enabled
		r.Event = node.Event.Token
	case NodeAbsoluteBox, NodeAnchorBox:
		b, err := deserializeBox(node.Box)
		if err != nil {
			return nil, err
		}
		r.Box = b
	case NodeColumn, NodeRow, NodeGrid, NodeSpacer:
	default:
		return nil, fmt.Errorf("uiir node deserialization failed: unsupported kind %q", node.Kind)
	}
	for _, c := range node.Children {
		n, err := deserializeNode(c)
		if err != nil {
			return nil, err
		}
		r.Children = append(r.Children, n)
	}
	return r, nil
}
func deserializeBox(box *serializedBox) (*BoxSpec, error) {
	if box == nil {
		return nil, fmt.Errorf("uiir box deserialization failed: box is required")
	}
	k := BoxKind(box.Kind)
	switch k {
	case BoxAbsolute:
		if box.Absolute == nil {
			return nil, fmt.Errorf("uiir box deserialization failed: absolute payload missing")
		}
		return &BoxSpec{Kind: BoxAbsolute, ZOrder: box.Z, X: box.Absolute.X, Y: box.Absolute.Y, Width: box.Absolute.Width, Height: box.Absolute.Height}, nil
	case BoxAnchored:
		if box.Anchored == nil {
			return nil, fmt.Errorf("uiir box deserialization failed: anchored payload missing")
		}
		return &BoxSpec{Kind: BoxAnchored, ZOrder: box.Z, Left: box.Anchored.Left, Top: box.Anchored.Top, Right: box.Anchored.Right, Bottom: box.Anchored.Bottom}, nil
	default:
		return nil, fmt.Errorf("uiir box deserialization failed: unsupported kind %q", box.Kind)
	}
}
func cloneBox(spec *BoxSpec) *BoxSpec {
	if spec == nil {
		return nil
	}
	return &BoxSpec{Kind: spec.Kind, ZOrder: spec.ZOrder, X: spec.X, Y: spec.Y, Width: spec.Width, Height: spec.Height, Left: spec.Left, Top: spec.Top, Right: spec.Right, Bottom: spec.Bottom, ResolvedRect: cloneRect(spec.ResolvedRect)}
}
func cloneRect(r *ResolvedRect) *ResolvedRect {
	if r == nil {
		return nil
	}
	return &ResolvedRect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}
func layoutSuffix(r *ResolvedRect) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("@(%.2f,%.2f,%.2f,%.2f)", r.X, r.Y, r.Width, r.Height)
}
func boxSignature(spec *BoxSpec) string {
	if spec == nil {
		return "<nil>"
	}
	if spec.Kind == BoxAbsolute {
		return fmt.Sprintf("absolute(z=%d,%.2f,%.2f,%.2f,%.2f)", spec.ZOrder, spec.X, spec.Y, spec.Width, spec.Height)
	}
	return fmt.Sprintf("anchored(z=%d,%.2f,%.2f,%.2f,%.2f)", spec.ZOrder, spec.Left, spec.Top, spec.Right, spec.Bottom)
}
