package render

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/machina/layout"
)

type SnapshotRecorder struct{}

func RecordSnapshot(commands []Command) (string, error) {
	return SnapshotRecorder{}.Record(commands)
}

func (SnapshotRecorder) Record(commands []Command) (string, error) {
	var lines []string
	frameOpen := false
	frameClosed := false
	clipStack := []layout.NodeID{}
	for i, command := range commands {
		if command == nil {
			return "", fmt.Errorf("render.err.nil_command: command at index %d is nil", i)
		}
		switch c := command.(type) {
		case BeginFrameCommand:
			if frameOpen {
				return "", fmt.Errorf("render.err.nested_begin_frame: command at index %d starts nested frame", i)
			}
			if frameClosed {
				return "", fmt.Errorf("render.err.command_after_end_frame: command at index %d appears after end frame", i)
			}
			frameOpen = true
			lines = append(lines, fmt.Sprintf("BeginFrame rect=%s", fmtRect(c.Root)))
		case EndFrameCommand:
			if !frameOpen {
				return "", fmt.Errorf("render.err.end_without_begin: command at index %d ends frame before begin", i)
			}
			if len(clipStack) > 0 {
				return "", fmt.Errorf("render.err.unbalanced_clip_stack: frame ended with %d active clip(s)", len(clipStack))
			}
			if frameClosed {
				return "", fmt.Errorf("render.err.command_after_end_frame: command at index %d appears after end frame", i)
			}
			frameClosed = true
			frameOpen = false
			lines = append(lines, "EndFrame")
		case DrawTextCommand:
			if !frameOpen || frameClosed {
				return "", fmt.Errorf("render.err.command_outside_frame: DrawText at index %d must be inside active frame", i)
			}
			lines = append(lines, fmt.Sprintf("DrawText node=%s rect=%s text=%q", c.NodeID, fmtRect(c.Rect), c.Text))
		case FillRectCommand:
			if !frameOpen || frameClosed {
				return "", fmt.Errorf("render.err.command_outside_frame: FillRect at index %d must be inside active frame", i)
			}
			lines = append(lines, fmt.Sprintf("FillRect node=%s rect=%s", c.NodeID, fmtRect(c.Rect)))
		case PushClipCommand:
			if !frameOpen || frameClosed {
				return "", fmt.Errorf("render.err.command_outside_frame: PushClip at index %d must be inside active frame", i)
			}
			clipStack = append(clipStack, c.NodeID)
			lines = append(lines, fmt.Sprintf("PushClip node=%s rect=%s", c.NodeID, fmtRect(c.Rect)))
		case PopClipCommand:
			if !frameOpen || frameClosed {
				return "", fmt.Errorf("render.err.command_outside_frame: PopClip at index %d must be inside active frame", i)
			}
			if len(clipStack) == 0 {
				return "", fmt.Errorf("render.err.unbalanced_clip_stack: pop without matching push at index %d", i)
			}
			clipStack = clipStack[:len(clipStack)-1]
			lines = append(lines, fmt.Sprintf("PopClip node=%s", c.NodeID))
		default:
			return "", fmt.Errorf("render.err.unknown_command: unsupported command type %T", command)
		}
	}
	if !frameClosed {
		return "", fmt.Errorf("render.err.missing_end_frame: frame did not terminate")
	}
	return strings.Join(lines, "\n"), nil
}

func fmtRect(r layout.Rect) string {
	return fmt.Sprintf("(%.2f,%.2f,%.2f,%.2f)", r.X, r.Y, r.Width, r.Height)
}
