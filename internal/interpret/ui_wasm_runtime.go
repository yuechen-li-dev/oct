package interpret

import (
	"fmt"
	machinalayout "github.com/yuechen-li-dev/oct/internal/machina/layout"
	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

const (
	uiWasmStatusOK             int32 = 0
	uiWasmStatusNotInitialized int32 = 1
	uiWasmStatusInvalidInput   int32 = 2
	uiWasmStatusRuntimeError   int32 = 3
	uiWasmStatusBufferTooSmall int32 = 4
)

const uiWasmDefaultMemoryBytes uint32 = 64 * 1024

type uiWasmBoundaryApp interface {
	InitialState() any
	Dispatch(state any, event uiirSerializedEvent) (any, error)
	Project(state any) (*machinauiir.Node, error)
}

type uiWasmRuntime struct {
	app         uiWasmBoundaryApp
	state       any
	memory      []byte
	outputLen   uint32
	initialized bool
	lastError   string
}

func newUIWasmRuntime(app uiWasmBoundaryApp, memoryBytes uint32) (*uiWasmRuntime, error) {
	if app == nil {
		return nil, fmt.Errorf("wasm ui runtime requires non-nil app")
	}
	if memoryBytes == 0 {
		memoryBytes = uiWasmDefaultMemoryBytes
	}
	return &uiWasmRuntime{
		app:    app,
		memory: make([]byte, memoryBytes),
	}, nil
}

func (rt *uiWasmRuntime) ExportMachinaUIInit() int32 {
	rt.state = rt.app.InitialState()
	rt.initialized = true
	rt.outputLen = 0
	rt.lastError = ""
	return uiWasmStatusOK
}

func (rt *uiWasmRuntime) ExportMachinaUIRender() int32 {
	if !rt.initialized {
		return rt.setError(uiWasmStatusNotInitialized, "runtime is not initialized")
	}
	root, err := rt.app.Project(rt.state)
	if err != nil {
		return rt.setError(uiWasmStatusRuntimeError, fmt.Sprintf("render projection failed: %v", err))
	}
	resolved, err := machinalayout.ResolveBoxes(machinauiir.WithNodeIDs(root), &machinauiir.ResolvedRect{X: 0, Y: 0, Width: machinalayout.RootLayoutExtent, Height: machinalayout.RootLayoutExtent})
	if err != nil {
		return rt.setError(uiWasmStatusRuntimeError, fmt.Sprintf("render layout resolution failed: %v", err))
	}
	encoded, err := machinauiir.SerializeCanonicalJSON(resolved)
	if err != nil {
		return rt.setError(uiWasmStatusRuntimeError, fmt.Sprintf("render serialization failed: %v", err))
	}
	if err := rt.writeOutput([]byte(encoded)); err != nil {
		return rt.setError(uiWasmStatusBufferTooSmall, err.Error())
	}
	rt.lastError = ""
	return uiWasmStatusOK
}

func (rt *uiWasmRuntime) ExportMachinaUIDispatchEventJSON(ptr uint32, length uint32) int32 {
	if !rt.initialized {
		return rt.setError(uiWasmStatusNotInitialized, "runtime is not initialized")
	}
	payload, err := rt.readInput(ptr, length)
	if err != nil {
		return rt.setError(uiWasmStatusInvalidInput, err.Error())
	}
	event, err := machinauiir.DeserializeEventCanonicalJSON(string(payload))
	if err != nil {
		return rt.setError(uiWasmStatusInvalidInput, err.Error())
	}
	next, err := rt.app.Dispatch(rt.state, event)
	if err != nil {
		return rt.setError(uiWasmStatusRuntimeError, fmt.Sprintf("event dispatch failed: %v", err))
	}
	rt.state = next
	rt.lastError = ""
	return uiWasmStatusOK
}

func (rt *uiWasmRuntime) ExportMachinaUIBufferPtr() uint32 {
	return 0
}

func (rt *uiWasmRuntime) ExportMachinaUIBufferLen() uint32 {
	return rt.outputLen
}

func (rt *uiWasmRuntime) LastError() string {
	return rt.lastError
}

func (rt *uiWasmRuntime) HostWriteInput(payload []byte) (uint32, uint32, error) {
	if uint32(len(payload)) > uint32(len(rt.memory))-rt.outputLen {
		return 0, 0, fmt.Errorf("input payload exceeds linear memory capacity")
	}
	ptr := rt.outputLen
	copy(rt.memory[ptr:ptr+uint32(len(payload))], payload)
	return ptr, uint32(len(payload)), nil
}

func (rt *uiWasmRuntime) HostReadOutput() []byte {
	if rt.outputLen == 0 {
		return nil
	}
	out := make([]byte, rt.outputLen)
	copy(out, rt.memory[:rt.outputLen])
	return out
}

func (rt *uiWasmRuntime) setError(status int32, message string) int32 {
	rt.lastError = message
	return status
}

func (rt *uiWasmRuntime) readInput(ptr uint32, length uint32) ([]byte, error) {
	if ptr > uint32(len(rt.memory)) {
		return nil, fmt.Errorf("input pointer out of bounds")
	}
	end := ptr + length
	if end > uint32(len(rt.memory)) || end < ptr {
		return nil, fmt.Errorf("input range out of bounds")
	}
	buf := make([]byte, length)
	copy(buf, rt.memory[ptr:end])
	return buf, nil
}

func (rt *uiWasmRuntime) writeOutput(payload []byte) error {
	if uint32(len(payload)) > uint32(len(rt.memory)) {
		rt.outputLen = 0
		return fmt.Errorf("output payload exceeds linear memory capacity")
	}
	copy(rt.memory[:len(payload)], payload)
	rt.outputLen = uint32(len(payload))
	return nil
}
