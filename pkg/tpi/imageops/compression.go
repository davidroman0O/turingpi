package imageops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DecompressImageXZ decompresses an XZ-compressed image file
func DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	// Create output filename
	baseFilename := filepath.Base(sourceImgXZAbs)
	if len(baseFilename) < 4 || baseFilename[len(baseFilename)-3:] != ".xz" {
		return "", fmt.Errorf("source file must have .xz extension")
	}

	decompressedPath := filepath.Join(tmpDir, baseFilename[:len(baseFilename)-3])

	// Check if source file exists
	if _, err := os.Stat(sourceImgXZAbs); err != nil {
		return "", fmt.Errorf("source file not found: %w", err)
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Use xz to decompress
	cmd := exec.Command("xz", "--decompress", "--keep", "--force", sourceImgXZAbs)
	if _, err := runCommand(cmd); err != nil {
		return "", fmt.Errorf("failed to decompress image: %w", err)
	}

	// Move decompressed file to temp directory
	decompressedSourcePath := sourceImgXZAbs[:len(sourceImgXZAbs)-3]
	if err := os.Rename(decompressedSourcePath, decompressedPath); err != nil {
		return "", fmt.Errorf("failed to move decompressed file: %w", err)
	}

	return decompressedPath, nil
}

// RecompressImageXZ compresses an image file using XZ compression
func RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(finalXZPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy the modified image to the final location
	if err := copyFile(modifiedImgPath, finalXZPath[:len(finalXZPath)-3]); err != nil {
		return fmt.Errorf("failed to copy modified image: %w", err)
	}

	// Compress the image
	cmd := exec.Command("xz", "--compress", "--force", finalXZPath[:len(finalXZPath)-3])
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to compress image: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, input, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}
