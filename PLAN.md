# Turing Pi Go SDK (`tpi`) Plan

## 1. Overview & Goals

This document outlines the plan for a Go library (`tpi`) designed to automate the configuration, OS installation, and post-installation setup of Turing Pi compute nodes. The library aims to provide a fluent, type-safe, and robust developer experience, respecting the constraints of the Turing Pi environment.

Key goals include:
*   Defining node configurations programmatically in Go.
*   Customizing base OS images via file manipulation before installation.
*   Orchestrating OS flashing onto target nodes.
*   Executing post-installation commands and file operations.
*   Ensuring idempotency and enabling resumption via state management.
*   Providing a clear API tailored to specific OS types (starting with Ubuntu).

## 2. Core Concepts

*   **TPI Configuration (`TPIConfig`)**: A central struct defining the Turing Pi BMC IP, node details, and execution environment (Cache Directory, State File).
*   **Executor (`TuringPiExecutor`)**: The main object created from `TPIConfig`, holding the configuration and internal clients (BMC, state manager). Created via `NewTuringPi`.
*   **Per-Node Workflow (`Run` method)**: The `TuringPiExecutor.Run` method accepts a function defining the workflow template for a *single* node. This function receives context (`tpi.Context`) and the specific node's details (`tpi.Node`). It returns a function that, when called with a `NodeID`, executes the workflow for that specific node.
*   **Fluent Phase Builders**: Specialized builders (e.g., `NewUbuntuImage`, `NewUbuntuOSInstaller`) guide the developer in defining the steps for each phase *within* the workflow function.
*   **Immediate Phase **Execution****: Each phase builder culminates in a `.Run(ctx)` method which executes that phase's logic immediately before the workflow definition proceeds. Results are passed between phases.
*   **State Management**: A local state file tracks the completion status, inputs, and outputs of each phase per node, enabling idempotency and resumption.
*   **Context (`tpi.Context`, `tpi.Node`)**: Passed through the workflow, providing cancellation, access to shared resources (logging, credentials), and node-specific details.

## 3. Configuration (`TPIConfig`)

The configuration struct, potentially loaded from YAML or defined directly in code.

```go
package tpi

type NodeID int // Enum to identify nodes

const (
	Node1 NodeID = 1
	Node2 NodeID = 2
	Node3 NodeID = 3
	Node4 NodeID = 4
)

type TPIConfig struct {
	// IP address of the Turing Pi Board Management Controller. REQUIRED.
	IP string `yaml:"ip"` // Changed from BMCIpAddress to IP

	// Path to the directory for caching prepared images and state.
	// Default: OS-specific user cache dir (e.g., ~/.cache/tpi).
	CacheDir string `yaml:"cacheDir,omitempty"`

	// Name of the state file within CacheDir.
	// Default: "tpi_state.json".
	StateFileName string `yaml:"stateFileName,omitempty"`

	// --- Node Configurations ---
	// Use pointers to allow optional configuration per node.
	Node1 *NodeConfig `yaml:"node1,omitempty"`
	Node2 *NodeConfig `yaml:"node2,omitempty"`
	Node3 *NodeConfig `yaml:"node3,omitempty"`
	Node4 *NodeConfig `yaml:"node4,omitempty"`

	// Future: Configuration for credentials access.
	// Credentials CredentialStoreConfig `yaml:"credentials"`
}

type NodeConfig struct {
	// Static IP assigned AFTER configuration. Used for post-install SSH. REQUIRED within NodeConfig.
	IP    string    `yaml:"ip"`
	// Board type. REQUIRED within NodeConfig.
	Board BoardType `yaml:"board"` // RK1, CM4, etc.
	// Optional: MAC Address, useful for identifying nodes before IP config.
	MacAddress string `yaml:"macAddress,omitempty"`
}

type BoardType string
const (
	RK1 BoardType = "rk1"
	CM4 BoardType = "cm4"
)
```

*Initialization (`NewTuringPi`)*:
*   Validates required fields (`IP`).
*   Validates configured Nodes (at least one should be present).
*   Resolves default `CacheDir` and `StateFileName` if empty.
*   Ensures `CacheDir` exists.
*   Initializes internal components (state manager, potentially BMC client using `IP`).
*   Returns the main `TuringPiExecutor` object.

## 4. API Usage Flow (Conceptual Example)

