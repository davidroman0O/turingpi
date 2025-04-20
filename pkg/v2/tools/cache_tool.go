package tools

import (
	"context"
	"io"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/cache"
)

// CacheToolImpl is the implementation of the CacheTool interface
type CacheToolImpl struct {
	cacheProvider cache.Cache
	cacheType     CacheType
}

// NewCacheTool creates a new CacheTool
func NewCacheTool(cacheProvider cache.Cache) CacheTool {
	return &CacheToolImpl{
		cacheProvider: cacheProvider,
		cacheType:     LocalCacheType, // Default to local type
	}
}

// Put stores content in the cache with associated metadata
func (t *CacheToolImpl) Put(ctx context.Context, key string, metadata cache.Metadata, reader io.Reader) (*cache.Metadata, error) {
	return t.cacheProvider.Put(ctx, key, metadata, reader)
}

// Get retrieves content and metadata from the cache
func (t *CacheToolImpl) Get(ctx context.Context, key string, getContent bool) (*cache.Metadata, io.ReadCloser, error) {
	return t.cacheProvider.Get(ctx, key, getContent)
}

// Stat retrieves only the metadata for a cached item
func (t *CacheToolImpl) Stat(ctx context.Context, key string) (*cache.Metadata, error) {
	return t.cacheProvider.Stat(ctx, key)
}

// Exists checks if an item exists in the cache
func (t *CacheToolImpl) Exists(ctx context.Context, key string) (bool, error) {
	return t.cacheProvider.Exists(ctx, key)
}

// List returns metadata for all items matching the filter tags
func (t *CacheToolImpl) List(ctx context.Context, filterTags map[string]string) ([]cache.Metadata, error) {
	return t.cacheProvider.List(ctx, filterTags)
}

// Delete removes an item from the cache
func (t *CacheToolImpl) Delete(ctx context.Context, key string) error {
	return t.cacheProvider.Delete(ctx, key)
}

// Location returns the base location of the cache
func (t *CacheToolImpl) Location() string {
	return t.cacheProvider.Location()
}

// GetIndex returns the current cache index
func (t *CacheToolImpl) GetIndex(ctx context.Context) (*cache.Index, error) {
	return t.cacheProvider.GetIndex(ctx)
}

// RebuildIndex forces a rebuild of the cache index
func (t *CacheToolImpl) RebuildIndex(ctx context.Context) error {
	return t.cacheProvider.RebuildIndex(ctx)
}

// VerifyIntegrity checks the cache for inconsistencies
func (t *CacheToolImpl) VerifyIntegrity(ctx context.Context) ([]string, error) {
	return t.cacheProvider.VerifyIntegrity(ctx)
}

// Close releases any resources used by the cache
func (t *CacheToolImpl) Close() error {
	return t.cacheProvider.Close()
}

// GetType returns the cache type
func (t *CacheToolImpl) GetType() CacheType {
	return t.cacheType
}

// IsRemote returns true if the cache is remote
func (t *CacheToolImpl) IsRemote() bool {
	return t.cacheType == RemoteCacheType
}

// LocalCacheToolImpl is the implementation of the LocalCacheTool interface
// which uses FSCache as its underlying cache implementation
type LocalCacheToolImpl struct {
	CacheToolImpl
	fsCache *cache.FSCache
}

// NewLocalCacheTool creates a new LocalCacheTool
func NewLocalCacheTool(fsCache *cache.FSCache) LocalCacheTool {
	return &LocalCacheToolImpl{
		CacheToolImpl: CacheToolImpl{
			cacheProvider: fsCache,
			cacheType:     LocalCacheType,
		},
		fsCache: fsCache,
	}
}

// Cleanup removes orphaned files and optionally performs deep cleanup
func (t *LocalCacheToolImpl) Cleanup(ctx context.Context, recursive bool) (int, error) {
	return t.fsCache.Cleanup(ctx, recursive)
}

// GetPath returns the absolute filesystem path of the cache
func (t *LocalCacheToolImpl) GetPath() string {
	// FSCache.Location() returns the base directory path
	return t.fsCache.Location()
}

// RemoteCacheToolImpl is the implementation of the RemoteCacheTool interface
// which uses SSHCache as its underlying cache implementation
type RemoteCacheToolImpl struct {
	CacheToolImpl
	nodeID         int
	nodeTool       NodeTool
	remotePath     string
	connectionInfo *RemoteCacheConnectionInfo
}

// NewRemoteCacheTool creates a new RemoteCacheTool
func NewRemoteCacheTool(nodeID int, nodeTool NodeTool, remoteCache cache.Cache, remotePath string) RemoteCacheTool {
	return &RemoteCacheToolImpl{
		CacheToolImpl: CacheToolImpl{
			cacheProvider: remoteCache,
			cacheType:     RemoteCacheType,
		},
		nodeID:     nodeID,
		nodeTool:   nodeTool,
		remotePath: remotePath,
		connectionInfo: &RemoteCacheConnectionInfo{
			RemotePath:   remotePath,
			LastSyncTime: time.Now(),
		},
	}
}

// Sync synchronizes the local cache index with the remote cache
func (t *RemoteCacheToolImpl) Sync(ctx context.Context) error {
	// Implementation depends on how you want to sync the caches
	// This would use the SSH connection to sync cache data
	_, _, err := t.nodeTool.ExecuteCommand(ctx, t.nodeID, "find "+t.remotePath+" -type f | wc -l")
	if err != nil {
		return err
	}

	t.connectionInfo.LastSyncTime = time.Now()
	t.connectionInfo.Connected = true
	return nil
}

// GetNodeID returns the ID of the node where the cache is located
func (t *RemoteCacheToolImpl) GetNodeID() int {
	return t.nodeID
}

// GetConnectionInfo returns information about the remote connection
func (t *RemoteCacheToolImpl) GetConnectionInfo() *RemoteCacheConnectionInfo {
	return t.connectionInfo
}
