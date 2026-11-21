package octapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/machina/layout"
	"github.com/yuechen-li-dev/oct/internal/machina/render"
	"github.com/yuechen-li-dev/oct/internal/machina/session"
)

// The thermostat example is the M0 reference app; its Oct-level behavior is
// pinned by Examples/MachinaThermostat/main.octest. This test only checks the
// Go adapter boundary: Init/View/Update are wired through a real session and
// a pointer press dispatches back into Oct.
const thermostat = "../../../Examples/MachinaThermostat/main.oct"

func frameTexts(t *testing.T, s *session.Session[interpret.Value]) []string {
	t.Helper()
	cmds, err := s.Commands()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range cmds {
		if d, ok := c.(render.DrawTextCommand); ok {
			out = append(out, d.Text)
		}
	}
	return out
}

func centerOf(t *testing.T, s *session.Session[interpret.Value], label string) session.PointerEvent {
	t.Helper()
	cmds, _ := s.Commands()
	for _, c := range cmds {
		if d, ok := c.(render.DrawTextCommand); ok && d.Text == label {
			return session.PointerEvent{X: d.Rect.X + d.Rect.Width/2, Y: d.Rect.Y + d.Rect.Height/2}
		}
	}
	t.Fatalf("label %q not in frame %v", label, frameTexts(t, s))
	return session.PointerEvent{}
}

