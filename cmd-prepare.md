# `turingpi prepare` Command

## Overview

The `turingpi prepare` command is designed to take a generic, compressed OS image file (e.g., `.img.xz`) compatible with Turing Pi compute modules (like the RK1) and customize it for a specific node. It injects node-specific network configurations (static IP address, hostname, gateway, DNS) into the image filesystem before recompressing it.

This allows you to create node-specific images that will automatically configure their network upon first boot, avoiding the need for manual configuration after flashing and ensuring consistent IP addresses.

## Purpose

- **Static IP Configuration:** Modifies the image's network settings (using Netplan for Ubuntu/Debian) to use a predefined static IP address.
- **Hostname Setting:** Sets the hostname within the image.
- **Prepare for Flashing:** Outputs a new, compressed image (`.img.xz`) ready to be transferred to the BMC and flashed onto the target node.
- **Caching:** Checks a local cache directory (`--cache-dir`) and skips preparation if an image for the specific hostname already exists.

## Dependencies

The `prepare` command relies on several standard Linux utilities that must be available in the execution environment:

- `sudo`: Required for mounting filesystems and mapping partitions.
- `kpartx`: Used to map partitions within the disk image file.
- `xz`: Used for decompressing the source image and recompressing the modified image.
- Standard filesystem tools (`mount`, `umount`, `mkdir`, `rm`, `tee`, `chmod`, `sync`).

## Running with Docker (Recommended)

Due to the Linux-specific dependencies and the need for `sudo`, the recommended way to run the `prepare` command is using the provided `Dockerfile.prepare`. This ensures a consistent environment with all necessary tools.

**Steps:**

1.  **Build the Docker Image:**
    ```bash
    docker build -t turingpi-prepare -f Dockerfile.prepare .
    ```

2.  **Run the `prepare` Command:**
    Execute the command within the container, mounting your source image directory and a local directory for the prepared output images.

    ```bash
    # Define variables for your specific setup
    NODE_NUM=1
    NODE_IP="192.168.1.101/24"
    NODE_GW="192.168.1.1"
    NODE_DNS="1.1.1.1,8.8.8.8"
    NODE_HOSTNAME="rk1-node${NODE_NUM}"
    # --- Adjust these paths ---
    SOURCE_IMAGE_DIR="/Users/davidroman/Documents/iso/turingpi" # Local path to your source images
    SOURCE_IMAGE_NAME="ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz" # Source image filename
    PREPARED_IMAGE_DIR="/Users/davidroman/Documents/code/github/turingpi/prepared-images" # Local path for output images
    # --- End Adjustments ---

    # Ensure the output directory exists on the host
    mkdir -p "${PREPARED_IMAGE_DIR}"

    # Run the command
    docker run --rm -it --privileged \
      -v "${SOURCE_IMAGE_DIR}:/images:ro" \
      -v "${PREPARED_IMAGE_DIR}:/prepared-images:rw" \
      turingpi-prepare \
      sudo /usr/local/bin/turingpi prepare \
        --source "/images/${SOURCE_IMAGE_NAME}" \
        --node "${NODE_NUM}" \
        --ip "${NODE_IP}" \
        --gateway "${NODE_GW}" \
        --dns "${NODE_DNS}" \
        --hostname "${NODE_HOSTNAME}" \
        --cache-dir /prepared-images
    ```

    **Explanation:**
    *   `--privileged`: Required for `kpartx` and `mount`.
    *   `-v "${SOURCE_IMAGE_DIR}:/images:ro"`: Mounts your source images read-only.
    *   `-v "${PREPARED_IMAGE_DIR}:/prepared-images:rw"`: Mounts your output directory read-write.
    *   The rest of the command invokes the `turingpi prepare` binary inside the container with the specified flags.

## Output

The command will output logs detailing the preparation steps (decompression, mounting, modification, recompression). Upon successful completion, a new compressed image file named after the specified hostname (e.g., `rk1-node1.img.xz`) will be created in the designated cache directory (`/prepared-images` inside the container, mapped to your `${PREPARED_IMAGE_DIR}` on the host).

## Compression Level

The command uses `xz` with compression level `-6` (`-zck6`) by default. This provides a good balance between file size and the memory required for decompression on the resource-constrained BMC. Level `-9` was previously used but caused Out-Of-Memory errors on the BMC during decompression.

## Next Steps

After preparing the image, you can transfer the resulting `.img.xz` file to your Turing Pi BMC and use the `install-ubuntu` (or similar) command to flash it onto the target node. 