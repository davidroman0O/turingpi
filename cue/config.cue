// Example cluster configuration
package config

import "tpi.io/defs"

// Cluster configuration
cluster: defs.#ClusterConfig & {
    // BMC connection details
    bmc: {
        ip: "192.168.1.90" // Replace with your BMC IP
        user: "root"       // Replace with your username
        password: "turing" // Replace with your password
    }
    
    // Optional cache paths
    cache: {
        localCachePath: "/tmp/turingpi/cache"
        tempCachePath: "/tmp/turingpi/temp"
    }
} 