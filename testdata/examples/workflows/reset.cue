// Example node reset workflow
package workflow

// A simple workflow to reset a node
reset: {
    name: "node-reset"
    description: "Resets a specific node"
    
    // Required parameters
    params: {
        nodeID: int & >=1 & <=4 // Node ID must be 1-4, supplied via command line
    }
    
    // Helper variables - using let binding to extract and reference the parameter value
    let targetNode = params.nodeID
    
    // Workflow stages
    stages: [
        {
            name: "reset-sequence"
            description: "Sequence of operations to reset the node"
            actions: [
                {
                    type: "bmc:get-power-status"
                    params: {
                        nodeID: targetNode
                    }
                },
                {
                    type: "bmc:power-off"
                    params: {
                        nodeID: targetNode
                    }
                },
                {
                    type: "common:wait"
                    params: {
                        seconds: 5
                    }
                },
                {
                    type: "bmc:power-on"
                    params: {
                        nodeID: targetNode
                    }
                },
                {
                    type: "common:wait"
                    params: {
                        seconds: 10
                    }
                },
                {
                    type: "bmc:get-power-status"
                    params: {
                        nodeID: targetNode
                    }
                },
            ]
        }
    ]
} 