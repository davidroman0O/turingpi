// Common action implementations
package defs

// Wait action definitions
#WaitParams: { 
    // Number of seconds to wait
    seconds: int & >0 
}

// WaitAction pauses workflow execution for a specified duration
#WaitAction: #Action & { 
    type: "common:wait"
    params: #WaitParams
}