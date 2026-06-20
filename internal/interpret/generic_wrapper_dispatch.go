package interpret

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yuechen-li-dev/oct/internal/manifestwrapper"
	"github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/internal/project"
)

type interpretedWrapperFunction struct {
	PackageName    string
	WrapperName    string
	Family         string
	SidecarCommand string
	Protocol       string
	TransportTypes []project.TransportTypeMetadata
	OctName        string
	WireName       string
	Args           []string
	Return         string
	Fallible       bool
}

type interpretedWrapperIndex struct {
	byPackage map[string]map[string]interpretedWrapperFunction
}

func buildInterpretedWrapperIndex(program project.Program) (interpretedWrapperIndex, error) {
	index := interpretedWrapperIndex{byPackage: map[string]map[string]interpretedWrapperFunction{}}
	for pkgName, pkg := range program.Packages {
		for _, wrapper := range pkg.Wrappers {
			if wrapper.Protocol != manifestwrapper.SupportedProtocol {
				continue
			}
			for _, fn := range wrapper.Functions {
				pkgIndex := index.byPackage[pkgName]
				if pkgIndex == nil {
					pkgIndex = map[string]interpretedWrapperFunction{}
					index.byPackage[pkgName] = pkgIndex
				}
				if prior, exists := pkgIndex[fn.OctName]; exists {
					return interpretedWrapperIndex{}, fmt.Errorf("interpreted wrapper dispatch duplicate raw function %s.%s in wrappers %s and %s", pkgName, fn.OctName, prior.WrapperName, wrapper.Name)
				}
				pkgIndex[fn.OctName] = interpretedWrapperFunction{
					PackageName:    pkgName,
					WrapperName:    wrapper.Name,
					Family:         wrapper.Family,
					SidecarCommand: wrapper.SidecarCommand,
					Protocol:       wrapper.Protocol,
					TransportTypes: append([]project.TransportTypeMetadata(nil), wrapper.TransportTypes...),
					OctName:        fn.OctName,
					WireName:       fn.WireName,
					Args:           append([]string(nil), fn.Args...),
					Return:         fn.Return,
					Fallible:       fn.Fallible,
				}
			}
		}
	}
	return index, nil
}

func (idx interpretedWrapperIndex) lookup(packageName, octName string) (interpretedWrapperFunction, bool) {
	pkgIndex, ok := idx.byPackage[packageName]
	if !ok {
		return interpretedWrapperFunction{}, false
	}
	fn, ok := pkgIndex[octName]
	return fn, ok
}

type interpretedWrapperClientCache struct {
	mu      sync.Mutex
	clients map[string]*interpretedWrapperClient
}

type interpretedWrapperClient struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    io.ReadCloser
	mu     sync.Mutex
	reqID  int
	err    error
	closed bool
}

func newInterpretedWrapperClientCache() *interpretedWrapperClientCache {
	return &interpretedWrapperClientCache{clients: map[string]*interpretedWrapperClient{}}
}

func (cache *interpretedWrapperClientCache) close() {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	clients := make([]*interpretedWrapperClient, 0, len(cache.clients))
	for _, client := range cache.clients {
		clients = append(clients, client)
	}
	cache.clients = map[string]*interpretedWrapperClient{}
	cache.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func (client *interpretedWrapperClient) close() {
	if client == nil {
		return
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return
	}
	client.closed = true
	if client.in != nil {
		_ = client.in.Close()
		client.in = nil
	}
	if client.cmd != nil && client.cmd.Process != nil {
		waitDone := make(chan error, 1)
		go func(cmd *exec.Cmd) { waitDone <- cmd.Wait() }(client.cmd)
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
			_ = client.cmd.Process.Kill()
			<-waitDone
		}
		client.cmd = nil
	}
	if client.out != nil {
		_ = client.out.Close()
		client.out = nil
	}
}

