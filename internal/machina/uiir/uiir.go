package uiir

import (
	"encoding/json"
	"fmt"
	"strings"
)

type NodeKind string

const (
	NodeText   NodeKind = "Text"
	NodeButton NodeKind = "Button"
	NodeColumn NodeKind = "Column"
	NodeRow    NodeKind = "Row"
	NodeGrid   NodeKind = "Grid"
	NodeSpacer NodeKind = "Spacer"
	// NodeSkip is a grid cell covered by another cell's span, or an
	// explicitly empty cell. It lowers to nothing.
	NodeSkip        NodeKind = "Skip"
	NodeAbsoluteBox NodeKind = "AbsoluteBox"
	NodeAnchorBox   NodeKind = "AnchorBox"
	// Controlled inputs (UI.Checkbox, UI.TextField, UI.Slider): the model
	// owns the value; the input proposes a change through Callback.
	NodeCheckbox  NodeKind = "Checkbox"
	NodeTextField NodeKind = "TextField"
	NodeSlider    NodeKind = "Slider"
	JSONABI                = "machina.uiir.v1"
)

type BoxKind string

const (
	BoxAbsolute BoxKind = "absolute"
	BoxAnchored BoxKind = "anchored"
	// BoxSpan places a box with one span per axis (Libraries/UI `UI.At` and
	// layout-table regions).
	BoxSpan BoxKind = "span"
)

// SpanKind is how one axis of a span box is constrained. Exactly one shape
// per axis, so a span box is always exactly constrained.
type SpanKind string

const (
	SpanStart    SpanKind = "start"    // A = offset from the near edge, B = size
	SpanEnd      SpanKind = "end"      // A = offset from the far edge, B = size
	SpanStretch  SpanKind = "stretch"  // A = near inset, B = far inset
	SpanCenter   SpanKind = "center"   // B = size
	SpanFraction SpanKind = "fraction" // A, B = near and far edge as parent fractions
)

// SpanSpec is one axis of a span box, in dp (fractions for SpanFraction).
type SpanSpec struct {
	Kind SpanKind
	A    float64
	B    float64
}

type BoxSpec struct {
	Kind   BoxKind
	ZOrder int64
	X      float64
	Y      float64
	Width  float64
	Height float64
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
	XSpan  SpanSpec
	YSpan  SpanSpec
	// Layer is the paint layer relative to Content (Base -1, Content 0,
	// Chrome 1, Overlay 2, Modal 3, Tooltip 4). Zero is Content.
	Layer int64
	// Region names the layout-table region this box places, for dumps.
	Region       string
	ResolvedRect *ResolvedRect
}

type ResolvedRect struct{ X, Y, Width, Height float64 }

// RGBA is an 8-bit-per-channel color.
type RGBA struct{ R, G, B, A uint8 }

// StyleSpec is the paint a node asks for (Libraries/UI `Style`). Padding is
// not part of it yet: insets change layout geometry and are deferred.
type StyleSpec struct {
	Background  RGBA
	Foreground  RGBA
	BorderColor RGBA
	BorderWidth float64
	Radius      float64
}

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
	Grid     *GridSpec
	Cell     *CellSpec
	GridRows int64
	GridCols int64
	Style    *StyleSpec
	// Stack is the arrangement of a Column or Row (gap, padding, alignment).
	Stack *StackSpec
	// Sizing says how this node takes space along its parent stack's main
	// axis (UI.Fixed, UI.Fill, UI.Auto, UI.Size).
	Sizing *SizingSpec
	// Offset moves the node after placement without moving its siblings
	// (UI.Nudge).
	Offset *Offset
	// Message is the typed value a Button sends (an Oct enum value); Event
	// is then its tool-facing alias. Message is opaque here and never
	// serialized.
	Message any
	// TextSpec is a Text node's size, weight, wrapping and overflow
	// (UI.TextWith); nil is the body spec.
	TextSpec *TextSpec
	// Input is a controlled input's current value.
	Input *InputSpec
	// Callback is the Oct function value an input calls with a proposed
	// value to build its message; Event is then its tool-facing alias.
	// Opaque here and never serialized.
	Callback any
	// States are the styles a node takes in interaction states
	// (UI.StyledStates); Style is its normal style.
	States *StateStyles
}

// InputSpec is the value a controlled input shows. Checked is a checkbox's;
// Value a text field's; Number, Min and Max a slider's.
type InputSpec struct {
	Checked bool
	Value   string
	Number  float64
	Min     float64
	Max     float64
}

