package operations

import (
	"context"
	"fmt"
	"os"
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
	// Add debug output to inspect paths
	fmt.Printf("DEBUG DecompressXZ: sourceXZPath=%s, outputDir=%s\n", sourceXZPath, outputDir)

	// Ensure source file exists
	testOutput, err := c.executor.Execute(ctx, "test", "-f", sourceXZPath)
	if err != nil {
		// Add debug output for test failure
		fmt.Printf("DEBUG DecompressXZ: source file test failed: %v, output: %s\n", err, string(testOutput))

		// Try to list the directory to debug
		lsOutput, lsErr := c.executor.Execute(ctx, "ls", "-la", filepath.Dir(sourceXZPath))
		if lsErr == nil {
			fmt.Printf("DEBUG DecompressXZ: directory contents: %s\n", string(lsOutput))
		} else {
			fmt.Printf("DEBUG DecompressXZ: ls failed: %v\n", lsErr)
		}

		return "", fmt.Errorf("source file does not exist: %s", sourceXZPath)
	}

	fmt.Printf("DEBUG DecompressXZ: Source file exists\n")

	// Create output directory if it doesn't exist
	mkdirOutput, err := c.executor.Execute(ctx, "mkdir", "-p", outputDir)
	if err != nil {
		fmt.Printf("DEBUG DecompressXZ: mkdir failed: %v, output: %s\n", err, string(mkdirOutput))
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output file path
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(sourceXZPath), ".xz"))
	fmt.Printf("DEBUG DecompressXZ: outputPath=%s\n", outputPath)

	// Use xz directly instead of through bash, more compatible with Alpine/BusyBox
	// First try the direct command
	xzOutput, err := c.executor.Execute(ctx, "xz", "-d", "-k", "-f", sourceXZPath)
	if err == nil {
		fmt.Printf("DEBUG DecompressXZ: xz command succeeded\n")
		// Direct command worked, check if file exists at expected location
		testOutput, testErr := c.executor.Execute(ctx, "test", "-f", strings.TrimSuffix(sourceXZPath, ".xz"))
		if testErr == nil {
			fmt.Printf("DEBUG DecompressXZ: Decompressed file exists at %s\n", strings.TrimSuffix(sourceXZPath, ".xz"))
			// If decompression created file in source directory, move it to target
			if outputPath != strings.TrimSuffix(sourceXZPath, ".xz") {
				mvOutput, err := c.executor.Execute(ctx, "mv", strings.TrimSuffix(sourceXZPath, ".xz"), outputPath)
				if err != nil {
					fmt.Printf("DEBUG DecompressXZ: mv failed: %v, output: %s\n", err, string(mvOutput))
					return "", fmt.Errorf("failed to move decompressed file: %w", err)
				}
				fmt.Printf("DEBUG DecompressXZ: Moved file to %s\n", outputPath)
			}
			return outputPath, nil
		} else {
			fmt.Printf("DEBUG DecompressXZ: Decompressed file test failed: %v, output: %s\n", testErr, string(testOutput))
		}
	} else {
		fmt.Printf("DEBUG DecompressXZ: direct xz command failed: %v, output: %s\n", err, string(xzOutput))
	}

	// Try alternate approach without -f flag
	xzOutput, err = c.executor.Execute(ctx, "xz", "--decompress", "--keep", "--stdout", sourceXZPath)
	if err == nil && len(xzOutput) > 0 {
		fmt.Printf("DEBUG DecompressXZ: xz with stdout succeeded, output size: %d bytes\n", len(xzOutput))
		// Create a temporary file with the content
		tempFile := fmt.Sprintf("%s.tmp.%d", outputPath, time.Now().UnixNano())

		// Write the output to a file
		if err := os.WriteFile(tempFile, xzOutput, 0644); err != nil {
			fmt.Printf("DEBUG DecompressXZ: Failed to write temp file: %v\n", err)
			return "", fmt.Errorf("failed to write decompressed content: %w", err)
		}

		// Move the file to its final destination
		if err := os.Rename(tempFile, outputPath); err != nil {
			fmt.Printf("DEBUG DecompressXZ: Failed to rename temp file: %v\n", err)
			return "", fmt.Errorf("failed to move temp file to destination: %w", err)
		}

		fmt.Printf("DEBUG DecompressXZ: Successfully wrote decompressed file to %s\n", outputPath)
		return outputPath, nil
	} else if err != nil {
		fmt.Printf("DEBUG DecompressXZ: xz stdout approach failed: %v\n", err)
	}

	// If direct approach failed, try with shell redirection as fallback
	// Use sh instead of bash for better compatibility with minimal containers
	shCmd := fmt.Sprintf("xz -d -c %s > %s", sourceXZPath, outputPath)
	fmt.Printf("DEBUG DecompressXZ: Trying shell command: %s\n", shCmd)
	output, err := c.executor.Execute(ctx, "sh", "-c", shCmd)
	if err != nil {
		fmt.Printf("DEBUG DecompressXZ: sh command failed: %v, output: %s\n", err, string(output))
		return "", fmt.Errorf("xz decompression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	testOutput, err = c.executor.Execute(ctx, "test", "-f", outputPath)
	if err != nil {
		fmt.Printf("DEBUG DecompressXZ: final file test failed: %v, output: %s\n", err, string(testOutput))
		return "", fmt.Errorf("decompressed file not found at %s", outputPath)
	}

	fmt.Printf("DEBUG DecompressXZ: Successfully decompressed to %s\n", outputPath)
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

	// First try direct command with xz that works on most systems
	_, err := c.executor.Execute(ctx, "xz", "-9", "-k", "-c", sourcePath)
	if err == nil {
		// If xz command succeeded but file is in wrong location, handle that case
		if _, err := c.executor.Execute(ctx, "test", "-f", sourcePath+".xz"); err == nil {
			// Move from source+.xz to target path if different
			if sourcePath+".xz" != outputXZPath {
				_, err := c.executor.Execute(ctx, "mv", sourcePath+".xz", outputXZPath)
				if err != nil {
					return fmt.Errorf("failed to move compressed file: %w", err)
				}
			}
			return nil
		}
	}

	// Fallback to using shell redirection if direct command didn't work
	shCmd := fmt.Sprintf("xz -9 -c %s > %s", sourcePath, outputXZPath)
	output, err := c.executor.Execute(ctx, "sh", "-c", shCmd)
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

	// Try direct gunzip command first (works on most systems)
	_, err := c.executor.Execute(ctx, "gunzip", "-k", "-f", sourceGZPath)
	if err == nil {
		// Check if file exists at expected location
		if _, err := c.executor.Execute(ctx, "test", "-f", strings.TrimSuffix(sourceGZPath, ".gz")); err == nil {
			// If decompression created file in source directory, move it to target
			if outputPath != strings.TrimSuffix(sourceGZPath, ".gz") {
				_, err := c.executor.Execute(ctx, "mv", strings.TrimSuffix(sourceGZPath, ".gz"), outputPath)
				if err != nil {
					return "", fmt.Errorf("failed to move decompressed file: %w", err)
				}
			}
			return outputPath, nil
		}
	}

	// Fallback to shell redirection if direct command failed
	shCmd := fmt.Sprintf("gunzip -c %s > %s", sourceGZPath, outputPath)
	output, err := c.executor.Execute(ctx, "sh", "-c", shCmd)
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

	// Try direct gzip command first (works on most systems)
	_, err := c.executor.Execute(ctx, "gzip", "-9", "-k", "-c", sourcePath)
	if err == nil {
		// If gzip command succeeded but file is in wrong location, handle that case
		if _, err := c.executor.Execute(ctx, "test", "-f", sourcePath+".gz"); err == nil {
			// Move from source+.gz to target path if different
			if sourcePath+".gz" != outputGZPath {
				_, err := c.executor.Execute(ctx, "mv", sourcePath+".gz", outputGZPath)
				if err != nil {
					return fmt.Errorf("failed to move compressed file: %w", err)
				}
			}
			return nil
		}
	}

	// Fallback to shell redirection if direct command didn't work
	shCmd := fmt.Sprintf("gzip -9 -c %s > %s", sourcePath, outputGZPath)
	output, err := c.executor.Execute(ctx, "sh", "-c", shCmd)
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

	// Use tar to create archive, but with sh instead of bash for wider compatibility
	cmd := fmt.Sprintf("cd %s && tar -czf %s %s", parentDir, outputTarGZPath, baseName)
	output, err := c.executor.Execute(ctx, "sh", "-c", cmd)
	if err != nil {
		return fmt.Errorf("tar compression failed: %w, output: %s", err, string(output))
	}

	// Verify the file was created successfully
	if _, err := c.executor.Execute(ctx, "test", "-f", outputTarGZPath); err != nil {
		return fmt.Errorf("compressed file not found at %s", outputTarGZPath)
	}

	return nil
}
