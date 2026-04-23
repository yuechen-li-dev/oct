package interpret

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapperHandleStoreAllocatesAndLooksUpDeterministically(t *testing.T) {
	store := newWrapperHandleStore[string]("workbook")
	first := store.allocate("first")
	second := store.allocate("second")
	if first != 1 || second != 2 {
		t.Fatalf("expected deterministic handles 1 and 2, got %d and %d", first, second)
	}
	value, err := store.get(second)
	if err != nil {
		t.Fatalf("lookup handle %d: %v", second, err)
	}
	if value != "second" {
		t.Fatalf("expected second handle value %q, got %q", "second", value)
	}
}

func TestWrapperHandleStoreRejectsInvalidHandle(t *testing.T) {
	store := newWrapperHandleStore[int]("workbook")
	_, err := store.get(42)
	if err == nil {
		t.Fatal("expected invalid handle lookup to fail")
	}
	if !strings.Contains(err.Error(), "InvalidHandle: invalid workbook handle 42") {
		t.Fatalf("expected deterministic invalid-handle message, got %q", err.Error())
	}
}

func TestWrapperRegistryComposesHandlerSets(t *testing.T) {
	registry := newWrapperBuiltinRegistry(
		map[string]wrapperBuiltinHandler{"First": nil},
		map[string]wrapperBuiltinHandler{"Second": nil},
	)
	if !registry.has("First") || !registry.has("Second") {
		t.Fatalf("expected composed registry to contain both wrapper builtin names")
	}
}

func TestWrapperResultHelpers(t *testing.T) {
	intResult := wrapperIntResult(7)
	if intResult.value.Kind != ValueInt || intResult.value.Int != 7 {
		t.Fatalf("expected Int result 7, got %+v", intResult.value)
	}
	textResult := wrapperStringResult("ok")
	if textResult.value.Kind != ValueString || textResult.value.Text != "ok" {
		t.Fatalf("expected String result 'ok', got %+v", textResult.value)
	}
}

func TestWrapperCallExpectArity(t *testing.T) {
	call := wrapperCall{callee: "JsonNormalize", args: nil}
	if err := call.expectArity(0); err != nil {
		t.Fatalf("expected arity check to pass: %v", err)
	}
	if err := call.expectArity(1); err == nil {
		t.Fatalf("expected arity mismatch to fail")
	}
}

func TestWrapperErrorResultMapsKindsIntoStableMessage(t *testing.T) {
	result := wrapperErrorResult("JsonNormalize", wrapperErrorf(wrapperErrorInvalidData, "bad input"))
	if !result.hasError {
		t.Fatalf("expected wrapper error result")
	}
	got := result.errorVal.Error.Message
	if got != "JsonNormalize: InvalidData: bad input" {
		t.Fatalf("unexpected error shape: %q", got)
	}
}

func TestWrapperErrorResultPreservesNonWrapperErrors(t *testing.T) {
	result := wrapperErrorResult("XlsxAddSheet", errors.New("backend exploded"))
	if !result.hasError {
		t.Fatalf("expected wrapper error result")
	}
	got := result.errorVal.Error.Message
	if got != "XlsxAddSheet: backend exploded" {
		t.Fatalf("unexpected fallback error shape: %q", got)
	}
}
