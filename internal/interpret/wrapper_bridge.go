package interpret

import (
	"fmt"

	"oct/internal/ast"
	"oct/internal/dimension"
)

type wrapperHandleStore[T any] struct {
	kind   string
	values map[int64]T
}

func newWrapperHandleStore[T any](kind string) wrapperHandleStore[T] {
	return wrapperHandleStore[T]{
		kind:   kind,
		values: make(map[int64]T),
	}
}

func (s *wrapperHandleStore[T]) allocate(value T) int64 {
	handle := int64(len(s.values) + 1)
	for {
		if _, exists := s.values[handle]; !exists {
			break
		}
		handle++
	}
	s.values[handle] = value
	return handle
}

func (s *wrapperHandleStore[T]) get(handle int64) (T, error) {
	value, ok := s.values[handle]
	if !ok {
		var zero T
		return zero, wrapperErrorf(wrapperErrorInvalidHandle, "invalid %s handle %d", s.kind, handle)
	}
	return value, nil
}

func (s *wrapperHandleStore[T]) release(handle int64) {
	delete(s.values, handle)
}

type wrapperBuiltinHandler func(i *interpreter, env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error)

type wrapperBuiltinRegistry struct {
	handlers map[string]wrapperBuiltinHandler
}

func newWrapperBuiltinRegistry(handlerSets ...map[string]wrapperBuiltinHandler) wrapperBuiltinRegistry {
	handlers := make(map[string]wrapperBuiltinHandler)
	for _, handlerSet := range handlerSets {
		for name, handler := range handlerSet {
			handlers[name] = handler
		}
	}
	return wrapperBuiltinRegistry{handlers: handlers}
}

func (r wrapperBuiltinRegistry) has(name string) bool {
	_, ok := r.handlers[name]
	return ok
}

func (r wrapperBuiltinRegistry) eval(i *interpreter, env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	handler, ok := r.handlers[callee]
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported wrapper builtin %s", callee)
	}
	return handler(i, env, pkgName, callee, argumentExprs)
}

type wrapperCall struct {
	interpreter *interpreter
	env         *environment
	pkgName     string
	callee      string
	args        []ast.Expr
}

func newWrapperCall(i *interpreter, env *environment, pkgName string, callee string, args []ast.Expr) wrapperCall {
	return wrapperCall{interpreter: i, env: env, pkgName: pkgName, callee: callee, args: args}
}

func (c wrapperCall) expectArity(expected int) error {
	if len(c.args) != expected {
		return fmt.Errorf("runtime invariant violation: %s expects %d arguments", c.callee, expected)
	}
	return nil
}

func (c wrapperCall) evalArg(index int) (Value, *evalResult, error) {
	if index < 0 || index >= len(c.args) {
		return Value{}, nil, fmt.Errorf("runtime invariant violation: %s missing argument %d", c.callee, index+1)
	}
	result, err := c.interpreter.evalExpr(c.env, c.pkgName, c.args[index])
	if err != nil {
		return Value{}, nil, err
	}
	if result.hasError {
		errResult := evalResult{hasError: true, errorVal: result.errorVal}
		return Value{}, &errResult, nil
	}
	return result.value, nil, nil
}

func (c wrapperCall) stringArg(index int) (string, *evalResult, error) {
	value, errResult, err := c.evalArg(index)
	if err != nil || errResult != nil {
		return "", errResult, err
	}
	if value.Kind != ValueString {
		return "", nil, fmt.Errorf("runtime invariant violation: %s argument %d expects String", c.callee, index+1)
	}
	return value.Text, nil, nil
}

func (c wrapperCall) intArg(index int) (int64, *evalResult, error) {
	value, errResult, err := c.evalArg(index)
	if err != nil || errResult != nil {
		return 0, errResult, err
	}
	if value.Kind != ValueInt {
		return 0, nil, fmt.Errorf("runtime invariant violation: %s argument %d expects Int", c.callee, index+1)
	}
	return value.Int, nil, nil
}

func (c wrapperCall) floatArg(index int) (float64, *evalResult, error) {
	value, errResult, err := c.evalArg(index)
	if err != nil || errResult != nil {
		return 0, errResult, err
	}
	switch value.Kind {
	case ValueFloat:
		return value.Float, nil, nil
	case ValueInt:
		return float64(value.Int), nil, nil
	default:
		return 0, nil, fmt.Errorf("runtime invariant violation: %s argument %d expects Int or Float", c.callee, index+1)
	}
}

