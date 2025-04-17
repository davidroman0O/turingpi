package ubuntu

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/turingpi/pkg/tpi"          // Base tpi types
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops" // Updated import path
	"github.com/davidroman0O/turingpi/pkg/tpi/platform" // Platform detection
)

// UbuntuImageBuilder defines the configuration for Phase 1: Image Customization for Ubuntu.
type UbuntuImageBuilder struct {
	nodeID           tpi.NodeID // The specific node this image is for
	baseImageXZPath  string
	networkConfig    *tpi.NetworkConfig // Pointer to allow optional config
	preInstallFunc   func(image tpi.ImageModifier) error
	stagedOperations []imageops.FileOperation // Collected from preInstallFunc
	outputDirectory  string                   // Custom output directory for prepared images
	imageOps         imageops.ImageOpsAdapter // Image operations adapter
}

// NewImageBuilder creates a new builder for customizing an Ubuntu image.
// It requires the target node ID.
func NewImageBuilder(nodeID tpi.NodeID) *UbuntuImageBuilder {
	return &UbuntuImageBuilder{
		nodeID:   nodeID,
		imageOps: imageops.NewImageOpsAdapter(),
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
		return nil, fmt.Errorf("phase 1 validation failed: source image path is required (use WithBaseImage)")
	}
	if b.outputDirectory == "" {
		return nil, fmt.Errorf("phase 1 validation failed: output directory is required (use WithOutputDirectory)")
	}

	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return nil, fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	if b.networkConfig == nil {
		return nil, fmt.Errorf("phase 1 validation failed: network configuration is required (use WithNetworkConfig)")
	}

	// Execute the pre-install function if provided, to stage file operations
	log.Printf("Executing pre-install callback to stage file operations...")
	b.stagedOperations = []imageops.FileOperation{} // Reset operations before collection

	// Create a fresh image modifier for the pre-install callback
	imageModifier := tpi.NewImageModifierImpl()

	// If preInstallFunc was provided, call it
	if b.preInstallFunc != nil {
		if err := b.preInstallFunc(imageModifier); err != nil {
			return nil, fmt.Errorf("pre-install callback failed: %w", err)
		}
	}

	// Collect the operations from the modifier
	b.stagedOperations = imageModifier.GetOperations()
	log.Printf("Staged %d file operations from pre-install callback.", len(b.stagedOperations))

	// --- Calculate Input Hash ---
	inputHash, err := b.calculateInputHash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate input hash: %w", err)
	}
	log.Printf("Calculated input hash: %s", inputHash)

	// --- Check State ---
	stateMgr := cluster.GetStateManager()
	currentState := stateMgr.GetNodeState(b.nodeID).ImageCustomization

	if currentState.Status == tpi.StatusCompleted &&
		currentState.InputHash == inputHash &&
		currentState.OutputImagePath != "" {
		// If completed with the same inputs and we have the output path
		log.Printf("Phase 1 already completed with matching inputs. Using cached image: %s",
			currentState.OutputImagePath)

		// Check if the cached output actually exists
		if _, err := os.Stat(currentState.OutputImagePath); err == nil {
			// Return the cached result
			return &tpi.ImageResult{
				ImagePath: currentState.OutputImagePath,
				Board:     nodeConfig.Board,
				InputHash: inputHash,
			}, nil
		} else {
			log.Printf("Warning: Cached image file %s not found, will rebuild",
				currentState.OutputImagePath)
		}
	}

	if currentState.Status == tpi.StatusRunning {
		return nil, fmt.Errorf("phase 1 is already marked as running for node %d (state timestamp: %s). Manual intervention might be required", b.nodeID, currentState.Timestamp)
	}

	// --- Mark State as Running ---
	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusRunning, inputHash, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update state to running: %w", err)
	}

	// --- Prepare Output Path ---
	log.Printf("Using custom output directory for prepared image: %s", b.outputDirectory)

	// Ensure the output directory exists
	if err := os.MkdirAll(b.outputDirectory, 0755); err != nil {
		return b.failPhase(cluster, fmt.Errorf("failed to create output directory: %w", err))
	}

	// Generate output filename based on hostname from network config
	hostname := b.networkConfig.Hostname
	if hostname == "" {
		hostname = fmt.Sprintf("node%d", b.nodeID)
	}

	outputFilename := fmt.Sprintf("%s.img.xz", hostname)
	finalImagePath := filepath.Join(b.outputDirectory, outputFilename)
	log.Printf("Final prepared image will be saved to: %s", finalImagePath)

	// --- Set Up Temporary Directory ---
	// First, check if cluster has a configured temp dir
	tempDir := cluster.GetPrepImageDir()
	if tempDir == "" {
		// Fallback to system temp dir
		tempDir = os.TempDir()
	}
	log.Printf("Using configured temporary processing directory: %s", tempDir)

	// Create a unique temporary directory for this run
	tempWorkDir, err := os.MkdirTemp(tempDir, fmt.Sprintf("tpi-img-prep-node%d-", b.nodeID))
	if err != nil {
		return b.failPhase(cluster, fmt.Errorf("failed to create temporary directory: %w", err))
	}
	log.Printf("Created temporary directory: %s", tempWorkDir)
	defer os.RemoveAll(tempWorkDir) // Clean up after ourselves

	// --- Configure Docker if Needed ---
	// If we're not on Linux, we need to use Docker
	if !platform.IsLinux() {
		log.Printf("Detected non-Linux platform, will use Docker for image operations")

		// Initialize Docker with proper configuration
		// Note: We add the output directory as a mount point for Docker to access it
		err := b.imageOps.InitDockerConfig(filepath.Dir(b.baseImageXZPath), tempWorkDir, b.outputDirectory)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to initialize Docker: %w", err))
		}
		log.Printf("Docker configuration initialized for image operations")
	}

	// --- Execute Image Preparation ---
	ipWithoutCIDR := b.networkConfig.IPCIDR
	cidrIdx := strings.Index(ipWithoutCIDR, "/")
	if cidrIdx > 0 {
		ipWithoutCIDR = ipWithoutCIDR[:cidrIdx]
	}

	cidrSuffix := ""
	if cidrIdx > 0 {
		cidrSuffix = b.networkConfig.IPCIDR[cidrIdx:]
	} else {
		cidrSuffix = "/24" // Default if no CIDR provided
	}

	// Prepare image prep options
	prepOpts := imageops.PrepareImageOptions{
		SourceImgXZ:  b.baseImageXZPath,
		NodeNum:      int(b.nodeID),
		IPAddress:    ipWithoutCIDR,
		IPCIDRSuffix: cidrSuffix,
		Hostname:     b.networkConfig.Hostname,
		Gateway:      b.networkConfig.Gateway,
		DNSServers:   b.networkConfig.DNSServers,
		OutputDir:    b.outputDirectory,
		TempDir:      tempWorkDir,
	}

	log.Printf("Running on non-Linux platform, using Docker")
	fmt.Println("Executing image preparation script in Docker...")

	// Execute the preparation
	outputPath, err := b.imageOps.PrepareImage(prepOpts)
	if err != nil {
		return b.failPhase(cluster, fmt.Errorf("image preparation failed: %w", err))
	}

	fmt.Printf("Docker preparation completed successfully. Output: %s\n", outputPath)
	finalImagePath = outputPath // Use the path returned by PrepareImage

	log.Printf("Image preparation completed successfully: %s", finalImagePath)

	// --- Apply Pre-Install File Operations (if any) ---
	if len(b.stagedOperations) > 0 {
		log.Printf("Applying %d pre-install file operations...", len(b.stagedOperations))

		// 1. Decompress the image for file operations
		tempDir := tempWorkDir
		decompImgPath, err := imageops.DecompressImageXZ(finalImagePath, tempDir)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to decompress for file ops: %w", err))
		}

		// 2. Map and mount the partitions
		rootPartDev, err := imageops.MapPartitions(decompImgPath)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to map partitions for file ops: %w", err))
		}

		// Ensure we clean up when done
		defer func() {
			_ = imageops.CleanupPartitions(decompImgPath)
		}()

		// 3. Mount the filesystem
		mountDir := filepath.Join(tempDir, "mnt")
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to create mount dir for file ops: %w", err))
		}

		err = imageops.MountFilesystem(rootPartDev, mountDir)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to mount filesystem for file ops: %w", err))
		}

		// Ensure we unmount when done
		defer func() {
			_ = imageops.UnmountFilesystem(mountDir)
		}()

		// 4. Execute the staged file operations
		fileOpsParams := imageops.ExecuteFileOperationsParams{
			MountDir:   mountDir,
			Operations: b.stagedOperations,
		}

		err = imageops.ExecuteFileOperations(fileOpsParams)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to apply file operations: %w", err))
		}

		// 5. Unmount and cleanup
		if err := imageops.UnmountFilesystem(mountDir); err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to unmount filesystem: %w", err))
		}

		if err := imageops.CleanupPartitions(decompImgPath); err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to cleanup partitions: %w", err))
		}

		// 6. Recompress the image
		modifiedImgName := filepath.Base(decompImgPath)
		xzImgName := modifiedImgName + ".xz"
		finalXZPath := filepath.Join(b.outputDirectory, xzImgName)

		err = imageops.RecompressImageXZ(decompImgPath, finalXZPath)
		if err != nil {
			return b.failPhase(cluster, fmt.Errorf("failed to recompress image: %w", err))
		}

		// Update the final image path to the recompressed version
		finalImagePath = finalXZPath
	}

	// --- Update State to Completed ---
	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusCompleted, inputHash, finalImagePath, nil)
	if err != nil {
		log.Printf("Warning: Failed to update state to completed, but phase finished successfully: %v", err)
	}

	// --- Clean up Docker resources if used ---
	if !platform.IsLinux() && b.imageOps.GetDockerAdapter() != nil {
		log.Printf("Cleaning up Docker resources...")
		b.imageOps.GetDockerAdapter().Cleanup()
	}

	log.Printf("--- Phase 1: %s for Node %d Completed Successfully ---", phaseName, b.nodeID)

	// --- Return Results ---
	return &tpi.ImageResult{
		ImagePath: finalImagePath,
		Board:     nodeConfig.Board,
		InputHash: inputHash,
	}, nil
}

// failPhase is a helper to update state on failure and return the error.
func (b *UbuntuImageBuilder) failPhase(cluster tpi.Cluster, err error) (*tpi.ImageResult, error) {
	phaseName := "ImageCustomization"
	log.Printf("--- Error in Phase 1: %s for Node %d ---", phaseName, b.nodeID)
	log.Printf("Error details: %v", err)
	_ = cluster.GetStateManager().UpdatePhaseState(b.nodeID, phaseName, tpi.StatusFailed, "", "", err)
	return nil, err
}
