package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// TempFSCache extends FSCache with temporary directory support
// It will automatically delete the cache directory on Close()
type TempFSCache struct {
	*FSCache
	isTemp      bool
	cleanupPath string
	mu          sync.RWMutex
	isClosed    bool
}

// NewTempFSCache creates a new temporary cache with automatic cleanup
// If basePath is empty, it will use os.TempDir() to create a unique temporary directory
// If basePath is provided, it will create the temporary directory inside that path
func NewTempFSCache(basePath string) (*TempFSCache, error) {
	var tempDir string
	var err error

	// // Create a temporary directory either in the system's temp dir or in the specified basePath
	// if basePath == "" {
	// 	tempDir, err = os.MkdirTemp("", "turingpi-cache-*")
	// } else {
	// 	// Ensure the base path exists
	// 	if err := os.MkdirAll(basePath, 0755); err != nil {
	// 		return nil, fmt.Errorf("failed to create base directory for temp cache: %w", err)
	// 	}
	// 	tempDir, err = os.MkdirTemp(basePath, "turingpi-cache-*")
	// }

	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	// }

	tempDir, err = filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for temp directory: %w", err)
	}

	// Create the underlying FSCache
	fsCache, err := NewFSCache(tempDir)
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on initialization error
		return nil, fmt.Errorf("failed to create filesystem cache: %w", err)
	}

	return &TempFSCache{
		FSCache:     fsCache,
		isTemp:      true,
		cleanupPath: tempDir,
		isClosed:    false,
	}, nil
}

// Close stops the index manager and removes the temporary directory if this is a temporary cache
func (c *TempFSCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already closed to prevent double close
	if c.isClosed {
		return nil
	}

	// Mark as closed immediately to prevent concurrent calls
	c.isClosed = true

	// First close the underlying FSCache (stops the index manager)
	if err := c.FSCache.Close(); err != nil {
		return err
	}

	// If this is a temporary cache, clean up the directory
	if c.isTemp && c.cleanupPath != "" {
		// Recursively remove all files in the temporary directory
		if err := os.RemoveAll(c.cleanupPath); err != nil {
			return fmt.Errorf("failed to clean up temporary cache directory: %w", err)
		}
	}

	return nil
}

// CleanupPath returns the path that will be cleaned up when Close() is called
func (c *TempFSCache) CleanupPath() string {
	return c.cleanupPath
}

// BaseDir returns the base directory of the cache
func (c *TempFSCache) BaseDir() string {
	return c.baseDir
}

// RegisterCleanupOnExit sets up the cache to be cleaned up when the program exits
// This should be called right after creating the cache to ensure cleanup
func RegisterCleanupOnExit(cache *TempFSCache) {
	if cache == nil || !cache.isTemp {
		return
	}

	// Get the path now in case it's needed later
	cleanupPath := cache.cleanupPath

	// Set up a cleanup handler to be called when the program exits
	c := make(chan os.Signal, 1)

	// Register for common termination signals
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// This will ensure cleanup even if the program crashes or is terminated
	go func() {
		<-c
		if cleanupPath != "" {
			// Attempt to clean up the directory
			os.RemoveAll(cleanupPath)
		}
		os.Exit(1)
	}()
}

// CreateTempCache is a convenience function that creates a temporary cache and registers it for cleanup
func CreateTempCache(basePath string) (*TempFSCache, error) {
	cache, err := NewTempFSCache(basePath)
	if err != nil {
		return nil, err
	}

	RegisterCleanupOnExit(cache)
	return cache, nil
}

// File and folder operations for TempFSCache

// CreateTempDir creates a new temporary directory inside the cache
// Returns the absolute path to the created directory
func (c *TempFSCache) CreateTempDir(ctx context.Context, prefix string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if c.isClosed {
		return "", fmt.Errorf("cache is closed")
	}

	tempDir, err := os.MkdirTemp(c.cleanupPath, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return tempDir, nil
}

// CreateDir creates a directory at the specified path relative to the cache root
func (c *TempFSCache) CreateDir(ctx context.Context, relPath string, perm os.FileMode) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	fullPath := filepath.Join(c.cleanupPath, relPath)
	return os.MkdirAll(fullPath, perm)
}

