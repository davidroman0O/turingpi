package imageops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DecompressImageXZ implements ImageOpsAdapter.DecompressImageXZ
func (a *imageOpsAdapter) DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	// Create output filename by replacing .xz extension
	outputImgPath := filepath.Join(tmpDir, filepath.Base(strings.TrimSuffix(sourceImgXZAbs, ".xz")))

	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for decompression...")

		// Add a lot more debug information
		fmt.Printf("DEBUG: Source file absolute path: %s\n", sourceImgXZAbs)
		fmt.Printf("DEBUG: Source file directory: %s\n", filepath.Dir(sourceImgXZAbs))
		fmt.Printf("DEBUG: Source file base name: %s\n", filepath.Base(sourceImgXZAbs))
		fmt.Printf("DEBUG: Output file path: %s\n", outputImgPath)
		fmt.Printf("DEBUG: Output file base name: %s\n", filepath.Base(outputImgPath))
		fmt.Printf("DEBUG: Container ID: %s\n", a.dockerAdapter.GetContainerID())
		fmt.Printf("DEBUG: Container Name: %s\n", a.dockerAdapter.GetContainerName())
		fmt.Printf("DEBUG: Docker Config Source Dir: %s\n", a.dockerConfig.SourceDir)
		fmt.Printf("DEBUG: Docker Config Temp Dir: %s\n", a.dockerConfig.TempDir)
		fmt.Printf("DEBUG: Docker Config Output Dir: %s\n", a.dockerConfig.OutputDir)

		// Check disk space in Docker
		diskSpaceOutput, err := a.executeDockerCommand("df -h")
		fmt.Printf("DEBUG: Docker disk space:\n%s\n", diskSpaceOutput)

		// First ensure that the /workspace directory exists and is writable in the container
		prepareCmd := "mkdir -p /workspace && chmod 777 /workspace && ls -la / | grep workspace"
		workspaceOutput, err := a.executeDockerCommand(prepareCmd)
		if err != nil {
			return "", fmt.Errorf("Failed to prepare /workspace directory: %w", err)
		}
		fmt.Printf("DEBUG: Workspace directory: %s\n", workspaceOutput)

		// Due to the disk space constraints, let's try to decompress directly on the host
		// rather than in the Docker container
		fmt.Printf("DEBUG: Attempting host-based decompression due to disk space concerns\n")

		// First, copy the compressed file to the host temporary directory if it's not already there
		hostSourcePath := sourceImgXZAbs
		if !strings.HasPrefix(sourceImgXZAbs, tmpDir) {
			hostTempSourcePath := filepath.Join(tmpDir, filepath.Base(sourceImgXZAbs))
			if _, err := os.Stat(hostTempSourcePath); os.IsNotExist(err) {
				fmt.Printf("DEBUG: Copying source file to temporary directory: %s -> %s\n", sourceImgXZAbs, hostTempSourcePath)
				data, err := os.ReadFile(sourceImgXZAbs)
				if err != nil {
					return "", fmt.Errorf("Failed to read source file: %w", err)
				}

				err = os.WriteFile(hostTempSourcePath, data, 0644)
				if err != nil {
					return "", fmt.Errorf("Failed to write source file to temp dir: %w", err)
				}
			}
			hostSourcePath = hostTempSourcePath
		}

		// Try decompression on the host
		fmt.Printf("DEBUG: Attempting decompression on host: %s -> %s\n", hostSourcePath, outputImgPath)

		// Create command to decompress on host
		hostCmd := exec.Command("xz", "--decompress", "--keep", "--stdout", hostSourcePath)
		outFile, err := os.Create(outputImgPath)
		if err != nil {
			return "", fmt.Errorf("Failed to create output file: %w", err)
		}
		defer outFile.Close()

		hostCmd.Stdout = outFile
		var stderr bytes.Buffer
		hostCmd.Stderr = &stderr

		err = hostCmd.Run()
		if err != nil {
			fmt.Printf("DEBUG: Host decompression failed: %v - %s\n", err, stderr.String())
			fmt.Printf("DEBUG: Falling back to Docker-based decompression\n")

			// Since we're in a disk space constrained environment,
			// Try to decompress and process in smaller chunks directly to save space
			fallingBackCmd := fmt.Sprintf("echo 'Using space-efficient approach' && xz --decompress --keep --stdout %s > %s && ls -lah %s",
				filepath.Join("/tmp", filepath.Base(sourceImgXZAbs)),
				outputImgPath,
				outputImgPath)

			output, err := a.executeDockerCommand(fallingBackCmd)
			if err != nil {
				fmt.Printf("DEBUG: Docker decompression failed too: %s\n", output)
				return "", fmt.Errorf("Both host and Docker decompression failed: %w", err)
			}

			fmt.Printf("DEBUG: Docker decompression output: %s\n", output)

			// Check if the file was created in the container
			checkCmd := fmt.Sprintf("ls -la %s", outputImgPath)
			checkOutput, err := a.executeDockerCommand(checkCmd)
			if err != nil {
				fmt.Printf("DEBUG: File does not exist in container: %s\n", checkOutput)
				return "", fmt.Errorf("Decompressed file not found in container")
			}

			fmt.Printf("DEBUG: File exists in container: %s\n", checkOutput)

			// Success - the file is decompressed in the container
			// We need to keep it there rather than copy it back to conserve disk space
			// Return a special path indicating this file is in the container
			return "DOCKER:" + outputImgPath, nil
		}

		// Host decompression succeeded
		fmt.Printf("DEBUG: Host decompression succeeded\n")

		// Verify the file was created
		if _, err := os.Stat(outputImgPath); os.IsNotExist(err) {
			return "", fmt.Errorf("Decompressed image not found at %s after host decompression", outputImgPath)
		}

		fmt.Printf("Decompression successful: %s\n", outputImgPath)
		return outputImgPath, nil
	}

	// Native Linux approach
	cmd := exec.Command("xz", "--decompress", "--keep", "--stdout", sourceImgXZAbs)
	outFile, err := os.Create(outputImgPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	fmt.Printf("Decompressing: %s -> %s\n", sourceImgXZAbs, outputImgPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("decompression failed: %w - %s", err, stderr.String())
	}

	// Verify the file was created
	if _, err := os.Stat(outputImgPath); os.IsNotExist(err) {
		return "", fmt.Errorf("decompressed image not found at %s", outputImgPath)
	}

	return outputImgPath, nil
}

