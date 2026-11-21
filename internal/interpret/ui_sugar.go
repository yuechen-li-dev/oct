package interpret

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Stack and grid sugar, keyed identity and typed button messages behind
// Libraries/UI (DESIGN.md §5.2, §5.5, §5.6, §5.8).

var uiAligns = map[string]string{"Start": "start", "Center": "center", "End": "end"}
var uiJustifies = map[string]string{"Start": "start", "Center": "center", "End": "end", "SpaceBetween": "space-between"}

func uiInsets(v Value) machinauiir.EdgeInsets {
	f := v.Record.Fields
	return machinauiir.EdgeInsets{Top: float64(f["Top"].Int), Right: float64(f["Right"].Int), Bottom: float64(f["Bottom"].Int), Left: float64(f["Left"].Int)}
}

// uiTrack decodes a UI.Track enum value.
func uiTrack(v Value) (machinauiir.GridTrack, error) {
	if v.Kind != ValueEnum {
		return machinauiir.GridTrack{}, fmt.Errorf("runtime invariant violation: grid track expects UI.Track")
	}
	switch v.Enum.Variant {
	case "Fixed":
		return machinauiir.GridTrack{Kind: machinauiir.GridTrackFixed, Size: v.Enum.Payload.Float}, nil
	case "Fill":
		return machinauiir.GridTrack{Kind: machinauiir.GridTrackFill, Weight: v.Enum.Payload.Float}, nil
	case "Auto":
		return machinauiir.GridTrack{Kind: machinauiir.GridTrackAuto}, nil
	}
	return machinauiir.GridTrack{}, fmt.Errorf("runtime invariant violation: unknown UI.Track variant %q", v.Enum.Variant)
}

func uiTracks(v Value) ([]machinauiir.GridTrack, error) {
	out := make([]machinauiir.GridTrack, 0, len(v.Array))
	for _, t := range v.Array {
		track, err := uiTrack(t)
		if err != nil {
			return nil, err
		}
		if track.Kind == machinauiir.GridTrackFixed && track.Size < 0 {
			return nil, fmt.Errorf("UI.Grid: a fixed track must be non-negative, got %g", track.Size)
		}
		if track.Kind == machinauiir.GridTrackFill && track.Weight <= 0 {
			return nil, fmt.Errorf("UI.Grid: a fill track weight must be positive, got %g", track.Weight)
		}
		out = append(out, track)
	}
	return out, nil
}

// MessageAlias is the canonical printed form of a button message, used by
// tools and dumps. Nothing parses it back.
func MessageAlias(v Value) string {
	if v.Kind != ValueEnum {
		return v.String()
	}
	name := v.Enum.TypeName
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	out := name + "." + v.Enum.Variant
	if p := v.Enum.Payload; p != nil {
		if p.Kind == ValueString {
			out += "(" + strconv.Quote(p.Text) + ")"
		} else {
			out += "(" + MessageAlias(*p) + ")"
		}
	}
	return out
}

func uiChild(v Value, callee string) (*uiirNode, error) {
	if v.Kind != ValueUI || v.UI == nil {
		return nil, fmt.Errorf("runtime invariant violation: %s expects a UI child", callee)
	}
	return cloneUIIR(v.UI), nil
}

func (i interpreter) evalUISugarBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	args, errResult, err := i.evalUIArgs(env, pkgName, argumentExprs)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	ui := func(node *uiirNode) (evalResult, error) {
		return evalResult{value: Value{Kind: ValueUI, UI: node}}, nil
	}
	switch callee {
	case "UISkip":
		return ui(&uiirNode{Kind: machinauiir.NodeSkip})
	case "UIStack":
		spec := args[0].Record.Fields
		kind := uiirNodeColumn
		if spec["Axis"].Enum.Variant == "Horizontal" {
			kind = uiirNodeRow
		}
		gap := spec["Gap"].Float
		if gap < 0 {
			return evalResult{}, fmt.Errorf("UI.Stack: gap must be non-negative, got %g", gap)
		}
		children := make([]*uiirNode, 0, len(args[1].Array))
		for _, c := range args[1].Array {
			child, err := uiChild(c, "UI.Stack")
			if err != nil {
				return evalResult{}, err
			}
			children = append(children, child)
		}
		stack := &machinauiir.StackSpec{Gap: gap, Padding: uiInsets(spec["Padding"]), Align: uiAligns[spec["Align"].Enum.Variant], Justify: uiJustifies[spec["Justify"].Enum.Variant]}
		return ui(&uiirNode{Kind: kind, Stack: stack, Children: children})
	case "UISizing":
		child, err := uiChild(args[3], "UI sizing")
		if err != nil {
			return evalResult{}, err
		}
		a, b := args[1].Float, args[2].Float
		var z machinauiir.SizingSpec
		switch args[0].Int {
		case 0:
			if a < 0 {
				return evalResult{}, fmt.Errorf("UI.Fixed: size must be non-negative, got %g", a)
			}
			z = machinauiir.SizingSpec{Kind: machinauiir.SizingFixed, Main: a}
		case 1:
			if a <= 0 {
				return evalResult{}, fmt.Errorf("UI.Fill: weight must be positive, got %g", a)
			}
			z = machinauiir.SizingSpec{Kind: machinauiir.SizingFill, Weight: a}
		case 2:
			z = machinauiir.SizingSpec{Kind: machinauiir.SizingAuto}
		default:
			if a < 0 || b < 0 {
				return evalResult{}, fmt.Errorf("UI.Size: width and height must be non-negative, got %g and %g", a, b)
			}
			z = machinauiir.SizingSpec{Kind: machinauiir.SizingSize, Width: a, Height: b}
		}
		child.Sizing = &z
		return ui(child)
	case "UINudge":
		child, err := uiChild(args[2], "UI.Nudge")
		if err != nil {
			return evalResult{}, err
		}
		child.Offset = &machinauiir.Offset{X: args[0].Float, Y: args[1].Float}
		return ui(child)
	case "UIKeyed":
		if args[0].Text == "" {
			return evalResult{}, fmt.Errorf("UI.Keyed: key must be non-empty")
		}
		child, err := uiChild(args[1], "UI.Keyed")
		if err != nil {
			return evalResult{}, err
		}
		child.Key = args[0].Text
		return ui(child)
	case "UISpan":
		cols, rows := args[0].Int, args[1].Int
		if cols < 1 || rows < 1 {
			return evalResult{}, fmt.Errorf("UI.Span: spans must be at least 1, got %d columns and %d rows", cols, rows)
		}
		child, err := uiChild(args[2], "UI.Span")
		if err != nil {
			return evalResult{}, err
		}
		child.Cell = &machinauiir.CellSpec{ColumnSpan: cols, RowSpan: rows}
		return ui(child)
	case "UIGridMatrix":
		return i.evalUIGridMatrix(args[0], args[1])
	default: // UIButtonMsg
		label, msg, enabled := args[0], args[1], args[2]
		node := &uiirNode{Kind: uiirNodeButton, Label: label.Text, Enabled: enabled.Bool}
		if msg.Kind == ValueEnum {
			node.Event = MessageAlias(msg)
			m := cloneValue(msg)
			node.Message = m
		} else {
			node.Event = msg.Record.Fields["Value"].Text // legacy UI.EventToken
		}
		return ui(node)
	}
}