// StateStyles are a node's styles for the interaction states the session
// tracks. Each replaces the normal style while its state holds.
type StateStyles struct {
	Hover    StyleSpec
	Pressed  StyleSpec
	Focused  StyleSpec
	Disabled StyleSpec
}

type Overflow string

const (
	OverflowEllipsis Overflow = "ellipsis"
	OverflowClip     Overflow = "clip"
	OverflowError    Overflow = "error"
)

// TextSpec is how a Text node is set (Libraries/UI TextSpec).
type TextSpec struct {
	Size     float64 // dp
	Weight   int
	Mono     bool
	Wrap     bool
	Overflow Overflow
	Color    *RGBA
}

// StackSpec is a Column or Row's arrangement (Libraries/UI StackSpec).
type StackSpec struct {
	Gap     float64
	Padding EdgeInsets
	Align   string // start | center | end (cross axis)
	Justify string // start | center | end | space-between (main axis)
}

type SizingKind string

const (
	SizingFixed SizingKind = "fixed" // Main along the stack axis
	SizingFill  SizingKind = "fill"  // Weight of the remaining main space
	SizingAuto  SizingKind = "auto"  // measured intrinsic size
	SizingSize  SizingKind = "size"  // Width and Height
)

type SizingSpec struct {
	Kind   SizingKind
	Main   float64
	Weight float64
	Width  float64
	Height float64
}

type Offset struct{ X, Y float64 }

type GridTrackKind string

const (
	GridTrackFixed GridTrackKind = "fixed"
	GridTrackFill  GridTrackKind = "fill"
	// GridTrackAuto is sized to its widest (or tallest) single-span cell's
	// measured size. Lowering turns it into a fixed track.
	GridTrackAuto GridTrackKind = "auto"
)

type GridTrack struct {
	Kind   GridTrackKind
	Size   float64
	Weight float64
}

type GridSpec struct {
	Columns   []GridTrack
	Rows      []GridTrack
	ColumnGap float64
	RowGap    float64
	Padding   EdgeInsets
}

type CellSpec struct {
	Column     int64
	Row        int64
	ColumnSpan int64
	RowSpan    int64
}

