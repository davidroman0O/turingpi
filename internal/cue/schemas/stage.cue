package schemas

// #Stage defines a logical grouping of actions that can be executed together
#Stage: {
    // Unique name of the stage
    name:        string @required()
    
    // Optional human-readable title
    title?:      string
    
    // Optional detailed description
    description?: string
    
    // Optional tags for filtering/organization
    tags?:       [...string]
    
    // List of actions to execute in sequence
    actions:     [...#Action] @required()
} 