// evalUIGridMatrix builds a grid from a spec and a row-major matrix. Cells a
// span covers must be UI.Skip; an uncovered UI.Skip is an empty cell. Rows
// past the declared row tracks use AutoRow; there are no implicit columns.
func (i interpreter) evalUIGridMatrix(specValue, rowsValue Value) (evalResult, error) {
	spec := specValue.Record.Fields
	columns, err := uiTracks(spec["Columns"])
	if err != nil {
		return evalResult{}, err
	}
	rowTracks, err := uiTracks(spec["Rows"])
	if err != nil {
		return evalResult{}, err
	}
	autoRow, err := uiTrack(spec["AutoRow"])
	if err != nil {
		return evalResult{}, err
	}
	matrix := rowsValue.Array
	if len(columns) == 0 {
		return evalResult{}, fmt.Errorf("UI.Grid: the spec declares no columns")
	}
	if len(matrix) < len(rowTracks) {
		return evalResult{}, fmt.Errorf("UI.Grid: the spec declares %d rows but the grid has %d", len(rowTracks), len(matrix))
	}
	for r := len(rowTracks); r < len(matrix); r++ {
		rowTracks = append(rowTracks, autoRow)
	}
	covered := map[[2]int]bool{}
	children := []*uiirNode{}
	for r, row := range matrix {
		if len(row.Array) != len(columns) {
			return evalResult{}, fmt.Errorf("UI.Grid: row %d has %d cells but the spec declares %d columns", r, len(row.Array), len(columns))
		}
		for c, cell := range row.Array {
			node, err := uiChild(cell, "UI.Grid")
			if err != nil {
				return evalResult{}, err
			}
			if node.Kind == machinauiir.NodeSkip {
				continue
			}
			if covered[[2]int{r, c}] {
				return evalResult{}, fmt.Errorf("UI.Grid: cell (row %d, column %d) is covered by a span; it must be UI.Skip()", r, c)
			}
			cs, rs := int64(1), int64(1)
			if node.Cell != nil {
				cs, rs = node.Cell.ColumnSpan, node.Cell.RowSpan
			}
			if c+int(cs) > len(columns) || r+int(rs) > len(matrix) {
				return evalResult{}, fmt.Errorf("UI.Grid: the span at (row %d, column %d) runs past the grid's %d columns and %d rows", r, c, len(columns), len(matrix))
			}
			for dr := 0; dr < int(rs); dr++ {
				for dc := 0; dc < int(cs); dc++ {
					if dr == 0 && dc == 0 {
						continue
					}
					target := matrix[r+dr].Array[c+dc]
					if target.UI == nil || target.UI.Kind != machinauiir.NodeSkip {
						return evalResult{}, fmt.Errorf("UI.Grid: the span at (row %d, column %d) covers (row %d, column %d), which must be UI.Skip()", r, c, r+dr, c+dc)
					}
					covered[[2]int{r + dr, c + dc}] = true
				}
			}
			node.Cell = &machinauiir.CellSpec{Column: int64(c), Row: int64(r), ColumnSpan: cs, RowSpan: rs}
			children = append(children, node)
		}
	}
	colGap, rowGap := spec["ColumnGap"].Float, spec["RowGap"].Float
	if colGap < 0 || rowGap < 0 {
		return evalResult{}, fmt.Errorf("UI.Grid: gaps must be non-negative")
	}
	grid := &machinauiir.GridSpec{Columns: columns, Rows: rowTracks, ColumnGap: colGap, RowGap: rowGap, Padding: uiInsets(spec["Padding"])}
	return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: uiirNodeGrid, Grid: grid, Children: children}}}, nil
}
