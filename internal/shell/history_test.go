package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

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
