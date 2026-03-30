package interpret

import (
	"fmt"

	"oct/internal/ast"
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
		return zero, fmt.Errorf("invalid %s handle %d", s.kind, handle)
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

func newWrapperBuiltinRegistry(handlers map[string]wrapperBuiltinHandler) wrapperBuiltinRegistry {
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

func wrapperErrorResult(callee string, err error) evalResult {
	return evalResult{
		hasError: true,
		errorVal: Value{
			Kind:  ValueError,
			Error: ErrorValue{Message: fmt.Sprintf("%s: %v", callee, err)},
		},
	}
}
