package interpret

import (
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
	if !strings.Contains(err.Error(), "invalid workbook handle 42") {
		t.Fatalf("expected deterministic invalid-handle message, got %q", err.Error())
	}
}
