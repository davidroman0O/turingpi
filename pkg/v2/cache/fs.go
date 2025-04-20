package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FSCache implements Cache interface using the local filesystem
type FSCache struct {
	baseDir  string
	mu       sync.RWMutex
	index    *Index
	indexMgr *IndexManager
}

// NewFSCache creates a new filesystem-based cache at the specified directory
func NewFSCache(baseDir string) (*FSCache, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &FSCache{
		baseDir: baseDir,
		index:   NewIndex(),
	}

	// Create index manager with 5-minute refresh interval
	cache.indexMgr = NewIndexManager(cache, 5*time.Minute)
	if err := cache.indexMgr.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start index manager: %w", err)
	}

	return cache, nil
}

// getMetadataPath returns the path where metadata file should be stored
func (c *FSCache) getMetadataPath(key string) string {
	return filepath.Join(c.baseDir, key+".meta")
}

// getContentPath returns the path where content file should be stored
func (c *FSCache) getContentPath(key string) string {
	return filepath.Join(c.baseDir, key+".data")
}

func (c *FSCache) Put(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Create content file
	contentPath := c.getContentPath(key)
	if err := os.MkdirAll(filepath.Dir(contentPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create content directory: %w", err)
	}

	contentFile, err := os.Create(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create content file: %w", err)
	}
	defer contentFile.Close()

	// Copy content and calculate hash
	if reader != nil {
		// Create a TeeReader to calculate hash while copying
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
	metadataPath := c.getMetadataPath(key)
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

	if err := json.NewEncoder(metadataFile).Encode(metadata); err != nil {
		os.Remove(contentPath)
		os.Remove(metadataPath)
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update index
	metadata.Key = key
	c.index.updateIndex(&metadata)

	return &metadata, nil
}

func (c *FSCache) Get(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	// Read metadata
	metadata, err := c.Stat(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	if !getContent {
		return metadata, nil, nil
	}

	// Open content file
	contentPath := c.getContentPath(key)
	content, err := os.Open(contentPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open content file: %w", err)
	}

	return metadata, content, nil
}

func (c *FSCache) Stat(ctx context.Context, key string) (*Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Read metadata file
	metadataPath := c.getMetadataPath(key)
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

func (c *FSCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	_, err := os.Stat(c.getMetadataPath(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *FSCache) List(ctx context.Context, filterTags map[string]string) ([]Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use index for efficient filtering
	var results []Metadata
	if len(filterTags) == 0 {
		// Return all items
		results = make([]Metadata, 0, len(c.index.Items))
		for _, meta := range c.index.Items {
			results = append(results, *meta)
		}
		return results, nil
	}

	// Find intersection of all tag filters
	var matchingKeys map[string]bool
	first := true

	for tagKey, tagValue := range filterTags {
		if tagMap, ok := c.index.TagIndex[tagKey]; ok {
			if keys, ok := tagMap[tagValue]; ok {
				if first {
					matchingKeys = make(map[string]bool)
					for _, key := range keys {
						matchingKeys[key] = true
					}
					first = false
				} else {
					// Intersect with existing matches
					newMatches := make(map[string]bool)
					for _, key := range keys {
						if matchingKeys[key] {
							newMatches[key] = true
						}
					}
					matchingKeys = newMatches
				}
			} else {
				return nil, nil // No matches for this tag value
			}
		} else {
			return nil, nil // No matches for this tag key
		}
	}

	// Convert matching keys to metadata list
	results = make([]Metadata, 0, len(matchingKeys))
	for key := range matchingKeys {
		if meta, ok := c.index.Items[key]; ok {
			results = append(results, *meta)
		}
	}

	return results, nil
}

func (c *FSCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Remove content and metadata files
	contentPath := c.getContentPath(key)
	metadataPath := c.getMetadataPath(key)

	if err := os.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove content file: %w", err)
	}
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove metadata file: %w", err)
	}

	// Update index
	c.index.removeFromIndex(key)

	return nil
}

func (c *FSCache) Location() string {
	return c.baseDir
}

func (c *FSCache) GetIndex(ctx context.Context) (*Index, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return c.index, nil
}

func (c *FSCache) RebuildIndex(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create new empty index
	newIndex := NewIndex()

	// Walk through all .meta files in the cache directory
	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-metadata files
		if info.IsDir() || !strings.HasSuffix(path, ".meta") {
			return nil
		}

		// Extract key from filename
		key := strings.TrimSuffix(filepath.Base(path), ".meta")

		// Read and parse metadata
		metadataFile, err := os.Open(path)
		if err != nil {
			return nil // Skip invalid files
		}
		defer metadataFile.Close()

		var metadata Metadata
		if err := json.NewDecoder(metadataFile).Decode(&metadata); err != nil {
			return nil // Skip invalid files
		}

		metadata.Key = key
		newIndex.updateIndex(&metadata)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	c.index = newIndex
	return nil
}

func (c *FSCache) Cleanup(ctx context.Context, recursive bool) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	cleaned := 0
	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if path == c.baseDir {
				return nil
			}
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}

		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".meta") && !strings.HasSuffix(base, ".data") {
			return nil
		}

		key := strings.TrimSuffix(base, filepath.Ext(base))
		metaPath := c.getMetadataPath(key)
		dataPath := c.getContentPath(key)

		metaExists := fileExists(metaPath)
		dataExists := fileExists(dataPath)

		if strings.HasSuffix(base, ".meta") && !dataExists {
			if err := os.Remove(path); err != nil {
				return err
			}
			cleaned++
		} else if strings.HasSuffix(base, ".data") && !metaExists {
			if err := os.Remove(path); err != nil {
				return err
			}
			cleaned++
		}

		return nil
	})

	if err != nil {
		return cleaned, fmt.Errorf("cleanup failed: %w", err)
	}

	return cleaned, nil
}

func (c *FSCache) VerifyIntegrity(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var issues []string

	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".meta") && !strings.HasSuffix(base, ".data") {
			return nil
		}

		key := strings.TrimSuffix(base, filepath.Ext(base))
		metaPath := c.getMetadataPath(key)
		dataPath := c.getContentPath(key)

		metaExists := fileExists(metaPath)
		dataExists := fileExists(dataPath)

		if strings.HasSuffix(base, ".meta") {
			if !dataExists {
				issues = append(issues, fmt.Sprintf("Orphaned metadata file: %s", path))
			} else {
				// Verify metadata content
				if metadata, err := c.Stat(ctx, key); err != nil {
					issues = append(issues, fmt.Sprintf("Invalid metadata file: %s - %v", path, err))
				} else if metadata.Hash != "" {
					// Verify content hash if available
					if hash, err := calculateFileHash(dataPath); err != nil {
						issues = append(issues, fmt.Sprintf("Failed to verify content hash: %s - %v", dataPath, err))
					} else if hash != metadata.Hash {
						issues = append(issues, fmt.Sprintf("Content hash mismatch: %s", dataPath))
					}
				}
			}
		} else if strings.HasSuffix(base, ".data") && !metaExists {
			issues = append(issues, fmt.Sprintf("Orphaned data file: %s", path))
		}

		return nil
	})

	if err != nil {
		return issues, fmt.Errorf("integrity check failed: %w", err)
	}

	return issues, nil
}

func (c *FSCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.indexMgr != nil {
		c.indexMgr.Stop()
	}
	return nil
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
