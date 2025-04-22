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

func TestTempFSCache(t *testing.T) {
	// Create a temp directory for tests
	baseTempDir, err := os.MkdirTemp("", "temp_cache_test_*")
	if err != nil {
		t.Fatalf("Failed to create base temp dir: %v", err)
	}
	defer os.RemoveAll(baseTempDir)

	// Test with a provided base path
	t.Run("With base path", func(t *testing.T) {
		cache, err := NewTempFSCache(baseTempDir)
		if err != nil {
			t.Fatalf("Failed to create TempFSCache: %v", err)
		}
		defer cache.Close()

		// Verify the cache directory exists
		if _, err := os.Stat(cache.CleanupPath()); os.IsNotExist(err) {
			t.Fatalf("Cache directory not created: %v", err)
		}

		// Verify it's a subdirectory of our base path
		if !strings.HasPrefix(cache.CleanupPath(), baseTempDir) {
			t.Errorf("Cache directory not created in base path. Got: %s", cache.CleanupPath())
		}

		ctx := context.Background()

		// Test basic cache operations
		content := "temporary test content"
		metadata := Metadata{
			Filename:    "temp_test.txt",
			ContentType: "text/plain",
			Size:        int64(len(content)),
			ModTime:     time.Now(),
			Tags:        map[string]string{"type": "temp_test"},
		}

		// Test Put
		reader := strings.NewReader(content)
		putMeta, err := cache.Put(ctx, "temp_test1", metadata, reader)
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if putMeta.Hash == "" {
			t.Error("Hash should be generated")
		}

		// Test Get with content
		getMeta, getReader, err := cache.Get(ctx, "temp_test1", true)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer getReader.Close()

		gotContent, err := io.ReadAll(getReader)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}
		if string(gotContent) != content {
			t.Errorf("Expected content %q, got %q", content, string(gotContent))
		}
		if getMeta.Filename != metadata.Filename {
			t.Errorf("Expected filename %s, got %s", metadata.Filename, getMeta.Filename)
		}

		// Manually check the file exists on disk
		contentPath := filepath.Join(cache.CleanupPath(), "temp_test1.data")
		if _, err := os.Stat(contentPath); os.IsNotExist(err) {
			t.Errorf("Content file not found on disk: %v", err)
		}

		// Close the cache and verify cleanup
		err = cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Verify the directory has been removed
		if _, err := os.Stat(cache.CleanupPath()); !os.IsNotExist(err) {
			t.Errorf("Cache directory not removed after Close()")
		}
	})

	// Test with system temp directory
	t.Run("With system temp", func(t *testing.T) {
		cache, err := NewTempFSCache("")
		if err != nil {
			t.Fatalf("Failed to create TempFSCache: %v", err)
		}
		defer cache.Close()

		// Verify the cache directory exists
		if _, err := os.Stat(cache.CleanupPath()); os.IsNotExist(err) {
			t.Fatalf("Cache directory not created: %v", err)
		}

		// Verify it's in the system temp directory
		systemTempDir := os.TempDir()
		if !strings.HasPrefix(cache.CleanupPath(), systemTempDir) {
			t.Errorf("Cache not created in system temp dir. Expected prefix %s, got: %s",
				systemTempDir, cache.CleanupPath())
		}

		// Close and verify cleanup
		err = cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Verify the directory has been removed
		if _, err := os.Stat(cache.CleanupPath()); !os.IsNotExist(err) {
			t.Errorf("Cache directory not removed after Close()")
		}
	})

	// Test the convenience function
	t.Run("CreateTempCache", func(t *testing.T) {
		cache, err := CreateTempCache(baseTempDir)
		if err != nil {
			t.Fatalf("Failed to create TempFSCache: %v", err)
		}
		defer cache.Close()

		// Verify the cache directory exists
		if _, err := os.Stat(cache.CleanupPath()); os.IsNotExist(err) {
			t.Fatalf("Cache directory not created: %v", err)
		}

		// Verify it's a subdirectory of our base path
		if !strings.HasPrefix(cache.CleanupPath(), baseTempDir) {
			t.Errorf("Cache directory not created in base path. Got: %s", cache.CleanupPath())
		}

		// Close and verify cleanup
		err = cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Verify the directory has been removed
		if _, err := os.Stat(cache.CleanupPath()); !os.IsNotExist(err) {
			t.Errorf("Cache directory not removed after Close()")
		}
	})
}