func (c wrapperCall) bytesArg(index int) ([]byte, *evalResult, error) {
	value, errResult, err := c.evalArg(index)
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	if value.Kind != ValueBytes {
		return nil, nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects Bytes", index+1)
	}
	return append([]byte(nil), value.Bytes...), nil, nil
}

func (c wrapperCall) stringArrayArg(index int) ([]string, *evalResult, error) {
	value, errResult, err := c.evalArg(index)
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	if value.Kind != ValueArray {
		return nil, nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects String[]", index+1)
	}
	values := make([]string, 0, len(value.Array))
	for elementIndex, current := range value.Array {
		if current.Kind != ValueString {
			return nil, nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects String[] (element %d)", index+1, elementIndex)
		}
		values = append(values, current.Text)
	}
	return values, nil, nil
}

func wrapperIntResult(value int64) evalResult {
	return evalResult{value: Value{Kind: ValueInt, Int: value}}
}

func wrapperIntDimensionResult(value int64, dim dimension.Dimension) evalResult {
	return evalResult{value: Value{Kind: ValueInt, Int: value, Dimension: dim}}
}

func wrapperBoolResult(value bool) evalResult {
	return evalResult{value: Value{Kind: ValueBool, Bool: value}}
}

func wrapperStringResult(value string) evalResult {
	return evalResult{value: Value{Kind: ValueString, Text: value}}
}

func wrapperBytesResult(value []byte) evalResult {
	return evalResult{value: Value{Kind: ValueBytes, Bytes: append([]byte(nil), value...)}}
}

func wrapperArrayResult(values []Value) evalResult {
	return evalResult{value: Value{Kind: ValueArray, Array: values}}
}

func wrapperStringArrayResult(values []string) evalResult {
	items := make([]Value, 0, len(values))
	for _, value := range values {
		items = append(items, Value{Kind: ValueString, Text: value})
	}
	return wrapperArrayResult(items)
}

func wrapperStringMatrixResult(rows [][]string) evalResult {
	rowValues := make([]Value, 0, len(rows))
	for _, row := range rows {
		rowValues = append(rowValues, wrapperStringArrayValue(row))
	}
	return wrapperArrayResult(rowValues)
}

func wrapperStringArrayValue(values []string) Value {
	items := make([]Value, 0, len(values))
	for _, value := range values {
		items = append(items, Value{Kind: ValueString, Text: value})
	}
	return Value{Kind: ValueArray, Array: items}
}

type wrapperErrorKind string

const (
	wrapperErrorInvalidArgument wrapperErrorKind = "InvalidArgument"
	wrapperErrorInvalidHandle   wrapperErrorKind = "InvalidHandle"
	wrapperErrorNotFound        wrapperErrorKind = "NotFound"
	wrapperErrorConflict        wrapperErrorKind = "Conflict"
	wrapperErrorInvalidData     wrapperErrorKind = "InvalidData"
	wrapperErrorBackendFailure  wrapperErrorKind = "BackendFailure"
)

type wrapperError struct {
	kind    wrapperErrorKind
	message string
}

func (e wrapperError) Error() string {
	return fmt.Sprintf("%s: %s", e.kind, e.message)
}

func wrapperErrorf(kind wrapperErrorKind, format string, args ...any) error {
	return wrapperError{kind: kind, message: fmt.Sprintf(format, args...)}
}

func wrapperErrorResult(callee string, err error) evalResult {
	if err == nil {
		return evalResult{}
	}
	var contractErr wrapperError
	message := err.Error()
	if asWrapperError(err, &contractErr) {
		message = contractErr.Error()
	}
	return evalResult{
		hasError: true,
		errorVal: Value{
			Kind:  ValueError,
			Error: ErrorValue{Message: fmt.Sprintf("%s: %s", callee, message)},
		},
	}
}

func asWrapperError(err error, target *wrapperError) bool {
	if err == nil {
		return false
	}
	value, ok := err.(wrapperError)
	if !ok {
		return false
	}
	*target = value
	return true
}
