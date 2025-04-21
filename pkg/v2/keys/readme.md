Let's establish a clear and consistent key naming convention for both the configuration injected into the workflow store and the runtime data generated during execution. This convention should leverage the existing patterns in `gostage` and your `keys` package while providing structure for `tftpi`-specific data.

**Goals for the Convention:**

1.  **Clarity:** Keys should be human-readable and indicate their purpose.
2.  **Namespace:** Avoid collisions between `tftpi` core data, user-defined data, and `gostage` internal data.
3.  **Structure:** Use a hierarchical structure (like dot notation) for organization.
4.  **Consistency:** Follow the same pattern across different types of data.
5.  **Discoverability:** Make it easier for action developers to find the data they need.

**Proposed Key Convention:**

We'll build upon the `turingpi.` namespace already established in `pkg/v2/keys` and the `gostage` prefixes.

1.  **Core Namespace:** `turingpi.`

2.  **Top-Level Categories:**
    *   `turingpi.config.*`: For configuration values injected by the `tftpi.Runner` from the `tftpi.Config` struct. These are generally static for the workflow's duration.
    *   `turingpi.workflow.*`: For parameters controlling the current workflow execution (e.g., target nodes, flags).
    *   `turingpi.node.{nodeID}.*`: For runtime state or configuration specific to a particular node.
    *   `turingpi.internal.*` or `$*`: For internal mechanisms used by the framework (like `$tools`). Let's stick with `$` for brevity as `gostage` doesn't seem to use it.

3.  **Detailed Structure:**

    *   **Configuration (`turingpi.config.*`)**
        *   Store individual configuration fields rather than the whole struct for better granularity and security.
        *   `turingpi.config.bmc.ip`: BMC IP address string.
        *   `turingpi.config.bmc.user`: BMC username string.
        *   `turingpi.config.bmc.password`: BMC password string (use with caution, consider alternatives).
        *   `turingpi.config.cache.localDir`: Local cache directory string.
        *   `turingpi.config.cache.remoteDir`: Remote cache directory string (if SSH is configured).
        *   `turingpi.config.ssh.host`: SSH host string.
        *   `turingpi.config.ssh.port`: SSH port int.
        *   `turingpi.config.ssh.user`: SSH username string.
        *   `turingpi.config.ssh.keyFile`: SSH key file path string.
        *   *(Add others as needed)*

    *   **Workflow Parameters (`turingpi.workflow.*`)**
        *   `turingpi.workflow.currentNodeID`: The single node ID currently being targeted by an action (int). *(Matches existing `keys.CurrentNodeID`)*
        *   `turingpi.workflow.targetNodeIDs`: A list/slice of node IDs the workflow operates on (`[]int`). *(Matches existing `keys.TargetNodes`)*
        *   `turingpi.workflow.flags.hardReset`: Boolean flag specific to a workflow run (bool).
        *   `turingpi.workflow.state`: Overall workflow state string (e.g., "running", "failed"). *(Matches existing `keys.WorkflowState`)*
        *   *(Add others as needed, e.g., `turingpi.workflow.imagePath`)*

    *   **Node-Specific State (`turingpi.node.{nodeID}.*`)**
        *   `turingpi.node.{nodeID}.power`: Power state ("On", "Off", "Unknown") string. *(Matches existing `keys.NodePower`)*
        *   `turingpi.node.{nodeID}.status`: Full status object (e.g., `*bmc.PowerStatus`). *(Matches existing `keys.NodeStatus`)*
        *   `turingpi.node.{nodeID}.ip`: Discovered/assigned IP address string. *(Matches existing `keys.NodeIP`)*
        *   `turingpi.node.{nodeID}.bootMode`: Current boot mode string. *(Matches existing `keys.NodeBootMode`)*
        *   `turingpi.node.{nodeID}.usbMode`: Current USB mode string. *(Matches existing `keys.NodeUSBMode`)*
        *   `turingpi.node.{nodeID}.lastOperation`: Result of the last operation on this node.
        *   `turingpi.node.{nodeID}.error`: Last error encountered for this node.

    *   **Internal (`$*`)**
        *   `$tools`: The initialized `tools.ToolProvider` instance.
