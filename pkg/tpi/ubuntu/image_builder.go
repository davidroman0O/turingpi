package ubuntu

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/turingpi/pkg/tpi"                   // Base tpi types
	"github.com/davidroman0O/turingpi/pkg/tpi/internal/imageops" // Internal helpers
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"          // Platform detection
)

// UbuntuImageBuilder defines the configuration for Phase 1: Image Customization for Ubuntu.
type UbuntuImageBuilder struct {
	nodeID           tpi.NodeID // The specific node this image is for
	baseImageXZPath  string
	networkConfig    *tpi.NetworkConfig // Pointer to allow optional config
	preInstallFunc   func(image tpi.ImageModifier) error
	stagedOperations []imageops.FileOperation // Collected from preInstallFunc
	outputDirectory  string                   // Custom output directory for prepared images
}

// NewImageBuilder creates a new builder for customizing an Ubuntu image.
// It requires the target node ID.
func NewImageBuilder(nodeID tpi.NodeID) *UbuntuImageBuilder {
	return &UbuntuImageBuilder{
		nodeID: nodeID,
	}
}

// WithBaseImage specifies the path to the source compressed image (.img.xz). REQUIRED.
func (b *UbuntuImageBuilder) WithBaseImage(path string) *UbuntuImageBuilder {
	b.baseImageXZPath = path
	return b
}

// WithNetworkConfig provides the network settings to be applied. REQUIRED.
func (b *UbuntuImageBuilder) WithNetworkConfig(config tpi.NetworkConfig) *UbuntuImageBuilder {
	b.networkConfig = &config
	return b
}

// WithPreInstall registers a function to perform file modifications within the image
// before it's recompressed. The function receives an ImageModifier to stage operations.
func (b *UbuntuImageBuilder) WithPreInstall(f func(image tpi.ImageModifier) error) *UbuntuImageBuilder {
	b.preInstallFunc = f
	return b
}

// WithOutputDirectory specifies a custom directory where the prepared image will be stored.
// If not provided, the executor's cache directory will be used.
func (b *UbuntuImageBuilder) WithOutputDirectory(path string) *UbuntuImageBuilder {
	b.outputDirectory = path
	return b
}

