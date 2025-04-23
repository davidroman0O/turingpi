package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// CompressionOperations provides operations for compressing and decompressing files
type CompressionOperations struct {
	executor CommandExecutor
}

// NewCompressionOperations creates a new CompressionOperations instance
func NewCompressionOperations(executor CommandExecutor) *CompressionOperations {
	return &CompressionOperations{
		executor: executor,
	}
}

// DecompressXZ decompresses an XZ-compressed file to the specified directory
// Returns the path to the decompressed file
func (c *CompressionOperations) DecompressXZ(ctx context.Context, sourceXZPath, outputDir string) (string, error) {
	// Ensure source file exists
	if _, err := c.executor.Execute(ctx, "test", "-f", sourceXZPath); err != nil {
		return "", fmt.Errorf("source file does not exist: %s", sourceXZPath)
	}

	// Create output directory if it doesn't exist
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output file path
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(sourceXZPath), ".xz"))

	// Generate a temporary file path
	tempFile := filepath.Join(outputDir, fmt.Sprintf("%s.tmp.%d", filepath.Base(outputPath), time.Now().UnixNano()))

	// Use xz with --stdout and redirect to the temporary file
	// This is the most reliable approach across different environments
	cmd := fmt.Sprintf("xz --decompress --keep --stdout %s > %s", sourceXZPath, tempFile)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return "", fmt.Errorf("xz decompression failed: %w, output: %s", err, string(output))
	}

	// Verify the temp file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", tempFile); err != nil {
		return "", fmt.Errorf("decompressed temp file not found at %s", tempFile)
	}

	// Move the temp file to final destination
	if _, err := c.executor.Execute(ctx, "mv", tempFile, outputPath); err != nil {
		// Try to clean up the temp file
		_, _ = c.executor.Execute(ctx, "rm", "-f", tempFile)
		return "", fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return outputPath, nil
}

// CompressXZ compresses a file using XZ compression
// Returns error if compression fails
func (c *CompressionOperations) CompressXZ(ctx context.Context, sourcePath, outputXZPath string) error {
	// Ensure source file exists
	if _, err := c.executor.Execute(ctx, "test", "-f", sourcePath); err != nil {
		return fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputXZPath)
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate a temporary file path
	tempFile := filepath.Join(outputDir, fmt.Sprintf("%s.tmp.%d", filepath.Base(outputXZPath), time.Now().UnixNano()))

	// Use xz with -zck6 flags and redirect to the temporary file
	// -z: force compression
	// -c: write to stdout
	// -k: keep input file
	// -6: compression level 6 (medium, good balance of speed and compression)
	cmd := fmt.Sprintf("xz -zck6 %s > %s", sourcePath, tempFile)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return fmt.Errorf("xz compression failed: %w, output: %s", err, string(output))
	}

	// Verify the temp file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", tempFile); err != nil {
		return fmt.Errorf("compressed temp file not found at %s", tempFile)
	}

	// Move the temp file to final destination
	if _, err := c.executor.Execute(ctx, "mv", tempFile, outputXZPath); err != nil {
		// Try to clean up the temp file
		_, _ = c.executor.Execute(ctx, "rm", "-f", tempFile)
		return fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return nil
}

// DecompressGZ decompresses a GZ-compressed file to the specified directory
// Returns the path to the decompressed file
func (c *CompressionOperations) DecompressGZ(ctx context.Context, sourceGZPath, outputDir string) (string, error) {
	// Ensure source file exists
	if _, err := c.executor.Execute(ctx, "test", "-f", sourceGZPath); err != nil {
		return "", fmt.Errorf("source file does not exist: %s", sourceGZPath)
	}

	// Create output directory if it doesn't exist
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output file path
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(sourceGZPath), ".gz"))

	// Generate a temporary file path
	tempFile := filepath.Join(outputDir, fmt.Sprintf("%s.tmp.%d", filepath.Base(outputPath), time.Now().UnixNano()))

	// Use gunzip with -c and redirect to the temporary file
	cmd := fmt.Sprintf("gunzip -c %s > %s", sourceGZPath, tempFile)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return "", fmt.Errorf("gunzip decompression failed: %w, output: %s", err, string(output))
	}

	// Verify the temp file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", tempFile); err != nil {
		return "", fmt.Errorf("decompressed temp file not found at %s", tempFile)
	}

	// Move the temp file to final destination
	if _, err := c.executor.Execute(ctx, "mv", tempFile, outputPath); err != nil {
		// Try to clean up the temp file
		_, _ = c.executor.Execute(ctx, "rm", "-f", tempFile)
		return "", fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return outputPath, nil
}

