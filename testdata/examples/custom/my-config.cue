package config

// Define the cluster configuration
cluster: {
	// BMC connection details
	bmc: {
		ip:       "127.0.0.1"
		user:     "admin"
		password: "admin"
	}
	
	// Optional cache paths
	cache?: {
		local?: string
		temp?:  string
	}
}