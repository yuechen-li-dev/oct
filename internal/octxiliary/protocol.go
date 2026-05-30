package octxiliary

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var handshakeMagic = []byte{'O', 'C', 'T', 'W', 'R', 'A', 'P', 0}

const ABIMajor uint16 = 0
const ABIMinor uint16 = 1

type ValueKind string

const (
	ValueVoid         ValueKind = "Void"
	ValueInt          ValueKind = "Int"
	ValueFloat        ValueKind = "Float"
	ValueBool         ValueKind = "Bool"
	ValueString       ValueKind = "String"
	ValueStringArray  ValueKind = "String[]"
	ValueStringMatrix ValueKind = "String[][]"
	ValueBytes        ValueKind = "Bytes"
)

type Value struct {
	Kind     ValueKind
	Int      int
	Float    float64
	Bool     bool
	String   string
	Strings  []string
	Strings2 [][]string
	Bytes    []byte
}

type Request struct {
	ID                     int
	Family, Function, Path string
	Text                   string
	Lines                  []string
	HasLines               bool
	Bytes                  []byte
	HasBytes               bool
	Args                   []Value
	HasArgs                bool
}
type Response struct {
	ID          int
	OK          bool
	Text, Error string
	Lines       []string
	Bytes       []byte
	Exists      bool
	HasExists   bool
	Value       Value
	HasValue    bool
}

func WriteHandshake(w io.Writer) error {
	if _, err := w.Write(handshakeMagic); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, []uint16{ABIMajor, ABIMinor})
}
func ReadHandshake(r io.Reader) error {
	m := make([]byte, len(handshakeMagic))
	if _, err := io.ReadFull(r, m); err != nil {
		return err
	}
	if string(m) != string(handshakeMagic) {
		return fmt.Errorf("invalid OCTWRAP handshake")
	}
	var abi [2]uint16
	if err := binary.Read(r, binary.LittleEndian, &abi); err != nil {
		return err
	}
	if abi[0] != ABIMajor {
		return fmt.Errorf("unsupported OCTWRAP ABI major %d", abi[0])
	}
	return nil
}
func WriteFrame(w io.Writer, body string) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(body))); err != nil {
		return err
	}
	_, err := io.WriteString(w, body)
	return err
}
func ReadFrame(r io.Reader) (string, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}

