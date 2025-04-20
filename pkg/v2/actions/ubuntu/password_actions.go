// Package ubuntu provides Ubuntu-specific actions for TuringPi
package ubuntu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/gostate/store"
	"github.com/davidroman0O/turingpi/pkg/v2/actions"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// SetRK1PasswordAction configures the password for the default user in Ubuntu on RK1
type SetRK1PasswordAction struct {
	actions.PlatformActionBase
}

// NewSetRK1PasswordAction creates a new SetRK1PasswordAction
func NewSetRK1PasswordAction() *SetRK1PasswordAction {
	return &SetRK1PasswordAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"SetRK1Password",
			"Configure password for the default user on RK1 node",
		),
	}
}

// ExecuteNative implements the native Linux execution path
func (a *SetRK1PasswordAction) ExecuteNative(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	imageTool := toolProvider.GetImageTool()
	if imageTool == nil {
		return fmt.Errorf("image tool is required but not available")
	}

	fsTool := toolProvider.GetFSTool()
	if fsTool == nil {
		return fmt.Errorf("filesystem tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get the password from the context
	password, err := store.GetOrDefault(ctx.Store, "nodePassword", "turingpi")
	if err != nil {
		return fmt.Errorf("failed to get node password: %w", err)
	}

	// Create a temporary mount point
	mountDir, err := store.GetOrDefault(ctx.Store, "mountPoint", filepath.Join(os.TempDir(), "turingpi-mnt"))
	if err != nil {
		return fmt.Errorf("failed to get mount point: %w", err)
	}

	// Ensure mount directory exists
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	ctx.Logger.Info("Setting user password in the image")

	// Map partitions
	devicePath, err := imageTool.MapPartitions(context.Background(), decompressedImagePath)
	if err != nil {
		return fmt.Errorf("failed to map partitions: %w", err)
	}

	// Ensure partitions are unmapped when done
	defer func() {
		ctx.Logger.Info("Unmapping partitions")
		_ = imageTool.UnmapPartitions(context.Background(), decompressedImagePath)
	}()

	// Mount the filesystem
	if err := imageTool.MountFilesystem(context.Background(), devicePath, mountDir); err != nil {
		return fmt.Errorf("failed to mount filesystem: %w", err)
	}

	// Ensure filesystem is unmounted when done
	defer func() {
		ctx.Logger.Info("Unmounting filesystem")
		_ = imageTool.UnmountFilesystem(context.Background(), mountDir)
	}()

	// Apply password configuration
	if err := applyPasswordConfig(ctx, imageTool, mountDir, password); err != nil {
		return fmt.Errorf("failed to apply password configuration: %w", err)
	}

	ctx.Logger.Info("Password configuration completed successfully")
	return nil
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *SetRK1PasswordAction) ExecuteDocker(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	containerTool := toolProvider.GetContainerTool()
	if containerTool == nil {
		return fmt.Errorf("container tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get the password from the context
	password, err := store.GetOrDefault(ctx.Store, "nodePassword", "turingpi")
	if err != nil {
		return fmt.Errorf("failed to get node password: %w", err)
	}

	// Get temp directory for mounting
	tempDir, err := store.GetOrDefault(ctx.Store, "tempDir", os.TempDir())
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	ctx.Logger.Info("Setting user password in Docker container")

	// Create a container for password configuration
	config := createPasswordContainerConfig(decompressedImagePath, tempDir)
	container, err := containerTool.CreateContainer(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		ctx.Logger.Info("Cleaning up container")
		_ = containerTool.RemoveContainer(context.Background(), container.ID())
	}()

	// Map partitions in container
	containerImgPath := filepath.Join("/images", filepath.Base(decompressedImagePath))
	mapCmd := []string{"kpartx", "-av", containerImgPath}
	output, err := containerTool.RunCommand(context.Background(), container.ID(), mapCmd)
	if err != nil {
		return fmt.Errorf("failed to map partitions in container: %w", err)
	}

	// Parse the output to find the device path
	devicePath, err := parseKpartxOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Ensure partitions are unmapped when done
	defer func() {
		ctx.Logger.Info("Unmapping partitions in container")
		unmapCmd := []string{"kpartx", "-d", containerImgPath}
		_, _ = containerTool.RunCommand(context.Background(), container.ID(), unmapCmd)
	}()

	// Create mount directory in container
	mkdirCmd := []string{"mkdir", "-p", "/mnt"}
	_, err = containerTool.RunCommand(context.Background(), container.ID(), mkdirCmd)
	if err != nil {
		return fmt.Errorf("failed to create mount directory in container: %w", err)
	}

	// Mount the filesystem in container
	mountCmd := []string{"mount", devicePath, "/mnt"}
	_, err = containerTool.RunCommand(context.Background(), container.ID(), mountCmd)
	if err != nil {
		return fmt.Errorf("failed to mount filesystem in container: %w", err)
	}

	// Ensure filesystem is unmounted when done
	defer func() {
		ctx.Logger.Info("Unmounting filesystem in container")
		unmountCmd := []string{"umount", "/mnt"}
		_, _ = containerTool.RunCommand(context.Background(), container.ID(), unmountCmd)
	}()

	// Apply password configuration in container
	if err := applyPasswordConfigInContainer(ctx, containerTool, container.ID(), password); err != nil {
		return fmt.Errorf("failed to apply password configuration in container: %w", err)
	}

	ctx.Logger.Info("Password configuration completed successfully in container")
	return nil
}

// applyPasswordConfig applies password configuration to a mounted filesystem
func applyPasswordConfig(ctx *gostate.ActionContext, imageTool tools.ImageTool, mountDir, password string) error {
	// For Ubuntu, we use shadow file manipulation or chpasswd when the system is running
	// Here we'll modify the shadow file directly to set a pre-encrypted password

	// The encrypted password is typically stored in the shadow file
	// For a more secure implementation, you'd want to:
	// 1. Generate a proper hashed password with salt
	// 2. Update the shadow file entry for the user (ubuntu or root)

	// For demonstration purposes, we'll create a simple chroot script that will
	// be executed first boot to set the password

	// Create the first-boot service to set the password
	firstBootService := `[Unit]
Description=First Boot Setup for TuringPi
After=network.target
ConditionPathExists=!/var/lib/turingpi/first-boot-done

[Service]
Type=oneshot
ExecStart=/usr/local/bin/turingpi-firstboot.sh
ExecStartPost=/bin/touch /var/lib/turingpi/first-boot-done

[Install]
WantedBy=multi-user.target
`
	// Create the first-boot script
	// In a real implementation, this would properly hash the password
	firstBootScript := fmt.Sprintf(`#!/bin/bash
echo "ubuntu:%s" | chpasswd
echo "root:%s" | chpasswd
`, password, password)

	// Create required directories
	servicePath := filepath.Join(mountDir, "etc/systemd/system/turingpi-firstboot.service")
	scriptPath := filepath.Join(mountDir, "usr/local/bin/turingpi-firstboot.sh")
	flagDirPath := filepath.Join(mountDir, "var/lib/turingpi")

	// Ensure flag directory exists
	os.MkdirAll(filepath.Dir(servicePath), 0755)
	os.MkdirAll(filepath.Dir(scriptPath), 0755)
	os.MkdirAll(flagDirPath, 0755)

	// Write the service file
	if err := imageTool.WriteFile(context.Background(), mountDir, "etc/systemd/system/turingpi-firstboot.service", []byte(firstBootService), 0644); err != nil {
		return fmt.Errorf("failed to write first-boot service: %w", err)
	}

	// Write the script and make it executable
	if err := imageTool.WriteFile(context.Background(), mountDir, "usr/local/bin/turingpi-firstboot.sh", []byte(firstBootScript), 0755); err != nil {
		return fmt.Errorf("failed to write first-boot script: %w", err)
	}

	// Create the systemd symlink to enable the service at first boot
	// Instead of using RunChroot, we'll create the symlink directly
	targetPath := filepath.Join(mountDir, "etc/systemd/system/multi-user.target.wants")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to create systemd wants directory: %w", err)
	}

	// Create the symlink
	symlinkTarget := filepath.Join(targetPath, "turingpi-firstboot.service")
	symlinkSource := "../turingpi-firstboot.service"
	if err := os.Symlink(symlinkSource, symlinkTarget); err != nil {
		return fmt.Errorf("failed to create symlink for service: %w", err)
	}

	return nil
}

// applyPasswordConfigInContainer applies password configuration in a container
func applyPasswordConfigInContainer(ctx *gostate.ActionContext, containerTool tools.ContainerTool, containerID, password string) error {
	// Similar to the native implementation, but using container commands

	// Create the first-boot service file
	firstBootService := `[Unit]
Description=First Boot Setup for TuringPi
After=network.target
ConditionPathExists=!/var/lib/turingpi/first-boot-done

[Service]
Type=oneshot
ExecStart=/usr/local/bin/turingpi-firstboot.sh
ExecStartPost=/bin/touch /var/lib/turingpi/first-boot-done

[Install]
WantedBy=multi-user.target
`

	// Create the first-boot script
	firstBootScript := fmt.Sprintf(`#!/bin/bash
echo "ubuntu:%s" | chpasswd
echo "root:%s" | chpasswd
`, password, password)

	// Create required directories
	mkdirCmd := "mkdir -p /mnt/etc/systemd/system /mnt/usr/local/bin /mnt/var/lib/turingpi"
	_, err := containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", mkdirCmd})
	if err != nil {
		return fmt.Errorf("failed to create directories in container: %w", err)
	}

	// Write the service file
	serviceCmd := fmt.Sprintf("cat > /mnt/etc/systemd/system/turingpi-firstboot.service << 'EOF'\n%s\nEOF", firstBootService)
	_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", serviceCmd})
	if err != nil {
		return fmt.Errorf("failed to write service file in container: %w", err)
	}

	// Write the script file
	scriptCmd := fmt.Sprintf("cat > /mnt/usr/local/bin/turingpi-firstboot.sh << 'EOF'\n%s\nEOF", firstBootScript)
	_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", scriptCmd})
	if err != nil {
		return fmt.Errorf("failed to write script file in container: %w", err)
	}

	// Make the script executable
	chmodCmd := "chmod 755 /mnt/usr/local/bin/turingpi-firstboot.sh"
	_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", chmodCmd})
	if err != nil {
		return fmt.Errorf("failed to make script executable in container: %w", err)
	}

	// Enable the service
	enableCmd := "mkdir -p /mnt/etc/systemd/system/multi-user.target.wants && ln -sf /etc/systemd/system/turingpi-firstboot.service /mnt/etc/systemd/system/multi-user.target.wants/turingpi-firstboot.service"
	_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", enableCmd})
	if err != nil {
		return fmt.Errorf("failed to enable service in container: %w", err)
	}

	return nil
}

// Helper function to create a container configuration for password operations
func createPasswordContainerConfig(imagePath, mountDir string) container.ContainerConfig {
	return container.ContainerConfig{
		Image:      "ubuntu:latest",
		Name:       "turingpi-password-config",
		Command:    []string{"sleep", "infinity"}, // Keep container running
		WorkDir:    "/workspace",
		Privileged: true, // Needed for mount operations
		Capabilities: []string{
			"SYS_ADMIN",
			"MKNOD",
		},
		Mounts: map[string]string{
			filepath.Dir(imagePath): "/images",
			mountDir:                "/output",
		},
	}
}
