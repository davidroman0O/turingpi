package ubuntu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops/ops"
	osapi "github.com/davidroman0O/turingpi/pkg/tpi/os"
)

// ImageBuilder implements the os.ImageBuilder interface for Ubuntu.
type ImageBuilder struct {
	nodeID tpi.NodeID
	config *ImageBuildConfig
}

// ImageBuildConfig holds configuration for building Ubuntu images
type ImageBuildConfig struct {
	BaseConfig
	NetworkingConfig

	// Board type this image is intended for
	Board tpi.Board

	// Optional customization function to modify the image
	ImageCustomizationFunc func(image tpi.ImageModifier) error

	// Optional override for local source image
	BaseImageXZPath string
}

// NetworkConfig holds network configuration for the image
type NetworkConfig struct {
	StaticIP   string
	Gateway    string
	Nameserver string
}

// GetCacheKey returns the cache key for this configuration
func (c *ImageBuildConfig) GetCacheKey() string {
	if c == nil {
		return ""
	}
	key := c.Key
	if c.StaticIP != "" {
		key += fmt.Sprintf("+net:%s", c.StaticIP)
	}
	if c.BaseImageXZPath != "" {
		key += fmt.Sprintf("+base:%s", filepath.Base(c.BaseImageXZPath))
	}
	return key
}

// NewImageBuilder creates a new Ubuntu image builder.
func NewImageBuilder(nodeID tpi.NodeID) *ImageBuilder {
	return &ImageBuilder{
		nodeID: nodeID,
	}
}

// Configure accepts an OS-specific configuration struct.
func (b *ImageBuilder) Configure(config interface{}) error {
	buildConfig, ok := config.(*ImageBuildConfig)
	if !ok {
		return fmt.Errorf("expected *ImageBuildConfig, got %T", config)
	}

	// Validate required fields
	if buildConfig.Key == "" {
		return fmt.Errorf("Key is required")
	}
	if buildConfig.Version == "" {
		return fmt.Errorf("Version is required")
	}
	if buildConfig.Board == "" {
		return fmt.Errorf("Board is required")
	}
	if buildConfig.StaticIP == "" {
		return fmt.Errorf("StaticIP is required")
	}

	b.config = buildConfig
	return nil
}

// CheckCache checks if an item exists for the configured CacheKey.
// Returns true and metadata-derived ImageResult if key found in remote cache.
func (b *ImageBuilder) CheckCache(ctx tpi.Context, cluster tpi.Cluster) (bool, tpi.ImageResult, error) {
	if b.config == nil {
		return false, tpi.ImageResult{}, fmt.Errorf("configuration not set")
	}

	key := b.config.GetCacheKey()
	if key == "" {
		return false, tpi.ImageResult{}, fmt.Errorf("cache key not available")
	}

	// Check remote cache first
	remoteCache := cluster.GetRemoteCache()
	if exists, err := remoteCache.Exists(ctx, key); err != nil {
		return false, tpi.ImageResult{}, fmt.Errorf("failed to check remote cache: %w", err)
	} else if exists {
		metadata, _, err := remoteCache.Get(ctx, key, false)
		if err != nil {
			return false, tpi.ImageResult{}, fmt.Errorf("failed to get metadata from remote cache: %w", err)
		}

		return true, tpi.ImageResult{
			ImagePath: metadata.Filename,
			Board:     b.config.Board,
			InputHash: metadata.Hash,
		}, nil
	}

	// Check local cache as fallback
	localCache := cluster.GetLocalCache()
	if exists, err := localCache.Exists(ctx, key); err != nil {
		return false, tpi.ImageResult{}, fmt.Errorf("failed to check local cache: %w", err)
	} else if exists {
		metadata, _, err := localCache.Get(ctx, key, false)
		if err != nil {
			return false, tpi.ImageResult{}, fmt.Errorf("failed to get metadata from local cache: %w", err)
		}

		return true, tpi.ImageResult{
			ImagePath: metadata.Filename,
			Board:     b.config.Board,
			InputHash: metadata.Hash,
		}, nil
	}

	return false, tpi.ImageResult{}, nil
}

// Run executes the image customization process.
func (b *ImageBuilder) Run(ctx tpi.Context, cluster tpi.Cluster) (tpi.ImageResult, error) {
	if b.config == nil {
		return tpi.ImageResult{}, fmt.Errorf("image builder not configured")
	}

	// Calculate input hash for caching
	inputHash := b.config.GetCacheKey()

	// Check cache first unless Force is true
	if !b.config.Force {
		exists, result, err := b.CheckCache(ctx, cluster)
		if err != nil {
			return tpi.ImageResult{}, fmt.Errorf("cache check failed: %w", err)
		}
		if exists {
			return result, nil
		}
	}

	// Get local cache for base image
	localCache := cluster.Cache()
	if localCache == nil {
		return tpi.ImageResult{}, fmt.Errorf("local cache not available")
	}

	// Resolve base image path (either from config or download)
	baseImagePath := b.config.BaseImageXZPath
	if baseImagePath == "" {
		// TODO: Implement auto-download based on Version/BoardType
		return tpi.ImageResult{}, fmt.Errorf("BaseImageXZPath required (auto-download not implemented)")
	}

	// Initialize image operations adapter
	imgOps, err := imageops.NewImageOpsAdapter(cluster.GetCacheDir(), cluster.GetPrepImageDir(), cluster.GetCacheDir())
	if err != nil {
		return tpi.ImageResult{}, fmt.Errorf("failed to create image operations adapter: %w", err)
	}
	defer func() {
		if err := imgOps.Cleanup(ctx); err != nil {
			fmt.Printf("Warning: failed to cleanup image operations: %v\n", err)
		}
	}()

	// Prepare the image
	opts := ops.PrepareImageOptions{
		SourceImgXZ: baseImagePath,
		NodeNum:     int(b.nodeID),
		IPAddress:   b.config.StaticIP,
		Gateway:     b.config.Gateway,
		DNSServers:  b.config.DNSServers,
		OutputDir:   cluster.GetCacheDir(),
		TempDir:     cluster.GetPrepImageDir(),
		Hostname:    b.config.Hostname,
	}

	if err := imgOps.PrepareImage(ctx, opts); err != nil {
		return tpi.ImageResult{}, fmt.Errorf("failed to prepare image: %w", err)
	}

	// Store in remote cache using user's Key
	remoteCache := cluster.GetRemoteCache()
	outputPath := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.img.xz", opts.Hostname))

	// Open the output file for reading
	outputFile, err := os.Open(outputPath)
	if err != nil {
		return tpi.ImageResult{}, fmt.Errorf("failed to open output file for caching: %w", err)
	}
	defer outputFile.Close()

	metadata := cache.Metadata{
		Key:      b.config.GetCacheKey(),
		Filename: outputPath,
		Tags:     b.config.Tags,
		OSType:   "ubuntu",
		Hash:     inputHash,
	}

	if _, err := remoteCache.Put(ctx, b.config.GetCacheKey(), metadata, outputFile); err != nil {
		return tpi.ImageResult{}, fmt.Errorf("failed to store in remote cache: %w", err)
	}

	return tpi.ImageResult{
		ImagePath: metadata.Filename,
		Board:     b.config.Board,
		InputHash: inputHash,
	}, nil
}

// Ensure ImageBuilder implements os.ImageBuilder interface
var _ osapi.ImageBuilder = (*ImageBuilder)(nil)
