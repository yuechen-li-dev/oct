package octxiliary

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

var handshakeMagic = []byte{'O', 'C', 'T', 'W', 'R', 'A', 'P', 0}

const ABIMajor uint16 = 0
const ABIMinor uint16 = 1

type Request struct {
	ID                     int
	Family, Function, Path string
	Text                   string
}
type Response struct {
	ID          int
	OK          bool
	Text, Error string
	Exists      bool
	HasExists   bool
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
	if req.Text != "" {
		return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q text: %q }", req.ID, req.Family, req.Function, req.Path, req.Text)
	}
	return fmt.Sprintf("OctxiliaryRequest { id: %d family: %q function: %q path: %q }", req.ID, req.Family, req.Function, req.Path)
}
func EncodeResponse(resp Response) string {
	if resp.OK {
		if resp.HasExists {
			return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true exists: %t }", resp.ID, resp.Exists)
		}
		return fmt.Sprintf("OctxiliaryResponse { id: %d ok: true text: %q }", resp.ID, resp.Text)
	}
	return fmt.Sprintf("OctxiliaryResponse { id: %d ok: false error: %q }", resp.ID, resp.Error)
}
func ParseRequest(s string) (Request, error) {
	var req Request
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
