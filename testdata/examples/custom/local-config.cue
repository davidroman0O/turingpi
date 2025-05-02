package config

// Define the cluster configuration
cluster: {
	// BMC connection details
	bmc: {
		ip:       "192.168.1.100"
		user:     "admin"
		password: "admin"
	}
	
	// Optional cache paths
	cache?: {
		local?: "/tmp/turingpi/local"
		temp?:  "/tmp/turingpi/temp"
	}
}