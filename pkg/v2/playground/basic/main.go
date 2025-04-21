package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/davidroman0O/gostage"
	tftpi "github.com/davidroman0O/turingpi/pkg/v2"
	"github.com/davidroman0O/turingpi/pkg/v2/actions/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/config"
)

func main() {
	// Enable debug logging
	os.Setenv("GOSTAGE_DEBUG", "true")

	tf, err := tftpi.New(tftpi.WithClusterConfig(
		&config.ClusterConfig{
			Name: "cluster1",
			BMC: config.BMCConfig{
				IP:       "192.168.1.90",
				Username: "root",
				Password: "turing",
			},
		},
	))

	if err != nil {
		log.Fatalf("failed to create turingpi provider: %v", err)
	}

	wrk := gostage.NewWorkflow("whatever", "whatever", "whatever")

	stage := gostage.NewStage("power-on", "power-on", "power-on")

	wrk.AddStage(stage)

	stage.AddAction(bmc.NewGetPowerStatusAction())
	stage.AddAction(bmc.NewPowerOnNodeAction())

	// Get a default logger - debug level should be controlled by the env var
	logger := NewDefaultLogger()

	// Execute on cluster1, targeting node 1
	if err := tf.Execute(context.Background(), wrk, logger, "cluster1", 1); err != nil {
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