// CompressGZ compresses a file using GZ compression
// Returns error if compression fails
func (c *CompressionOperations) CompressGZ(ctx context.Context, sourcePath, outputGZPath string) error {
	// Ensure source file exists
	if _, err := c.executor.Execute(ctx, "test", "-f", sourcePath); err != nil {
		return fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputGZPath)
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate a temporary file path
	tempFile := filepath.Join(outputDir, fmt.Sprintf("%s.tmp.%d", filepath.Base(outputGZPath), time.Now().UnixNano()))

	// Use gzip with -9 for maximum compression, -c to write to stdout
	cmd := fmt.Sprintf("gzip -9c %s > %s", sourcePath, tempFile)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return fmt.Errorf("gzip compression failed: %w, output: %s", err, string(output))
	}

	// Verify the temp file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", tempFile); err != nil {
		return fmt.Errorf("compressed temp file not found at %s", tempFile)
	}

	// Move the temp file to final destination
	if _, err := c.executor.Execute(ctx, "mv", tempFile, outputGZPath); err != nil {
		// Try to clean up the temp file
		_, _ = c.executor.Execute(ctx, "rm", "-f", tempFile)
		return fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return nil
}

// DecompressTarGZ decompresses a tar.gz archive to the specified directory
func (c *CompressionOperations) DecompressTarGZ(ctx context.Context, sourceTarGZPath, outputDir string) error {
	// Ensure source file exists
	if _, err := c.executor.Execute(ctx, "test", "-f", sourceTarGZPath); err != nil {
		return fmt.Errorf("source file does not exist: %s", sourceTarGZPath)
	}

	// Create output directory if it doesn't exist
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use tar to extract directly to the output directory
	// -x: extract
	// -z: decompress with gzip
	// -f: specify archive file
	// -C: change to directory
	output, err := c.executor.Execute(ctx, "tar", "-xzf", sourceTarGZPath, "-C", outputDir)
	if err != nil {
		return fmt.Errorf("tar extraction failed: %w, output: %s", err, string(output))
	}

	// No need for a temp file here as tar extracts directly to the output directory

	return nil
}

// CompressTarGZ compresses a directory to a tar.gz archive
func (c *CompressionOperations) CompressTarGZ(ctx context.Context, sourceDir, outputTarGZPath string) error {
	// Ensure source directory exists
	if _, err := c.executor.Execute(ctx, "test", "-d", sourceDir); err != nil {
		return fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputTarGZPath)
	if _, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate a temporary file path
	tempFile := filepath.Join(outputDir, fmt.Sprintf("%s.tmp.%d", filepath.Base(outputTarGZPath), time.Now().UnixNano()))

	// Get the parent directory and base name of the source directory
	parentDir := filepath.Dir(sourceDir)
	baseName := filepath.Base(sourceDir)

	// Use tar to create archive directly to the temp file
	// -c: create archive
	// -z: compress with gzip
	// -f: specify archive file
	cmd := fmt.Sprintf("cd %s && tar -czf %s %s", parentDir, tempFile, baseName)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return fmt.Errorf("tar compression failed: %w, output: %s", err, string(output))
	}

	// Verify the temp file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", tempFile); err != nil {
		return fmt.Errorf("compressed temp file not found at %s", tempFile)
	}

	// Move the temp file to final destination
	if _, err := c.executor.Execute(ctx, "mv", tempFile, outputTarGZPath); err != nil {
		// Try to clean up the temp file
		_, _ = c.executor.Execute(ctx, "rm", "-f", tempFile)
		return fmt.Errorf("failed to move temp file to destination: %w", err)
	}

	return nil
}