func (cache *interpretedWrapperClientCache) genericCall(fn interpretedWrapperFunction, args []octxiliary.Value, expected octxiliary.ValueKind) (octxiliary.Value, error) {
	client := cache.clientFor(fn)
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err != nil {
		return octxiliary.Value{}, client.err
	}
	client.reqID++
	req := octxiliary.Request{ID: client.reqID, Family: fn.Family, Function: fn.WireName, Args: args, HasArgs: true}
	if err := octxiliary.ValidateRequest(req); err != nil {
		return octxiliary.Value{}, err
	}
	if err := octxiliary.WriteFrame(client.in, octxiliary.EncodeRequest(req)); err != nil {
		client.err = err
		return octxiliary.Value{}, err
	}
	frame, err := octxiliary.ReadFrame(client.out)
	if err != nil {
		client.err = err
		return octxiliary.Value{}, err
	}
	resp, err := octxiliary.ParseResponse(frame)
	if err != nil {
		client.err = err
		return octxiliary.Value{}, err
	}
	if err := octxiliary.ValidateResponse(resp); err != nil {
		client.err = err
		return octxiliary.Value{}, err
	}
	if !resp.OK {
		return octxiliary.Value{}, errors.New(resp.Error)
	}
	if !resp.HasValue {
		return octxiliary.Value{}, errors.New("Octxiliary generic response missing typed value")
	}
	if resp.Value.Kind != expected {
		return octxiliary.Value{}, fmt.Errorf("Octxiliary generic response kind mismatch: expected %s, got %s", expected, resp.Value.Kind)
	}
	return resp.Value, nil
}

