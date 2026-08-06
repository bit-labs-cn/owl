package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStoragePutUsesCallerPath(t *testing.T) {
	root := t.TempDir()
	ls := NewLocalStorage(&LocalConfig{
		Root:       root,
		URLPrefix:  "/storage",
		CreateDirs: true,
		DirMode:    0755,
		FileMode:   0644,
	})

	ctx := context.Background()
	content := []byte("hello-template")
	key := "gui-lv/daily-templates/2026/08/06/demo.xlsx"
	info, err := ls.Put(ctx, key, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != key {
		t.Fatalf("Path=%q want %q", info.Path, key)
	}
	if info.URL != "/storage/"+key {
		t.Fatalf("URL=%q", info.URL)
	}

	phys := filepath.Join(root, filepath.FromSlash(key))
	if _, err := os.Stat(phys); err != nil {
		t.Fatalf("physical file missing: %v", err)
	}

	rc, err := ls.Get(ctx, info.Path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: %q", got)
	}

	if err := ls.Delete(ctx, info.Path); err != nil {
		t.Fatal(err)
	}
}

func TestLocalStoragePutStream(t *testing.T) {
	root := t.TempDir()
	ls := NewLocalStorage(&LocalConfig{
		Root:       root,
		URLPrefix:  "/storage",
		CreateDirs: true,
		DirMode:    0755,
		FileMode:   0644,
	})

	content := []byte(strings.Repeat("stream-", 100))
	key := "gui-lv/daily-reports/1/2026-08-05/abc.xlsx"
	info, err := ls.PutStream(context.Background(), key, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != key || info.Size != int64(len(content)) {
		t.Fatalf("info=%+v", info)
	}
	phys := filepath.Join(root, filepath.FromSlash(key))
	data, err := os.ReadFile(phys)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Fatal("stream content mismatch")
	}
}

func TestLocalStorageGetDoesNotRePrefixDate(t *testing.T) {
	root := t.TempDir()
	ls := NewLocalStorage(&LocalConfig{
		Root:       root,
		URLPrefix:  "/storage",
		CreateDirs: true,
		DirMode:    0755,
		FileMode:   0644,
	})

	oldKey := "2026/08/05/template/old.xlsx"
	phys := filepath.Join(root, filepath.FromSlash(oldKey))
	if err := os.MkdirAll(filepath.Dir(phys), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phys, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	rc, err := ls.Get(context.Background(), oldKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "old" {
		t.Fatalf("got %q", got)
	}

	url, err := ls.URL(context.Background(), oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if url != "/storage/"+oldKey {
		t.Fatalf("URL=%q", url)
	}
}
