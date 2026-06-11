// Package octxiliary provides the small framed protocol/runtime helpers used by
// Oct native wrapper sidecars.
//
// It is intended for authors exposing Go libraries to Oct through explicit
// Octxiliary sidecar processes declared by Oct package manifests. Sidecars use
// Serve or Main instead of hand-rolling the OCTWRAP handshake, frame loop,
// request parsing, and response encoding.
//
// The package is part of the pre-1.0 Oct toolchain preview, so API details may
// evolve before a stable 1.0 release.
package octxiliary

import (
	"errors"
	"fmt"
	"io"

	internal "github.com/yuechen-li-dev/oct/internal/octxiliary"
)

// ValueKind identifies the transport type carried by a Value.
type ValueKind = internal.ValueKind

const (
	ValueVoid         = internal.ValueVoid
	ValueInt          = internal.ValueInt
	ValueFloat        = internal.ValueFloat
	ValueBool         = internal.ValueBool
	ValueString       = internal.ValueString
	ValueStringArray  = internal.ValueStringArray
	ValueStringMatrix = internal.ValueStringMatrix
	ValueFloatArray   = internal.ValueFloatArray
	ValueBytes        = internal.ValueBytes
	ValueRecord       = internal.ValueRecord
	ValueHandle       = internal.ValueHandle
)

// FieldValue is a named field in an Octxiliary record value.
type FieldValue = internal.FieldValue

// Value is the generic Octxiliary transport value used for request arguments
// and typed responses.
type Value = internal.Value

// Request is the decoded Octxiliary request passed to a sidecar handler.
type Request = internal.Request

// Response is the decoded Octxiliary response returned by a sidecar handler.
type Response = internal.Response

// Handler handles one decoded Octxiliary request and returns one response.
type Handler func(Request) Response

// Serve runs the Octxiliary sidecar protocol over r and w.
//
// Serve reads and validates the host handshake, writes the sidecar handshake,
// then processes length-prefixed request frames until r reaches a clean EOF.
// Malformed request frames are converted into error responses when possible;
// protocol, validation, and IO errors are returned with context.
func Serve(r io.Reader, w io.Writer, handler Handler) error {
	if handler == nil {
		return fmt.Errorf("octxiliary serve: nil handler")
	}
	if err := internal.ReadHandshake(r); err != nil {
		return fmt.Errorf("octxiliary serve: read handshake: %w", err)
	}
	if err := internal.WriteHandshake(w); err != nil {
		return fmt.Errorf("octxiliary serve: write handshake: %w", err)
	}
	for {
		frame, err := internal.ReadFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("octxiliary serve: read frame: %w", err)
		}
		req, err := internal.ParseRequest(frame)
		if err != nil {
			if writeErr := internal.WriteResponseFrame(w, ErrString(req.ID, fmt.Sprintf("parse request: %v", err))); writeErr != nil {
				return fmt.Errorf("octxiliary serve: write parse-error response: %w", writeErr)
			}
			continue
		}
		if err := internal.ValidateRequest(req); err != nil {
			if writeErr := internal.WriteResponseFrame(w, ErrString(req.ID, fmt.Sprintf("validate request: %v", err))); writeErr != nil {
				return fmt.Errorf("octxiliary serve: write validation-error response: %w", writeErr)
			}
			continue
		}
		resp := handler(req)
		if resp.ID == 0 && req.ID != 0 {
			resp.ID = req.ID
		}
		if err := internal.WriteResponseFrame(w, resp); err != nil {
			return fmt.Errorf("octxiliary serve: write response for request %d: %w", req.ID, err)
		}
	}
}

// Main is a small convenience wrapper for sidecar main packages.
// It returns 0 after a clean Serve exit and 1 when Serve returns an error.
func Main(r io.Reader, w io.Writer, handler Handler) int {
	if err := Serve(r, w, handler); err != nil {
		return 1
	}
	return 0
}