```go
package main

import (
	"context"
	"fmt" // Added for Sprintf
	"log"
	"github.com/davidroman0O/turingpi/pkg/tpi" // Public API
)

func main() {
	ctx := context.Background()

	// 1. Configure the Turing Pi setup (Matches user example)
	cluster, err := tpi.NewTuringPi(tpi.TPIConfig{
		IP: "192.168.1.90", // BMC IP
		// CacheDir:     "/path/to/my/tpi_cache", // Optional override
		// StateFileName: "my_state.json", // Optional override
		Node1: &tpi.NodeConfig{ // Use pointer
			IP: "192.168.1.100", Board: tpi.RK1,
		},
		Node2: &tpi.NodeConfig{ // Use pointer
			IP: "192.168.1.101", Board: tpi.RK1,
		},
		Node3: &tpi.NodeConfig{ // Use pointer
			IP: "192.168.1.102", Board: tpi.RK1,
		},
		Node4: &tpi.NodeConfig{ // Use pointer
			IP: "192.168.1.103", Board: tpi.RK1,
		},
	})
	if err != nil { log.Fatalf("Config error: %v", err) }

	// 2. Define the per-node workflow template
	runWorkflowForNodeFn := cluster.Run(func(ctx tpi.Context, node tpi.Node) error { // Assuming tpi.Node has derived fields like IPAddress, Hostname etc.
		log.Printf("Starting workflow for Node %d (%s)", node.ID, node.Config.Board) // Use node.ID (enum)

		// --- Phase 1: Image Customization ---
		customImageResult, err := tpi.NewUbuntuImage(). // Assuming this exists
			WithBaseImage("/path/to/base.img").
			WithNetworkConfig(tpi.NetworkConfig{ // Use node-specific details from tpi.Node
				IPCIDR:     fmt.Sprintf("%s/24", node.IPAddress), // Assuming IPAddress field in tpi.Node
				Hostname:   node.Hostname,                     // Assuming Hostname field in tpi.Node
				Gateway:    node.Gateway,                      // Assuming Gateway field in tpi.Node
				DNSServers: node.DNSServers,                   // Assuming DNSServers field in tpi.Node
			}).
			WithPreInstall(func(image *tpi.ImageModifier) error { // Adjusted type name for clarity
				log.Println("Applying pre-install file modifications...")
				// Assuming methods exist on ImageModifier:
				if err := image.WriteFile("/etc/my_app/config", []byte("data"), 0644); err != nil { return err }
				if err := image.CopyLocalFile("./files/script.sh", "/usr/local/bin/script.sh"); err != nil { return err }
				// image.Chmod("/usr/local/bin/script.sh", 0755)
				return nil
			}).
			Run(ctx) // Execute image customization immediately
		if err != nil { return fmt.Errorf("image customization failed: %w", err) }

		// --- Phase 2: OS Installation ---
		err = tpi.NewUbuntuOSInstaller(tpi.UbuntuInstallConfig{ // Assuming this exists
			InitialUserPassword: "ubuntu", // Changed from NewPassword to match example
		}).
			UsingImage(customImageResult). // Renamed UsingImagePlan to UsingImage
			WithGenericConfig(tpi.OSInstallConfig{ // Optional generic params
				SSHKeys: []string{"ssh-rsa AAA..."},
			}).
			Run(ctx) // Execute OS installation immediately
		if err != nil { return fmt.Errorf("OS installation failed: %w", err) }

		// --- Phase 3: Post-Installation ---
		err = tpi.NewUbuntuPostInstaller(). // Assuming this exists
			RunActions(ctx, func(local *tpi.LocalRuntime, remote *tpi.UbuntuRuntime) error { // Adjusted method name from example
				log.Println("Running post-install actions...")
				if err := local.CopyFile("./app_binary", "/usr/local/bin/app", true); err != nil { return err } // Local -> Remote
				if err := remote.RunCommand("systemctl enable my_app", 0); err != nil { return err }
				// Example OS-specific helper:
				// if _, err := remote.AptInstall("htop"); err != nil { return err }
				return nil
			}) // Execute post-install actions immediately
		if err != nil { return fmt.Errorf("post-installation failed: %w", err) }

		log.Printf("Workflow completed successfully for Node %d", node.ID)
		return nil // Success for this node's workflow
	})

	// 3. Execute the workflow for specific node(s) (Matches user example)
	// Pass background context, could pass a more specific one if needed.
	if err := runWorkflowForNodeFn(context.Background(), tpi.Node1); err != nil {
		log.Fatalf("Node 1 failed: %v", err)
	}
	// if err := runWorkflowForNodeFn(context.Background(), tpi.Node2); err != nil { ... } // Run for other nodes

	log.Println("All specified node workflows completed.")
}
```

