package tester

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/interpret"
)

type milestoneKind int

const (
	milestoneUnknown milestoneKind = iota
	milestoneCanonical
	milestoneAuxiliary
)

type milestoneRef struct {
	Name   string
	Path   string
	Number int
	Suffix string
	Kind   milestoneKind
}

func executeForPathOrExperiment(path string, stdout io.Writer, command string, single func(string, io.Writer) error) error {
	milestones, isExperiment, err := discoverCanonicalExperimentMilestones(path)
	if err != nil {
		return err
	}
	if !isExperiment {
		return single(path, stdout)
	}
	if len(milestones) == 0 {
		return fmt.Errorf("experiment root has no canonical milestones")
	}
	failed := make([]string, 0)
	for _, milestone := range milestones {
		_, _ = fmt.Fprintf(stdout, "MILESTONE %s (%s)\n", milestone.Name, command)
		clearPrefix := interpret.SetOutputPathPrefix(milestone.Name)
		logger := &milestoneLogWriter{target: stdout, milestone: milestone.Name, atStartOfNewLine: true}
		err := single(milestone.Path, logger)
		clearPrefix()
		if err != nil {
			failed = append(failed, milestone.Name)
			_, _ = fmt.Fprintf(stdout, "MILESTONE FAIL %s (%s): %v\n", milestone.Name, command, err)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "MILESTONE PASS %s (%s)\n", milestone.Name, command)
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d milestone(s) failed: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

type milestoneLogWriter struct {
	target           io.Writer
	milestone        string
	atStartOfNewLine bool
}

func (w *milestoneLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	prefix := []byte("[" + w.milestone + "] ")
	decorated := make([]byte, 0, len(p)+len(prefix)*2)
	atStart := w.atStartOfNewLine
	for _, b := range p {
		if atStart {
			decorated = append(decorated, prefix...)
			atStart = false
		}
		decorated = append(decorated, b)
		if b == '\n' {
			atStart = true
		}
	}
	_, err := w.target.Write(decorated)
	if err != nil {
		return 0, err
	}
	w.atStartOfNewLine = atStart
	return len(p), nil
}

func discoverCanonicalExperimentMilestones(path string) ([]milestoneRef, bool, error) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, false, nil
	}
	if !hasFile(path, "manifest.oct") || !hasFile(path, "REPORT.md") {
		return nil, false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, false, fmt.Errorf("read experiment root %s: %w", path, err)
	}
	milestones := make([]milestoneRef, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ref, ok := parseMilestoneName(entry.Name())
		if !ok {
			continue
		}
		if ref.Kind != milestoneCanonical {
			continue
		}
		ref.Path = filepath.Join(path, entry.Name())
		milestones = append(milestones, ref)
	}
	slices.SortFunc(milestones, compareMilestones)
	return milestones, true, nil
}

func hasFile(root string, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && !info.IsDir()
}

func parseMilestoneName(name string) (milestoneRef, bool) {
	kind := milestoneCanonical
	body := name
	if strings.HasPrefix(body, "Mx") {
		kind = milestoneAuxiliary
		body = strings.TrimPrefix(body, "Mx")
	} else if strings.HasPrefix(body, "M") {
		body = strings.TrimPrefix(body, "M")
	} else {
		return milestoneRef{}, false
	}
	if body == "" {
		return milestoneRef{}, false
	}
	idx := 0
	for idx < len(body) && body[idx] >= '0' && body[idx] <= '9' {
		idx++
	}
	if idx == 0 {
		return milestoneRef{}, false
	}
	number, err := strconv.Atoi(body[:idx])
	if err != nil {
		return milestoneRef{}, false
	}
	suffix := body[idx:]
	if suffix != "" {
		if len(suffix) != 1 || suffix[0] < 'a' || suffix[0] > 'z' {
			return milestoneRef{}, false
		}
	}
	return milestoneRef{Name: name, Number: number, Suffix: suffix, Kind: kind}, true
}

func compareMilestones(a milestoneRef, b milestoneRef) int {
	if a.Number != b.Number {
		return a.Number - b.Number
	}
	if a.Suffix == b.Suffix {
		return 0
	}
	if a.Suffix == "" {
		return -1
	}
	if b.Suffix == "" {
		return 1
	}
	if a.Suffix < b.Suffix {
		return -1
	}
	return 1
}
