package router

import (
	"testing"
)

func TestRequestIndexStoreAndLookup(t *testing.T) {
	ri := newRequestIndex(10)

	meta := &RequestMeta{
		RequestID: "req-001",
		Level:     "easy",
		Model:     "model-a",
		Status:    200,
	}
	ri.Store(meta)

	found, ok := ri.Lookup("req-001")
	if !ok {
		t.Fatal("expected to find request")
	}
	if found.Level != "easy" {
		t.Errorf("level = %s, want easy", found.Level)
	}
}

func TestRequestIndexNotFound(t *testing.T) {
	ri := newRequestIndex(10)
	_, ok := ri.Lookup("nonexistent")
	if ok {
		t.Error("should not find nonexistent request")
	}
}

func TestRequestIndexBoundedSize(t *testing.T) {
	ri := newRequestIndex(3)

	for i := 0; i < 5; i++ {
		ri.Store(&RequestMeta{
			RequestID: "req-" + string(rune('A'+i)),
		})
	}

	if ri.Len() != 3 {
		t.Errorf("expected 3 entries (bounded), got %d", ri.Len())
	}

	if _, ok := ri.Lookup("req-A"); ok {
		t.Error("oldest entry should have been evicted")
	}

	if _, ok := ri.Lookup("req-E"); !ok {
		t.Error("newest entry should exist")
	}
}

func TestRequestIndexNoPromptText(t *testing.T) {
	ri := newRequestIndex(10)
	meta := &RequestMeta{
		RequestID: "req-001",
		Level:     "hard",
		Model:     "model-b",
	}
	ri.Store(meta)
	found, _ := ri.Lookup("req-001")
	if found.Level != "hard" {
		t.Errorf("level = %s, want hard", found.Level)
	}
}
