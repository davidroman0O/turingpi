// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions"
)

// ImageFinalizeAction handles cleanup, compression, and caching of the prepared image
type ImageFinalizeAction struct {
	actions.PlatformActionBase
}

// NewImageFinalizeAction creates a new action to finalize the image preparation
func NewImageFinalizeAction() *ImageFinalizeAction {
	return &ImageFinalizeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-finalize",
			"Finalizes the Ubuntu image preparation by configuring network, mounting, and customizing the image",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImageFinalizeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImageFinalizeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *ImageFinalizeAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get the decompressed image file path
	ubuntuImageDecompressedFile, err := store.Get[string](ctx.Store(), "ubuntu.image.decompressed.file")
	if err != nil {
		return fmt.Errorf("failed to get ubuntu image decompressed path: %w", err)
	}

	ctx.Logger.Info("Finalizing Ubuntu image for node %d", nodeID)
	ctx.Logger.Info("Decompressed image file: %s", ubuntuImageDecompressedFile)

	// Map the partitions
	ctx.Logger.Info("Mapping partitions...")
	rootDevPath, err := toolsProvider.GetOperationsTool().MapPartitions(ctx.GoContext, ubuntuImageDecompressedFile)
	if err != nil {
		return fmt.Errorf("failed to map partitions: %w", err)
	}

	// Setup deferred cleanup of mapped partitions
	defer func() {
		ctx.Logger.Info("Cleaning up mapped partitions...")
		if err := toolsProvider.GetOperationsTool().UnmapPartitions(ctx.GoContext, ubuntuImageDecompressedFile); err != nil {
			ctx.Logger.Warn("Error unmapping partitions: %v", err)
		}
	}()

	// Create mount point
	mountPoint := "/mnt/ubuntu" // Standard mount point for Ubuntu
	ctx.Logger.Info("Using mount point: %s", mountPoint)

	// Mount the filesystem
	ctx.Logger.Info("Mounting root partition %s to %s...", rootDevPath, mountPoint)
	if err := toolsProvider.GetOperationsTool().Mount(ctx.GoContext, rootDevPath, mountPoint, "", []string{"rw"}); err != nil {
		return fmt.Errorf("failed to mount image: %w", err)
	}

	// Setup deferred unmount
	defer func() {
		ctx.Logger.Info("Unmounting filesystem...")
		if err := toolsProvider.GetOperationsTool().Unmount(ctx.GoContext, mountPoint); err != nil {
			ctx.Logger.Warn("Error unmounting filesystem: %v", err)
		}
	}()

	// Apply network configuration if provided
	hostname, err := store.GetOrDefault[string](ctx.Store(), "Hostname", "")
	hasHostname := err == nil && hostname != ""

	ipCIDR, err := store.GetOrDefault[string](ctx.Store(), "IPCIDR", "")
	hasIPCIDR := err == nil && ipCIDR != ""

	gateway, err := store.GetOrDefault[string](ctx.Store(), "Gateway", "")
	hasGateway := err == nil && gateway != ""

	dnsServersStr, err := store.GetOrDefault[string](ctx.Store(), "DNSServers", "")
	hasDNS := err == nil && dnsServersStr != ""

	// Apply network configuration if all required components are available
	if hasHostname && hasIPCIDR && hasGateway && hasDNS {
		ctx.Logger.Info("Applying network configuration...")

		// Parse DNS servers from string representation
		dnsServers := parseDNSServers(dnsServersStr)

		// Default hostname if not provided or empty
		if hostname == "" {
			hostname = fmt.Sprintf("node%d", nodeID)
			ctx.Logger.Info("Using default hostname: %s", hostname)
		}

		// Apply the network configuration
		if err := toolsProvider.GetOperationsTool().ApplyNetworkConfig(
			ctx.GoContext,
			mountPoint,
			hostname,
			ipCIDR,
			gateway,
			dnsServers,
		); err != nil {
			return fmt.Errorf("failed to apply network configuration: %w", err)
		}

		ctx.Logger.Info("Network configuration applied successfully")
	} else {
		ctx.Logger.Info("Skipping network configuration as not all parameters are provided")
		if !hasHostname {
			ctx.Logger.Info("Missing hostname parameter")
		}
		if !hasIPCIDR {
			ctx.Logger.Info("Missing IP CIDR parameter")
		}
		if !hasGateway {
			ctx.Logger.Info("Missing gateway parameter")
		}
		if !hasDNS {
			ctx.Logger.Info("Missing DNS servers parameter")
		}
	}

	ctx.Logger.Info("Image customization completed successfully")

	// Unmount the filesystem before compression
	ctx.Logger.Info("Unmounting filesystem before compression...")
	if err := toolsProvider.GetOperationsTool().Unmount(ctx.GoContext, mountPoint); err != nil {
		return fmt.Errorf("failed to unmount filesystem: %w", err)
	}

	// Unmap partitions before compression
	ctx.Logger.Info("Unmapping partitions before compression...")
	if err := toolsProvider.GetOperationsTool().UnmapPartitions(ctx.GoContext, ubuntuImageDecompressedFile); err != nil {
		return fmt.Errorf("failed to unmap partitions: %w", err)
	}

	// Generate the compressed output file path
	outputDir := filepath.Dir(ubuntuImageDecompressedFile)
	compressedImagePath := filepath.Join(outputDir, filepath.Base(ubuntuImageDecompressedFile)+".xz")

	// Compress the image
	ctx.Logger.Info("Compressing finalized image to %s...", compressedImagePath)
	if err := toolsProvider.GetOperationsTool().CompressXZ(ctx.GoContext, ubuntuImageDecompressedFile, compressedImagePath); err != nil {
		return fmt.Errorf("failed to compress image: %w", err)
	}

	// Store the compressed image path in the context for later use
	if err := ctx.Store().Put("ubuntu.image.compressed.file", compressedImagePath); err != nil {
		ctx.Logger.Warn("Failed to store compressed image path in context: %v", err)
		// This is a context storage issue, not an image issue, so we can continue
		return fmt.Errorf("failed to store compressed image path in context: %w", err)
	}

	ctx.Logger.Info("Image finalization and compression completed successfully")
	return nil
}

// parseDNSServers parses a string representation of DNS servers into a string slice
func parseDNSServers(dnsStr string) []string {
	// Remove brackets, spaces and split by commas
	dnsStr = strings.ReplaceAll(dnsStr, "[", "")
	dnsStr = strings.ReplaceAll(dnsStr, "]", "")
	dnsStr = strings.ReplaceAll(dnsStr, " ", "")

	// If empty, return empty slice
	if dnsStr == "" {
		return []string{}
	}

	return strings.Split(dnsStr, ",")
}
