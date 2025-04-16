# Turing Pi CLI

A command-line tool for managing Turing Pi compute nodes, offering complete OS image preparation, installation, and post-installation configuration.

## Overview

Turing Pi CLI is designed to simplify the process of setting up and managing compute nodes in a [Turing Pi](https://turingpi.com/) cluster. It automates the three key phases of node management:

1. **Image Customization**: Prepares OS images with network settings, hostname, and custom files
2. **OS Installation**: Flashes the customized images onto compute nodes 
3. **Post-Installation**: Configures the system after first boot

## Features

- Complete end-to-end workflow for node setup
- UART access to nodes for direct terminal access
- Power management (on, off, reset) for each node
- Individual commands for running each phase separately
- Direct node access via SSH and SFTP for custom operations
- Status checks for all nodes in the cluster

## Installation

### From Source

```bash
git clone https://github.com/davidroman0O/turingpi.git
cd turingpi
go build -o turingpi
# Optional: Move to a directory in your PATH
sudo mv turingpi /usr/local/bin/
```

### Prebuilt Binaries

Prebuilt binaries are available on the [Releases](https://github.com/davidroman0O/turingpi/releases) page.

## Quick Start

### Setup and Configuration

The CLI connects to your Turing Pi BMC (Board Management Controller) to manage the compute nodes. You'll need to provide the BMC's IP address and credentials.

Default Turing Pi BMC credentials:
- IP: 192.168.1.90
- User: root
- Password: turing

Basic usage:

```bash
turingpi --host 192.168.1.90 --user root --password turing status
```

For convenience, you can set environment variables:

```bash
export TPI_HOST=192.168.1.90
export TPI_USER=root
export TPI_PASSWORD=turing

# Now you can omit those parameters
turingpi status
```

### Complete Workflow

The most powerful command is `workflow`, which runs all three phases in sequence:

```bash
turingpi workflow \
  --node 1 \
  --base-image ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz \
  --node-ip 192.168.1.101 \
  --gateway 192.168.1.1 \
  --hostname rk1-node1
```

This will:
1. Customize the base image with the specified network settings
2. Flash the image to node 1
3. Perform post-installation configuration

### Power Management

```bash
# Power on node 2
turingpi power on --node 2

# Power off node 3
turingpi power off --node 3

# Reset (power cycle) node 1
turingpi power reset --node 1

# Get status of all nodes
turingpi status
```

### Node Access

```bash
# Execute a command on a node
turingpi node exec \
  --node-ip 192.168.1.101 \
  --user ubuntu \
  --password ubuntu \
  --command "uname -a"

# Copy a file to a node
turingpi node copy \
  --node-ip 192.168.1.101 \
  --user ubuntu \
  --password ubuntu \
  --source /local/path/file.txt \
  --dest /remote/path/file.txt
```

### UART Access

Access the console of a node directly:

```bash
# Get UART output from node 1
turingpi uart --node 1 get

# Send a command via UART to node 1
turingpi uart --node 1 set --cmd "ls -la"
```

## Individual Phase Commands

### Phase 1: Image Preparation

```bash
turingpi prepare \
  --source ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz \
  --node 1 \
  --ip 192.168.1.101/24 \
  --hostname rk1-node1 \
  --gateway 192.168.1.1 \
  --dns 1.1.1.1,8.8.8.8 \
  --cache-dir ./prepared-images
```

### Phase 2: OS Installation

```bash
turingpi install \
  --node 1 \
  --image /path/to/prepared-image.img.xz
```

### Phase 3: Post-Installation

```bash
turingpi post-install-ubuntu \
  --node-ip 192.168.1.101 \
  --initial-user ubuntu \
  --initial-password ubuntu \
  --new-password your-secure-password
```

## License

[MIT](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. 