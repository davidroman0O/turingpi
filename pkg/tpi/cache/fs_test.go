package cache

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFSCache(t *testing.T) {
	// Create temp directory for tests
	tempDir, err := os.MkdirTemp("", "fscache_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewFSCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FSCache: %v", err)
	}

	ctx := context.Background()

	t.Run("Put and Get", func(t *testing.T) {
		content := "test content"
		metadata := Metadata{
			Filename:    "test.txt",
			ContentType: "text/plain",
			Size:        int64(len(content)),
			ModTime:     time.Now(),
			Tags:        map[string]string{"type": "test"},
			OSType:      "linux",
			OSVersion:   "5.10",
		}

		// Test Put
		reader := strings.NewReader(content)
		putMeta, err := cache.Put(ctx, "test1", metadata, reader)
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if putMeta.Hash == "" {
			t.Error("Hash should be generated")
		}

		// Test Get with content
		getMeta, getReader, err := cache.Get(ctx, "test1", true)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer getReader.Close()

		if getMeta.Filename != metadata.Filename {
			t.Errorf("Expected filename %s, got %s", metadata.Filename, getMeta.Filename)
		}

		gotContent, err := io.ReadAll(getReader)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}
		if string(gotContent) != content {
			t.Errorf("Expected content %q, got %q", content, string(gotContent))
		}

		// Test Get metadata only
		statMeta, statReader, err := cache.Get(ctx, "test1", false)
		if err != nil {
			t.Fatalf("Get metadata failed: %v", err)
		}
		if statReader != nil {
			t.Error("Reader should be nil when getContent is false")
		}
		if statMeta.Hash != putMeta.Hash {
			t.Errorf("Expected hash %s, got %s", putMeta.Hash, statMeta.Hash)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		// Test non-existent key
		exists, err := cache.Exists(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist")
		}

		// Put something and test existence
		content := "test content"
		metadata := Metadata{Filename: "test.txt"}
		_, err = cache.Put(ctx, "test2", metadata, strings.NewReader(content))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		exists, err = cache.Exists(ctx, "test2")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if !exists {
			t.Error("Key should exist")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Put multiple items with different tags
		items := []struct {
			key     string
			content string
			tags    map[string]string
		}{
			{"list1", "content1", map[string]string{"type": "doc", "env": "prod"}},
			{"list2", "content2", map[string]string{"type": "img", "env": "prod"}},
			{"list3", "content3", map[string]string{"type": "doc", "env": "dev"}},
		}

		for _, item := range items {
			metadata := Metadata{
				Filename: item.key + ".txt",
				Tags:     item.tags,
			}
			_, err := cache.Put(ctx, item.key, metadata, strings.NewReader(item.content))
			if err != nil {
				t.Fatalf("Put failed for %s: %v", item.key, err)
			}
		}

		// List all items
		allItems, err := cache.List(ctx, nil)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(allItems) < len(items) {
			t.Errorf("Expected at least %d items, got %d", len(items), len(allItems))
		}

		// List with tag filter
		prodDocs, err := cache.List(ctx, map[string]string{"type": "doc", "env": "prod"})
		if err != nil {
			t.Fatalf("List with filter failed: %v", err)
		}
		if len(prodDocs) != 1 {
			t.Errorf("Expected 1 prod doc, got %d", len(prodDocs))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Put something
		content := "test content"
		metadata := Metadata{Filename: "test.txt"}
		_, err := cache.Put(ctx, "test3", metadata, strings.NewReader(content))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		// Delete it
		err = cache.Delete(ctx, "test3")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify it's gone
		exists, err := cache.Exists(ctx, "test3")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist after deletion")
		}

		// Check both .data and .meta files are gone
		if _, err := os.Stat(filepath.Join(tempDir, "test3.data")); !os.IsNotExist(err) {
			t.Error("Data file should not exist")
		}
		if _, err := os.Stat(filepath.Join(tempDir, "test3.meta")); !os.IsNotExist(err) {
			t.Error("Meta file should not exist")
		}
	})

	t.Run("Location", func(t *testing.T) {
		loc := cache.Location()
		if loc != tempDir {
			t.Errorf("Expected location %s, got %s", tempDir, loc)
		}
	})
}

func TestFSCacheCleanup(t *testing.T) {
	tempDir := t.TempDir()
	cache, err := NewFSCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FSCache: %v", err)
	}

	ctx := context.Background()

	// Create a valid file with metadata
	validKey := "valid/file"
	validContent := "valid content"
	validMeta := Metadata{
		Key:       validKey,
		OSType:    "linux",
		OSVersion: "ubuntu-20.04",
		Tags:      map[string]string{"arch": "amd64"},
	}
	_, err = cache.Put(ctx, validKey, validMeta, strings.NewReader(validContent))
	if err != nil {
		t.Fatalf("Failed to put valid file: %v", err)
	}

	// Create orphaned .data files
	orphanedFiles := []string{
		filepath.Join(tempDir, "orphaned1.data"),
		filepath.Join(tempDir, "nested", "orphaned2.data"),
	}

	// Create nested directory and orphaned files
	if err := os.MkdirAll(filepath.Join(tempDir, "nested"), 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	for _, file := range orphanedFiles {
		dir := filepath.Dir(file)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(file, []byte("orphaned content"), 0644); err != nil {
			t.Fatalf("Failed to create orphaned file %s: %v", file, err)
		}
	}

	// Test non-recursive cleanup (should only clean up root level files)
	count, err := cache.Cleanup(ctx, false)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if count != 1 { // Should only clean up root orphaned file
		t.Errorf("Expected 1 file cleaned at root level, got %d", count)
	}

	// Verify root orphaned file is removed but nested one still exists
	if _, err := os.Stat(orphanedFiles[0]); !os.IsNotExist(err) {
		t.Error("Root orphaned file should have been removed")
	}
	if _, err := os.Stat(orphanedFiles[1]); os.IsNotExist(err) {
		t.Error("Nested orphaned file should still exist")
	}

	// Test recursive cleanup (should clean up remaining nested files)
	count, err = cache.Cleanup(ctx, true)
	if err != nil {
		t.Fatalf("Recursive cleanup failed: %v", err)
	}
	expectedCount := 2 // Nested orphaned file + empty nested directory
	if count != expectedCount {
		t.Errorf("Expected %d items cleaned (1 file + 1 directory), got %d", expectedCount, count)
	}

	// Verify all orphaned files are removed
	for _, file := range orphanedFiles {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("Orphaned file %s should have been removed", file)
		}
	}

	// Verify nested directory is removed
	if _, err := os.Stat(filepath.Join(tempDir, "nested")); !os.IsNotExist(err) {
		t.Error("Empty nested directory should have been removed")
	}

	// Verify valid file still exists
	_, reader, err := cache.Get(ctx, validKey, true)
	if err != nil {
		t.Errorf("Valid file was removed: %v", err)
	}
	if reader != nil {
		defer reader.Close()
	}
}
