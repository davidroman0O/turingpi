package tools

import (
	"context"
	"io"

	"github.com/davidroman0O/turingpi/pkg/v2/cache"
)

// CacheToolImpl is the implementation of the CacheTool interface
type CacheToolImpl struct {
	cache cache.Cache
}

// NewCacheTool creates a new CacheTool
func NewCacheTool(cache cache.Cache) CacheTool {
	return &CacheToolImpl{
		cache: cache,
	}
}

// Put stores content in the cache with associated metadata
func (t *CacheToolImpl) Put(ctx context.Context, key string, metadata cache.Metadata, reader io.Reader) (*cache.Metadata, error) {
	return t.cache.Put(ctx, key, metadata, reader)
}

// Get retrieves content and metadata from the cache
func (t *CacheToolImpl) Get(ctx context.Context, key string) (*cache.Metadata, io.ReadCloser, error) {
	return t.cache.Get(ctx, key, true)
}

// Exists checks if an item exists in the cache
func (t *CacheToolImpl) Exists(ctx context.Context, key string) (bool, error) {
	return t.cache.Exists(ctx, key)
}

// List returns metadata for all items matching the filter tags
func (t *CacheToolImpl) List(ctx context.Context, filterTags map[string]string) ([]cache.Metadata, error) {
	return t.cache.List(ctx, filterTags)
}

// Delete removes an item from the cache
func (t *CacheToolImpl) Delete(ctx context.Context, key string) error {
	return t.cache.Delete(ctx, key)
}
