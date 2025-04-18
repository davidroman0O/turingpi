package cache

import (
	"context"
	"io"
	"time"
)

// Metadata represents information about a cached item
type Metadata struct {
	Key         string
	Filename    string
	ContentType string
	Size        int64
	ModTime     time.Time
	Hash        string            // Optional SHA256
	Tags        map[string]string // User-defined tags
	OSType      string
	OSVersion   string
}

// Index represents the in-memory index of cached items
type Index struct {
	Items     map[string]*Metadata           // Key -> Metadata mapping
	TagIndex  map[string]map[string][]string // TagKey -> TagValue -> []Key mapping
	OSIndex   map[string]map[string][]string // OSType -> OSVersion -> []Key mapping
	UpdatedAt time.Time
}

// NewIndex creates a new empty index
func NewIndex() *Index {
	return &Index{
		Items:     make(map[string]*Metadata),
		TagIndex:  make(map[string]map[string][]string),
		OSIndex:   make(map[string]map[string][]string),
		UpdatedAt: time.Now(),
	}
}

// Cache defines the interface for interacting with different cache implementations
type Cache interface {
	// Put stores content in the cache with associated metadata
	Put(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error)

	// Get retrieves content and metadata from the cache
	// If getContent is false, only metadata is returned and the reader will be nil
	Get(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error)

	// Stat retrieves only the metadata for a cached item
	Stat(ctx context.Context, key string) (*Metadata, error)

	// Exists checks if an item exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// List returns metadata for all items matching the filter tags
	List(ctx context.Context, filterTags map[string]string) ([]Metadata, error)

	// Delete removes an item from the cache
	Delete(ctx context.Context, key string) error

	// Location returns the base location of the cache (e.g., directory path or remote URL)
	Location() string

	// Index returns the current cache index
	GetIndex(ctx context.Context) (*Index, error)

	// RebuildIndex forces a rebuild of the cache index
	RebuildIndex(ctx context.Context) error

	// Cleanup removes orphaned files and optionally performs deep cleanup of nested directories
	// Returns the number of cleaned files and any error encountered
	Cleanup(ctx context.Context, recursive bool) (int, error)

	// VerifyIntegrity checks the cache for inconsistencies
	// Returns a list of issues found and any error encountered
	// Issues can include:
	// - Orphaned .data files (no corresponding .meta)
	// - Orphaned .meta files (no corresponding .data)
	// - Corrupted metadata files
	// - Hash mismatches between stored and computed values
	VerifyIntegrity(ctx context.Context) ([]string, error)

	// Close releases any resources used by the cache
	Close() error
}

// IndexManager handles background indexing for a cache implementation
type IndexManager struct {
	cache       Cache
	index       *Index
	indexChan   chan struct{}
	stopChan    chan struct{}
	indexPeriod time.Duration
}

// NewIndexManager creates a new index manager for the given cache
func NewIndexManager(cache Cache, indexPeriod time.Duration) *IndexManager {
	return &IndexManager{
		cache:       cache,
		index:       NewIndex(),
		indexChan:   make(chan struct{}, 1),
		stopChan:    make(chan struct{}),
		indexPeriod: indexPeriod,
	}
}

// Start begins background indexing
func (im *IndexManager) Start(ctx context.Context) error {
	// Initial index build
	if err := im.cache.RebuildIndex(ctx); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(im.indexPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-im.stopChan:
				return
			case <-ticker.C:
				im.triggerIndex()
			case <-im.indexChan:
				if err := im.cache.RebuildIndex(ctx); err != nil {
					// Log error but continue
					// TODO: Add proper logging
					_ = err
				}
			}
		}
	}()

	return nil
}

// Stop halts background indexing
func (im *IndexManager) Stop() {
	close(im.stopChan)
}

// TriggerIndex requests an immediate index rebuild
func (im *IndexManager) triggerIndex() {
	select {
	case im.indexChan <- struct{}{}:
	default:
		// Index rebuild already pending
	}
}

// updateIndex adds or updates an item in the index
func (idx *Index) updateIndex(metadata *Metadata) {
	// Update main items map
	idx.Items[metadata.Key] = metadata

	// Update tag index
	for tagKey, tagValue := range metadata.Tags {
		if _, ok := idx.TagIndex[tagKey]; !ok {
			idx.TagIndex[tagKey] = make(map[string][]string)
		}
		if _, ok := idx.TagIndex[tagKey][tagValue]; !ok {
			idx.TagIndex[tagKey][tagValue] = []string{}
		}
		idx.TagIndex[tagKey][tagValue] = append(idx.TagIndex[tagKey][tagValue], metadata.Key)
	}

	// Update OS index
	if metadata.OSType != "" {
		if _, ok := idx.OSIndex[metadata.OSType]; !ok {
			idx.OSIndex[metadata.OSType] = make(map[string][]string)
		}
		if _, ok := idx.OSIndex[metadata.OSType][metadata.OSVersion]; !ok {
			idx.OSIndex[metadata.OSType][metadata.OSVersion] = []string{}
		}
		idx.OSIndex[metadata.OSType][metadata.OSVersion] = append(
			idx.OSIndex[metadata.OSType][metadata.OSVersion],
			metadata.Key,
		)
	}

	idx.UpdatedAt = time.Now()
}

// removeFromIndex removes an item from the index
func (idx *Index) removeFromIndex(key string) {
	metadata, exists := idx.Items[key]
	if !exists {
		return
	}

	// Remove from tag index
	for tagKey, tagValue := range metadata.Tags {
		if tagMap, ok := idx.TagIndex[tagKey]; ok {
			if keys, ok := tagMap[tagValue]; ok {
				newKeys := make([]string, 0, len(keys)-1)
				for _, k := range keys {
					if k != key {
						newKeys = append(newKeys, k)
					}
				}
				tagMap[tagValue] = newKeys
			}
		}
	}

	// Remove from OS index
	if metadata.OSType != "" {
		if osMap, ok := idx.OSIndex[metadata.OSType]; ok {
			if keys, ok := osMap[metadata.OSVersion]; ok {
				newKeys := make([]string, 0, len(keys)-1)
				for _, k := range keys {
					if k != key {
						newKeys = append(newKeys, k)
					}
				}
				osMap[metadata.OSVersion] = newKeys
			}
		}
	}

	// Remove from items map
	delete(idx.Items, key)
	idx.UpdatedAt = time.Now()
}