// WriteFile writes data to a file at the specified path relative to the cache root
func (c *TempFSCache) WriteFile(ctx context.Context, relPath string, data []byte, perm os.FileMode) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	fullPath := filepath.Join(c.cleanupPath, relPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	return os.WriteFile(fullPath, data, perm)
}

// ReadFile reads the content of a file at the specified path relative to the cache root
func (c *TempFSCache) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, fmt.Errorf("cache is closed")
	}

	fullPath := filepath.Join(c.cleanupPath, relPath)
	return os.ReadFile(fullPath)
}

// FileExists checks if a file exists at the specified path relative to the cache root
func (c *TempFSCache) FileExists(ctx context.Context, relPath string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	if c.isClosed {
		return false, fmt.Errorf("cache is closed")
	}

	fullPath := filepath.Join(c.cleanupPath, relPath)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemoveFile removes a file at the specified path relative to the cache root
func (c *TempFSCache) RemoveFile(ctx context.Context, relPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	fullPath := filepath.Join(c.cleanupPath, relPath)
	return os.Remove(fullPath)
}

// CopyFile copies a file from the specified source path to the destination path
// Both paths are relative to the cache root
func (c *TempFSCache) CopyFile(ctx context.Context, srcRelPath, dstRelPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	srcPath := filepath.Join(c.cleanupPath, srcRelPath)
	dstPath := filepath.Join(c.cleanupPath, dstRelPath)

	// Ensure source file exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source file error: %w", err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, not a file")
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the content
	if _, err = io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Preserve file mode
	return os.Chmod(dstPath, srcInfo.Mode())
}

// CopyFromExternalPath copies a file from an external path into the cache
func (c *TempFSCache) CopyFromExternalPath(ctx context.Context, externalPath, dstRelPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	dstPath := filepath.Join(c.cleanupPath, dstRelPath)

	// Ensure source file exists
	srcInfo, err := os.Stat(externalPath)
	if err != nil {
		return fmt.Errorf("external file error: %w", err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("external source is a directory, not a file")
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	src, err := os.Open(externalPath)
	if err != nil {
		return fmt.Errorf("failed to open external file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the content
	if _, err = io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Preserve file mode
	return os.Chmod(dstPath, srcInfo.Mode())
}

// WalkFiles walks the file tree rooted at the cache root, calling the provided function
// for each file or directory in the tree, including the root.
func (c *TempFSCache) WalkFiles(ctx context.Context, fn filepath.WalkFunc) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	return filepath.Walk(c.cleanupPath, fn)
}

// GetAbsolutePath returns the absolute path for a relative path within the cache
func (c *TempFSCache) GetAbsolutePath(relPath string) string {
	return filepath.Join(c.cleanupPath, relPath)
}

// Additional cache-specific functions that mirror FSCache functionality

// GetTempMetadataPath returns the path where metadata file should be stored for a given key
func (c *TempFSCache) GetTempMetadataPath(key string) string {
	return filepath.Join(c.cleanupPath, key+".meta")
}

// GetTempContentPath returns the path where content file should be stored for a given key
func (c *TempFSCache) GetTempContentPath(key string) string {
	return filepath.Join(c.cleanupPath, key+".data")
}

// PutTemp creates a new cache entry in the temporary cache
// Similar to Put but stores files directly in the temp directory
func (c *TempFSCache) PutTemp(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, fmt.Errorf("cache is closed")
	}

	// Create content file
	contentPath := c.GetTempContentPath(key)
	if err := os.MkdirAll(filepath.Dir(contentPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create content directory: %w", err)
	}

	if reader != nil {
		contentFile, err := os.Create(contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create content file: %w", err)
		}
		defer contentFile.Close()

		// Copy content and calculate hash
		hash := sha256.New()
		teeReader := io.TeeReader(reader, hash)

		if _, err := io.Copy(contentFile, teeReader); err != nil {
			os.Remove(contentPath)
			return nil, fmt.Errorf("failed to write content: %w", err)
		}

		if metadata.Hash == "" {
			metadata.Hash = hex.EncodeToString(hash.Sum(nil))
		}
	}

	// Write metadata
	metadataPath := c.GetTempMetadataPath(key)
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		os.Remove(contentPath)
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	metadataFile, err := os.Create(metadataPath)
	if err != nil {
		os.Remove(contentPath)
		return nil, fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metadataFile.Close()

	metadata.Key = key
	if err := json.NewEncoder(metadataFile).Encode(metadata); err != nil {
		os.Remove(contentPath)
		os.Remove(metadataPath)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return &metadata, nil
}

// GetTemp retrieves content and metadata from the temporary cache
func (c *TempFSCache) GetTemp(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, nil, fmt.Errorf("cache is closed")
	}

	// Read metadata
	metadata, err := c.StatTemp(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	if !getContent {
		return metadata, nil, nil
	}

	// Open content file
	contentPath := c.GetTempContentPath(key)
	content, err := os.Open(contentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open content file: %w", err)
	}

	return metadata, content, nil
}

// StatTemp retrieves only the metadata for a cached item in the temporary cache
func (c *TempFSCache) StatTemp(ctx context.Context, key string) (*Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, fmt.Errorf("cache is closed")
	}

	// Read metadata file
	metadataPath := c.GetTempMetadataPath(key)
	metadataFile, err := os.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer metadataFile.Close()

	var metadata Metadata
	if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return &metadata, nil
}

// ExistsTemp checks if an item exists in the temporary cache
func (c *TempFSCache) ExistsTemp(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	if c.isClosed {
		return false, fmt.Errorf("cache is closed")
	}

	_, err := os.Stat(c.GetTempMetadataPath(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ListTemp returns metadata for all items matching the filter tags in the temporary cache
func (c *TempFSCache) ListTemp(ctx context.Context, filterTags map[string]string) ([]Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, fmt.Errorf("cache is closed")
	}

	var results []Metadata

	// Walk through the temporary directory to find .meta files
	err := filepath.Walk(c.cleanupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Only process .meta files
		if !info.IsDir() && strings.HasSuffix(path, ".meta") {
			// Read metadata file
			metadataFile, err := os.Open(path)
			if err != nil {
				// Skip this file if it can't be opened
				return nil
			}
			defer metadataFile.Close()

			var metadata Metadata
			if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
				// Skip this file if it can't be decoded
				return nil
			}

			// Get key from filename
			rel, err := filepath.Rel(c.cleanupPath, path)
			if err == nil {
				metadata.Key = strings.TrimSuffix(rel, ".meta")
			}

			// Check if it matches the filter tags
			matches := true
			for k, v := range filterTags {
				if tagVal, ok := metadata.Tags[k]; !ok || tagVal != v {
					matches = false
					break
				}
			}

			if matches {
				results = append(results, metadata)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list cache files: %w", err)
	}

	return results, nil
}

// DeleteTemp removes an item from the temporary cache
func (c *TempFSCache) DeleteTemp(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if c.isClosed {
		return fmt.Errorf("cache is closed")
	}

	// Remove both metadata and content files
	metadataPath := c.GetTempMetadataPath(key)
	contentPath := c.GetTempContentPath(key)

	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove metadata file: %w", err)
	}

	if err := os.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove content file: %w", err)
	}

	return nil
}

// CleanupTemp removes orphaned files in the temporary cache
// Returns the number of cleaned files and any error encountered
func (c *TempFSCache) CleanupTemp(ctx context.Context, recursive bool) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	if c.isClosed {
		return 0, fmt.Errorf("cache is closed")
	}

	cleanedCount := 0
	var emptyDirs []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories unless recursive is true
		if info.IsDir() {
			if !recursive && path != c.cleanupPath {
				return filepath.SkipDir
			}
			// Store directory for later empty check
			if path != c.cleanupPath {
				emptyDirs = append(emptyDirs, path)
			}
			return nil
		}

		// Process only .data files
		if filepath.Ext(path) == ".data" {
			// Check if corresponding .meta file exists
			metaPath := strings.TrimSuffix(path, ".data") + ".meta"
			if _, err := os.Stat(metaPath); os.IsNotExist(err) {
				// No metadata file exists, remove the orphaned data file
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("failed to remove orphaned file %s: %w", path, err)
				}
				cleanedCount++
			}
		}

		return nil
	}

	err := filepath.Walk(c.cleanupPath, walkFn)
	if err != nil {
		return cleanedCount, fmt.Errorf("cleanup walk failed: %w", err)
	}

	// Clean up empty directories from deepest to shallowest
	if recursive {
		// Sort directories by depth (deepest first)
		sort.Slice(emptyDirs, func(i, j int) bool {
			return len(strings.Split(emptyDirs[i], string(os.PathSeparator))) > len(strings.Split(emptyDirs[j], string(os.PathSeparator)))
		})

		for _, dir := range emptyDirs {
			// Check if directory is empty
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue // Skip if can't read directory
			}
			if len(entries) == 0 {
				if err := os.Remove(dir); err != nil {
					continue // Skip if can't remove directory
				}
				cleanedCount++
			}
		}
	}

	return cleanedCount, nil
}

// VerifyTempIntegrity checks for file integrity issues in the temporary cache
func (c *TempFSCache) VerifyTempIntegrity(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.isClosed {
		return nil, fmt.Errorf("cache is closed")
	}

	var issues []string

	// Walk through all files in the temporary cache directory
	err := filepath.Walk(c.cleanupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(c.cleanupPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".meta":
			// Check if corresponding .data file exists
			dataPath := strings.TrimSuffix(path, ".meta") + ".data"
			if _, err := os.Stat(dataPath); os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("orphaned metadata file: %s (no corresponding .data file)", relPath))
				return nil
			}

			// Check if metadata is valid
			metadataFile, err := os.Open(path)
			if err != nil {
				issues = append(issues, fmt.Sprintf("failed to open metadata file: %s: %v", relPath, err))
				return nil
			}
			defer metadataFile.Close()

			var metadata Metadata
			if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
				issues = append(issues, fmt.Sprintf("corrupted metadata file: %s: %v", relPath, err))
				return nil
			}

			// If hash is present, verify it
			if metadata.Hash != "" {
				dataFile, err := os.Open(dataPath)
				if err != nil {
					issues = append(issues, fmt.Sprintf("failed to open data file for hash verification: %s: %v", strings.TrimSuffix(relPath, ".meta")+".data", err))
					return nil
				}
				defer dataFile.Close()

				hash := sha256.New()
				if _, err := io.Copy(hash, dataFile); err != nil {
					issues = append(issues, fmt.Sprintf("failed to read data file for hash verification: %s: %v", strings.TrimSuffix(relPath, ".meta")+".data", err))
					return nil
				}

				computedHash := hex.EncodeToString(hash.Sum(nil))
				if computedHash != metadata.Hash {
					issues = append(issues, fmt.Sprintf("hash mismatch for %s: stored=%s computed=%s", strings.TrimSuffix(relPath, ".meta"), metadata.Hash, computedHash))
				}
			}

		case ".data":
			// Check if corresponding .meta file exists
			metaPath := strings.TrimSuffix(path, ".data") + ".meta"
			if _, err := os.Stat(metaPath); os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("orphaned data file: %s (no corresponding .meta file)", relPath))
			}
		}

		return nil
	})

	if err != nil {
		return issues, fmt.Errorf("integrity check walk failed: %w", err)
	}

	return issues, nil
}
