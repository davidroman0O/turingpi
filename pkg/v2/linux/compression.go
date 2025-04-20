package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Compression provides operations for handling Linux compression utilities
type Compression struct{}

// NewCompression creates a new Compression operations instance
func NewCompression() *Compression {
	return &Compression{}
}

// DecompressXZ decompresses an XZ-compressed file
func (c *Compression) DecompressXZ(ctx context.Context, sourceXZ, outputDir string) (string, error) {
	// Get the output path by removing .xz extension
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(sourceXZ), ".xz"))

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use xz command to decompress
	cmd := exec.Command("xz", "-d", "-k", "-f", sourceXZ, "-c")

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Set output to the created file
	cmd.Stdout = outFile

	// Run the command
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("xz decompression failed: %w", err)
	}

	return outputPath, nil
}

// CompressXZ compresses a file using XZ compression
func (c *Compression) CompressXZ(ctx context.Context, sourcePath, outputXZ string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputXZ), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use xz command to compress with maximum compression
	cmd := exec.Command("xz", "-9", "-k", "-f", sourcePath, "-c")

	// Create output file
	outFile, err := os.Create(outputXZ)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Set output to the created file
	cmd.Stdout = outFile

	// Run the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("xz compression failed: %w", err)
	}

	return nil
}

// DecompressGZ decompresses a GZ-compressed file
func (c *Compression) DecompressGZ(ctx context.Context, sourceGZ, outputDir string) (string, error) {
	// Get the output path by removing .gz extension
	outputPath := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(sourceGZ), ".gz"))

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use gunzip command to decompress
	cmd := exec.Command("gunzip", "-c", sourceGZ)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Set output to the created file
	cmd.Stdout = outFile

	// Run the command
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("gunzip decompression failed: %w", err)
	}

	return outputPath, nil
}

// CompressGZ compresses a file using GZ compression
func (c *Compression) CompressGZ(ctx context.Context, sourcePath, outputGZ string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputGZ), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use gzip command to compress
	cmd := exec.Command("gzip", "-9", "-c", sourcePath)

	// Create output file
	outFile, err := os.Create(outputGZ)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Set output to the created file
	cmd.Stdout = outFile

	// Run the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("gzip compression failed: %w", err)
	}

	return nil
}
