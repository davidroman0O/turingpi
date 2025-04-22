package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostage"
	tftpi "github.com/davidroman0O/turingpi/pkg/v2"
	"github.com/davidroman0O/turingpi/pkg/v2/config"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/ubuntu"
)

func main() {
	// Enable debug logging
	os.Setenv("GOSTAGE_DEBUG", "true")

	cacheDir := filepath.Join(".turingpi", "cache")

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("start failed to create cache directory: %v", err)
	}

	// Create with absolutely minimal config - just BMC info and cache dir
	tf, err := tftpi.New(tftpi.WithClusterConfig(
		&config.ClusterConfig{
			Name: "cluster1",
			BMC: config.BMCConfig{
				IP:       "192.168.1.90",
				Username: "root",
				Password: "turing",
			},
			Cache: &config.CacheConfig{
				LocalDir:  cacheDir,
				RemoteDir: "/var/cache/turingpi",
				TempDir:   ".turingpi/tmp",
			},

			// No nodes configuration needed!
		},
	))

	if err != nil {
		log.Fatalf("failed to create turingpi provider: %v", err)
	}

	wrk := ubuntu.CreateUbuntuRK1Deployment(1, ubuntu.UbuntuRK1DeploymentOptions{
		SourceImagePath: "/Users/davidroman/Documents/iso/turingpi/ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz",
		NetworkConfig: &ubuntu.NetworkConfig{
			Hostname:   "rk1-node-1",
			IPCIDR:     "192.168.1.101/24",
			Gateway:    "192.168.1.1",
			DNSServers: []string{"8.8.8.8", "8.8.4.4"},
		},
		NewPassword: "turing1234",
	})

	if err := tf.Execute(context.Background(), wrk, NewDefaultLogger(), "cluster1", 1); err != nil {
		log.Fatalf("failed to execute workflow: %v", err)
	}

}

// DefaultLogger is a no-op logger implementation
type DefaultLogger struct{}

// Debug implements Logger.Debug
func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

// Info implements Logger.Info
func (l *DefaultLogger) Info(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

// Warn implements Logger.Warn
func (l *DefaultLogger) Warn(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

// Error implements Logger.Error
func (l *DefaultLogger) Error(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

// NewDefaultLogger creates a new default no-op logger
func NewDefaultLogger() gostage.Logger {
	return &DefaultLogger{}
}