type EdgeInsets struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
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
	Grid     *serializedGrid  `json:"grid"`
	Cell     *serializedCell  `json:"cell"`
	GridRows int64            `json:"gridRows,omitempty"`
	GridCols int64            `json:"gridColumns,omitempty"`
	// Style is omitted when unset so unstyled documents keep their bytes.
	Style *serializedStyle `json:"style,omitempty"`
}
type serializedStyle struct {
	Background  [4]uint8 `json:"background"`
	Foreground  [4]uint8 `json:"foreground"`
	BorderColor [4]uint8 `json:"borderColor"`
	BorderWidth float64  `json:"borderWidth"`
	Radius      float64  `json:"radius"`
}
type serializedGrid struct {
	Columns   []serializedGridTrack `json:"columns"`
	Rows      []serializedGridTrack `json:"rows"`
	ColumnGap float64               `json:"columnGap"`
	RowGap    float64               `json:"rowGap"`
	Padding   serializedInsets      `json:"padding"`
}
type serializedGridTrack struct {
	Kind   string  `json:"kind"`
	Size   float64 `json:"size,omitempty"`
	Weight float64 `json:"weight,omitempty"`
}
type serializedCell struct {
	Column     int64 `json:"column"`
	Row        int64 `json:"row"`
	ColumnSpan int64 `json:"columnSpan"`
	RowSpan    int64 `json:"rowSpan"`
}
type serializedInsets struct{ Top, Right, Bottom, Left float64 }
type serializedBox struct {
	Kind     string                 `json:"kind"`
	Z        int64                  `json:"z"`
	Absolute *serializedRect        `json:"absolute"`
	Anchored *serializedBoxAnchored `json:"anchored"`
	Span     *serializedBoxSpan     `json:"span,omitempty"`
	Layer    int64                  `json:"layer,omitempty"`
	Region   string                 `json:"region,omitempty"`
}
type serializedBoxSpan struct {
	X serializedSpan `json:"x"`
	Y serializedSpan `json:"y"`
}
type serializedSpan struct {
	Kind string  `json:"kind"`
	A    float64 `json:"a"`
	B    float64 `json:"b"`
}
type serializedBoxAnchored struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}
type serializedRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

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
	c := *node
	c.Children = ch
	c.Box, c.Layout, c.Grid, c.Cell = cloneBox(node.Box), cloneRect(node.Layout), cloneGrid(node.Grid), cloneCell(node.Cell)
	c.Style, c.Stack, c.Sizing, c.Offset, c.TextSpec = cloneStyle(node.Style), cloneStack(node.Stack), cloneSizing(node.Sizing), cloneOffset(node.Offset), cloneTextSpec(node.TextSpec)
	if node.Input != nil {
		in := *node.Input
		c.Input = &in
	}
	if node.States != nil {
		st := *node.States
		c.States = &st
	}
	return &c
}
func cloneStack(s *StackSpec) *StackSpec {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
func cloneSizing(s *SizingSpec) *SizingSpec {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
func cloneTextSpec(t *TextSpec) *TextSpec {
	if t == nil {
		return nil
	}
	c := *t
	if t.Color != nil {
		color := *t.Color
		c.Color = &color
	}
	return &c
}
func cloneOffset(o *Offset) *Offset {
	if o == nil {
		return nil
	}
	c := *o
	return &c
}
func cloneStyle(s *StyleSpec) *StyleSpec {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
func rgbaArray(c RGBA) [4]uint8    { return [4]uint8{c.R, c.G, c.B, c.A} }
func rgbaOf(a [4]uint8) RGBA       { return RGBA{R: a[0], G: a[1], B: a[2], A: a[3]} }
func WithNodeIDs(node *Node) *Node { root := Clone(node); assignNodeIDs(root, "0"); return root }
func assignNodeIDs(node *Node, id string) {
	if node == nil {
		return
	}
	node.NodeID = id
	for i, child := range node.Children {
		segment := fmt.Sprintf("%d", i)
		if child != nil && child.Key != "" {
			segment = EscapeKey(child.Key)
		}
		assignNodeIDs(child, id+"."+segment)
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
	p += styleSuffix(node.Style) + sugarSuffix(node)
	switch node.Kind {
	case NodeText:
		return p + layoutSuffix(node.Layout) + "(" + node.Text + ")"
	case NodeSpacer, NodeSkip:
		return p + layoutSuffix(node.Layout)
	case NodeButton:
		return p + layoutSuffix(node.Layout) + "(" + node.Label + "->" + node.Event + ",enabled=" + fmt.Sprintf("%t", node.Enabled) + ")"
	case NodeCheckbox:
		return p + layoutSuffix(node.Layout) + "(" + node.Label + ",checked=" + fmt.Sprintf("%t", node.Input.Checked) + "->" + node.Event + ")"
	case NodeTextField:
		return p + layoutSuffix(node.Layout) + "(" + fmt.Sprintf("%q", node.Input.Value) + "->" + node.Event + ")"
	case NodeSlider:
		return p + layoutSuffix(node.Layout) + "(" + fmt.Sprintf("%g in %g..%g", node.Input.Number, node.Input.Min, node.Input.Max) + "->" + node.Event + ")"
	case NodeColumn, NodeRow, NodeGrid, NodeAbsoluteBox, NodeAnchorBox:
		parts := make([]string, 0, len(node.Children))
		for _, c := range node.Children {
			parts = append(parts, Signature(c))
		}
		if node.Box != nil {
			return p + "(" + boxSignature(node.Box) + ")" + layoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
		}
		if node.Kind == NodeGrid && node.Grid != nil {
			return p + "(gridSpec)" + layoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
		}
		if node.Cell != nil {
			return p + "(cell=" + fmt.Sprintf("%d,%d,%d,%d", node.Cell.Column, node.Cell.Row, node.Cell.ColumnSpan, node.Cell.RowSpan) + ")" + layoutSuffix(node.Layout) + "[" + strings.Join(parts, ",") + "]"
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
	if node.Style != nil {
		s.Style = &serializedStyle{Background: rgbaArray(node.Style.Background), Foreground: rgbaArray(node.Style.Foreground),
			BorderColor: rgbaArray(node.Style.BorderColor), BorderWidth: node.Style.BorderWidth, Radius: node.Style.Radius}
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
	if node.Grid != nil {
		s.Grid = serializeGrid(node.Grid)
	}
	if node.Cell != nil {
		s.Cell = &serializedCell{Column: node.Cell.Column, Row: node.Cell.Row, ColumnSpan: node.Cell.ColumnSpan, RowSpan: node.Cell.RowSpan}
	}
	s.GridRows = node.GridRows
	s.GridCols = node.GridCols
	for _, c := range node.Children {
		s.Children = append(s.Children, serializeNode(c))
	}
	return s
}
func serializeGrid(grid *GridSpec) *serializedGrid {
	if grid == nil {
		return nil
	}
	s := &serializedGrid{
		Columns:   make([]serializedGridTrack, 0, len(grid.Columns)),
		Rows:      make([]serializedGridTrack, 0, len(grid.Rows)),
		ColumnGap: grid.ColumnGap,
		RowGap:    grid.RowGap,
		Padding:   serializedInsets{Top: grid.Padding.Top, Right: grid.Padding.Right, Bottom: grid.Padding.Bottom, Left: grid.Padding.Left},
	}
	for _, c := range grid.Columns {
		s.Columns = append(s.Columns, serializedGridTrack{Kind: string(c.Kind), Size: c.Size, Weight: c.Weight})
	}
	for _, r := range grid.Rows {
		s.Rows = append(s.Rows, serializedGridTrack{Kind: string(r.Kind), Size: r.Size, Weight: r.Weight})
	}
	return s
}
func serializeBox(box *BoxSpec) *serializedBox {
	if box == nil {
		return nil
	}
	s := &serializedBox{Kind: string(box.Kind), Z: box.ZOrder, Layer: box.Layer, Region: box.Region}
	if box.Kind == BoxSpan {
		s.Span = &serializedBoxSpan{
			X: serializedSpan{Kind: string(box.XSpan.Kind), A: box.XSpan.A, B: box.XSpan.B},
			Y: serializedSpan{Kind: string(box.YSpan.Kind), A: box.YSpan.A, B: box.YSpan.B},
		}
		return s
	}
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
	if node.Style != nil {
		r.Style = &StyleSpec{Background: rgbaOf(node.Style.Background), Foreground: rgbaOf(node.Style.Foreground),
			BorderColor: rgbaOf(node.Style.BorderColor), BorderWidth: node.Style.BorderWidth, Radius: node.Style.Radius}
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
	if node.Grid != nil {
		r.Grid = deserializeGrid(node.Grid)
	}
	if node.Cell != nil {
		r.Cell = &CellSpec{Column: node.Cell.Column, Row: node.Cell.Row, ColumnSpan: node.Cell.ColumnSpan, RowSpan: node.Cell.RowSpan}
	}
	r.GridRows = node.GridRows
	r.GridCols = node.GridCols
	for _, c := range node.Children {
		n, err := deserializeNode(c)
		if err != nil {
			return nil, err
		}
		r.Children = append(r.Children, n)
	}
	return r, nil
}
func deserializeGrid(grid *serializedGrid) *GridSpec {
	if grid == nil {
		return nil
	}
	out := &GridSpec{
		Columns:   make([]GridTrack, 0, len(grid.Columns)),
		Rows:      make([]GridTrack, 0, len(grid.Rows)),
		ColumnGap: grid.ColumnGap,
		RowGap:    grid.RowGap,
		Padding:   EdgeInsets{Top: grid.Padding.Top, Right: grid.Padding.Right, Bottom: grid.Padding.Bottom, Left: grid.Padding.Left},
	}
	for _, c := range grid.Columns {
		out.Columns = append(out.Columns, GridTrack{Kind: GridTrackKind(c.Kind), Size: c.Size, Weight: c.Weight})
	}
	for _, r := range grid.Rows {
		out.Rows = append(out.Rows, GridTrack{Kind: GridTrackKind(r.Kind), Size: r.Size, Weight: r.Weight})
	}
	return out
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
		return &BoxSpec{Kind: BoxAbsolute, ZOrder: box.Z, Layer: box.Layer, X: box.Absolute.X, Y: box.Absolute.Y, Width: box.Absolute.Width, Height: box.Absolute.Height}, nil
	case BoxAnchored:
		if box.Anchored == nil {
			return nil, fmt.Errorf("uiir box deserialization failed: anchored payload missing")
		}
		return &BoxSpec{Kind: BoxAnchored, ZOrder: box.Z, Layer: box.Layer, Left: box.Anchored.Left, Top: box.Anchored.Top, Right: box.Anchored.Right, Bottom: box.Anchored.Bottom}, nil
	case BoxSpan:
		if box.Span == nil {
			return nil, fmt.Errorf("uiir box deserialization failed: span payload missing")
		}
		return &BoxSpec{Kind: BoxSpan, ZOrder: box.Z, Layer: box.Layer, Region: box.Region,
			XSpan: SpanSpec{Kind: SpanKind(box.Span.X.Kind), A: box.Span.X.A, B: box.Span.X.B},
			YSpan: SpanSpec{Kind: SpanKind(box.Span.Y.Kind), A: box.Span.Y.A, B: box.Span.Y.B}}, nil
	default:
		return nil, fmt.Errorf("uiir box deserialization failed: unsupported kind %q", box.Kind)
	}
}
func cloneBox(spec *BoxSpec) *BoxSpec {
	if spec == nil {
		return nil
	}
	return &BoxSpec{Kind: spec.Kind, ZOrder: spec.ZOrder, X: spec.X, Y: spec.Y, Width: spec.Width, Height: spec.Height, Left: spec.Left, Top: spec.Top, Right: spec.Right, Bottom: spec.Bottom, XSpan: spec.XSpan, YSpan: spec.YSpan, Layer: spec.Layer, Region: spec.Region, ResolvedRect: cloneRect(spec.ResolvedRect)}
}
func cloneRect(r *ResolvedRect) *ResolvedRect {
	if r == nil {
		return nil
	}
	return &ResolvedRect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}
func cloneGrid(spec *GridSpec) *GridSpec {
	if spec == nil {
		return nil
	}
	out := &GridSpec{Columns: append([]GridTrack{}, spec.Columns...), Rows: append([]GridTrack{}, spec.Rows...), ColumnGap: spec.ColumnGap, RowGap: spec.RowGap, Padding: spec.Padding}
	return out
}
func cloneCell(spec *CellSpec) *CellSpec {
	if spec == nil {
		return nil
	}
	return &CellSpec{Column: spec.Column, Row: spec.Row, ColumnSpan: spec.ColumnSpan, RowSpan: spec.RowSpan}
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
	if spec.Kind == BoxSpan {
		return fmt.Sprintf("span(z=%d,layer=%d,region=%s,x=%s,y=%s)", spec.ZOrder, spec.Layer, spec.Region, spanSignature(spec.XSpan), spanSignature(spec.YSpan))
	}
	if spec.Kind == BoxAbsolute {
		return fmt.Sprintf("absolute(z=%d,%.2f,%.2f,%.2f,%.2f)", spec.ZOrder, spec.X, spec.Y, spec.Width, spec.Height)
	}
	return fmt.Sprintf("anchored(z=%d,%.2f,%.2f,%.2f,%.2f)", spec.ZOrder, spec.Left, spec.Top, spec.Right, spec.Bottom)
}

// styleSuffix is empty for unstyled nodes so existing signatures are stable.
func styleSuffix(s *StyleSpec) string {
	if s == nil {
		return ""
	}
	hex := func(c RGBA) string { return fmt.Sprintf("%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }
	return fmt.Sprintf("{style=bg:%s,fg:%s,border:%s/%g,radius:%g}", hex(s.Background), hex(s.Foreground), hex(s.BorderColor), s.BorderWidth, s.Radius)
}

func spanSignature(s SpanSpec) string {
	return fmt.Sprintf("%s:%.2f/%.2f", s.Kind, s.A, s.B)
}

// EscapeKey makes a UI.Keyed key safe as one node-id segment: "." and "%"
// are percent-escaped, so a key can never collide with a nested path.
func EscapeKey(key string) string {
	return strings.NewReplacer("%", "%25", ".", "%2E").Replace(key)
}

// sugarSuffix is empty for nodes without stack spec, sizing or offset, so
// existing signatures are stable.
func sugarSuffix(node *Node) string {
	out := ""
	if node.Stack != nil {
		st := node.Stack
		out += fmt.Sprintf("{stack=gap:%g,pad:%g/%g/%g/%g,align:%s,justify:%s}", st.Gap, st.Padding.Top, st.Padding.Right, st.Padding.Bottom, st.Padding.Left, st.Align, st.Justify)
	}
	if node.Sizing != nil {
		z := node.Sizing
		out += fmt.Sprintf("{size=%s:%g/%g/%g/%g}", z.Kind, z.Main, z.Weight, z.Width, z.Height)
	}
	if node.Offset != nil {
		out += fmt.Sprintf("{nudge=%g,%g}", node.Offset.X, node.Offset.Y)
	}
	if st := node.States; st != nil {
		out += "{states=hover" + styleSuffix(&st.Hover) + ",pressed" + styleSuffix(&st.Pressed) + ",focused" + styleSuffix(&st.Focused) + ",disabled" + styleSuffix(&st.Disabled) + "}"
	}
	if t := node.TextSpec; t != nil {
		color := "inherit"
		if t.Color != nil {
			color = fmt.Sprintf("%d,%d,%d,%d", t.Color.R, t.Color.G, t.Color.B, t.Color.A)
		}
		out += fmt.Sprintf("{text=size:%g,weight:%d,mono:%t,wrap:%t,overflow:%s,color:%s}", t.Size, t.Weight, t.Mono, t.Wrap, t.Overflow, color)
	}
	return out
}
