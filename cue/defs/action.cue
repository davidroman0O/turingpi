// Common action schema definitions
package defs

// Action defines the base structure for all workflow actions
#Action: {
    // Type uniquely identifies the action implementation to use
    type:        string @required()
    
    // Optional description of what this action does
    description?: string
    
    // Optional parameters specific to the action type
    params?:     {...}
}