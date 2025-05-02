package definitions

import schemas "turingpi.com/schemas/schemas"

// Node ID parameter definition
#NodeIDParam: { 
    // ID of the node to operate on (1-4)
    nodeID: int & >=1 & <=4 
}

// #PowerOnAction turns on a specific node
#PowerOnAction: schemas.#Action & { 
    type: "bmc:power-on"
    params?: #NodeIDParam
}

// #PowerOffAction turns off a specific node
#PowerOffAction: schemas.#Action & { 
    type: "bmc:power-off"
    params?: #NodeIDParam
}

// #ResetAction performs a hard reset on a specific node
#ResetAction: schemas.#Action & { 
    type: "bmc:reset"
    params?: #NodeIDParam
}

// #GetPowerStatusAction retrieves the current power status of a node
#GetPowerStatusAction: schemas.#Action & { 
    type: "bmc:get-power-status"
    params?: #NodeIDParam
}

// #FlashNodeAction flashes a node with an image
#FlashNodeAction: schemas.#Action & {
    type: "bmc:flash-node"
    params: #NodeIDParam & {
        // Path to the image file to flash
        imagePath: string
    }
}

// #SetNodeModeAction sets the node to a specific mode
#SetNodeModeAction: schemas.#Action & {
    type: "bmc:set-node-mode"
    params: #NodeIDParam & {
        // Mode to set ("normal" or "msd")
        mode: "normal" | "msd"
    }
} 