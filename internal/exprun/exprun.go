package exprun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/pkgmgr"
	"github.com/yuechen-li-dev/oct/internal/tester"
)

const experimentKind = "experiment"

type Result struct {
	GetResult      pkgmgr.GetResult
	SyncResult     pkgmgr.SyncResult
	EntryMilestone string
}

func RunFromGit(source string, stdout io.Writer) (Result, error) {
	manager, err := pkgmgr.NewManager()
	if err != nil {
		return Result{}, err
	}
	getResult, err := manager.Get(source)
	if err != nil {
		return Result{}, err
	}
	status := "fetched"
	if getResult.Hit {
		status = "cache hit"
	}
	_, _ = fmt.Fprintf(stdout, "experiment source: %s\n", getResult.Source)
	_, _ = fmt.Fprintf(stdout, "experiment cache: %s\n", getResult.Path)
	_, _ = fmt.Fprintf(stdout, "experiment fetch: %s\n", status)

	manifest := getResult.Manifest
	if manifest.Kind != experimentKind {
		return Result{}, fmt.Errorf("fetched package is not an experiment (manifest Kind=%q)", manifest.Kind)
	}
	if !hasFile(getResult.Path, "REPORT.md") {
		return Result{}, fmt.Errorf("fetched package is not a runnable experiment: missing REPORT.md")
	}

	syncResult, err := manager.Sync(getResult.Path)
	if err != nil {
		return Result{}, fmt.Errorf("dependency sync failed: %w", err)
	}
	_, _ = fmt.Fprintf(stdout, "dependency sync: %d direct dependency(ies)\n", len(syncResult.Dependencies))
	for _, dep := range syncResult.Dependencies {
		depStatus := "fetched"
		if dep.GetResult.Hit {
			depStatus = "cache hit"
		}
		_, _ = fmt.Fprintf(stdout, "- %s (%s) [%s]\n", dep.Name, dep.VersionRequirement, depStatus)
	}

	entryPath, entryMilestone, err := selectEntry(getResult.Path, manifest.EntryMilestone)
	if err != nil {
		return Result{}, err
	}
	if entryMilestone != "" {
		_, _ = fmt.Fprintf(stdout, "entry milestone: %s\n", entryMilestone)
	} else {
		_, _ = fmt.Fprintln(stdout, "entry milestone: canonical experiment root")
	}

	if err := tester.Execute(entryPath, stdout); err != nil {
		return Result{}, err
	}
	return Result{GetResult: getResult, SyncResult: syncResult, EntryMilestone: entryMilestone}, nil
}

func selectEntry(root string, explicit string) (string, string, error) {
	if strings.TrimSpace(explicit) != "" {
		ref, ok := parseMilestoneName(explicit)
		if !ok || ref.Kind != milestoneCanonical {
			return "", "", fmt.Errorf("explicit entry milestone %q is invalid; expected canonical milestone name like M1 or M1a", explicit)
		}
		entryPath := filepath.Join(root, explicit)
		info, err := os.Stat(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", "", fmt.Errorf("explicit entry milestone %q not found in fetched experiment", explicit)
			}
			return "", "", fmt.Errorf("check explicit entry milestone %q: %w", explicit, err)
		}
		if !info.IsDir() {
			return "", "", fmt.Errorf("explicit entry milestone %q is not a directory", explicit)
		}
		return entryPath, explicit, nil
	}
	milestones, err := discoverCanonicalMilestones(root)
	if err != nil {
		return "", "", err
	}
	if len(milestones) == 0 {
		return "", "", fmt.Errorf("fetched package is not a runnable experiment: no canonical milestones found")
	}
	return root, "", nil
}

func discoverCanonicalMilestones(root string) ([]milestoneRef, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read experiment root %s: %w", root, err)
	}
	milestones := make([]milestoneRef, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ref, ok := parseMilestoneName(entry.Name())
		if !ok || ref.Kind != milestoneCanonical {
			continue
		}
		milestones = append(milestones, ref)
	}
	sort.Slice(milestones, func(i, j int) bool {
		if milestones[i].Number != milestones[j].Number {
			return milestones[i].Number < milestones[j].Number
		}
		if milestones[i].Suffix == milestones[j].Suffix {
			return false
		}
		if milestones[i].Suffix == "" {
			return true
		}
		if milestones[j].Suffix == "" {
			return false
		}
		return milestones[i].Suffix < milestones[j].Suffix
	})
	return milestones, nil
}

func hasFile(root string, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && !info.IsDir()
}

type milestoneKind int

const (
	milestoneUnknown milestoneKind = iota
	milestoneCanonical
	milestoneAuxiliary
)

type milestoneRef struct {
	Name   string
	Number int
	Suffix string
	Kind   milestoneKind
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