// calculateInputHash generates a hash representing the inputs to this phase.
func (b *UbuntuImageBuilder) calculateInputHash() (string, error) {
	h := sha256.New()

	// Hash base image path (consider hashing content? Very slow!)
	if _, err := h.Write([]byte(b.baseImageXZPath)); err != nil {
		return "", err
	}

	// Hash network config
	if b.networkConfig != nil {
		ncString := fmt.Sprintf("%+v", *b.networkConfig)
		if _, err := h.Write([]byte(ncString)); err != nil {
			return "", err
		}
	}

	// Hash staged file operations (represent them as strings)
	for _, op := range b.stagedOperations {
		// Use a stable representation of the operation
		// TODO: Enhance this representation to be more robust
		opString := fmt.Sprintf("%s:%+v", op.Type(), op)
		if _, err := h.Write([]byte(opString)); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Run executes the image customization phase.
func (b *UbuntuImageBuilder) Run(ctx tpi.Context, cluster tpi.Cluster) (*tpi.ImageResult, error) {
	phaseName := "ImageCustomization"
	log.Printf("--- Starting Phase 1: %s for Node %d (Ubuntu) ---", phaseName, b.nodeID)

	// --- Validate Builder Config ---
	if b.baseImageXZPath == "" {
		return nil, fmt.Errorf("phase 1 validation failed: WithBaseImage is required")
	}
	if b.networkConfig == nil {
		return nil, fmt.Errorf("phase 1 validation failed: WithNetworkConfig is required")
	}
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return nil, fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	// --- Execute PreInstall Callback to Stage Operations ---
	modifier := tpi.NewImageModifierImpl()
	if b.preInstallFunc != nil {
		log.Println("Executing pre-install callback to stage file operations...")
		if err := b.preInstallFunc(modifier); err != nil {
			return nil, fmt.Errorf("pre-install callback failed: %w", err)
		}
		b.stagedOperations = modifier.GetOperations()
		log.Printf("Staged %d file operations from pre-install callback.", len(b.stagedOperations))
	}

	// --- Calculate Input Hash ---
	inputHash, err := b.calculateInputHash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate input hash: %w", err)
	}
	log.Printf("Calculated input hash: %s", inputHash)

	// --- Check State ---
	stateMgr := cluster.GetStateManager()
	currentState := stateMgr.GetNodeState(b.nodeID).ImageCustomization

	if currentState.Status == tpi.StatusCompleted && currentState.InputHash == inputHash {
		log.Printf("Phase 1 already completed with matching inputs. Skipping execution.")
		log.Printf("Using cached image: %s", currentState.OutputImagePath)
		return &tpi.ImageResult{
			ImagePath: currentState.OutputImagePath,
			Board:     nodeConfig.Board,
			InputHash: inputHash,
		}, nil
	}
	if currentState.Status == tpi.StatusRunning {
		return nil, fmt.Errorf("phase 1 is already marked as running for node %d (state timestamp: %s). Manual intervention might be required", b.nodeID, currentState.Timestamp)
	}

	// --- Mark State as Running ---
	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusRunning, inputHash, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update state to running: %w", err)
	}

	// --- Prepare Paths ---
	sourceImgXZAbs, err := filepath.Abs(b.baseImageXZPath)
	if err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("failed to get absolute path for source image: %w", err))
	}
	if _, err := os.Stat(sourceImgXZAbs); os.IsNotExist(err) {
		return nil, b.failPhase(cluster, fmt.Errorf("source image '%s' not found", sourceImgXZAbs))
	}

	safeHostname := strings.ReplaceAll(b.networkConfig.Hostname, "/", "_")
	safeHostname = strings.ReplaceAll(safeHostname, "\\", "_")
	outputFilename := fmt.Sprintf("%s.img.xz", safeHostname)

	// Determine the final output directory for the prepared image
	// This is where the completed, customized image will be placed
	var outputDir string
	if b.outputDirectory != "" {
		// Use the explicitly set output directory if provided
		if err := os.MkdirAll(b.outputDirectory, 0755); err != nil {
			return nil, b.failPhase(cluster, fmt.Errorf("failed to create output directory '%s': %w", b.outputDirectory, err))
		}
		outputDir = b.outputDirectory
		log.Printf("Using custom output directory for prepared image: %s", outputDir)
	} else {
		// Fall back to cache dir if no explicit output directory is set
		outputDir = cluster.GetCacheDir()
		log.Printf("Using cache directory for prepared image: %s", outputDir)
	}
	preparedImageXZPath := filepath.Join(outputDir, outputFilename)
	log.Printf("Final prepared image will be saved to: %s", preparedImageXZPath)

	// Determine the temporary working directory for image processing
	// This is where all the temporary files (decompressed image, mounts, etc.) will be created
	var tmpDirBase string
	if prepDir := cluster.GetPrepImageDir(); prepDir != "" {
		// Use the configured preparation directory for temporary files
		if err := os.MkdirAll(prepDir, 0755); err != nil {
			return nil, b.failPhase(cluster, fmt.Errorf("failed to create temporary image processing directory '%s': %w", prepDir, err))
		}
		tmpDirBase = prepDir
		log.Printf("Using configured temporary processing directory: %s", tmpDirBase)
	} else {
		// If no specific temporary directory is configured, use the system's temp directory
		tmpDirBase = os.TempDir()
		log.Printf("Using system temporary directory: %s", tmpDirBase)
	}

	// Create a unique temporary directory within the base temporary directory
	tmpDir, err := os.MkdirTemp(tmpDirBase, fmt.Sprintf("tpi-img-prep-node%d-*", b.nodeID))
	if err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("failed to create temporary directory: %w", err))
	}
	log.Printf("Created temporary directory: %s", tmpDir)
	mountDir := filepath.Join(tmpDir, "mnt")

	// Check if we're running on a non-Linux platform and need Docker
	if !platform.IsLinux() {
		log.Println("Detected non-Linux platform, will use Docker for image operations")
		// Check if Docker is available
		if !platform.DockerAvailable() {
			return nil, b.failPhase(cluster, fmt.Errorf("Docker is required for image operations on non-Linux platforms but is not available"))
		}

		// Get the source image directory (parent directory of the image file)
		sourceDir := filepath.Dir(sourceImgXZAbs)

		// Initialize Docker configuration for image operations
		imageops.InitDockerConfig(sourceDir, tmpDir, outputDir)
		log.Println("Docker configuration initialized for image operations")
	}

	var decompressedImgPath string
	var rootPartitionDevice string
	defer func(dirToClean string) {
		log.Printf("--- Starting deferred cleanup for Phase 1 (Node %d) ---", b.nodeID)
		if mountDir != "" && rootPartitionDevice != "" {
			_ = imageops.UnmountFilesystem(mountDir)
		}
		if decompressedImgPath != "" {
			_ = imageops.CleanupPartitions(decompressedImgPath)
		}
		log.Printf("Removing temporary directory %s...", dirToClean)
		if err := os.RemoveAll(dirToClean); err != nil {
			log.Printf("Warning: Failed to remove temporary directory %s: %v", dirToClean, err)
		} else {
			log.Println("Temporary directory removed.")
		}
		log.Println("--- Finished deferred cleanup ---")
	}(tmpDir)

	// --- Execute Image Operations (using internal/imageops) ---
	decompressedImgPath, err = imageops.DecompressImageXZ(sourceImgXZAbs, tmpDir)
	if err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("decompression failed: %w", err))
	}

	rootPartitionDevice, err = imageops.MapPartitions(decompressedImgPath)
	if err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("partition mapping failed: %w", err))
	}

	if err := imageops.MountFilesystem(rootPartitionDevice, mountDir); err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("filesystem mounting failed: %w", err))
	}

	if err := imageops.ApplyNetworkConfig(mountDir, b.networkConfig.Hostname, b.networkConfig.IPCIDR, b.networkConfig.Gateway, b.networkConfig.DNSServers); err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("applying network config failed: %w", err))
	}

	if len(b.stagedOperations) > 0 {
		fileOpParams := imageops.ExecuteFileOperationsParams{
			MountDir:   mountDir,
			Operations: b.stagedOperations,
		}
		if err := imageops.ExecuteFileOperations(fileOpParams); err != nil {
			return nil, b.failPhase(cluster, fmt.Errorf("executing file operations failed: %w", err))
		}
	} else {
		log.Println("No pre-install file operations were staged.")
	}

	if err := imageops.UnmountFilesystem(mountDir); err != nil {
		log.Printf("Warning: Unmount failed during main execution: %v", err)
	}

	if err := imageops.CleanupPartitions(decompressedImgPath); err != nil {
		log.Printf("Warning: Kpartx cleanup failed during main execution: %v", err)
	}

	if err := imageops.RecompressImageXZ(decompressedImgPath, preparedImageXZPath); err != nil {
		return nil, b.failPhase(cluster, fmt.Errorf("recompression failed: %w", err))
	}

	// --- Mark State as Completed ---
	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusCompleted, inputHash, preparedImageXZPath, nil)
	if err != nil {
		log.Printf("Warning: Failed to update state to completed, but phase finished: %v", err)
	}

	log.Printf("--- Finished Phase 1: %s for Node %d ---", phaseName, b.nodeID)
	log.Printf("Output image: %s", preparedImageXZPath)

	return &tpi.ImageResult{
		ImagePath: preparedImageXZPath,
		Board:     nodeConfig.Board,
		InputHash: inputHash,
	}, nil
}

// failPhase is a helper to update state on failure and return the error.
func (b *UbuntuImageBuilder) failPhase(cluster tpi.Cluster, err error) error {
	phaseName := "ImageCustomization"
	log.Printf("--- Error in Phase 1: %s for Node %d ---", phaseName, b.nodeID)
	log.Printf("Error details: %v", err)
	_ = cluster.GetStateManager().UpdatePhaseState(b.nodeID, phaseName, tpi.StatusFailed, "", "", err)
	return err
}