func EncodeRequest(req Request) string {
	if req.HasArgs {
		return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q args: [ %s ] }", req.ID, req.Family, req.Function, encodeValuesList(req.Args))
	}
	if req.HasBytes {
		return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q bytes: { %s } }", req.ID, req.Family, req.Function, req.Path, encodeBytesList(req.Bytes))
	}
	if req.HasLines {
		return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q lines: { %s } }", req.ID, req.Family, req.Function, req.Path, encodeLinesList(req.Lines))
	}
	if req.Text != "" {
		return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q text: %q }", req.ID, req.Family, req.Function, req.Path, req.Text)
	}
	return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q }", req.ID, req.Family, req.Function, req.Path)
}
func EncodeResponse(resp Response) string {
	if resp.OK {
		if resp.HasValue {
			return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true value: %s }", resp.ID, encodeValue(resp.Value))
		}
		if resp.Bytes != nil {
			return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true bytes: { %s } }", resp.ID, encodeBytesList(resp.Bytes))
		}
		if resp.Lines != nil {
			return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true lines: { %s } }", resp.ID, encodeLinesList(resp.Lines))
		}
		if resp.HasExists {
			return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true exists: %t }", resp.ID, resp.Exists)
		}
		return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true text: %q }", resp.ID, resp.Text)
	}
	return fmt.Sprintf("OctxiliaryResponse { id: %d ok: false error: %q }", resp.ID, resp.Error)
}
func ParseRequest(s string) (Request, error) {
	var req Request
	if parsed, ok, err := parseRequestWithArgs(s); ok {
		if err != nil {
			return Request{}, err
		}
		parsed.HasArgs = true
		return parsed, nil
	}
	if parsed, ok, err := parseRequestWithBytes(s); ok {
		if err != nil {
			return Request{}, err
		}
		parsed.HasBytes = true
		return parsed, nil
	}
	if parsed, ok := parseRequestWithLines(s); ok {
		req = parsed
		req.HasLines = true
		return req, nil
	}
	if _, err := fmt.Sscanf(s, "OctxiliaryRequest { id: %d family: %q function: %q path: %q text: %q }", &req.ID, &req.Family, &req.Function, &req.Path, &req.Text); err == nil {
		return req, nil
	}
	if _, err := fmt.Sscanf(s, "OctxiliaryRequest { id: %d family: %q function: %q path: %q }", &req.ID, &req.Family, &req.Function, &req.Path); err != nil {
		return Request{}, err
	}
	return req, nil
}
func ParseResponse(s string) (Response, error) {
	var r Response
	if parsed, ok, err := parseResponseWithValue(s); ok {
		if err != nil {
			return Response{}, err
		}
		parsed.OK = true
		parsed.HasValue = true
		return parsed, nil
	}
	if parsed, ok, err := parseResponseWithBytes(s); ok {
		if err != nil {
			return Response{}, err
		}
		parsed.OK = true
		return parsed, nil
	}
	if parsed, ok := parseResponseWithLines(s); ok {
		r = parsed
		r.OK = true
		return r, nil
	}
	if _, err := fmt.Sscanf(s, "OctxiliaryResponse { id: %d ok: true text: %q }", &r.ID, &r.Text); err == nil {
		r.OK = true
		return r, nil
	}
	if _, err := fmt.Sscanf(s, "OctxiliaryResponse { id: %d ok: true exists: %t }", &r.ID, &r.Exists); err == nil {
		r.OK = true
		r.HasExists = true
		return r, nil
	}
	if _, err := fmt.Sscanf(s, "OctxiliaryResponse { id: %d ok: false error: %q }", &r.ID, &r.Error); err != nil {
		return Response{}, err
	}
	return r, nil
}
func NewReader(r io.Reader) *bufio.Reader { return bufio.NewReader(r) }

func encodeValuesList(values []Value) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, encodeValue(value))
	}
	return strings.Join(parts, " ")
}

func encodeValue(value Value) string {
	switch value.Kind {
	case ValueVoid:
		return `OctxiliaryValue { kind: "Void" }`
	case ValueInt:
		return fmt.Sprintf(`OctxiliaryValue { kind: "Int" int: %d }`, value.Int)
	case ValueFloat:
		return fmt.Sprintf(`OctxiliaryValue { kind: "Float" float: %s }`, strconv.FormatFloat(value.Float, 'g', -1, 64))
	case ValueBool:
		return fmt.Sprintf(`OctxiliaryValue { kind: "Bool" bool: %t }`, value.Bool)
	case ValueString:
		return fmt.Sprintf(`OctxiliaryValue { kind: "String" string: %q }`, value.String)
	case ValueStringArray:
		return fmt.Sprintf(`OctxiliaryValue { kind: "String[]" strings: [ %s ] }`, encodeLinesList(value.Strings))
	case ValueStringMatrix:
		return fmt.Sprintf(`OctxiliaryValue { kind: "String[][]" strings2: [ %s ] }`, encodeStringMatrixList(value.Strings2))
	case ValueBytes:
		return fmt.Sprintf(`OctxiliaryValue { kind: "Bytes" bytes: { %s } }`, encodeBytesList(value.Bytes))
	default:
		return fmt.Sprintf(`OctxiliaryValue { kind: %q }`, string(value.Kind))
	}
}

func encodeBytesList(bytes []byte) string {
	parts := make([]string, 0, len(bytes))
	for _, b := range bytes {
		parts = append(parts, strconv.Itoa(int(b)))
	}
	return strings.Join(parts, " ")
}

func encodeLinesList(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, strconv.Quote(line))
	}
	return strings.Join(parts, " ")
}

