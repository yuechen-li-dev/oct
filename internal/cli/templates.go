package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/templatecatalog"
)

func executeTemplates(args []string, stdout, stderr io.Writer, workingDir string) error {
	if isHelpArg(args) {
		return writeTemplatesHelp(stdout)
	}
	if len(args) == 0 {
		return reportCommandError(stderr, "templates", fmt.Errorf("usage: oct templates <list|describe>"))
	}
	switch args[0] {
	case "list":
		root, jsonOutput, err := parseTemplateCatalogArgs(args[1:], workingDir, false)
		if err != nil {
			return reportCommandError(stderr, "templates list", err)
		}
		entries, err := templatecatalog.Load(root)
		if err != nil {
			return reportCommandError(stderr, "templates list", err)
		}
		if jsonOutput {
			return writeTemplateJSON(stdout, entries)
		}
		if len(entries) == 0 {
			_, err = fmt.Fprintln(stdout, "No *.template.oct declarations found.")
			return err
		}
		for _, entry := range entries {
			if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", entry.Name, entry.Category, entry.Kind, entry.Summary); err != nil {
				return err
			}
		}
		return nil
	case "describe":
		if len(args) < 2 {
			return reportCommandError(stderr, "templates describe", fmt.Errorf("usage: oct templates describe <Name> [root] [--json]"))
		}
		name := args[1]
		root, jsonOutput, err := parseTemplateCatalogArgs(args[2:], workingDir, false)
		if err != nil {
			return reportCommandError(stderr, "templates describe", err)
		}
		entries, err := templatecatalog.Load(root)
		if err != nil {
			return reportCommandError(stderr, "templates describe", err)
		}
		matches := make([]templatecatalog.Entry, 0, 1)
		for _, entry := range entries {
			if entry.Name == name {
				matches = append(matches, entry)
			}
		}
		if len(matches) == 0 {
			return reportCommandError(stderr, "templates describe", fmt.Errorf("template %q was not found under %s", name, root))
		}
		if len(matches) > 1 {
			return reportCommandError(stderr, "templates describe", fmt.Errorf("template %q is ambiguous under %s", name, root))
		}
		if jsonOutput {
			return writeTemplateJSON(stdout, matches[0])
		}
		return writeTemplateDescription(stdout, matches[0])
	default:
		return reportCommandError(stderr, "templates", fmt.Errorf("unknown templates command %q; expected list or describe", args[0]))
	}
}

func parseTemplateCatalogArgs(args []string, workingDir string, requireName bool) (string, bool, error) {
	_ = requireName
	root := workingDir
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, fmt.Errorf("unknown option %s", arg)
			}
			if root != workingDir {
				return "", false, fmt.Errorf("only one catalog root may be provided")
			}
			root = resolveWorkingPath(workingDir, arg)
		}
	}
	return filepath.Clean(root), jsonOutput, nil
}

func writeTemplateJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeTemplateDescription(out io.Writer, entry templatecatalog.Entry) error {
	requirements := make([]string, len(entry.Requirements))
	for i := range entry.Requirements {
		requirements[i] = "[" + entry.RequirementKinds[i] + "] " + entry.Requirements[i]
	}
	lines := []string{
		entry.Name + " (" + entry.Kind + ")",
		"Category: " + entry.Category,
		"Summary: " + entry.Summary,
		"Type parameters: " + strings.Join(entry.TypeParameters, ", "),
		"Configurable fields: " + strings.Join(entry.ConfigurableFields, "; "),
		"Requires: " + strings.Join(requirements, "; "),
		"Provides: " + strings.Join(entry.ProvidedSemantics, "; "),
		"Use when: " + strings.Join(entry.UseWhen, "; "),
		"Avoid when: " + strings.Join(entry.AvoidWhen, "; "),
		"Source: " + entry.SourcePath,
	}
	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

func writeTemplatesHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct templates <list|describe> [options]\n\ncommands:\n  oct templates list [root] [--json]             List documented *.template.oct declarations\n  oct templates describe <Name> [root] [--json]  Describe one template from source-derived metadata")
	return err
}
