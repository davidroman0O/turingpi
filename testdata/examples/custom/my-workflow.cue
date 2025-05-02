package workflow

// Define a workflow for resetting a node
reset: {
	// Metadata about this workflow
	title:       "node-reset"
	description: "Resets a specific node"
	
	// Parameters required by this workflow
	params: {
		// Node ID must be between 1 and 4
		nodeID: int & >=1 & <=4
	}
	
	// Use the nodeID parameter throughout the workflow
	let targetNode = params.nodeID
	
	// Stages of the workflow
	stages: [{
		name:        "reset-sequence"
		title:       "Sequence of operations to reset the node"
		description: "Gets the power status, powers off, waits, powers on, then gets status again"
		
		// Actions to execute in this stage
		actions: [
			// First check the current power status
			{
				type: "bmc:get-power-status"
				params: {
					nodeID: targetNode
				}
			},
			// Power off the node
			{
				type: "bmc:power-off"
				params: {
					nodeID: targetNode
				}
			},
			// Wait for 5 seconds
			{
				type: "common:wait"
				params: {
					seconds: 5
				}
			},
			// Power on the node
			{
				type: "bmc:power-on"
				params: {
					nodeID: targetNode
				}
			},
			// Wait for 10 seconds to allow the node to boot
			{
				type: "common:wait"
				params: {
					seconds: 10
				}
			},
			// Check the power status again
			{
				type: "bmc:get-power-status"
				params: {
					nodeID: targetNode
				}
			},
		]
	}]
}