// Example cluster configuration
package config

// Cluster configuration
cluster: {
    // BMC connection details
    bmc: {
        ip: "192.168.1.91" // Replace with your BMC IP
        user: "root"       // Replace with your username
        password: "turing" // Replace with your password
    }
    
    // Optional cache paths
    cache: {
        localCachePath: "/tmp/turingpi/cache"
        tempCachePath: "/tmp/turingpi/temp"
    }
} 