## 5. Phase 1: Image Customization (Ubuntu Example)

*   **Builder**: `tpi.NewUbuntuImage()`
*   **Key Methods**:
    *   `WithBaseImage(path string)`: Specifies the path to the input `.img.xz` file.
    *   `WithNetworkConfig(tpi.NetworkConfig)`: Defines static network settings (IP/CIDR, Hostname, Gateway, DNS). The `tpi.Node` object passed to the main workflow function will contain derived values for these fields based on `TPIConfig` and potentially naming conventions.
    *   `WithPreInstall(func(image *tpi.ImageModifier) error)`: Accepts a callback function for file-based modifications. The `tpi.ImageModifier` provides methods like `WriteFile`, `CopyLocalFile`, `MkdirAll` (operating safely within the mounted image context). **No command execution is possible here.**
    *   `Run(ctx tpi.Context)`: Executes the customization.
*   **Internal Execution (`Run` logic)**:
    1.  Check state for completion based on hashed inputs (base image path, network config, file op list). Skip if completed and inputs match.
    2.  Mark state as "running".
    3.  Decompress base image (from `prep.go`).
    4.  Map partitions (`kpartx`, from `prep.go`).
    5.  Mount root filesystem (from `prep.go`).
    6.  Apply Network Config: Write `/etc/hostname`, `/etc/netplan/01-turing-static.yaml` using node-specific values (from `prep.go`, adapted).
    7.  Execute `WithPreInstall` Callback: Run the provided function, executing the requested file operations using internal helpers (e.g., `sudo tee`, `sudo cp`, `sudo mkdir`).
    8.  Sync, Unmount, Cleanup (`kpartx -d`, from `prep.go`).
    9.  Recompress image to cache dir (e.g., `<cacheDir>/<hostname>.img.xz`, from `prep.go`).
    10. Update state: Mark "completed", store input hash, store output image path.
    11. Return `*tpi.ImageResult` (containing the path) or error.

## 6. Phase 2: OS Installation (Ubuntu Example)

*   **Builder**: `tpi.NewUbuntuOSInstaller(tpi.UbuntuInstallConfig)` (OS-specific config passed initially).
*   **Key Methods**:
    *   `UsingImage(*tpi.ImageResult)`: Specifies the customized image result from Phase 1.
    *   `WithGenericConfig(tpi.OSInstallConfig)`: Adds generic OS configuration (e.g., SSH keys).
    *   `Run(ctx tpi.Context)`: Executes the installation.
*   **Internal Execution (`Run` logic)**:
    1.  Check state for completion based on hashed inputs (customized image path, install configs). Skip if completed and inputs match.
    2.  Mark state as "running".
    3.  Retrieve the actual customized image path from the `ImageResult` or state.
    4.  Select the appropriate flashing mechanism based on board type (e.g., `rpiboot` for CM4, potentially different for RK1).
    5.  Interact with BMC (using `TPIConfig.IP`) if needed for power cycling or enabling flashing mode.
    6.  Execute the flashing tool, passing the image path. Potentially inject `UbuntuInstallConfig` / `OSInstallConfig` parameters if the flashing tool or image's first-boot process (e.g., cloud-init) supports it.
    7.  Wait for flashing completion. Handle tool errors.
    8.  Update state: Mark "completed", store input hash.
    9.  Return `error`.

## 7. Phase 3: Post-Installation (Ubuntu Example)

*   **Builder**: `tpi.NewUbuntuPostInstaller()`
*   **Key Methods**:
    *   `RunActions(ctx tpi.Context, func(local *tpi.LocalRuntime, remote *tpi.UbuntuRuntime) error)`: Accepts a callback containing the sequence of post-install actions.
        *   `tpi.LocalRuntime`: Provides methods operating on the control machine (e.g., `CopyFile` where source is local).
        *   `tpi.UbuntuRuntime`: Provides methods operating on the remote node via SSH/SFTP (e.g., `RunCommand`, `CopyFile` where target is remote, `AptInstall`, etc.).
