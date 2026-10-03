package ocfmt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/judgment"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type Mode string

const (
	ModeDefault      Mode = ""
	ModeReadable     Mode = "readable"
	ModeCompact      Mode = "compact"
	ModeEnLLM        Mode = "en-llm"
	ModeEnLLMCompact Mode = "en-llm-compact"
)

// Arrows says what the formatter does with the two spellings of the arrow
// token, `->` and `=>`. They are one token to the language. The default keeps
// each arrow as its author wrote it.
type Arrows string

const (
	ArrowsKeep Arrows = "keep"
	ArrowsThin Arrows = "thin"
	ArrowsFat  Arrows = "fat"
)

type Options struct {
	Mode   Mode
	Arrows Arrows
	Check  bool
}

// settings is Options after validation: what the layout needs to know.
type settings struct {
	compact bool
	arrow   string // the spelling every arrow is written with; "" keeps each one
}

func resolveSettings(options Options) (settings, error) {
	mode, err := resolveMode(options.Mode)
	if err != nil {
		return settings{}, err
	}
	resolved := settings{compact: mode == ModeEnLLMCompact}
	switch options.Arrows {
	case "", ArrowsKeep:
	case ArrowsThin:
		resolved.arrow = "->"
	case ArrowsFat:
		resolved.arrow = "=>"
	default:
		return settings{}, fmt.Errorf("invalid --arrows %q; expected keep|thin|fat", options.Arrows)
	}
	return resolved, nil
}

type DecisionDiagnostics struct{ Traces []judgment.Result }

var octExtensions = map[string]struct{}{
	".oct":     {},
	".octest":  {},
	".octfail": {},
}

func FormatPath(path string) error { return FormatPathWithOptions(path, Options{}) }
func FormatPathWithOptions(path string, options Options) error { /* unchanged body omitted */
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		// One file that cannot be formatted does not stop the rest of the
		// tree: every file is attempted and every failure is reported.
		var failures []error
		walkErr := filepath.WalkDir(path, func(current string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !isOctFile(current) {
				return nil
			}
			if err := formatFile(current, options); err != nil {
				failures = append(failures, err)
			}
			return nil
		})
		if walkErr != nil {
			return walkErr
		}
		return errors.Join(failures...)
	}
	if !isOctFile(path) {
		return fmt.Errorf("unsupported file extension: %s", path)
	}
	return formatFile(path, options)
}
func FormatSource(src string) (string, error) { return FormatSourceWithOptions(src, Options{}) }
func FormatSourceWithOptions(src string, options Options) (string, error) {
	return formatSourceWithPath("<format>.oct", src, options)
}
func formatSourceWithPath(path string, src string, options Options) (string, error) {
	resolved, err := resolveSettings(options)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(path), ".octfail") {
		return formatOctFailSource(src, resolved)
	}
	out, _, err := formatRegularSource(path, src, resolved)
	return out, err
}
func formatSourceWithDiagnostics(src string, options Options) (string, DecisionDiagnostics, error) {
	resolved, err := resolveSettings(options)
	if err != nil {
		return "", DecisionDiagnostics{}, err
	}
	return formatRegularSource("<format>.oct", src, resolved)
}
func resolveMode(mode Mode) (Mode, error) {
	switch mode {
	case ModeDefault, ModeReadable:
		if mode == ModeDefault {
			return ModeEnLLM, nil
		}
		return ModeEnLLM, nil
	case ModeCompact:
		return ModeEnLLMCompact, nil
	case ModeEnLLM, ModeEnLLMCompact:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid --mode %q; expected en-llm|en-llm-compact", mode)
	}
}

// errSourceRejected marks a source the formatter refuses because it does not
// lex or parse. The text of the lexer or parser error follows it.
var errSourceRejected = errors.New("source is not valid Oct")

func formatRegularSource(path string, src string, resolved settings) (string, DecisionDiagnostics, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lexed, err := lex.Analyze(source.File{Path: path, Text: src})
	if err != nil {
		return "", DecisionDiagnostics{}, fmt.Errorf("%w: %w", errSourceRejected, err)
	}
	file, err := parse.BuildFile(lexed)
	if err != nil {
		return "", DecisionDiagnostics{}, fmt.Errorf("%w: %w", errSourceRejected, err)
	}
	out, err := formatLayout(src, lexed.Tokens, file.MarkupSpans, resolved)
	return out, DecisionDiagnostics{}, err
}

var octFailHeaderPattern = regexp.MustCompile(`^expect (runtime )?error:\s*"(.*)"\s*$`)

func formatOctFailSource(src string, resolved settings) (string, error) {
	lines := strings.Split(src, "\n")
	headerIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		headerIndex = i
		break
	}
	if headerIndex == -1 {
		return "", fmt.Errorf("missing expectation header")
	}
	header := strings.TrimSpace(lines[headerIndex])
	if !octFailHeaderPattern.MatchString(header) {
		return "", fmt.Errorf("malformed expectation header")
	}
	formattedSource, _, err := formatRegularSource("<format>.octfail", strings.Join(lines[headerIndex+1:], "\n"), resolved)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 0; i < headerIndex; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(formattedSource)
	return b.String(), nil
}
func isOctFile(path string) bool {
	_, ok := octExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
func formatFile(path string, options Options) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	formatted, err := formatSourceWithPath(path, string(bytes), options)
	if err != nil {
		// An .octfail may be a contract for a lexer or parser error, and then
		// it cannot be formatted. It is left as it is, so that one such file
		// does not stop a directory from being formatted or checked.
		if errors.Is(err, errSourceRejected) && strings.EqualFold(filepath.Ext(path), ".octfail") {
			return nil
		}
		return fmt.Errorf("format %s: %w", path, err)
	}
	if options.Check {
		if string(bytes) != formatted {
			return fmt.Errorf("%s is not formatted", path)
		}
		return nil
	}
	return os.WriteFile(path, []byte(formatted), 0o644)
}
