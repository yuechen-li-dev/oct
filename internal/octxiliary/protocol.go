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

type Request struct {
	ID                     int
	Family, Function, Path string
}
type Response struct {
	ID          int
	OK          bool
	Text, Error string
}

func WriteHandshake(w io.Writer) error {
	_, e := w.Write(handshakeMagic)
	if e != nil {
		return e
	}
	return binary.Write(w, binary.LittleEndian, []uint16{ABIMajor, ABIMinor})
}
func ReadHandshake(r io.Reader) error {
	m := make([]byte, len(handshakeMagic))
	if _, e := io.ReadFull(r, m); e != nil {
		return e
	}
	if string(m) != string(handshakeMagic) {
		return fmt.Errorf("invalid OCTWRAP handshake")
	}
	var abi [2]uint16
	if e := binary.Read(r, binary.LittleEndian, &abi); e != nil {
		return e
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
	return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q }", req.ID, req.Family, req.Function, req.Path)
}
func EncodeResponse(resp Response) string {
	if resp.OK {
		return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true text: %q }", resp.ID, resp.Text)
	}
	return fmt.Sprintf("OctxiliaryResponse { id: %d ok: false error: %q }", resp.ID, resp.Error)
}

func ParseRequest(s string) (Request, error) {
	return Request{ID: mustInt(field(s, "id")), Family: mustStr(field(s, "family")), Function: mustStr(field(s, "function")), Path: mustStr(field(s, "path"))}, nil
}
func ParseResponse(s string) (Response, error) {
	ok := strings.Contains(s, "ok: true")
	r := Response{ID: mustInt(field(s, "id")), OK: ok}
	if ok {
		r.Text = mustStr(field(s, "text"))
	} else {
		r.Error = mustStr(field(s, "error"))
	}
	return r, nil
}

func field(s, k string) string {
	i := strings.Index(s, k+":")
	if i < 0 {
		return ""
	}
	t := strings.TrimSpace(s[i+len(k)+1:])
	return strings.FieldsFunc(t, func(r rune) bool { return r == '\n' || r == '}' })[0]
}
func mustInt(v string) int { n, _ := strconv.Atoi(strings.Trim(v, ",")); return n }
func mustStr(v string) string {
	x, _ := strconv.Unquote(strings.Trim(strings.TrimSpace(v), ","))
	return x
}

func NewReader(r io.Reader) *bufio.Reader { return bufio.NewReader(r) }
