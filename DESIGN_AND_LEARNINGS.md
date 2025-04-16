# Turing Pi CLI - Design Notes and Learnings (As of 2025-04-16)

## 1. Project Goal

To create a Go-based Command Line Interface (CLI) tool to manage a Turing Pi 2 board and its compute nodes, aiming for automation and overcoming limitations of manual processes or the base `tpi` tool alone.

## 2. Core Problem Addressed

The primary challenge identified was provisioning nodes with specific configurations reliably. Simply flashing a generic OS image using `tpi flash` typically results in:

1.  **Dynamic IP Addresses:** Nodes acquire IPs via DHCP, making them unpredictable for subsequent scripting or direct SSH access without discovery.
2.  **Manual First-Boot Configuration:** Many OS images (like the Ubuntu server image used) require manual intervention on the first boot (e.g., changing the default password via console/SSH) before they are fully usable.

## 3. Chosen Strategy: Multi-Stage Provisioning

To address the core problem, a multi-stage strategy was adopted, breaking the process into distinct, manageable commands:

1.  **`prepare`:**
    *   **Goal:** Create node-specific, boot-ready OS images *before* flashing.
    *   **Input:** Generic compressed OS image (`.img.xz`), target node number, desired static IP configuration (IP/CIDR, gateway, DNS), hostname.
    *   **Process:** Decompress image, mount root filesystem, modify network configuration files (`/etc/hostname`, `/etc/netplan/`), unmount, recompress to a new node-specific file (e.g., `rk1-node1.img.xz`).
    *   **Rationale:** Ensures network configuration is baked into the image, guaranteeing the node comes up with the correct static IP and hostname from the very first boot. Avoids runtime configuration failures.
    *   **Execution:** Uses Linux tools (`kpartx`, `mount`, `xz`, `tee`, etc.) requiring `sudo`. Best run within a controlled Docker environment (`Dockerfile.prepare`) for dependency management and isolation.

2.  **`install-ubuntu` (OS-Specific Install):**
    *   **Goal:** Deploy a *prepared* image (`.img.xz`) onto a specific node via the BMC.
    *   **Input:** Target node number, path to the local *prepared* `.img.xz` file, BMC credentials.
    *   **Process (Automated via SSH to BMC):**
        1.  Check if the *uncompressed* image (`.img`) already exists on the BMC (e.g., in `/root/imgs/<node_id>/`).
        2.  If not, transfer the compressed image (`.img.xz`) from the local machine to the BMC via SCP.
        3.  If transferred or not previously decompressed, run `unxz` on the BMC.
        4.  Run `tpi flash -n <node> -i <path_to_img_on_bmc>` on the BMC.
        5.  Run `tpi power off -n <node>` and `tpi power on -n <node>` on the BMC.
        6.  Update local state file (`~/.config/turingpi-cli/state.json`).
    *   **Rationale:** Automates the necessary steps on the BMC, handling file transfer, decompression, flashing, and power cycling in a single command. Caches decompressed images on the BMC to speed up subsequent flashes.

3.  **`post-install-ubuntu` (OS-Specific Post-Install):**
    *   **Goal:** Automate the mandatory first-boot interactions required by a specific OS after it has been flashed and booted.
    *   **Input:** Node's static IP address, initial credentials (user/pass), desired new password.
    *   **Process (Automated via direct SSH to the Node):**
        1.  Connect to the node's IP address using initial credentials.
        2.  Request a Pseudo-Terminal (PTY).
        3.  Interact with the shell by sending expected inputs (initial password, new password, confirm new password) based on matching expected prompts (`Current password:`, `New password:`, etc.) read from the output stream.
        4.  Verify success based on output messages ("password updated successfully").
    *   **Rationale:** Completes the provisioning process by handling OS-specific requirements that cannot be baked into the image, making the node immediately accessible with the new credentials.

## 4. Key Technical Approaches & Libraries

*   **Go:** Chosen language for its suitability for CLI tools, concurrency features, and strong standard library (and ecosystem).
*   **Cobra:** Standard library for building structured CLI applications in Go. Used for command definition, flag parsing (including persistent flags for BMC credentials).
*   **`firm-go`:** Used in `install-ubuntu` (and initially in `discover`) to manage the complex, sequential, asynchronous workflow involving multiple steps (check, transfer, decompress, flash, power cycle). Provides a reactive state machine approach.
*   **`golang.org/x/crypto/ssh`:** Standard Go library for SSH client connections. Used for:
    *   Executing remote commands on the BMC (`install-ubuntu`, `discover`).
    *   Establishing interactive PTY sessions directly with the node (`post-install-ubuntu`).
