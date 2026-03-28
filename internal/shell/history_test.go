package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryUpEmpty(t *testing.T) {
	h := newHistory(nil)
	val, ok := h.up("draft")
	if ok {
		t.Fatal("expected ok=false for empty history")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestHistoryUpAtTop(t *testing.T) {
	h := newHistory([]string{"first", "second"})
	// Move to top
	h.up("draft") // index -> 1, returns "second"
	h.up("draft") // index -> 0, returns "first"

	// Already at top, should return current entry and false
	val, ok := h.up("draft")
	if ok {
		t.Fatal("expected ok=false at top of history")
	}
	if val != "first" {
		t.Fatalf("expected %q, got %q", "first", val)
	}
}

func TestHistoryDownAtBottom(t *testing.T) {
	h := newHistory([]string{"first", "second"})
	// index == len(entries), already at bottom
	val, ok := h.down()
	if ok {
		t.Fatal("expected ok=false at bottom of history")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestHistoryDownAfterNavigatingReturnsDraft(t *testing.T) {
	h := newHistory([]string{"first", "second"})
	// Navigate up (saves draft, moves to last entry)
	val, ok := h.up("my draft")
	if !ok || val != "second" {
		t.Fatalf("expected (second, true), got (%q, %t)", val, ok)
	}

	// Navigate down should return draft
	val, ok = h.down()
	if !ok {
		t.Fatal("expected ok=true when returning to draft")
	}
	if val != "my draft" {
		t.Fatalf("expected draft %q, got %q", "my draft", val)
	}
}

func TestHistoryUpNilReceiver(t *testing.T) {
	var h *historyState
	val, ok := h.up("draft")
	if ok {
		t.Fatal("expected ok=false for nil history")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestHistoryDownNilReceiver(t *testing.T) {
	var h *historyState
	val, ok := h.down()
	if ok {
		t.Fatal("expected ok=false for nil history")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestHistoryLoadEmptyFile(t *testing.T) {
	entries, err := loadHistory("testdata/empty-history")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestHistoryAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := appendHistory(path, "ls"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, err := loadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0] != "ls" {
		t.Fatalf("expected [ls], got %v", entries)
	}
}

func TestHistoryDownAtBottomReturnsFalse(t *testing.T) {
	h := newHistory([]string{"one"})
	// index starts at len(entries), i.e. at bottom
	val, ok := h.down()
	if ok {
		t.Fatal("expected ok=false when already at bottom")
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestHistoryDownReturnsLastThenDraft(t *testing.T) {
	h := newHistory([]string{"one", "two"})
	// Navigate up to index 0
	h.up("my draft") // index -> 1
	h.up("my draft") // index -> 0

	// First down: should return "two" (index 1)
	val, ok := h.down()
	if !ok {
		t.Fatal("expected ok=true on first down")
	}
	if val != "two" {
		t.Fatalf("expected %q, got %q", "two", val)
	}

	// Second down: index moves past last entry, returns draft
	val, ok = h.down()
	if !ok {
		t.Fatal("expected ok=true returning to draft")
	}
	if val != "my draft" {
		t.Fatalf("expected draft %q, got %q", "my draft", val)
	}
}

func TestHistoryAppendNilReceiver(t *testing.T) {
	var h *historyState
	// Must not panic
	h.append("anything")
}

func TestHistoryAppendEmptyLine(t *testing.T) {
	h := newHistory([]string{"existing"})
	h.append("   ")
	if len(h.entries) != 1 {
		t.Fatalf("expected 1 entry after appending whitespace, got %d", len(h.entries))
	}
}

func TestHistoryAppendResetsIndex(t *testing.T) {
	h := newHistory([]string{"one", "two"})
	// Navigate up so index != len(entries)
	h.up("draft")
	if h.index == len(h.entries) {
		t.Fatal("expected index to move away from bottom after up()")
	}

	h.append("three")
	if h.index != len(h.entries) {
		t.Fatalf("expected index=%d after append, got %d", len(h.entries), h.index)
	}
	if h.draft != "" {
		t.Fatalf("expected draft to be cleared, got %q", h.draft)
	}
}

func TestAppendHistoryEmptyLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := appendHistory(path, "  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File should not be created for empty line
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file for empty line append")
	}
}

func TestAppendHistoryBadPath(t *testing.T) {
	err := appendHistory("/nonexistent/dir/history", "ls")
	if err == nil {
		t.Fatal("expected error for bad path")
	}
}

func TestLoadHistoryNonExistentFile(t *testing.T) {
	entries, err := loadHistory("/nonexistent/path/history")
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %v", entries)
	}
}

func TestHistoryLoadCapsEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	writer := bufio.NewWriter(f)
	total := maxHistoryEntries + 2
	for i := 0; i < total; i++ {
		if _, err := fmt.Fprintf(writer, "line-%d\n", i); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := loadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != maxHistoryEntries {
		t.Fatalf("expected %d entries, got %d", maxHistoryEntries, len(entries))
	}
	wantFirst := fmt.Sprintf("line-%d", total-maxHistoryEntries)
	if entries[0] != wantFirst {
		t.Fatalf("expected first entry %q, got %q", wantFirst, entries[0])
	}
	wantLast := fmt.Sprintf("line-%d", total-1)
	if entries[len(entries)-1] != wantLast {
		t.Fatalf("expected last entry %q, got %q", wantLast, entries[len(entries)-1])
	}
}