func encodeStringMatrixList(rows [][]string) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("[ %s ]", encodeLinesList(row)))
	}
	return strings.Join(parts, " ")
}

func parseRequestWithArgs(s string) (Request, bool, error) {
	var req Request
	prefix := "OctxiliaryRequest { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") || !strings.Contains(s, " args: [ ") {
		return req, false, nil
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	req.ID, body, err = scanIntThen(body, " family: ")
	if err != nil {
		return req, true, err
	}
	req.Family, body, err = scanQuotedThen(body, " function: ")
	if err != nil {
		return req, true, err
	}
	req.Function, body, err = scanQuotedThen(body, " args: [ ")
	if err != nil {
		return req, true, err
	}
	if !strings.HasSuffix(body, " ]") {
		return req, true, fmt.Errorf("malformed args payload")
	}
	req.Args, err = parseValuesList(strings.TrimSpace(strings.TrimSuffix(body, " ]")))
	if err != nil {
		return req, true, err
	}
	return req, true, nil
}

func parseResponseWithValue(s string) (Response, bool, error) {
	var resp Response
	prefix := "OctxiliaryResponse { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") || !strings.Contains(s, " value: ") {
		return resp, false, nil
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	resp.ID, body, err = scanIntThen(body, " ok: true value: ")
	if err != nil {
		return resp, true, err
	}
	resp.Value, err = parseValue(strings.TrimSpace(body))
	if err != nil {
		return resp, true, err
	}
	return resp, true, nil
}

func parseValuesList(s string) ([]Value, error) {
	values := []Value{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		valueText, next, err := takeValueText(rest)
		if err != nil {
			return nil, err
		}
		value, err := parseValue(valueText)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		rest = strings.TrimSpace(next)
	}
	return values, nil
}

func takeValueText(s string) (string, string, error) {
	prefix := "OctxiliaryValue { "
	if !strings.HasPrefix(s, prefix) {
		return "", "", fmt.Errorf("expected OctxiliaryValue")
	}
	depth := 0
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		if ch == '"' {
			inQuote = true
			continue
		}
		switch ch {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return s[:i+1], s[i+1:], nil
			}
		}
	}
	return "", "", fmt.Errorf("unterminated OctxiliaryValue")
}

func parseValue(s string) (Value, error) {
	prefix := "OctxiliaryValue { kind: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") {
		return Value{}, fmt.Errorf("malformed OctxiliaryValue")
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	kindText, rest, err := scanQuotedRemainder(body)
	if err != nil {
		return Value{}, err
	}
	value := Value{Kind: ValueKind(kindText)}
	rest = strings.TrimSpace(rest)
	switch value.Kind {
	case ValueVoid:
		if rest != "" {
			return Value{}, fmt.Errorf("Void value must not carry payload")
		}
	case ValueInt:
		if !strings.HasPrefix(rest, "int: ") {
			return Value{}, fmt.Errorf("Int value missing int payload")
		}
		value.Int, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(rest, "int: ")))
		if err != nil {
			return Value{}, err
		}
	case ValueFloat:
		if !strings.HasPrefix(rest, "float: ") {
			return Value{}, fmt.Errorf("Float value missing float payload")
		}
		value.Float, err = strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(rest, "float: ")), 64)
		if err != nil {
			return Value{}, err
		}
	case ValueBool:
		if !strings.HasPrefix(rest, "bool: ") {
			return Value{}, fmt.Errorf("Bool value missing bool payload")
		}
		value.Bool, err = strconv.ParseBool(strings.TrimSpace(strings.TrimPrefix(rest, "bool: ")))
		if err != nil {
			return Value{}, err
		}
	case ValueString:
		if !strings.HasPrefix(rest, "string: ") {
			return Value{}, fmt.Errorf("String value missing string payload")
		}
		value.String, err = strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(rest, "string: ")))
		if err != nil {
			return Value{}, err
		}
	case ValueStringArray:
		if !strings.HasPrefix(rest, "strings: [ ") || !strings.HasSuffix(rest, " ]") {
			return Value{}, fmt.Errorf("String[] value missing strings payload")
		}
		value.Strings, err = parseLinesList(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "strings: [ "), " ]")))
		if err != nil {
			return Value{}, err
		}
	case ValueStringMatrix:
		if !strings.HasPrefix(rest, "strings2: [ ") || !strings.HasSuffix(rest, " ]") {
			return Value{}, fmt.Errorf("String[][] value missing strings2 payload")
		}
		value.Strings2, err = parseStringMatrixList(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "strings2: [ "), " ]")))
		if err != nil {
			return Value{}, err
		}
	case ValueBytes:
		if !strings.HasPrefix(rest, "bytes: { ") || !strings.HasSuffix(rest, " }") {
			return Value{}, fmt.Errorf("Bytes value missing bytes payload")
		}
		value.Bytes, err = parseBytesList(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "bytes: { "), " }")))
		if err != nil {
			return Value{}, err
		}
	default:
		return Value{}, fmt.Errorf("unsupported Octxiliary value kind %q", kindText)
	}
	return value, nil
}