*   **`github.com/pkg/sftp`:** Library built on `crypto/ssh` to handle SFTP file transfers (used in `install-ubuntu` for uploading the image to the BMC).
*   **Docker:** Used via `Dockerfile.prepare` to create a consistent Linux environment with necessary dependencies (`kpartx`, `xz`, `sudo`, etc.) for the `prepare` command, ensuring it runs correctly regardless of the host OS. Mounts are used to access source images and store prepared images.
*   **`pkg/state`:** Simple package using JSON for persisting basic node status (IP, OS, status) locally (`~/.config/turingpi-cli/state.json` or similar).

## 5. Key Learnings & Iterations During Development

*   **BMC Resource Constraints (`unxz` OOM):** Attempting to decompress highly compressed `xz` archives (level -9) directly on the BMC failed due to insufficient RAM, causing the OOM Killer to terminate `unxz`.
    *   *Lesson:* Be mindful of BMC resource limits. Offload heavy tasks (like decompression) to the host when possible, or use less resource-intensive settings (e.g., `xz -6`) if tasks must run on the BMC. Changed `prepare` to use `-zck6`.
*   **BMC Shell Environment (`stat` vs `ls`):** Initial attempts to check remote file existence using `stat` failed ("stat: not found") when run via non-interactive SSH.
    *   *Lesson:* Embedded Linux environments accessed via SSH might be minimal. Use more fundamental/portable commands (`ls`) and rely on checking exit codes and parsing `stderr`/`stdout` robustly rather than assuming specific utilities exist. Switched `checkRemoteFileExists` to use `ls`.
*   **SSH Host Key Changes:** Successfully connecting via SSH after re-flashing triggered host key mismatch warnings.
    *   *Lesson:* This is expected SSH security behavior. Re-flashing generates new host keys. The client-side fix is required (`ssh-keygen -R <ip_address>`). Not an issue with the CLI tool itself.
*   **Go Build Process Nuances:** Encountered issues where new commands weren't recognized or the build produced an incorrect file type (`!<arch>`).
    *   *Lesson:* Ensure new command packages (`cmd/foo.go`) are explicitly registered with Cobra's root command (`rootCmd.AddCommand`). Ensure the build command correctly targets the main package or uses a pattern (`./...` or `.`) that includes all necessary code to produce a valid executable. Ensure the output binary has execute permissions (`chmod +x`).
*   **Automating Interactive SSH:** Implementing `post-install-ubuntu` required careful handling of interactive prompts.
    *   *Lesson:* Direct SSH automation needs PTY allocation (`RequestPty`). Interaction requires a loop that reads `stdout` until an expected prompt is found, then writes the corresponding input to `stdin`. Timeouts and careful parsing of success/error messages in the final output are crucial.
*   **Network Connectivity (`no route to host`):** Initial connection attempts failed despite direct SSH working.
    *   *Lesson:* Network issues can be intermittent or specific to the client/library used. Verify IP addresses. Using mDNS hostnames (`.local`) can sometimes be more resilient than IPs in local networks. Ensure no firewalls are blocking the connection.
*   **Docker Volume Paths:** Docker requires absolute paths for host volume mounts.
    *   *Lesson:* Use absolute paths (e.g., derived from the workspace path) rather than relative paths or commands like `$(pwd)` in `-v` flags for `docker run`.

## 6. Future Considerations

*   **Refactoring:** Move SSH/SFTP helper functions (`executeSSHCommand`, `checkRemoteFileExists`, `scpUploadFile`) into a shared package (e.g., `pkg/bmc`).
*   **Error Handling:** Improve error reporting, potentially providing more user-friendly messages and guidance. Add more specific error checks in SSH interactions.
*   **OS Support:** Create `install-debian`, `post-install-debian`, etc., following the established patterns.
*   **Testing:** Implement unit and integration tests.
*   **Command Consolidation:** Consider if `install-*` and `post-install-*` could be combined with a flag (e.g., `install-ubuntu --auto-post-install`).
*   **User Experience:** Add progress indicators for long operations like SCP and flashing.

## 7. Conclusion

The current design successfully addresses the initial problems of static IP assignment and first-boot configuration for Ubuntu nodes on the Turing Pi 2. The multi-stage approach (`prepare`, `install-ubuntu`, `post-install-ubuntu`) provides a clear separation of concerns. Key technologies like Go, Cobra, SSH/SFTP libraries, and Docker have proven effective, despite requiring iteration to handle BMC constraints and build process details. The project now has a solid foundation for reliable node provisioning. 