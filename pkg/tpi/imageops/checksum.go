package imageops

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const (
	// bufferSize is the size of the buffer used for reading files
	bufferSize = 32 * 1024 // 32KB buffer
)

// FileChecksum represents a file's checksum information
type FileChecksum struct {
	Path     string
	Hash     string
	Size     int64
	Modified int64
}

// CalculateFileChecksum computes the SHA-256 hash of a file
func CalculateFileChecksum(path string) (*FileChecksum, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	// Get file info for metadata
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for checksum: %w", err)
	}

	// Initialize hash
	hash := sha256.New()
	buffer := make([]byte, bufferSize)
	totalBytes := int64(0)

	// Read file in chunks and update hash
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading file for checksum: %w", err)
		}
		if bytesRead == 0 {
			break
		}

		totalBytes += int64(bytesRead)
		if _, err := hash.Write(buffer[:bytesRead]); err != nil {
			return nil, fmt.Errorf("error updating checksum: %w", err)
		}

		// Log progress for large files
		if info.Size() > 100*1024*1024 { // 100MB
			progress := float64(totalBytes) / float64(info.Size()) * 100
			if int64(progress)%10 == 0 { // Log every 10%
				log.Printf("Calculating checksum: %.0f%% complete", progress)
			}
		}
	}

	// Verify total bytes read matches file size
	if totalBytes != info.Size() {
		return nil, fmt.Errorf("checksum calculation incomplete: read %d bytes, expected %d", totalBytes, info.Size())
	}

	return &FileChecksum{
		Path:     path,
		Hash:     fmt.Sprintf("%x", hash.Sum(nil)),
		Size:     info.Size(),
		Modified: info.ModTime().Unix(),
	}, nil
}

// VerifyFileChecksum verifies that a file matches its expected checksum
func VerifyFileChecksum(path string, expectedHash string) (bool, error) {
	checksum, err := CalculateFileChecksum(path)
	if err != nil {
		return false, err
	}
	return checksum.Hash == expectedHash, nil
}

// CalculateDirectoryChecksums calculates checksums for all files in a directory
func CalculateDirectoryChecksums(dir string) (map[string]*FileChecksum, error) {
	checksums := make(map[string]*FileChecksum)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate checksum for file
		checksum, err := CalculateFileChecksum(path)
		if err != nil {
			return fmt.Errorf("error calculating checksum for %s: %w", path, err)
		}

		// Store relative path as key
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path for %s: %w", path, err)
		}

		checksums[relPath] = checksum
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return checksums, nil
}

// VerifyDirectoryChecksums verifies all files in a directory against expected checksums
func VerifyDirectoryChecksums(dir string, expectedChecksums map[string]*FileChecksum) ([]string, error) {
	var mismatches []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path for %s: %w", path, err)
		}

		// Check if file exists in expected checksums
		expected, exists := expectedChecksums[relPath]
		if !exists {
			mismatches = append(mismatches, fmt.Sprintf("%s: unexpected file", relPath))
			return nil
		}

		// Calculate actual checksum
		actual, err := CalculateFileChecksum(path)
		if err != nil {
			return fmt.Errorf("error calculating checksum for %s: %w", path, err)
		}

		// Compare checksums
		if actual.Hash != expected.Hash {
			mismatches = append(mismatches, fmt.Sprintf("%s: checksum mismatch", relPath))
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error verifying checksums: %w", err)
	}

	// Check for missing files
	for path := range expectedChecksums {
		fullPath := filepath.Join(dir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			mismatches = append(mismatches, fmt.Sprintf("%s: missing file", path))
		}
	}

	return mismatches, nil
}