func (cache *interpretedWrapperClientCache) clientFor(fn interpretedWrapperFunction) *interpretedWrapperClient {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if client, ok := cache.clients[fn.SidecarCommand]; ok {
		return client
	}
	client := &interpretedWrapperClient{}
	path, err := interpretedWrapperSidecarPath(fn)
	if err != nil {
		client.err = err
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	cmd := exec.Command(path)
	in, inErr := cmd.StdinPipe()
	out, outErr := cmd.StdoutPipe()
	if inErr != nil {
		client.err = inErr
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	if outErr != nil {
		client.err = outErr
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	if err := cmd.Start(); err != nil {
		client.err = err
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	client.cmd, client.in, client.out = cmd, in, out
	if err := octxiliary.WriteHandshake(in); err != nil {
		client.err = err
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	if err := octxiliary.ReadHandshake(out); err != nil {
		client.err = err
		cache.clients[fn.SidecarCommand] = client
		return client
	}
	cache.clients[fn.SidecarCommand] = client
	return client
}

func interpretedWrapperSidecarPath(fn interpretedWrapperFunction) (string, error) {
	command := fn.SidecarCommand
	if command == "" {
		return "", interpretedWrapperMissingSidecarError(fn)
	}
	if exe, err := os.Executable(); err == nil {
		if path, ok := resolveSidecarInDir(filepath.Dir(exe), command, runtime.GOOS); ok {
			return path, nil
		}
	}
	wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
	if wrapperPath != "" {
		if path, ok := resolveSidecarFromWrapperPath(wrapperPath, command, runtime.GOOS); ok {
			return path, nil
		}
	}
	return "", interpretedWrapperMissingSidecarError(fn)
}

func resolveSidecarFromWrapperPath(wrapperPath string, command string, goos string) (string, bool) {
	info, err := os.Stat(wrapperPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return resolveSidecarInDir(wrapperPath, command, goos)
	}
	if isSidecarCommandBasename(filepath.Base(wrapperPath), command, goos) && isExecutableFile(wrapperPath) {
		return wrapperPath, true
	}
	return "", false
}

func resolveSidecarInDir(dir string, command string, goos string) (string, bool) {
	for _, name := range sidecarCommandCandidates(command, goos) {
		candidate := filepath.Join(dir, name)
		if isExecutableFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func sidecarCommandCandidates(command string, goos string) []string {
	if goos == "windows" && !strings.HasSuffix(strings.ToLower(command), ".exe") {
		return []string{command, command + ".exe"}
	}
	return []string{command}
}

func isSidecarCommandBasename(base string, command string, goos string) bool {
	for _, candidate := range sidecarCommandCandidates(command, goos) {
		if base == candidate {
			return true
		}
	}
	return false
}

func interpretedWrapperMissingSidecarError(fn interpretedWrapperFunction) error {
	return fmt.Errorf("wrapper %s.%s (family %s, wire %s): Octxiliary sidecar %q not found; set OCT_WRAPPER_PATH or place it beside the oct executable", fn.PackageName, fn.OctName, fn.Family, fn.WireName, fn.SidecarCommand)
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (i interpreter) evalGenericWrapperCall(fn interpretedWrapperFunction, arguments []Value) (evalResult, error) {
	args, err := packInterpretedWrapperArgs(fn, arguments)
	if err != nil {
		return i.wrapperBoundaryResult(fn, err)
	}
	expected, err := octxiliaryKindForInterpretedType(fn.Return, fn.TransportTypes)
	if err != nil {
		return i.wrapperBoundaryResult(fn, err)
	}
	value, err := i.wrapperClients.genericCall(fn, args, expected)
	if err != nil {
		return i.wrapperBoundaryResult(fn, err)
	}
	out, err := unpackInterpretedWrapperReturn(fn, value)
	if err != nil {
		return i.wrapperBoundaryResult(fn, err)
	}
	return evalResult{value: out}, nil
}

func (i interpreter) wrapperBoundaryResult(fn interpretedWrapperFunction, err error) (evalResult, error) {
	if fn.Fallible {
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: fmt.Sprintf("%s: %s", fn.OctName, err.Error())}}}, nil
	}
	return evalResult{}, fmt.Errorf("runtime error: wrapper %s.%s boundary failure: %w", fn.PackageName, fn.OctName, err)
}

func packInterpretedWrapperArgs(fn interpretedWrapperFunction, arguments []Value) ([]octxiliary.Value, error) {
	if len(arguments) != len(fn.Args) {
		return nil, fmt.Errorf("wrapper %s.%s expects %d arguments, got %d", fn.PackageName, fn.OctName, len(fn.Args), len(arguments))
	}
	out := make([]octxiliary.Value, 0, len(arguments))
	for idx, arg := range arguments {
		packed, err := packInterpretedWrapperValue(fn, fn.Args[idx], arg)
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", idx+1, err)
		}
		out = append(out, packed)
	}
	return out, nil
}

func packInterpretedWrapperValue(fn interpretedWrapperFunction, typ string, value Value) (octxiliary.Value, error) {
	if strings.HasPrefix(typ, "Int<") && strings.HasSuffix(typ, ">") {
		typ = "Int"
	}
	switch typ {
	case "Void":
		return octxiliary.Value{Kind: octxiliary.ValueVoid}, nil
	case "Int":
		if value.Kind != ValueInt {
			return octxiliary.Value{}, fmt.Errorf("expects Int, got %s", value.Kind)
		}
		return octxiliary.Value{Kind: octxiliary.ValueInt, Int: int(value.Int)}, nil
	case "Float":
		if value.Kind != ValueFloat {
			return octxiliary.Value{}, fmt.Errorf("expects Float, got %s", value.Kind)
		}
		return octxiliary.Value{Kind: octxiliary.ValueFloat, Float: value.Float}, nil
	case "Bool":
		if value.Kind != ValueBool {
			return octxiliary.Value{}, fmt.Errorf("expects Bool, got %s", value.Kind)
		}
		return octxiliary.Value{Kind: octxiliary.ValueBool, Bool: value.Bool}, nil
	case "String":
		if value.Kind != ValueString {
			return octxiliary.Value{}, fmt.Errorf("expects String, got %s", value.Kind)
		}
		return octxiliary.Value{Kind: octxiliary.ValueString, String: value.Text}, nil
	case "Bytes":
		if value.Kind != ValueBytes {
			return octxiliary.Value{}, fmt.Errorf("expects Bytes, got %s", value.Kind)
		}
		return octxiliary.Value{Kind: octxiliary.ValueBytes, Bytes: append([]byte(nil), value.Bytes...)}, nil
	case "String[]":
		stringsValue, err := packStringArray(value)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueStringArray, Strings: stringsValue}, nil
	case "String[][]":
		stringsValue, err := packStringMatrix(value)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueStringMatrix, Strings2: stringsValue}, nil
	case "Float[]":
		floats, err := packFloatArray(value)
		if err != nil {
			return octxiliary.Value{}, err
		}
		return octxiliary.Value{Kind: octxiliary.ValueFloatArray, Floats: floats}, nil
	default:
		transport, ok := findInterpretedTransport(fn.TransportTypes, typ)
		if !ok {
			return octxiliary.Value{}, fmt.Errorf("unsupported transport type %s", typ)
		}
		if transport.Kind == "handle" {
			return packInterpretedHandle(fn, transport, value)
		}
		return packInterpretedRecord(fn, transport, value)
	}
}

func packStringArray(value Value) ([]string, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("expects String[], got %s", value.Kind)
	}
	out := make([]string, 0, len(value.Array))
	for idx, element := range value.Array {
		if element.Kind != ValueString {
			return nil, fmt.Errorf("expects String[] element %d to be String, got %s", idx, element.Kind)
		}
		out = append(out, element.Text)
	}
	return out, nil
}

func packStringMatrix(value Value) ([][]string, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("expects String[][], got %s", value.Kind)
	}
	out := make([][]string, 0, len(value.Array))
	for idx, row := range value.Array {
		stringsValue, err := packStringArray(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", idx, err)
		}
		out = append(out, stringsValue)
	}
	return out, nil
}

