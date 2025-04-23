package operations_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidroman0O/turingpi/operations"
)

func TestCompressionOperations(t *testing.T) {
	// Create a real executor for integration testing
	executor := &operations.NativeExecutor{}
	compressionOps := operations.NewCompressionOperations(executor)

	// Create a temporary directory for our test files
	tempDir, err := os.MkdirTemp("", "compression-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Test file contents
	testContent := []byte("This is test content for compression operations")

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test XZ compression and decompression
	t.Run("XZCompression", func(t *testing.T) {
		// Skip test if xz command is not available
		if _, err := executor.Execute(ctx, "which", "xz"); err != nil {
			t.Skip("xz command not available, skipping test")
		}

		outputXZPath := filepath.Join(tempDir, "test-file.txt.xz")
		outputDecompressDir := filepath.Join(tempDir, "xz-output")

		// Compress the file
		err := compressionOps.CompressXZ(ctx, testFilePath, outputXZPath)
		if err != nil {
			t.Fatalf("CompressXZ failed: %v", err)
		}

		// Verify the compressed file was created
		if _, err := os.Stat(outputXZPath); os.IsNotExist(err) {
			t.Fatalf("Compressed file was not created at %s", outputXZPath)
		}

		// Decompress the file
		decompressedPath, err := compressionOps.DecompressXZ(ctx, outputXZPath, outputDecompressDir)
		if err != nil {
			t.Fatalf("DecompressXZ failed: %v", err)
		}

		// Verify the decompressed file was created
		if _, err := os.Stat(decompressedPath); os.IsNotExist(err) {
			t.Fatalf("Decompressed file was not created at %s", decompressedPath)
		}

		// Read the decompressed file and verify its contents
		decompressedContent, err := os.ReadFile(decompressedPath)
		if err != nil {
			t.Fatalf("Failed to read decompressed file: %v", err)
		}

		if string(decompressedContent) != string(testContent) {
			t.Fatalf("Decompressed content does not match original. Expected %q, got %q", testContent, decompressedContent)
		}
	})

	// Test GZ compression and decompression
	t.Run("GZCompression", func(t *testing.T) {
		// Skip test if gzip command is not available
		if _, err := executor.Execute(ctx, "which", "gzip"); err != nil {
			t.Skip("gzip command not available, skipping test")
		}

		outputGZPath := filepath.Join(tempDir, "test-file.txt.gz")
		outputDecompressDir := filepath.Join(tempDir, "gz-output")

		// Compress the file
		err := compressionOps.CompressGZ(ctx, testFilePath, outputGZPath)
		if err != nil {
			t.Fatalf("CompressGZ failed: %v", err)
		}

		// Verify the compressed file was created
		if _, err := os.Stat(outputGZPath); os.IsNotExist(err) {
			t.Fatalf("Compressed file was not created at %s", outputGZPath)
		}

		// Decompress the file
		decompressedPath, err := compressionOps.DecompressGZ(ctx, outputGZPath, outputDecompressDir)
		if err != nil {
			t.Fatalf("DecompressGZ failed: %v", err)
		}

		// Verify the decompressed file was created
		if _, err := os.Stat(decompressedPath); os.IsNotExist(err) {
			t.Fatalf("Decompressed file was not created at %s", decompressedPath)
		}

		// Read the decompressed file and verify its contents
		decompressedContent, err := os.ReadFile(decompressedPath)
		if err != nil {
			t.Fatalf("Failed to read decompressed file: %v", err)
		}

		if string(decompressedContent) != string(testContent) {
			t.Fatalf("Decompressed content does not match original. Expected %q, got %q", testContent, decompressedContent)
		}
	})

	// Test TAR.GZ compression and decompression
	t.Run("TarGZCompression", func(t *testing.T) {
		// Skip test if tar command is not available
		if _, err := executor.Execute(ctx, "which", "tar"); err != nil {
			t.Skip("tar command not available, skipping test")
		}

		// Create a directory structure to compress
		testDirPath := filepath.Join(tempDir, "test-dir")
		if err := os.MkdirAll(testDirPath, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Create a few test files in the directory
		for i := 1; i <= 3; i++ {
			fileName := filepath.Join(testDirPath, fmt.Sprintf("file%d.txt", i))
			fileContent := []byte(fmt.Sprintf("This is test file %d", i))
			if err := os.WriteFile(fileName, fileContent, 0644); err != nil {
				t.Fatalf("Failed to create test file %s: %v", fileName, err)
			}
		}

		outputTarGZPath := filepath.Join(tempDir, "test-dir.tar.gz")
		outputDecompressDir := filepath.Join(tempDir, "tar-output")

		// Compress the directory
		err := compressionOps.CompressTarGZ(ctx, testDirPath, outputTarGZPath)
		if err != nil {
			t.Fatalf("CompressTarGZ failed: %v", err)
		}

		// Verify the compressed file was created
		if _, err := os.Stat(outputTarGZPath); os.IsNotExist(err) {
			t.Fatalf("Compressed file was not created at %s", outputTarGZPath)
		}

		// Decompress the archive
		err = compressionOps.DecompressTarGZ(ctx, outputTarGZPath, outputDecompressDir)
		if err != nil {
			t.Fatalf("DecompressTarGZ failed: %v", err)
		}

		// Verify the extracted directory structure
		extractedDir := filepath.Join(outputDecompressDir, "test-dir")
		files, err := os.ReadDir(extractedDir)
		if err != nil {
			t.Fatalf("Failed to read extracted directory: %v", err)
		}

		// Check if all files are there
		if len(files) != 3 {
			t.Fatalf("Expected 3 files in extracted directory, got %d", len(files))
		}

		// Verify one of the files
		extractedFile := filepath.Join(extractedDir, "file1.txt")
		extractedContent, err := os.ReadFile(extractedFile)
		if err != nil {
			t.Fatalf("Failed to read extracted file: %v", err)
		}

		expectedContent := "This is test file 1"
		if string(extractedContent) != expectedContent {
			t.Fatalf("Extracted content does not match. Expected %q, got %q", expectedContent, extractedContent)
		}
	})

	// Test error cases with invalid paths
	t.Run("ErrorCases", func(t *testing.T) {
		// Non-existent source file for decompression
		nonExistentFile := filepath.Join(tempDir, "non-existent.xz")
		_, err := compressionOps.DecompressXZ(ctx, nonExistentFile, tempDir)
		if err == nil || !strings.Contains(err.Error(), "source file does not exist") {
			t.Fatalf("Expected 'source file does not exist' error, got: %v", err)
		}

		// Non-existent source file for compression
		_, err = compressionOps.DecompressXZ(ctx, nonExistentFile, tempDir)
		if err == nil || !strings.Contains(err.Error(), "source file does not exist") {
			t.Fatalf("Expected 'source file does not exist' error, got: %v", err)
		}
	})
}