*   **Internal Execution (`RunActions` logic)**:
    1.  Check state for completion based on hashed inputs (the sequence/content of actions defined in the callback). Skip if completed and inputs match.
    2.  Mark state as "running".
    3.  Establish SSH/SFTP connection to the node (using node IP from `NodeConfig` and credentials from `tpi.Context`).
    4.  Execute the callback function (`func(local..., remote...)`).
    5.  Internally, `local.CopyFile` uses local filesystem access, while `remote.RunCommand`, `remote.CopyFile`, `remote.AptInstall` use the established SSH/SFTP connection (leveraging `pkg/node` functions).
    6.  Log output from remote commands. Handle errors from individual actions within the callback. If the callback returns an error, the phase fails.
    7.  Update state: Mark "completed", store input hash.
    8.  Return `error` from the callback or connection errors.

## 8. State Management

*   **Purpose**: Achieve idempotency (running the same workflow multiple times yields the same result without re-executing completed steps) and allow resumption after failures.
*   **Location**: Local file, defaults to `<UserCacheDir>/tpi/tpi_state.json`. Path configurable via `TPIConfig`.
*   **Structure**: JSON object keyed by `NodeID` (e.g., "Node1", "Node2"). Each node has phase entries (`image_customization`, `os_installation`, `post_installation`) storing:
    *   `status`: "pending", "running", "failed", "completed"
    *   `timestamp`: Last status update time.
    *   `input_hash`: SHA256 hash of relevant inputs for the phase (to detect config changes).
    *   `output_image_path` (for image customization): Path to the resulting cached image.
*   **Locking**: Use file locking when reading/writing the state file to prevent corruption.
*   **Execution Logic**: Before executing any phase's `.Run`, check the state. If `completed` and `input_hash` matches, skip. Otherwise, run, and update state on success or failure.

## 9. Error Handling

*   Each phase's `.Run(ctx)` method returns an `error`.
*   The developer's workflow function (`func(ctx tpi.Context, node tpi.Node) error`) uses standard Go `if err != nil { return err }` checks after each phase's `Run` call to stop processing for that node on failure.
*   The function returned by `cluster.Run` also returns an error, indicating the final success or failure for that specific node's execution.
*   Internal errors (mounting, flashing, SSH, state file access) are wrapped and propagated up.

## 10. Cross-Platform Compatibility

*   **OS Detection**: The library must detect the host operating system to determine the appropriate execution strategy.
*   **Linux Execution**: On Linux hosts, tools like `kpartx` and other system utilities can be executed directly.
*   **Non-Linux Execution (macOS, Windows)**:
    * For image preparation and manipulation that requires Linux-specific tools (e.g., `kpartx`), the library must:
        * Automatically spawn a Docker container with the necessary tools
        * Mount appropriate volumes between host and container:
            * Source image directory
            * Temporary processing directory
            * Output/cache directory
        * Execute the Linux-specific operations inside the container
        * Clean up the container when finished
    * For operations that can run on any platform (file operations, SSH connections), execute them directly on the host.
*   **Configuration**: Allow users to customize Docker image, container options, and volume mappings if needed.
*   **Fallback**: Provide clear error messages with installation instructions if Docker is not available on non-Linux systems.

## 11. Next Steps (Implementation Order)

1.  Define core structs (`TPIConfig`, `NodeID`, `NodeConfig`, `BoardType`, `Context`, `Node`).
2.  Implement `NewTuringPi` configuration loading and default handling.
3.  Implement the state management basics (file loading, saving, locking, basic status tracking).
4.  Refactor `pkg/imageprep/prep.go` logic into modular internal functions (mount, unmount, write_config, run_file_ops, compress, etc.).
5.  Implement `NewUbuntuImage` builder and its `.Run` method, integrating the refactored image prep logic and state checks/updates. Include `ImageModifier` for the `WithPreInstall` callback.
6.  Implement `NewUbuntuOSInstaller` builder and its `.Run` method (initially focusing on one board type's flashing mechanism) with state checks/updates.
7.  Implement `NewUbuntuPostInstaller` builder and its `RunActions` method, including `LocalRuntime`, `UbuntuRuntime`, SSH/SFTP integration (using `pkg/node`), and state checks/updates.
8.  Implement the main `TuringPiExecutor.Run` method orchestrator.
9.  Add input hashing to state management for improved idempotency.
10. Develop tests and examples. 

## 12. Extensibility

*   Add support for other OS types (e.g., Debian, Arch) by creating corresponding builders (`NewDebianImage`, `NewDebianOSInstaller`, `NewDebianPostInstaller`) and OS-specific runtime helpers (`DebianRuntime`).
*   The core execution logic (`TuringPiExecutor.Run`) and state management should remain largely OS-agnostic.
