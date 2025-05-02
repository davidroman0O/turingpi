// Configuration schemas
package defs

// BMCConfig defines the configuration for connecting to the BMC
#BMCConfig: {
    // IP address of the BMC
    ip:       string @required() 
    
    // Username for authentication
    user:     string @required()
    
    // Password for authentication
    password: string @required()
}

// CacheConfig defines configuration for cache directories
#CacheConfig: {
    // Path to local cache directory
    localCachePath?: string
    
    // Path to temporary cache directory
    tempCachePath?: string
}

// ClusterConfig defines the complete cluster configuration
#ClusterConfig: {
    // BMC connection configuration
    bmc: #BMCConfig @required()
    
    // Optional cache configuration
    cache?: #CacheConfig
}