// RecompressImageXZ implements ImageOpsAdapter.RecompressImageXZ
func (a *imageOpsAdapter) RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for image compression...")
		// Docker command to compress: xz -zck6 input.img > output.img.xz
		// Note: In the turingpi-prepare Docker container:
		// - Source directory is mounted at /images
		// - Temp directory is mounted at /tmp
		// - Output directory is mounted at /prepared-images
		dockerCmd := fmt.Sprintf("xz -zck6 %s > %s",
			filepath.Join("/tmp", filepath.Base(modifiedImgPath)),
			filepath.Join("/prepared-images", filepath.Base(finalXZPath)))

		_, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker compression failed: %w", err)
		}

		fmt.Printf("Docker compression completed: %s\n", finalXZPath)
		return nil
	}

	// Native Linux approach
	// Create directory if it doesn't exist
	outputDir := filepath.Dir(finalXZPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use xz to compress, with -6 compression level
	fmt.Printf("Compressing: %s -> %s (level 6)\n", modifiedImgPath, finalXZPath)
	cmd := exec.Command("xz", "-zck6", "--stdout", modifiedImgPath)

	// Direct output to file
	outFile, err := os.Create(finalXZPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compression failed: %w - %s", err, stderr.String())
	}

	// Verify the compressed file exists
	if _, err := os.Stat(finalXZPath); os.IsNotExist(err) {
		return fmt.Errorf("compressed image not found at %s", finalXZPath)
	}

	return nil
}
