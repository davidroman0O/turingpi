package container

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMain is the entry point for all tests in this package
func TestMain(m *testing.M) {
	// Set up custom interrupt handler for tests
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create channel for test completion
	testDone := make(chan int, 1)

	// Handle async cleanup in a goroutine
	go func() {
		select {
		case <-sigCh:
			// If we receive an interrupt, force cleanup before exiting
			fmt.Println("=== INTERRUPT DETECTED - Emergency test cleanup ===")
			CleanupContainers()
			fmt.Println("=== Emergency cleanup complete, exiting ===")
			os.Exit(130) // Exit directly with SIGINT code
		case code := <-testDone:
			// Normal test completion, let it finish properly
			os.Exit(code)
		}
	}()

	// Clean up any lingering containers from previous test runs
	fmt.Println("=== Running pre-test cleanup ===")
	cleanupTestContainers()
	fmt.Println("=== Pre-test cleanup complete ===")

	// Run the tests
	exitCode := m.Run()

	// Clean up after tests
	fmt.Println("=== Running post-test cleanup ===")

	// Set a timeout for cleanup operations
	cleanupDone := make(chan struct{}, 1)
	go func() {
		// Run cleanup operations
		cleanupTestContainers()

		// Force check for containers every 500ms for up to 3 seconds
		for i := 0; i < 6; i++ {
			if err := ensureNoTestContainers(); err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
			cleanupTestContainers()
		}

		cleanupDone <- struct{}{}
	}()

	// Wait for cleanup with timeout
	select {
	case <-cleanupDone:
		// Cleanup completed normally
	case <-time.After(5 * time.Second):
		fmt.Println("WARNING: Cleanup timed out, forcing final cleanup")
		CleanupContainers()
	}

	// Final verification
	if err := ensureNoTestContainers(); err != nil {
		fmt.Printf("WARNING: %v\n", err)
		fmt.Println("Some test containers were not properly cleaned up!")
		fmt.Println("You may want to run 'docker ps -a | grep turingpi' and remove them manually")

		// One last attempt
		CleanupContainers()
	} else {
		fmt.Println("All test containers successfully cleaned up")
	}
	fmt.Println("=== Post-test cleanup complete ===")

	// Signal test completion through channel
	testDone <- exitCode

	// This line shouldn't be reached, but just in case
	select {}
}

// ensureNoTestContainers checks if any test containers are running and returns an error if they are
func ensureNoTestContainers() error {
	// Test container name patterns to look for
	testPrefixes := []string{
		"turingpi-test-",
		"test-registry-",
		"registry-test-",
		"test-docker-",
	}

	for _, prefix := range testPrefixes {
		// Find all containers with this prefix
		cleanCmd := exec.Command("docker", "ps", "-a", "-q", "--filter", fmt.Sprintf("name=%s", prefix))
		output, err := cleanCmd.Output()
		if err == nil && len(output) > 0 {
			containerList := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(containerList) > 0 && containerList[0] != "" {
				return fmt.Errorf("found %d test containers with prefix '%s' that were not cleaned up",
					len(containerList), prefix)
			}
		}
	}

	return nil
}