// TestTempFSCacheFileOperations tests the file and folder operations of TempFSCache
func TestTempFSCacheFileOperations(t *testing.T) {
	// Create a temp directory for tests
	tempDir, err := os.MkdirTemp("", "temp_cache_fileops_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTempFSCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create TempFSCache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Test CreateDir
	t.Run("CreateDir", func(t *testing.T) {
		dirPath := "test/nested/directory"
		err := cache.CreateDir(ctx, dirPath, 0755)
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		// Verify directory exists
		fullPath := cache.GetAbsolutePath(dirPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Directory not created: %v", err)
		}
	})

	// Test CreateTempDir
	t.Run("CreateTempDir", func(t *testing.T) {
		tempDirPath, err := cache.CreateTempDir(ctx, "testdir-")
		if err != nil {
			t.Fatalf("CreateTempDir failed: %v", err)
		}

		// Verify directory exists
		if _, err := os.Stat(tempDirPath); os.IsNotExist(err) {
			t.Errorf("Temporary directory not created: %v", err)
		}

		// Verify it's within the cache directory
		if !strings.HasPrefix(tempDirPath, cache.CleanupPath()) {
			t.Errorf("Temporary directory not created in cache path. Got: %s", tempDirPath)
		}
	})

	// Test WriteFile and ReadFile
	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		filePath := "test/file.txt"
		content := []byte("Hello, TempFSCache!")

		err := cache.WriteFile(ctx, filePath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file exists
		fullPath := cache.GetAbsolutePath(filePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("File not created: %v", err)
		}

		// Test ReadFile
		readContent, err := cache.ReadFile(ctx, filePath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(readContent) != string(content) {
			t.Errorf("Expected content %q, got %q", string(content), string(readContent))
		}
	})

	// Test FileExists
	t.Run("FileExists", func(t *testing.T) {
		// Check existing file
		exists, err := cache.FileExists(ctx, "test/file.txt")
		if err != nil {
			t.Fatalf("FileExists check failed: %v", err)
		}
		if !exists {
			t.Error("File should exist")
		}

		// Check non-existing file
		exists, err = cache.FileExists(ctx, "test/nonexistent.txt")
		if err != nil {
			t.Fatalf("FileExists check failed: %v", err)
		}
		if exists {
			t.Error("File should not exist")
		}
	})

	// Test CopyFile
	t.Run("CopyFile", func(t *testing.T) {
		srcPath := "test/file.txt"
		dstPath := "test/copy/file_copy.txt"

		err := cache.CopyFile(ctx, srcPath, dstPath)
		if err != nil {
			t.Fatalf("CopyFile failed: %v", err)
		}

		// Verify destination file exists
		fullDstPath := cache.GetAbsolutePath(dstPath)
		if _, err := os.Stat(fullDstPath); os.IsNotExist(err) {
			t.Errorf("Destination file not created: %v", err)
		}

		// Check content of copied file
		content, err := cache.ReadFile(ctx, dstPath)
		if err != nil {
			t.Fatalf("ReadFile of copied file failed: %v", err)
		}

		expectedContent, err := cache.ReadFile(ctx, srcPath)
		if err != nil {
			t.Fatalf("ReadFile of source file failed: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("Expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	// Test CopyFromExternalPath
	t.Run("CopyFromExternalPath", func(t *testing.T) {
		// Create a temporary external file
		externalFile, err := os.CreateTemp("", "external-*.txt")
		if err != nil {
			t.Fatalf("Failed to create external file: %v", err)
		}
		defer os.Remove(externalFile.Name())

		externalContent := []byte("External file content")
		if _, err := externalFile.Write(externalContent); err != nil {
			t.Fatalf("Failed to write to external file: %v", err)
		}
		externalFile.Close()

		// Copy the external file to the cache
		dstPath := "test/external_copy.txt"
		err = cache.CopyFromExternalPath(ctx, externalFile.Name(), dstPath)
		if err != nil {
			t.Fatalf("CopyFromExternalPath failed: %v", err)
		}

		// Verify destination file exists
		fullDstPath := cache.GetAbsolutePath(dstPath)
		if _, err := os.Stat(fullDstPath); os.IsNotExist(err) {
			t.Errorf("Destination file not created: %v", err)
		}

		// Check content of copied file
		content, err := cache.ReadFile(ctx, dstPath)
		if err != nil {
			t.Fatalf("ReadFile of copied external file failed: %v", err)
		}

		if string(content) != string(externalContent) {
			t.Errorf("Expected content %q, got %q", string(externalContent), string(content))
		}
	})

	// Test RemoveFile
	t.Run("RemoveFile", func(t *testing.T) {
		filePath := "test/file.txt"
		err := cache.RemoveFile(ctx, filePath)
		if err != nil {
			t.Fatalf("RemoveFile failed: %v", err)
		}

		// Verify file no longer exists
		exists, err := cache.FileExists(ctx, filePath)
		if err != nil {
			t.Fatalf("FileExists check failed: %v", err)
		}
		if exists {
			t.Error("File should have been removed")
		}
	})

	// Test WalkFiles
	t.Run("WalkFiles", func(t *testing.T) {
		// Create a few more files for the walk
		filePaths := []string{
			"walk/file1.txt",
			"walk/nested/file2.txt",
			"walk/nested/deep/file3.txt",
		}

		for _, path := range filePaths {
			err := cache.WriteFile(ctx, path, []byte("content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create file for walk test: %v", err)
			}
		}

		// Count the number of files found during walk
		fileCount := 0
		err := cache.WalkFiles(ctx, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				fileCount++
			}
			return nil
		})

		if err != nil {
			t.Fatalf("WalkFiles failed: %v", err)
		}

		// We should find at least the files we created (may find more from other tests)
		if fileCount < len(filePaths) {
			t.Errorf("Expected at least %d files, found %d", len(filePaths), fileCount)
		}
	})
}