func TestPointerPressRoundTripsThroughOct(t *testing.T) {
	s, err := NewSession(thermostat, layout.Rect{Width: 640, Height: 480}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := strings.Join(frameTexts(t, s), "|")
	res, err := s.PointerDown(centerOf(t, s, "Raise"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.Action == nil || res.Action.Name != "Msg.Raise" {
		t.Fatalf("dispatch = %+v", res)
	}
	after := strings.Join(frameTexts(t, s), "|")
	if before == after {
		t.Fatalf("frame did not change after Raise: %s", after)
	}
	if res.SnapshotBefore == res.SnapshotAfter {
		t.Fatalf("session snapshot did not change")
	}
}

func TestResizeRelayoutsWithoutReloading(t *testing.T) {
	s, err := NewSession(thermostat, layout.Rect{Width: 640, Height: 480}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetViewport(layout.Rect{Width: 480, Height: 320}); err != nil {
		t.Fatal(err)
	}
	if len(frameTexts(t, s)) == 0 {
		t.Fatal("no frame after resize")
	}
	// Too narrow for the button row: an explicit overflow error, and the
	// session keeps the last frame that fit.
	before := strings.Join(frameTexts(t, s), "|")
	err = s.SetViewport(layout.Rect{Width: 200, Height: 320})
	if err == nil || !strings.Contains(err.Error(), "stack_overflow") {
		t.Fatalf("narrow viewport error = %v", err)
	}
	if after := strings.Join(frameTexts(t, s), "|"); after != before {
		t.Fatalf("frame changed after a failed resize: %s", after)
	}
}

func TestNonAppProgramsAreRejectedWithTheConvention(t *testing.T) {
	dir := t.TempDir()
	b, err := os.ReadFile(filepath.Join("testdata", "notanapp", "main.oct.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.oct")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = Load(path, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"not a Machina UI app", "fn Init()", "fn Update(", "fn View("} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

// loadVariant writes the layout-table fixture with one line replaced into a
// directory inside the repository (so `import UI` resolves) and loads it.
func loadVariant(t *testing.T, oldLine, newLine string) error {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "layouttable", "main.oct.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if oldLine != "" {
		if !strings.Contains(src, oldLine) {
			t.Fatalf("fixture lacks %q", oldLine)
		}
		src = strings.Replace(src, oldLine, newLine, 1)
	}
	dir, err := os.MkdirTemp("testdata", "variant-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(abs, "main.oct")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = NewSession(path, layout.Rect{Width: 320, Height: 240}, nil)
	return err
}

func TestLayoutTableArrangesEveryRegion(t *testing.T) {
	if err := loadVariant(t, "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutTableValidation(t *testing.T) {
	body := "UI.Region(Shell.Body, UI.StretchX(0px, 0px), UI.StretchY(40px, 0px)) // BODY"
	cases := []struct {
		name, replacement, want string
	}{
		{"missing region", "UI.Region(Shell.Header, UI.StretchX(0px, 0px), UI.Top(0px, 40px)) // BODY", "region Shell.Header is placed twice"},
		{"duplicate", "UI.Region(Shell.Body, UI.StretchX(0px, 0px), UI.StretchY(40px, 0px)),\n        UI.Region(Shell.Body, UI.StretchX(0px, 0px), UI.Top(0px, 1px))", "region Shell.Body is placed twice"},
		{"mixed enums", "UI.Region(Other.Body, UI.StretchX(0px, 0px), UI.StretchY(40px, 0px))", "row 1 places Other.Body, but the table's regions are Shell"},
		{"z out of range", "UI.Region(Shell.Body, UI.StretchX(0px, 0px), UI.StretchY(40px, 0px)) with { Z: 9 }", "region Shell.Body has z 9, outside -5..5"},
		{"stretch overflow", "UI.Region(Shell.Body, UI.StretchX(200px, 200px), UI.StretchY(40px, 0px))", "stretch insets 200 + 200 exceed the parent's 320"},
	}
	for _, c := range cases {
		err := loadVariant(t, body, c.replacement)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
	}
}

func TestLayoutTableMustPlaceEveryRegion(t *testing.T) {
	err := loadVariant(t, "UI.Region(Shell.Header, UI.StretchX(0px, 0px), UI.Top(0px, 40px)),\n", "")
	if err == nil || !strings.Contains(err.Error(), "the layout table does not place region Shell.Header") {
		t.Fatalf("error %v", err)
	}
}

func TestLayoutTableMustMatchTheContentRegions(t *testing.T) {
	err := loadVariant(t, "fn Content(model: Model, region: Shell) -> UI {\n    return switch region {\n        case Shell.Header => UI.Text(\"header\")\n        case Shell.Body => UI.Text(\"body\")",
		"fn Content(model: Model, region: Other) -> UI {\n    return switch region {\n        case Other.Header => UI.Text(\"header\")\n        case Other.Body => UI.Text(\"body\")")
	if err == nil || !strings.Contains(err.Error(), "places Shell regions, but the content function takes Other") {
		t.Fatalf("error %v", err)
	}
}

// clickVariant loads a layout-table fixture variant and clicks the button
// labeled label.
func clickVariant(t *testing.T, replacements [][2]string, label string) error {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "layouttable", "main.oct.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, r := range replacements {
		if !strings.Contains(src, r[0]) {
			t.Fatalf("fixture lacks %q", r[0])
		}
		src = strings.Replace(src, r[0], r[1], 1)
	}
	dir, err := os.MkdirTemp("testdata", "variant-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	abs, _ := filepath.Abs(dir)
	path := filepath.Join(abs, "main.oct")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(path, layout.Rect{Width: 320, Height: 240}, nil)
	if err != nil {
		return err
	}
	_, err = s.PointerDown(centerOf(t, s, label))
	return err
}

const headerText = `case Shell.Header => UI.Text("header")`

func TestTypedMessageToAStringUpdateIsAnError(t *testing.T) {
	err := clickVariant(t, [][2]string{{headerText, `case Shell.Header => UI.Button("Go", Other.Body, true)`}}, "Go")
	if err == nil || !strings.Contains(err.Error(), "button sends Other.Body, but Update takes an event String") {
		t.Fatalf("error %v", err)
	}
}

func TestStringEventToAnEnumUpdateIsAnError(t *testing.T) {
	err := clickVariant(t, [][2]string{
		{"fn Update(model: Model, event: String) -> Model", "fn Update(model: Model, msg: Other) -> Model"},
		{headerText, `case Shell.Header => UI.ButtonRaw("Go", "go", true)`},
	}, "Go")
	if err == nil || !strings.Contains(err.Error(), `button sends the event string "go", but Update takes Other`) {
		t.Fatalf("error %v", err)
	}
}

func TestUpdateMustTakeAMessageEnumOrString(t *testing.T) {
	err := clickVariant(t, [][2]string{{"fn Update(model: Model, event: String) -> Model", "fn Update(model: Model, event: Int) -> Model"}}, "header")
	if err == nil || !strings.Contains(err.Error(), "Update's second parameter must be the app's message enum") {
		t.Fatalf("error %v", err)
	}
}

func TestGridMatrixValidation(t *testing.T) {
	const body = `case Shell.Body => UI.Text("body")`
	two := `UI.Tracks([UI.Track.Fill(1.0), UI.Track.Fill(1.0)])`
	cases := []struct{ grid, want string }{
		{`UI.Grid(` + two + `, [[UI.Span(2, 1, UI.Text("a")), UI.Text("b")]])`, "covers (row 0, column 1), which must be UI.Skip()"},
		{`UI.Grid(` + two + `, [[UI.Text("a")]])`, "row 0 has 1 cells but the spec declares 2 columns"},
		{`UI.Grid(` + two + `, [[UI.Span(3, 1, UI.Text("a")), UI.Skip()]])`, "runs past the grid's 2 columns"},
		{`UI.Grid(` + two + ` with { Rows: [UI.Track.Auto, UI.Track.Auto] }, [[UI.Text("a"), UI.Text("b")]])`, "declares 2 rows but the grid has 1"},
	}
	for _, c := range cases {
		err := clickVariant(t, [][2]string{{body, "case Shell.Body => " + c.grid}}, "header")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.grid, err, c.want)
		}
	}
}