// Constructors for transport values.
func VoidValue() Value           { return Value{Kind: ValueVoid} }
func IntValue(v int) Value       { return Value{Kind: ValueInt, Int: v} }
func FloatValue(v float64) Value { return Value{Kind: ValueFloat, Float: v} }
func BoolValue(v bool) Value     { return Value{Kind: ValueBool, Bool: v} }
func StringValue(v string) Value { return Value{Kind: ValueString, String: v} }
func StringsValue(v []string) Value {
	return Value{Kind: ValueStringArray, Strings: append([]string(nil), v...)}
}
func StringMatrixValue(v [][]string) Value {
	return Value{Kind: ValueStringMatrix, Strings2: cloneStringMatrix(v)}
}
func BytesValue(v []byte) Value { return Value{Kind: ValueBytes, Bytes: append([]byte(nil), v...)} }
func FloatsValue(v []float64) Value {
	return Value{Kind: ValueFloatArray, Floats: append([]float64(nil), v...)}
}
func HandleValue(family string, typ string, handleID int) Value {
	return Value{Kind: ValueHandle, HandleFamily: family, HandleType: typ, HandleID: handleID}
}
func RecordValue(recordType string, fields []FieldValue) Value {
	return Value{Kind: ValueRecord, RecordType: recordType, Fields: cloneFields(fields)}
}

func OkValue(id int, v Value) Response             { return Response{ID: id, OK: true, HasValue: true, Value: v} }
func OkVoid(id int) Response                       { return OkValue(id, VoidValue()) }
func OkInt(id int, v int) Response                 { return OkValue(id, IntValue(v)) }
func OkFloat(id int, v float64) Response           { return OkValue(id, FloatValue(v)) }
func OkBool(id int, v bool) Response               { return OkValue(id, BoolValue(v)) }
func OkString(id int, v string) Response           { return OkValue(id, StringValue(v)) }
func OkStrings(id int, v []string) Response        { return OkValue(id, StringsValue(v)) }
func OkStringMatrix(id int, v [][]string) Response { return OkValue(id, StringMatrixValue(v)) }
func OkBytes(id int, v []byte) Response            { return OkValue(id, BytesValue(v)) }
func OkFloats(id int, v []float64) Response        { return OkValue(id, FloatsValue(v)) }
func OkHandle(id int, family string, typ string, handleID int) Response {
	return OkValue(id, HandleValue(family, typ, handleID))
}
func OkRecord(id int, recordType string, fields []FieldValue) Response {
	return OkValue(id, RecordValue(recordType, fields))
}

func Err(id int, err error) Response {
	if err == nil {
		return ErrString(id, "")
	}
	return ErrString(id, err.Error())
}
func ErrString(id int, message string) Response { return Response{ID: id, OK: false, Error: message} }
func ErrUnsupported(id int, function string) Response {
	return ErrString(id, fmt.Sprintf("unsupported function %q", function))
}

// Request argument helpers.
func Arg(req Request, index int) (Value, error) {
	if !req.HasArgs {
		return Value{}, fmt.Errorf("arg %d: request has no generic args", index)
	}
	if index < 0 || index >= len(req.Args) {
		return Value{}, fmt.Errorf("arg %d: index out of range for %d args", index, len(req.Args))
	}
	return req.Args[index], nil
}

