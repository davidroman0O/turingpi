package operations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageOperations_CopyToDevice(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		mockExec.MockResponses["dd if=/path/to/image.img of=/dev/sdX bs=4M status=progress"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		mockExec.MockResponses["sync"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.CopyToDevice(ctx, "/path/to/image.img", "/dev/sdX")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 3, len(mockExec.Calls), "Expected 3 commands to be executed")
	})

	t.Run("image not found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/nonexistent.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("file not found")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.CopyToDevice(ctx, "/path/to/nonexistent.img", "/dev/sdX")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image file does not exist")
	})

	t.Run("dd error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		mockExec.MockResponses["dd if=/path/to/image.img of=/dev/sdX bs=4M status=progress"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("dd: failed to open '/dev/sdX': Permission denied")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.CopyToDevice(ctx, "/path/to/image.img", "/dev/sdX")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy image")
	})
}

func TestImageOperations_ResizePartition(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["fdisk -l /dev/sdX"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte(`Disk /dev/sdX: 32 GiB
			Device     Boot   Start      End  Sectors  Size Id Type
			/dev/sdX1  *       2048   999423   997376  487M  c W95 FAT32 (LBA)
			/dev/sdX2       1001470 62521343 61519874 29.3G 83 Linux`),
			Err: nil,
		}
		mockExec.MockResponses["growpart /dev/sdX 2"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		// Add a response for the filesystem type check with correct parameter order
		mockExec.MockResponses["blkid -o value -s TYPE /dev/sdX2"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("ext4"), Err: nil}
		// Add a response for the resize2fs command
		mockExec.MockResponses["resize2fs /dev/sdX2"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ResizePartition(ctx, "/dev/sdX")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 4, len(mockExec.Calls), "Expected 4 commands to be executed")
	})

	t.Run("fdisk error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["fdisk -l /dev/sdX"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("unable to open device")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ResizePartition(ctx, "/dev/sdX")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get device info")
	})

	t.Run("no partitions found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["fdisk -l /dev/sdX"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte(`Disk /dev/sdX: 32 GiB`),
			Err:    nil,
		}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ResizePartition(ctx, "/dev/sdX")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no partitions found")
	})

	t.Run("growpart error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["fdisk -l /dev/sdX"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte(`Disk /dev/sdX: 32 GiB
			Device     Boot   Start      End  Sectors  Size Id Type
			/dev/sdX1  *       2048   999423   997376  487M  c W95 FAT32 (LBA)
			/dev/sdX2       1001470 62521343 61519874 29.3G 83 Linux`),
			Err: nil,
		}
		mockExec.MockResponses["growpart /dev/sdX 2"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("NOCHANGE: partition 2 is already at the disk end")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ResizePartition(ctx, "/dev/sdX")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resize partition")
	})

	t.Run("growpart NOCHANGE success", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["fdisk -l /dev/sdX"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte(`Disk /dev/sdX: 32 GiB
			Device     Boot   Start      End  Sectors  Size Id Type
			/dev/sdX1  *       2048   999423   997376  487M  c W95 FAT32 (LBA)
			/dev/sdX2       1001470 62521343 61519874 29.3G 83 Linux`),
			Err: nil,
		}
		mockExec.MockResponses["growpart /dev/sdX 2"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("NOCHANGE: partition 2 is already at the disk end"), Err: fmt.Errorf("NOCHANGE")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ResizePartition(ctx, "/dev/sdX")

		// Assert
		assert.NoError(t, err)
	})
}

func TestImageOperations_ValidateImage(t *testing.T) {
	ctx := context.Background()

	t.Run("valid image", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		mockExec.MockResponses["fdisk -l /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte(`Disk /path/to/image.img: 8 GiB
			Device              Boot   Start      End  Sectors  Size Id Type
			/path/to/image.img1 *       2048   999423   997376  487M  c W95 FAT32 (LBA)
			/path/to/image.img2       1001470 16777215 15775746  7.5G 83 Linux`),
			Err: nil,
		}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ValidateImage(ctx, "/path/to/image.img")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("image not found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/nonexistent.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("file not found")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ValidateImage(ctx, "/path/to/nonexistent.img")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image file does not exist")
	})

	t.Run("not a valid disk image", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}
		mockExec.MockResponses["fdisk -l /path/to/image.img"] = struct {
			Output []byte
			Err    error
		}{
			Output: []byte("fdisk: cannot open /path/to/image.img: Invalid argument"),
			Err:    fmt.Errorf("fdisk error"),
		}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ValidateImage(ctx, "/path/to/image.img")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a valid disk image")
	})
}

func TestImageOperations_ExtractBootFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["mkdir -p /tmp/boot-extract"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Use bash find command which is what the actual implementation uses
		mockExec.MockResponses["bash -c find /boot/boot -name 'vmlinuz*' -o -name 'kernel*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("/boot/boot/vmlinuz-5.4.0-144-generic"), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'initrd*' -o -name 'initramfs*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("/boot/boot/initrd.img-5.4.0-144-generic"), Err: nil}

		mockExec.MockResponses["cp /boot/boot/vmlinuz-5.4.0-144-generic /tmp/boot-extract/vmlinuz-5.4.0-144-generic"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cp /boot/boot/initrd.img-5.4.0-144-generic /tmp/boot-extract/initrd.img-5.4.0-144-generic"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute - ExtractBootFiles returns 3 values
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, "/boot/boot", "/tmp/boot-extract")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/boot-extract/vmlinuz-5.4.0-144-generic", kernel)
		assert.Equal(t, "/tmp/boot-extract/initrd.img-5.4.0-144-generic", initrd)
		assert.Equal(t, 5, len(mockExec.Calls), "Expected 5 commands to be executed")
	})

	t.Run("mkdir error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["mkdir -p /tmp/boot-extract"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("permission denied")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, "/boot/boot", "/tmp/boot-extract")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create output directory")
		assert.Empty(t, kernel)
		assert.Empty(t, initrd)
	})

	t.Run("kernel not found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["mkdir -p /tmp/boot-extract"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'vmlinuz*' -o -name 'kernel*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, "/boot/boot", "/tmp/boot-extract")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kernel file not found in boot partition")
		assert.Empty(t, kernel)
		assert.Empty(t, initrd)
	})

	t.Run("initrd not found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["mkdir -p /tmp/boot-extract"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'vmlinuz*' -o -name 'kernel*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("/boot/boot/vmlinuz-5.4.0-144-generic"), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'initrd*' -o -name 'initramfs*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, "/boot/boot", "/tmp/boot-extract")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initrd file not found in boot partition")
		assert.Empty(t, kernel)
		assert.Empty(t, initrd)
	})

	t.Run("copy kernel error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["mkdir -p /tmp/boot-extract"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'vmlinuz*' -o -name 'kernel*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("/boot/boot/vmlinuz-5.4.0-144-generic"), Err: nil}

		mockExec.MockResponses["bash -c find /boot/boot -name 'initrd*' -o -name 'initramfs*' | sort | tail -1"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("/boot/boot/initrd.img-5.4.0-144-generic"), Err: nil}

		mockExec.MockResponses["cp /boot/boot/vmlinuz-5.4.0-144-generic /tmp/boot-extract/vmlinuz-5.4.0-144-generic"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("no space left on device")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, "/boot/boot", "/tmp/boot-extract")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy kernel file")
		assert.Empty(t, kernel)
		assert.Empty(t, initrd)
	})
}

func TestImageOperations_ApplyDTBOverlay(t *testing.T) {
	ctx := context.Background()

	t.Run("success - config.txt exists", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Test overlays directory
		mockExec.MockResponses["test -d /boot/boot/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cp /path/to/overlay.dtbo /boot/boot/overlays/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["test -f /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cat /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("# Raspberry Pi config\ndtparam=i2c_arm=on"), Err: nil}

		// Add execution for creating new config
		mockExec.MockResponses["cat > /tmp/config.txt.new"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["mv /tmp/config.txt.new /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/overlay.dtbo")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 7, len(mockExec.Calls), "Expected 7 commands to be executed")
	})

	t.Run("success - overlay already in config", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Test overlays directory
		mockExec.MockResponses["test -d /boot/boot/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cp /path/to/overlay.dtbo /boot/boot/overlays/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["test -f /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cat /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte("# Raspberry Pi config\ndtparam=i2c_arm=on\ndtoverlay=overlay"), Err: nil}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/overlay.dtbo")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 5, len(mockExec.Calls), "Expected 5 commands to be executed")
	})

	t.Run("no config.txt file", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Test overlays directory
		mockExec.MockResponses["test -d /boot/boot/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cp /path/to/overlay.dtbo /boot/boot/overlays/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["test -f /boot/boot/config.txt"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("not found")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/overlay.dtbo")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 4, len(mockExec.Calls), "Expected 4 commands to be executed")
	})

	t.Run("overlay file not found", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/nonexistent.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("file not found")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/nonexistent.dtbo")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dtb overlay file does not exist")
	})

	t.Run("no overlays directory", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Test both possible overlays directories
		mockExec.MockResponses["test -d /boot/boot/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("not found")}

		mockExec.MockResponses["test -d /boot/boot/dtbs/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("not found")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/overlay.dtbo")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "overlays directory not found")
	})

	t.Run("copy error", func(t *testing.T) {
		// Setup
		mockExec := NewMockExecutor()
		mockExec.MockResponses["test -f /path/to/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		// Test overlays directory
		mockExec.MockResponses["test -d /boot/boot/overlays"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: nil}

		mockExec.MockResponses["cp /path/to/overlay.dtbo /boot/boot/overlays/overlay.dtbo"] = struct {
			Output []byte
			Err    error
		}{Output: []byte(""), Err: fmt.Errorf("no space left on device")}

		imgOps := NewImageOperations(mockExec)

		// Execute
		err := imgOps.ApplyDTBOverlay(ctx, "/boot/boot", "/path/to/overlay.dtbo")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy dtb overlay file")
	})
}
