// Example node reset workflow
package workflow

import "tpi.io/defs"

// A simple workflow to reset a node
reset: defs.#Workflow & {
    name: "node-reset"
    description: "Resets a specific node"
    
    // Required parameters
    params: {
        nodeID: int & >=1 & <=4 @required() // Node ID must be 1-4
    }
    
    // Workflow stages
    stages: [
        {
            name: "reset-sequence"
            description: "Sequence of operations to reset the node"
            actions: [
                defs.#GetPowerStatusAction & {
                    params: {
                        nodeID: params.nodeID
                    }
                },
                defs.#PowerOffAction & {
                    params: {
                        nodeID: params.nodeID
                    }
                },
                defs.#WaitAction & {
                    params: {
                        seconds: 5
                    }
                },
                defs.#PowerOnAction & {
                    params: {
                        nodeID: params.nodeID
                    }
                },
                defs.#WaitAction & {
                    params: {
                        seconds: 10
                    }
                },
                defs.#GetPowerStatusAction & {
                    params: {
                        nodeID: params.nodeID
                    }
                },
            ]
        }
    ]
} 