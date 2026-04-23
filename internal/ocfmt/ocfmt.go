package ocfmt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
)

var octExtensions = map[string]struct{}{
	".oct":     {},
	".octest":  {},
	".octfail": {},
}

func FormatPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(path, func(current string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !isOctFile(current) {
				return nil
			}
			return formatFile(current)
		})
	}
	if !isOctFile(path) {
		return fmt.Errorf("unsupported file extension: %s", path)
	}
	return formatFile(path)
}

func FormatSource(src string) (string, error) {
	return formatSourceWithPath("<format>.oct", src)
}

func formatSourceWithPath(path string, src string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".octfail") {
		return formatOctFailSource(src)
	}
	return formatRegularSource(path, src)
}

func formatRegularSource(path string, src string) (string, error) {
	lexed, err := lex.Analyze(source.File{Path: path, Text: src})
	if err != nil {
		return "", err
	}
	if _, err := parse.BuildFile(lexed); err != nil {
		return "", err
	}
	return normalize(src), nil
}

func isOctFile(path string) bool {
	_, ok := octExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func formatFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	formatted, err := formatSourceWithPath(path, string(bytes))
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(formatted), 0o644)
}

var octFailHeaderPattern = regexp.MustCompile(`^expect error:\s*"(.*)"\s*$`)

func formatOctFailSource(src string) (string, error) {
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

	formattedSource, err := formatRegularSource("<format>.octfail", strings.Join(lines[headerIndex+1:], "\n"))
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

func normalize(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	indent := 0
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(trimmed, "}") {
			if indent > 0 {
				indent--
			}
		}
		if strings.HasPrefix(trimmed, "//") {
			out = append(out, strings.Repeat("    ", indent)+trimmed)
			continue
		}
		code, comment := splitCodeAndComment(trimmed)
		normCode := normalizeCode(code)
		line := strings.Repeat("    ", indent) + normCode
		if comment != "" {
			if normCode != "" {
				line += " "
			}
			line += comment
		}
		out = append(out, line)
		if strings.HasSuffix(normCode, "{") {
			indent++
		}
	}
	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

func splitCodeAndComment(line string) (string, string) {
	inString := false
	escaped := false
	for i := 0; i < len(line)-1; i++ {
		c := line[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '/' && line[i+1] == '/' {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i:])
		}
	}
	return strings.TrimSpace(line), ""
}

func normalizeCode(code string) string {
	tokens := scanTokens(code)
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tok := range tokens {
		if tok == "=>" {
			tok = "->"
		}
		prev := ""
		if i > 0 {
			prev = tokens[i-1]
			if prev == "=>" {
				prev = "->"
			}
		}
		if i > 0 && needsSpace(prev, tok) {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
	}
	return b.String()
}

func scanTokens(code string) []string {
	var tokens []string
	for i := 0; i < len(code); {
		c := code[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if c == '"' {
			j := i + 1
			escaped := false
			for j < len(code) {
				if escaped {
					escaped = false
					j++
					continue
				}
				if code[j] == '\\' {
					escaped = true
					j++
					continue
				}
				if code[j] == '"' {
					j++
					break
				}
				j++
			}
			tokens = append(tokens, code[i:j])
			i = j
			continue
		}
		if i+1 < len(code) {
			two := code[i : i+2]
			switch two {
			case "->", "=>", "==", "!=", "<=", ">=", "..":
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}
		switch c {
		case '(', ')', '{', '}', '[', ']', ',', ':', '.', '+', '-', '*', '/', '=', '<', '>', '?', '!', '^', '@':
			tokens = append(tokens, string(c))
			i++
			continue
		}
		j := i
		for j < len(code) {
			ch := code[j]
			if ch == ' ' || ch == '\t' || strings.ContainsRune("(){}[],:.+-*/=<>?!^@", rune(ch)) {
				break
			}
			j++
		}
		tokens = append(tokens, code[i:j])
		i = j
	}
	return tokens
}

func needsSpace(prev string, curr string) bool {
	if prev == "" || curr == "" {
		return false
	}
	if curr == "," || curr == ")" || curr == "]" || curr == "}" || curr == "." || curr == ":" {
		return false
	}
	if prev == "(" || prev == "[" || prev == "{" || prev == "." {
		return false
	}
	if curr == "(" {
		return prev == "if" || prev == "switch" || prev == "match" || prev == "while" || prev == "for" || prev == "state" || prev == "when"
	}
	if curr == "[" {
		return false
	}
	if isOperator(prev) || isOperator(curr) {
		if curr == "!" || curr == "?" || prev == "!" || prev == "?" {
			return false
		}
		return true
	}
	return true
}

func isOperator(tok string) bool {
	switch tok {
	case "+", "-", "*", "/", "%", "=", "==", "!=", "<=", ">=", "<", ">", "->", "=>", "..", "^", "and", "or", "not":
		return true
	default:
		return false
	}
}
