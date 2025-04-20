package imageops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DecompressImageXZ implements ImageOpsAdapter.DecompressImageXZ
func (a *imageOpsAdapter) DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() {
		if !a.isDockerInitialized() {
			return "", fmt.Errorf("Docker configuration is not initialized, but required for non-Linux platforms")
		}
		fmt.Println("Using Docker for image decompression...")

		// Copy the source image to the container's temp directory
		imgBaseName := filepath.Base(sourceImgXZAbs)
		containerImgPath := fmt.Sprintf("/tmp/%s", imgBaseName)
		decompressedPath := fmt.Sprintf("/tmp/%s", strings.TrimSuffix(imgBaseName, ".xz"))

		// Check if the image exists inside the container
		checkCmd := fmt.Sprintf("ls -la %s", containerImgPath)
		_, err := a.executeDockerCommand(checkCmd)
		if err != nil {
			fmt.Printf("DEBUG: Image not found in container at %s\n", containerImgPath)
			fmt.Printf("DEBUG: Will copy image to container\n")

			// Copy image to container using the Docker adapter's container name
			containerName := a.dockerAdapter.Container.ContainerID
			copyCmd := exec.Command("docker", "cp",
				sourceImgXZAbs,
				fmt.Sprintf("%s:%s", containerName, containerImgPath))

			copyOutput, err := runCommand(copyCmd)
			if err != nil {
				return "", fmt.Errorf("failed to copy image to container: %w, output: %s", err, string(copyOutput))
			}

			fmt.Printf("DEBUG: Copied image to container: %s -> %s\n", sourceImgXZAbs, containerImgPath)
		}

		// Execute xz decompression in Docker
		dockerCmd := fmt.Sprintf("xz -d -k -f %s", containerImgPath)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return "", fmt.Errorf("Docker decompression failed: %w\nOutput: %s", err, output)
		}

		// Return the path to the decompressed file in the container
		return decompressedPath, nil
	}

	// Native Linux approach
	// Get the output path by removing .xz extension
	outputPath := filepath.Join(tmpDir, strings.TrimSuffix(filepath.Base(sourceImgXZAbs), ".xz"))

	// Create a copy of the source file in the temp directory
	srcFile, err := os.Open(sourceImgXZAbs)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer dstFile.Close()

	// Use xz command to decompress
	cmd := exec.Command("xz", "-d", "-k", sourceImgXZAbs)
	cmd.Dir = tmpDir
	output, err := runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("xz decompression failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// RecompressImageXZ implements ImageOpsAdapter.RecompressImageXZ
func (a *imageOpsAdapter) RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for image compression...")

		// The image should already be in the container from previous operations
		// Remove any existing .xz file first
		rmCmd := fmt.Sprintf("rm -f %s.xz", modifiedImgPath)
		fmt.Printf("DEBUG: Removing existing compressed file: %s\n", rmCmd)
		if _, err := a.executeDockerCommand(rmCmd); err != nil {
			return fmt.Errorf("failed to remove existing compressed file: %w", err)
		}

		// Execute xz compression
		dockerCmd := fmt.Sprintf("xz -9 -k -f %s", modifiedImgPath)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := a.executeDockerCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker compression failed: %w\nOutput: %s", err, output)
		}

		// Copy the compressed file from the container
		copyCmd := exec.Command("docker", "cp",
			fmt.Sprintf("%s:%s.xz", a.dockerAdapter.Container.ContainerID, modifiedImgPath),
			finalXZPath)

		copyOutput, err := runCommand(copyCmd)
		if err != nil {
			return fmt.Errorf("failed to copy compressed image from container: %w, output: %s", err, string(copyOutput))
		}

		return nil
	}

	// Native Linux approach
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(finalXZPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use xz command to compress with maximum compression
	cmd := exec.Command("xz", "-9", "-k", modifiedImgPath)
	output, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("xz compression failed: %w\nOutput: %s", err, string(output))
	}

	// Move the compressed file to the final location
	compressedPath := modifiedImgPath + ".xz"
	if err := os.Rename(compressedPath, finalXZPath); err != nil {
		return fmt.Errorf("failed to move compressed file: %w", err)
	}

	return nil
}