func packFloatArray(value Value) ([]float64, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("expects Float[], got %s", value.Kind)
	}
	out := make([]float64, 0, len(value.Array))
	for idx, element := range value.Array {
		if element.Kind != ValueFloat {
			return nil, fmt.Errorf("expects Float[] element %d to be Float, got %s", idx, element.Kind)
		}
		out = append(out, element.Float)
	}
	return out, nil
}

func packInterpretedHandle(fn interpretedWrapperFunction, transport project.TransportTypeMetadata, value Value) (octxiliary.Value, error) {
	if value.Kind != ValueRecord {
		return octxiliary.Value{}, fmt.Errorf("expects handle record %s, got %s", transport.Name, value.Kind)
	}
	if value.Record.TypeName != transport.Name {
		return octxiliary.Value{}, fmt.Errorf("expects handle record %s, got %s", transport.Name, value.Record.TypeName)
	}
	handle, ok := value.Record.Fields["Handle"]
	if !ok {
		return octxiliary.Value{}, fmt.Errorf("handle record %s missing Handle field", transport.Name)
	}
	if handle.Kind != ValueInt {
		return octxiliary.Value{}, fmt.Errorf("handle record %s Handle expects Int, got %s", transport.Name, handle.Kind)
	}
	if handle.Int <= 0 {
		return octxiliary.Value{}, fmt.Errorf("handle record %s Handle must be positive", transport.Name)
	}
	return octxiliary.Value{Kind: octxiliary.ValueHandle, HandleFamily: fn.Family, HandleType: transport.Name, HandleID: int(handle.Int)}, nil
}

func packInterpretedRecord(fn interpretedWrapperFunction, transport project.TransportTypeMetadata, value Value) (octxiliary.Value, error) {
	if value.Kind != ValueRecord {
		return octxiliary.Value{}, fmt.Errorf("expects record %s, got %s", transport.Name, value.Kind)
	}
	if value.Record.TypeName != transport.Name {
		return octxiliary.Value{}, fmt.Errorf("expects record %s, got %s", transport.Name, value.Record.TypeName)
	}
	fields := make([]octxiliary.FieldValue, 0, len(transport.Fields))
	for _, field := range transport.Fields {
		fieldValue, ok := value.Record.Fields[field.Name]
		if !ok {
			return octxiliary.Value{}, fmt.Errorf("record %s missing field %s", transport.Name, field.Name)
		}
		packed, err := packInterpretedWrapperValue(fn, field.Type, fieldValue)
		if err != nil {
			return octxiliary.Value{}, fmt.Errorf("record %s field %s: %w", transport.Name, field.Name, err)
		}
		fields = append(fields, octxiliary.FieldValue{Name: field.Name, Value: packed})
	}
	return octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: transport.Name, Fields: fields}, nil
}

