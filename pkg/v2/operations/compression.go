package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

	// Use xz to decompress
	// Execute via bash to handle output redirection
	bashCmd := fmt.Sprintf("xz -d -c '%s' > '%s'", sourceXZPath, outputPath)
	output, err := c.executor.Execute(ctx, "bash", "-c", bashCmd)
	if err != nil {
		return "", fmt.Errorf("xz decompression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputPath); err != nil {
		return "", fmt.Errorf("decompressed file not found at %s", outputPath)
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

	// Use xz to compress with high compression level
	// Execute via bash to handle output redirection
	bashCmd := fmt.Sprintf("xz -9 -c '%s' > '%s'", sourcePath, outputXZPath)
	output, err := c.executor.Execute(ctx, "bash", "-c", bashCmd)
	if err != nil {
		return fmt.Errorf("xz compression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputXZPath); err != nil {
		return fmt.Errorf("compressed file not found at %s", outputXZPath)
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

	// Use gunzip to decompress
	// Execute via bash to handle output redirection
	bashCmd := fmt.Sprintf("gunzip -c '%s' > '%s'", sourceGZPath, outputPath)
	output, err := c.executor.Execute(ctx, "bash", "-c", bashCmd)
	if err != nil {
		return "", fmt.Errorf("gunzip decompression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputPath); err != nil {
		return "", fmt.Errorf("decompressed file not found at %s", outputPath)
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

	// Use gzip to compress
	// Execute via bash to handle output redirection
	bashCmd := fmt.Sprintf("gzip -9 -c '%s' > '%s'", sourcePath, outputGZPath)
	output, err := c.executor.Execute(ctx, "bash", "-c", bashCmd)
	if err != nil {
		return fmt.Errorf("gzip compression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputGZPath); err != nil {
		return fmt.Errorf("compressed file not found at %s", outputGZPath)
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

	// Use tar to extract
	output, err := c.executor.Execute(ctx, "tar", "-xzf", sourceTarGZPath, "-C", outputDir)
	if err != nil {
		return fmt.Errorf("tar extraction failed: %w, output: %s", err, string(output))
	}

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

	// Get the parent directory and base name of the source directory
	parentDir := filepath.Dir(sourceDir)
	baseName := filepath.Base(sourceDir)

	// Use tar to create archive
	// We need to cd to the parent directory to avoid including the full path in the archive
	cmd := fmt.Sprintf("cd '%s' && tar -czf '%s' '%s'", parentDir, outputTarGZPath, baseName)
	output, err := c.executor.Execute(ctx, "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("tar compression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputTarGZPath); err != nil {
		return fmt.Errorf("compressed file not found at %s", outputTarGZPath)
	}

	return nil
}
