package filepicker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAtListsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.OpenAt(dir)
	if !m.IsVisible() {
		t.Fatal("expected picker visible")
	}
	if m.FilteredCount() == 0 {
		t.Fatal("expected entries")
	}
}

func TestHandleKeyFiltersEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.OpenAt(dir)
	m.HandleKey("a")
	if m.Query() != "a" {
		t.Fatalf("expected query a, got %q", m.Query())
	}
	if m.FilteredCount() == 0 {
		t.Fatal("expected filtered result")
	}
}

func TestEnterOnDirectoryNavigates(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.OpenAt(root)
	m.HandleKey("s")
	m.HandleKey("u")
	m.HandleKey("b")
	if result := m.HandleKey("enter"); result != nil {
		t.Fatal("expected directory enter to navigate, not return file")
	}
	if m.Cwd() != sub {
		t.Fatalf("expected cwd %q, got %q", sub, m.Cwd())
	}
}

func TestBackspaceWithoutQueryGoesParent(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.OpenAt(sub)
	m.HandleKey("backspace")
	if m.Cwd() != root {
		t.Fatalf("expected parent cwd %q, got %q", root, m.Cwd())
	}
}

func TestEnterOnFileReturnsResult(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.OpenAt(dir)
	m.HandleKey("n")
	m.HandleKey("o")
	m.HandleKey("t")
	m.HandleKey("e")
	result := m.HandleKey("enter")
	if result == nil {
		t.Fatal("expected file result")
	}
	if result.Path != file {
		t.Fatalf("expected path %q, got %q", file, result.Path)
	}
	if m.IsVisible() {
		t.Fatal("expected picker closed after file selection")
	}
}

func TestEscClosesPicker(t *testing.T) {
	m := New()
	m.OpenAt(t.TempDir())
	m.HandleKey("esc")
	if m.IsVisible() {
		t.Fatal("expected picker hidden after esc")
	}
}
