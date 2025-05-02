// BMC-related action implementations
package defs

// NodeIDParam defines a parameter for specifying a node ID
#NodeIDParam: {
    // NodeID must be an integer between 1 and 4
    nodeID: int & >=1 & <=4
}

// BMC Actions for controlling the Turing Pi BMC

// PowerOnAction turns on a specific node
#PowerOnAction: #Action & {
    type: "bmc:power-on"
    description: "Powers on a specific node"
    params: #NodeIDParam
}

// PowerOffAction turns off a specific node
#PowerOffAction: #Action & {
    type: "bmc:power-off"
    description: "Powers off a specific node"
    params: #NodeIDParam
}

// ResetAction performs a hard reset on a specific node
#ResetAction: #Action & {
    type: "bmc:reset"
    description: "Performs a hard reset on a specific node"
    params: #NodeIDParam
}

// GetPowerStatusAction retrieves the current power status of a node
#GetPowerStatusAction: #Action & {
    type: "bmc:get-power-status"
    description: "Retrieves the current power status of a node"
    params: #NodeIDParam
}

// FlashNodeAction flashes a node with a disk image
#FlashNodeAction: #Action & {
    type: "bmc:flash-node"
    description: "Flashes a node with a disk image"
    params: #NodeIDParam & {
        // Path to the image file
        imagePath: string
    }
}

// SetNodeModeParams defines parameters for setting a node mode
#SetNodeModeParams: #NodeIDParam & {
    // Mode must be either "normal" or "msd"
    mode: "normal" | "msd"
}

// SetNodeModeAction sets a node to a specific operating mode
#SetNodeModeAction: #Action & {
    type: "bmc:set-node-mode"
    description: "Sets a node to a specific operating mode (normal or MSD)"
    params: #SetNodeModeParams
}