func ArgString(req Request, index int) (string, error) {
	value, err := argKind(req, index, ValueString)
	if err != nil {
		return "", err
	}
	return value.String, nil
}
func ArgInt(req Request, index int) (int, error) {
	value, err := argKind(req, index, ValueInt)
	if err != nil {
		return 0, err
	}
	return value.Int, nil
}
func ArgFloat(req Request, index int) (float64, error) {
	value, err := argKind(req, index, ValueFloat)
	if err != nil {
		return 0, err
	}
	return value.Float, nil
}
func ArgBool(req Request, index int) (bool, error) {
	value, err := argKind(req, index, ValueBool)
	if err != nil {
		return false, err
	}
	return value.Bool, nil
}
func ArgBytes(req Request, index int) ([]byte, error) {
	value, err := argKind(req, index, ValueBytes)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), value.Bytes...), nil
}
func ArgStrings(req Request, index int) ([]string, error) {
	value, err := argKind(req, index, ValueStringArray)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), value.Strings...), nil
}
func ArgStringMatrix(req Request, index int) ([][]string, error) {
	value, err := argKind(req, index, ValueStringMatrix)
	if err != nil {
		return nil, err
	}
	return cloneStringMatrix(value.Strings2), nil
}
func ArgFloats(req Request, index int) ([]float64, error) {
	value, err := argKind(req, index, ValueFloatArray)
	if err != nil {
		return nil, err
	}
	return append([]float64(nil), value.Floats...), nil
}
func ArgHandle(req Request, index int, family string, typ string) (int, error) {
	value, err := argKind(req, index, ValueHandle)
	if err != nil {
		return 0, err
	}
	if value.HandleFamily != family {
		return 0, fmt.Errorf("arg %d: expected handle family %q, got %q", index, family, value.HandleFamily)
	}
	if value.HandleType != typ {
		return 0, fmt.Errorf("arg %d: expected handle type %q, got %q", index, typ, value.HandleType)
	}
	if value.HandleID <= 0 {
		return 0, fmt.Errorf("arg %d: handle ID must be positive, got %d", index, value.HandleID)
	}
	return value.HandleID, nil
}
func ArgRecord(req Request, index int, recordType string) ([]FieldValue, error) {
	value, err := argKind(req, index, ValueRecord)
	if err != nil {
		return nil, err
	}
	if value.RecordType != recordType {
		return nil, fmt.Errorf("arg %d: expected record type %q, got %q", index, recordType, value.RecordType)
	}
	return cloneFields(value.Fields), nil
}

func argKind(req Request, index int, kind ValueKind) (Value, error) {
	value, err := Arg(req, index)
	if err != nil {
		return Value{}, err
	}
	if value.Kind != kind {
		return Value{}, fmt.Errorf("arg %d: expected %s, got %s", index, kind, value.Kind)
	}
	return value, nil
}

// Dispatcher routes requests by family and function name.
type Dispatcher struct {
	Family   string
	handlers map[string]Handler
}

func NewDispatcher(family string) *Dispatcher {
	return &Dispatcher{Family: family, handlers: map[string]Handler{}}
}

func (d *Dispatcher) HandleFunc(function string, handler Handler) {
	if d.handlers == nil {
		d.handlers = map[string]Handler{}
	}
	d.handlers[function] = handler
}

func (d *Dispatcher) HandleRequest(req Request) Response {
	if d == nil {
		return ErrString(req.ID, "nil dispatcher")
	}
	if d.Family != "" && req.Family != d.Family {
		return ErrString(req.ID, fmt.Sprintf("unsupported family %q", req.Family))
	}
	handler := d.handlers[req.Function]
	if handler == nil {
		return ErrUnsupported(req.ID, req.Function)
	}
	resp := handler(req)
	if resp.ID == 0 && req.ID != 0 {
		resp.ID = req.ID
	}
	return resp
}

func cloneStringMatrix(v [][]string) [][]string {
	out := make([][]string, len(v))
	for i, row := range v {
		out[i] = append([]string(nil), row...)
	}
	return out
}

func cloneFields(fields []FieldValue) []FieldValue {
	out := make([]FieldValue, len(fields))
	for i, field := range fields {
		out[i] = FieldValue{Name: field.Name, Value: cloneValue(field.Value)}
	}
	return out
}

func cloneValue(value Value) Value {
	cloned := value
	cloned.Strings = append([]string(nil), value.Strings...)
	cloned.Strings2 = cloneStringMatrix(value.Strings2)
	cloned.Floats = append([]float64(nil), value.Floats...)
	cloned.Bytes = append([]byte(nil), value.Bytes...)
	cloned.Fields = cloneFields(value.Fields)
	return cloned
}
