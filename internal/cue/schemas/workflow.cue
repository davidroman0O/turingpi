package schemas

// #InputParams defines a generic structure for workflow input parameters
// Will be specialized by each workflow
#InputParams: {...}

// #Workflow defines the complete workflow structure
#Workflow: {
    // Unique workflow name
    name:        string @required()
    
    // Optional human-readable title
    title?:      string
    
    // Optional detailed description
    description?: string
    
    // Schema for input parameters this workflow accepts
    params?:     #InputParams 
    
    // List of stages to execute in sequence
    stages:      [...#Stage] @required()
} 