func unpackInterpretedWrapperReturn(fn interpretedWrapperFunction, value octxiliary.Value) (Value, error) {
	typ := fn.Return
	if strings.HasPrefix(typ, "Int<") && strings.HasSuffix(typ, ">") {
		typ = "Int"
	}
	switch typ {
	case "Void":
		return Value{Kind: ValueInt, Int: 0}, nil
	case "Int":
		return Value{Kind: ValueInt, Int: int64(value.Int)}, nil
	case "Float":
		return Value{Kind: ValueFloat, Float: value.Float}, nil
	case "Bool":
		return Value{Kind: ValueBool, Bool: value.Bool}, nil
	case "String":
		return Value{Kind: ValueString, Text: value.String}, nil
	case "Bytes":
		return Value{Kind: ValueBytes, Bytes: append([]byte(nil), value.Bytes...)}, nil
	case "String[]":
		return wrapperStringArrayValue(value.Strings), nil
	case "String[][]":
		rows := make([]Value, 0, len(value.Strings2))
		for _, row := range value.Strings2 {
			rows = append(rows, wrapperStringArrayValue(row))
		}
		return Value{Kind: ValueArray, Array: rows}, nil
	case "Float[]":
		values := make([]Value, 0, len(value.Floats))
		for _, f := range value.Floats {
			values = append(values, Value{Kind: ValueFloat, Float: f})
		}
		return Value{Kind: ValueArray, Array: values}, nil
	default:
		transport, ok := findInterpretedTransport(fn.TransportTypes, typ)
		if !ok {
			return Value{}, fmt.Errorf("unsupported transport return type %s", typ)
		}
		if transport.Kind != "handle" {
			if value.Kind != octxiliary.ValueRecord {
				return Value{}, fmt.Errorf("Octxiliary record response type mismatch: expected %s, got %s", typ, value.Kind)
			}
			if value.RecordType != typ {
				return Value{}, fmt.Errorf("Octxiliary record response type mismatch: expected %s, got %s", typ, value.RecordType)
			}
			fields := make(map[string]Value, len(transport.Fields))
			order := make([]string, 0, len(transport.Fields))
			byName := make(map[string]octxiliary.Value, len(value.Fields))
			for _, field := range value.Fields {
				byName[field.Name] = field.Value
			}
			for _, field := range transport.Fields {
				raw, ok := byName[field.Name]
				if !ok {
					return Value{}, fmt.Errorf("Octxiliary record response %s missing field %s", typ, field.Name)
				}
				unpacked, err := unpackInterpretedWrapperReturn(interpretedWrapperFunction{Return: field.Type, TransportTypes: fn.TransportTypes, Family: fn.Family}, raw)
				if err != nil {
					return Value{}, fmt.Errorf("Octxiliary record response %s field %s: %w", typ, field.Name, err)
				}
				fields[field.Name] = unpacked
				order = append(order, field.Name)
			}
			return Value{Kind: ValueRecord, Record: RecordValue{TypeName: typ, FieldOrder: order, Fields: fields}}, nil
		}
		if value.HandleFamily != fn.Family {
			return Value{}, fmt.Errorf("Octxiliary handle response family mismatch: expected %s, got %s", fn.Family, value.HandleFamily)
		}
		if value.HandleType != typ {
			return Value{}, fmt.Errorf("Octxiliary handle response type mismatch: expected %s, got %s", typ, value.HandleType)
		}
		if value.HandleID <= 0 {
			return Value{}, fmt.Errorf("Octxiliary handle response ID must be positive")
		}
		return Value{Kind: ValueRecord, Record: RecordValue{TypeName: typ, FieldOrder: []string{"Handle"}, Fields: map[string]Value{"Handle": {Kind: ValueInt, Int: int64(value.HandleID)}}}}, nil
	}
}

func octxiliaryKindForInterpretedType(typ string, transports []project.TransportTypeMetadata) (octxiliary.ValueKind, error) {
	if strings.HasPrefix(typ, "Int<") && strings.HasSuffix(typ, ">") {
		return octxiliary.ValueInt, nil
	}
	if transport, ok := findInterpretedTransport(transports, typ); ok {
		if transport.Kind == "handle" {
			return octxiliary.ValueHandle, nil
		}
		return octxiliary.ValueRecord, nil
	}
	switch typ {
	case "Void":
		return octxiliary.ValueVoid, nil
	case "Int":
		return octxiliary.ValueInt, nil
	case "Float":
		return octxiliary.ValueFloat, nil
	case "Bool":
		return octxiliary.ValueBool, nil
	case "String":
		return octxiliary.ValueString, nil
	case "String[]":
		return octxiliary.ValueStringArray, nil
	case "String[][]":
		return octxiliary.ValueStringMatrix, nil
	case "Float[]":
		return octxiliary.ValueFloatArray, nil
	case "Bytes":
		return octxiliary.ValueBytes, nil
	default:
		return "", fmt.Errorf("unsupported transport type %s", typ)
	}
}

func findInterpretedTransport(types []project.TransportTypeMetadata, name string) (project.TransportTypeMetadata, bool) {
	for _, typ := range types {
		if typ.Name == name {
			return typ, true
		}
	}
	return project.TransportTypeMetadata{}, false
}
