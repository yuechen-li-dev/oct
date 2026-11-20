// Package lsp implements the Oct language server (`oct lsp`).
//
// LSP-M0 scope: diagnostics, whole-document formatting, semantic tokens and
// document symbols. The server speaks JSON-RPC 2.0 over stdio with the
// standard Content-Length framing and processes messages sequentially.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

type conn struct {
	in  *bufio.Reader
	out io.Writer
	mu  sync.Mutex
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{in: bufio.NewReader(r), out: w}
}

// read returns the next framed message. io.EOF means the client went away.
func (c *conn) read() (message, error) {
	tp := textproto.NewReader(c.in)
	header, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF || strings.Contains(err.Error(), "EOF") {
			return message{}, io.EOF
		}
		return message{}, fmt.Errorf("read header: %w", err)
	}
	length, err := strconv.Atoi(header.Get("Content-Length"))
	if err != nil || length < 0 {
		return message{}, fmt.Errorf("invalid Content-Length %q", header.Get("Content-Length"))
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.in, body); err != nil {
		return message{}, fmt.Errorf("read body: %w", err)
	}
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		return message{}, fmt.Errorf("decode message: %w", err)
	}
	return msg, nil
}

func (c *conn) write(msg message) error {
	msg.JSONRPC = "2.0"
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.out.Write(body)
	return err
}

func (c *conn) reply(id json.RawMessage, result any) error {
	if result == nil {
		// A request must carry a result member; JSON null is the empty result.
		return c.writeRaw(id, json.RawMessage("null"))
	}
	return c.write(message{ID: id, Result: result})
}

func (c *conn) writeRaw(id json.RawMessage, result json.RawMessage) error {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, id, result)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := io.WriteString(c.out, body)
	return err
}

func (c *conn) replyError(id json.RawMessage, code int, text string) error {
	return c.write(message{ID: id, Error: &responseError{Code: code, Message: text}})
}

func (c *conn) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(message{Method: method, Params: raw})
}
