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
	if !res.Hit || res.Action == nil || res.Action.Name != "thermostat.raise" {
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
	if err := s.SetViewport(layout.Rect{Width: 320, Height: 240}); err != nil {
		t.Fatal(err)
	}
	if len(frameTexts(t, s)) == 0 {
		t.Fatal("no frame after resize")
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
