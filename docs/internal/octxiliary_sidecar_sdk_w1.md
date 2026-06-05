# W1 Public Octxiliary Sidecar SDK

## Why the public SDK exists

Octxiliary is the sidecar protocol used by compiled Oct programs when they call native wrapper sidecars. The wire protocol already covers the OCTWRAP handshake, ABI version negotiation, length-prefixed frames, typed request/response values, family/function dispatch, and generic transport values for scalars, arrays, bytes, records, and handles.

Before W1, a sidecar author had to hand-roll too much protocol code:

- read the host handshake and write the sidecar handshake;
- read and write length-prefixed frames;
- parse `OctxiliaryRequest` payloads;
- construct and validate `OctxiliaryResponse` payloads;
- distinguish clean EOF from protocol/IO failure;
- route by family/function;
- decode typed request arguments.

W1 adds `github.com/yuechen-li-dev/oct/pkg/octxiliary`, a public Go package for sidecar authors. Third-party packages can import this package and write only business logic.

## Why not import `internal/octxiliary` directly?

`internal/octxiliary` remains the repository's implementation package for the protocol. Go's `internal/` import rule prevents code outside the parent tree from importing it, so a third-party sidecar such as `oct-opencv`, `oct-postgres`, or `oct-hdf5` cannot depend on `github.com/yuechen-li-dev/oct/internal/octxiliary`.

The public SDK package wraps and aliases stable protocol-facing types from the internal package while keeping third-party source code outside `internal/`.

## Minimal sidecar example

```go
package main

import (
    "os"

    "github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func main() {
    dispatcher := octxiliary.NewDispatcher("Echo")

    dispatcher.HandleFunc("EchoString", func(req octxiliary.Request) octxiliary.Response {
        text, err := octxiliary.ArgString(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkString(req.ID, text)
    })

    dispatcher.HandleFunc("ByteLength", func(req octxiliary.Request) octxiliary.Response {
        data, err := octxiliary.ArgBytes(req, 0)
        if err != nil {
            return octxiliary.Err(req.ID, err)
        }
        return octxiliary.OkInt(req.ID, len(data))
    })

    os.Exit(octxiliary.Main(os.Stdin, os.Stdout, dispatcher.HandleRequest))
}
```

Unknown functions routed through the dispatcher return an `ErrUnsupported` response automatically.

## Serve loop behavior

The core API is:

```go
func Serve(r io.Reader, w io.Writer, handler Handler) error

type Handler func(Request) Response
```

`Serve` performs the protocol boilerplate for a sidecar:

1. read and validate the host OCTWRAP handshake;
2. write the sidecar OCTWRAP handshake;
3. read request frames until clean EOF;
4. parse each request;
5. validate generic transport argument values;
6. call the handler;
7. validate and encode the response;
8. write the response frame.

Clean EOF after the handshake returns `nil`. Protocol, validation, and IO failures return errors with `octxiliary serve:` context. Ordinary malformed request payloads are reported to the host as error responses when a response frame can still be written.

`Main(r, w, handler) int` is a convenience wrapper around `Serve`: it returns `0` for a clean exit and `1` for an error. It does not call `os.Exit` itself.

## Public transport types

The SDK exposes stable sidecar-authoring aliases for the protocol-facing types:

- `Request`
- `Response`
- `Value`
- `ValueKind`
- `FieldValue`

It also exposes the current value-kind constants:

- `ValueVoid`
- `ValueInt`
- `ValueFloat`
- `ValueBool`
- `ValueString`
- `ValueStringArray`
- `ValueStringMatrix`
- `ValueFloatArray`
- `ValueBytes`
- `ValueRecord`
- `ValueHandle`

These are aliases over the internal implementation types, but sidecar authors import only `pkg/octxiliary`.

## Response constructors

Handlers should prefer constructors over manually filling response fields:

```go
func OkValue(id int, v Value) Response
func OkVoid(id int) Response
func OkInt(id int, v int) Response
func OkFloat(id int, v float64) Response
func OkBool(id int, v bool) Response
func OkString(id int, v string) Response
func OkStrings(id int, v []string) Response
func OkStringMatrix(id int, v [][]string) Response
func OkBytes(id int, v []byte) Response
func OkFloats(id int, v []float64) Response
func OkHandle(id int, family string, typ string, handleID int) Response
func OkRecord(id int, recordType string, fields []FieldValue) Response

func Err(id int, err error) Response
func ErrString(id int, message string) Response
func ErrUnsupported(id int, function string) Response
```

The package also provides value constructors such as `StringValue`, `BytesValue`, `FloatsValue`, `HandleValue`, and `RecordValue` for nested record fields or tests.

## Request argument helpers

Sidecar handlers should use argument helpers instead of indexing `Request.Args` directly:

```go
func Arg(req Request, index int) (Value, error)
func ArgString(req Request, index int) (string, error)
func ArgInt(req Request, index int) (int, error)
func ArgFloat(req Request, index int) (float64, error)
func ArgBool(req Request, index int) (bool, error)
func ArgBytes(req Request, index int) ([]byte, error)
func ArgStrings(req Request, index int) ([]string, error)
func ArgStringMatrix(req Request, index int) ([][]string, error)
func ArgFloats(req Request, index int) ([]float64, error)
func ArgHandle(req Request, index int, family string, typ string) (int, error)
func ArgRecord(req Request, index int, recordType string) ([]FieldValue, error)
```

The helpers validate that generic arguments are present, the index is in bounds, and the argument kind matches the expected transport kind. Handle helpers also validate family, type, and a positive handle ID. Slice and byte helpers return defensive copies so handlers do not accidentally mutate the decoded request.

## Dispatcher

The W1 dispatcher is intentionally small:

```go
func NewDispatcher(family string) *Dispatcher
func (d *Dispatcher) HandleFunc(function string, handler Handler)
func (d *Dispatcher) HandleRequest(req Request) Response
```

It validates the request family when a family name is configured, routes by `req.Function`, and returns `ErrUnsupported` for unknown functions. It does not use reflection and does not try to infer bindings from Go function signatures.

## Relationship to future third-party wrapper work

This SDK only improves the sidecar authoring layer. It is a prerequisite for a future third-party wrapper path because external packages need a public package to import, but W1 does not define how packages are discovered, built, installed, or selected by compiled Oct programs.

Future work may add manifest dispatch, package-manager build lifecycle integration, registry or federation concepts, and generated bindings. Those layers should build on top of the public SDK instead of asking sidecar authors to reimplement the wire protocol.

## Explicit non-goals

W1 does not:

- add `@extern`;
- change Oct language syntax;
- change the Octxiliary wire protocol;
- implement third-party wrapper manifest dispatch;
- implement interpreted generic wrapper dispatch;
- implement package-manager wrapper build lifecycle;
- implement registry, federation, or P2P distribution;
- change PATH-based sidecar discovery;
- change wrapper manifests;
- add native permission prompts;
- generate lockfiles;
- refactor all standard-library sidecars.
