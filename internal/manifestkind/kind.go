package manifestkind

import "fmt"

const (
	Pure        = "pure"
	Experiment  = "experiment"
	Wrapper     = "wrapper"
	Application = "application"
)

func Normalize(kind string) (string, error) {
	switch kind {
	case "":
		return Pure, nil
	case Pure, Experiment, Wrapper, Application:
		return kind, nil
	default:
		return "", fmt.Errorf("invalid manifest Kind %q", kind)
	}
}

func ValidateEntryMilestone(kind string, entryMilestone string) error {
	if entryMilestone == "" {
		return nil
	}
	if kind != Experiment {
		return fmt.Errorf("manifest EntryMilestone is only valid for Kind %q", Experiment)
	}
	return nil
}