func scanQuotedRemainder(s string) (string, string, error) {
	if len(s) == 0 || s[0] != '"' {
		return "", "", fmt.Errorf("missing quote")
	}
	i := 1
	escaped := false
	for i < len(s) {
		ch := s[i]
		if ch == '\\' && !escaped {
			escaped = true
			i++
			continue
		}
		if ch == '"' && !escaped {
			break
		}
		escaped = false
		i++
	}
	if i >= len(s) {
		return "", "", fmt.Errorf("unterminated quote")
	}
	value, err := strconv.Unquote(s[:i+1])
	if err != nil {
		return "", "", err
	}
	return value, s[i+1:], nil
}

func parseRequestWithBytes(s string) (Request, bool, error) {
	var req Request
	prefix := "OctxiliaryRequest { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") || !strings.Contains(s, " bytes: { ") {
		return req, false, nil
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	req.ID, body, err = scanIntThen(body, " family: ")
	if err != nil {
		return req, true, err
	}
	req.Family, body, err = scanQuotedThen(body, " function: ")
	if err != nil {
		return req, true, err
	}
	req.Function, body, err = scanQuotedThen(body, " path: ")
	if err != nil {
		return req, true, err
	}
	req.Path, body, err = scanQuotedThen(body, " bytes: { ")
	if err != nil {
		return req, true, err
	}
	if !strings.HasSuffix(body, " }") {
		return req, true, fmt.Errorf("malformed bytes payload")
	}
	req.Bytes, err = parseBytesList(strings.TrimSuffix(body, " }"))
	if err != nil {
		return req, true, err
	}
	return req, true, nil
}

func parseRequestWithLines(s string) (Request, bool) {
	var req Request
	prefix := "OctxiliaryRequest { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") {
		return req, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	req.ID, body, err = scanIntThen(body, " family: ")
	if err != nil {
		return req, false
	}
	req.Family, body, err = scanQuotedThen(body, " function: ")
	if err != nil {
		return req, false
	}
	req.Function, body, err = scanQuotedThen(body, " path: ")
	if err != nil {
		return req, false
	}
	req.Path, body, err = scanQuotedThen(body, " lines: { ")
	if err != nil || !strings.HasSuffix(body, " }") {
		return req, false
	}
	req.Lines, err = parseLinesList(strings.TrimSuffix(body, " }"))
	if err != nil {
		return req, false
	}
	return req, true
}

func parseResponseWithBytes(s string) (Response, bool, error) {
	var resp Response
	prefix := "OctxiliaryResponse { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") || !strings.Contains(s, " bytes: { ") {
		return resp, false, nil
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	resp.ID, body, err = scanIntThen(body, " ok: true bytes: { ")
	if err != nil {
		return resp, true, err
	}
	if !strings.HasSuffix(body, " }") {
		return resp, true, fmt.Errorf("malformed bytes payload")
	}
	resp.Bytes, err = parseBytesList(strings.TrimSuffix(body, " }"))
	if err != nil {
		return resp, true, err
	}
	return resp, true, nil
}

