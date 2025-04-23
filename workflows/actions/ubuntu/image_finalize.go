// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
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
	ctx.Logger.Info("Checking for network configuration parameters...")

	hostname, err := store.GetOrDefault[string](ctx.Store(), "Hostname", "")
	hasHostname := err == nil && hostname != ""

	// Get IP address and CIDR separately like in the old codebase
	ipAddr, err := store.GetOrDefault[string](ctx.Store(), "IPAddress", "")
	hasIPAddr := err == nil && ipAddr != ""

	cidrSuffix, err := store.GetOrDefault[string](ctx.Store(), "IPCIDRSuffix", "")
	hasCIDR := err == nil && cidrSuffix != ""

	// If separate IP/CIDR not found, try to get and split the combined IPCIDR
	if !hasIPAddr || !hasCIDR {
		ipCIDR, err := store.GetOrDefault[string](ctx.Store(), "IPCIDR", "")
		if err == nil && ipCIDR != "" {
			parts := strings.Split(ipCIDR, "/")
			if len(parts) == 2 {
				ipAddr = parts[0]
				cidrSuffix = "/" + parts[1]
				hasIPAddr = true
				hasCIDR = true
				ctx.Logger.Info("Split IPCIDR %s into IP=%s and CIDR=%s", ipCIDR, ipAddr, cidrSuffix)
			}
		}
	}

	// Combine IP and CIDR for the API call
	combinedIPCIDR := ""
	if hasIPAddr && hasCIDR {
		// If CIDR doesn't start with "/", add it
		if !strings.HasPrefix(cidrSuffix, "/") {
			cidrSuffix = "/" + cidrSuffix
		}
		combinedIPCIDR = ipAddr + cidrSuffix
		ctx.Logger.Info("Combined IP address: %s", combinedIPCIDR)
	}

	gateway, err := store.GetOrDefault[string](ctx.Store(), "Gateway", "")
	hasGateway := err == nil && gateway != ""

	// Get DNS servers (either as string or directly as slice)
	var dnsServers []string
	dnsServersStr, err := store.GetOrDefault[string](ctx.Store(), "DNSServers", "")
	if err == nil && dnsServersStr != "" {
		dnsServers = parseDNSServers(dnsServersStr)
		ctx.Logger.Info("Parsed DNS servers from string: %v", dnsServers)
	} else {
		// Try to get DNS servers as slice directly
		dnsServersSlice, err := store.GetOrDefault[[]string](ctx.Store(), "DNSServersList", nil)
		if err == nil && len(dnsServersSlice) > 0 {
			dnsServers = dnsServersSlice
			ctx.Logger.Info("Retrieved DNS servers as slice: %v", dnsServers)
		}
	}
	hasDNS := len(dnsServers) > 0

	// Default hostname if not provided or empty
	if !hasHostname {
		hostname = fmt.Sprintf("rk1-node-%d", nodeID)
		hasHostname = true
		ctx.Logger.Info("Using default hostname: %s", hostname)
	}

	// Apply network configuration if we have required components
	if hasHostname && hasIPAddr && hasCIDR && hasGateway && hasDNS {
		ctx.Logger.Info("Applying network configuration...")
		ctx.Logger.Info("Hostname: %s", hostname)
		ctx.Logger.Info("IP Address: %s", ipAddr)
		ctx.Logger.Info("CIDR Suffix: %s", cidrSuffix)
		ctx.Logger.Info("Combined IPCIDR: %s", combinedIPCIDR)
		ctx.Logger.Info("Gateway: %s", gateway)
		ctx.Logger.Info("DNS Servers: %v", dnsServers)

		// Apply the network configuration
		if err := toolsProvider.GetOperationsTool().ApplyNetworkConfig(
			ctx.GoContext,
			mountPoint,
			hostname,
			combinedIPCIDR,
			gateway,
			dnsServers,
		); err != nil {
			return fmt.Errorf("failed to apply network configuration: %w", err)
		}

		ctx.Logger.Info("Network configuration applied successfully")
	} else {
		ctx.Logger.Warn("Skipping network configuration as not all parameters are provided:")
		if !hasHostname {
			ctx.Logger.Warn("- Missing hostname parameter")
		}
		if !hasIPAddr {
			ctx.Logger.Warn("- Missing IP address parameter")
		}
		if !hasCIDR {
			ctx.Logger.Warn("- Missing CIDR suffix parameter")
		}
		if !hasGateway {
			ctx.Logger.Warn("- Missing gateway parameter")
		}
		if !hasDNS {
			ctx.Logger.Warn("- Missing DNS servers parameter")
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
	// Extensive cleaning to handle various formats
	// Remove common formatting characters
	dnsStr = strings.ReplaceAll(dnsStr, "[", "")
	dnsStr = strings.ReplaceAll(dnsStr, "]", "")
	dnsStr = strings.ReplaceAll(dnsStr, "{", "")
	dnsStr = strings.ReplaceAll(dnsStr, "}", "")
	dnsStr = strings.ReplaceAll(dnsStr, "\"", "")
	dnsStr = strings.ReplaceAll(dnsStr, "'", "")

	// Split by commas
	parts := strings.Split(dnsStr, ",")

	// Clean each part and collect non-empty values
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	// If empty, return empty slice
	if len(result) == 0 {
		return []string{}
	}

	return result
}