func parseResponseWithLines(s string) (Response, bool) {
	var resp Response
	prefix := "OctxiliaryResponse { id: "
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, " }") {
		return resp, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, prefix), " }")
	var err error
	resp.ID, body, err = scanIntThen(body, " ok: true lines: { ")
	if err != nil || !strings.HasSuffix(body, " }") {
		return resp, false
	}
	resp.Lines, err = parseLinesList(strings.TrimSuffix(body, " }"))
	if err != nil {
		return resp, false
	}
	return resp, true
}

func scanIntThen(s, delim string) (int, string, error) {
	idx := strings.Index(s, delim)
	if idx < 0 {
		return 0, "", fmt.Errorf("missing delimiter")
	}
	n, err := strconv.Atoi(strings.TrimSpace(s[:idx]))
	if err != nil {
		return 0, "", err
	}
	return n, s[idx+len(delim):], nil
}
func scanQuotedThen(s, delim string) (string, string, error) {
	if len(s) == 0 || s[0] != '"' {
		return "", "", fmt.Errorf("missing quote")
	}
	i := 1
	escaped := false
	for i < len(s) {
		ch := s[i]
		if ch == '\\' && !escaped {
			escaped = true
			i++
			continue
		}
		if ch == '"' && !escaped {
			break
		}
		escaped = false
		i++
	}
	if i >= len(s) {
		return "", "", fmt.Errorf("unterminated quote")
	}
	token := s[:i+1]
	value, err := strconv.Unquote(token)
	if err != nil {
		return "", "", err
	}
	rest := s[i+1:]
	if !strings.HasPrefix(rest, delim) {
		return "", "", fmt.Errorf("missing delimiter")
	}
	return value, rest[len(delim):], nil
}
func parseLinesList(s string) ([]string, error) {
	out := []string{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		v, next, err := scanQuotedThen(rest, " ")
		if err != nil {
			// allow last token without trailing space
			if len(rest) > 0 && rest[0] == '"' {
				i := strings.LastIndex(rest, "\"")
				if i <= 0 {
					return nil, err
				}
				v2, e2 := strconv.Unquote(rest[:i+1])
				if e2 != nil {
					return nil, err
				}
				out = append(out, v2)
				return out, nil
			}
			return nil, err
		}
		out = append(out, v)
		rest = strings.TrimSpace(next)
	}
	return out, nil
}

func parseBytesList(s string) ([]byte, error) {
	out := []byte{}
	rest := strings.TrimSpace(s)
	if rest == "" {
		return out, nil
	}
	for _, field := range strings.Fields(rest) {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, err
		}
		if n < 0 || n > 255 {
			return nil, fmt.Errorf("byte value %d outside [0, 255]", n)
		}
		out = append(out, byte(n))
	}
	return out, nil
}

func parseStringMatrixList(s string) ([][]string, error) {
	rows := [][]string{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		rowText, next, err := takeBracketList(rest)
		if err != nil {
			return nil, fmt.Errorf("malformed String[][] payload: %w", err)
		}
		if !strings.HasPrefix(rowText, "[ ") || !strings.HasSuffix(rowText, " ]") {
			return nil, fmt.Errorf("malformed String[][] row payload")
		}
		row, err := parseLinesList(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rowText, "[ "), " ]")))
		if err != nil {
			return nil, fmt.Errorf("malformed String[][] row payload: %w", err)
		}
		rows = append(rows, row)
		rest = strings.TrimSpace(next)
	}
	return rows, nil
}

func takeBracketList(s string) (string, string, error) {
	if !strings.HasPrefix(s, "[ ") {
		return "", "", fmt.Errorf("expected row list")
	}
	depth := 0
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		if ch == '"' {
			inQuote = true
			continue
		}
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[:i+1], s[i+1:], nil
			}
		}
		if depth < 0 {
			return "", "", fmt.Errorf("unexpected closing bracket")
		}
	}
	return "", "", fmt.Errorf("unterminated row list")
}
